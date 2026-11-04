package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"komanook/internal/auth"
	"komanook/internal/config"
	"komanook/internal/database"
	"komanook/internal/database/store"
	"komanook/internal/media"
	"komanook/internal/web"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("application stopped", "error", err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	c, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	pool, err := database.Open(startup, c.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	switch command {
	case "migrate":
		return database.Migrate(startup, c.DatabaseURL, pool)
	case "seed":
		if c.Production() {
			return errors.New("sample seed is available in development/test only")
		}
		return database.Seed(startup, pool)
	case "user":
		f := flag.NewFlagSet("user", flag.ContinueOnError)
		email := f.String("email", "", "email address")
		role := f.String("role", "reader", "reader or admin")
		if err = f.Parse(os.Args[2:]); err != nil {
			return err
		}
		address, err := auth.Email(*email)
		if err != nil {
			return err
		}
		if *role != "reader" && *role != "admin" {
			return errors.New("role must be reader or admin")
		}
		var password []byte
		if term.IsTerminal(int(os.Stdin.Fd())) {
			_, _ = fmt.Fprint(os.Stderr, "Password (12–72 bytes): ")
			password, err = term.ReadPassword(int(os.Stdin.Fd()))
			_, _ = fmt.Fprintln(os.Stderr)
		} else {
			var line string
			line, err = bufio.NewReaderSize(os.Stdin, 128).ReadString('\n')
			if errors.Is(err, os.ErrClosed) {
				return err
			}
			if len(line) > 73 {
				return errors.New("password too long")
			}
			password = []byte(strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"))
			if len(password) > 0 {
				err = nil
			}
		}
		if err != nil {
			return err
		}
		hash, err := auth.Hash(password)
		clear(password)
		if err != nil {
			return err
		}
		_, err = store.New(pool).ProvisionUser(startup, store.ProvisionUserParams{Email: address, PasswordHash: hash, Role: *role})
		if err == nil {
			logger.Info("account provisioned; previous sessions revoked")
		}
		return err
	case "serve":
		if err = database.Ready(startup, pool); err != nil {
			return fmt.Errorf("run migrate before serve: %w", err)
		}
	default:
		return fmt.Errorf("unknown command %q (serve, migrate, seed, user)", command)
	}
	m, err := media.New(pool, c.MediaDir, logger)
	if err != nil {
		return err
	}
	if err = m.Jobs.Start(ctx); err != nil {
		return err
	}
	app := web.New(c, pool, m, logger)
	srv := &http.Server{Addr: c.Addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	errorsCh := make(chan error, 1)
	go func() { logger.Info("listening", "address", c.Addr); errorsCh <- srv.ListenAndServe() }()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				_, e := pool.Exec(cleanCtx, "DELETE FROM sessions WHERE expiry < now(); DELETE FROM login_limits WHERE expires_at < now()")
				cancel()
				if e != nil {
					logger.Error("expired security record cleanup failed")
				}
			}
		}
	}()
	var serveErr error
	select {
	case err = <-errorsCh:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
		}
		stop()
	case <-ctx.Done():
	}
	shutdown, done := context.WithTimeout(context.Background(), 15*time.Second)
	defer done()
	serverErr := srv.Shutdown(shutdown)
	workerErr := m.Jobs.Stop(shutdown)
	return errors.Join(serveErr, serverErr, workerErr)
}
