package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/justinas/nosurf"
	"komanook/internal/auth"
	"komanook/internal/config"
	"komanook/internal/database"
	"komanook/internal/database/store"
	"komanook/internal/mangadex"
	"komanook/internal/media"
)

type App struct {
	Config    config.Config
	Pool      *pgxpool.Pool
	Q         *store.Queries
	Sessions  *scs.SessionManager
	Media     *media.Service
	MangaDex  *mangadex.Client
	Log       *slog.Logger
	verifiers chan struct{}
}

func New(c config.Config, p *pgxpool.Pool, m *media.Service, logger *slog.Logger) *App {
	s := scs.New()
	s.Store = pgxstore.NewWithCleanupInterval(p, 0)
	s.Lifetime = 7 * 24 * time.Hour
	s.IdleTimeout = 24 * time.Hour
	s.Cookie.Name = "komanook_session"
	if c.Production() {
		s.Cookie.Name = "__Host-komanook_session"
	}
	s.Cookie.Path = "/"
	s.Cookie.HttpOnly = true
	s.Cookie.Secure = c.Production()
	s.Cookie.SameSite = http.SameSiteLaxMode
	s.Cookie.Persist = true
	a := &App{Config: c, Pool: p, Q: store.New(p), Sessions: s, Media: m, MangaDex: mangadex.NewClient(nil), Log: logger, verifiers: make(chan struct{}, 4)}
	s.ErrorFunc = func(w http.ResponseWriter, r *http.Request, err error) { a.failure(w, r, err) }
	return a
}

func (a *App) Handler() http.Handler {
	public := http.NewServeMux()
	public.Handle("GET /assets/", assets())
	public.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok\n") })
	public.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := database.Ready(ctx, a.Pool); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "ready\n")
	})
	public.HandleFunc("GET /{$}", a.catalog)
	public.HandleFunc("GET /catalog", a.catalog)
	public.HandleFunc("GET /series/{slug}", a.series)
	public.HandleFunc("GET /u/{handle}", a.publicProfile)
	public.HandleFunc("GET /mangadex", a.mangaDexSearch)
	public.HandleFunc("GET /mangadex/manga/{id}", a.mangaDexManga)
	public.HandleFunc("GET /mangadex/read/{id}", a.mangaDexRead)
	private := http.NewServeMux()
	private.HandleFunc("GET /login", a.loginPage)
	private.HandleFunc("POST /login", a.login)
	private.HandleFunc("POST /logout", a.logout)
	private.HandleFunc("GET /account", a.account)
	private.HandleFunc("GET /account/profile", a.profileSettings)
	private.HandleFunc("POST /account/profile", a.saveProfile)
	private.HandleFunc("GET /account/profile/preview", a.profilePreview)
	private.HandleFunc("GET /account/library", a.readerLibrary)
	private.HandleFunc("POST /account/library/{work}", a.saveLibraryEntry)
	private.HandleFunc("GET /read/{chapter}", a.reader)
	private.HandleFunc("GET /api/progress/{chapter}", a.getProgress)
	private.HandleFunc("POST /api/progress/{chapter}", a.saveProgress)
	private.HandleFunc("POST /api/progress/import", a.importProgress)
	private.HandleFunc("POST /admin/uploads", a.upload)
	private.HandleFunc("GET /admin/uploads/{id}", a.uploadStatus)
	private.HandleFunc("POST /admin/mangadex/group-removals", a.addMangaDexGroupRemoval)
	csrf := nosurf.New(a.Sessions.LoadAndSave(private))
	csrf.SetBaseCookie(http.Cookie{Name: a.csrfName(), Path: "/", HttpOnly: true, Secure: a.Config.Production(), SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	csrf.SetIsTLSFunc(func(*http.Request) bool { return a.Config.Production() })
	u, _ := url.Parse(a.Config.BaseURL)
	csrf.SetIsAllowedOriginFunc(func(origin *url.URL) bool { return origin.Scheme == u.Scheme && origin.Host == u.Host })
	csrf.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "The security token expired. Reload the page and try again.", http.StatusForbidden)
	}))
	public.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		if r.Method == http.MethodPost && (r.URL.Path == "/account/profile" || strings.HasPrefix(r.URL.Path, "/account/library/")) {
			r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		}
		csrf.ServeHTTP(w, r)
	}))
	return a.middleware(public)
}
func (a *App) csrfName() string {
	if a.Config.Production() {
		return "__Host-komanook_csrf"
	}
	return "komanook_csrf"
}

func (a *App) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	var b bytes.Buffer
	if err := c.Render(r.Context(), &b); err != nil {
		a.failure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b.Bytes())
}
func (a *App) renderStatus(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	var b bytes.Buffer
	if err := c.Render(r.Context(), &b); err != nil {
		a.failure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}
func (a *App) failure(w http.ResponseWriter, r *http.Request, err error) {
	a.Log.ErrorContext(r.Context(), "request failed", "path", r.URL.Path, "error_type", fmt.Sprintf("%T", err))
	w.Header().Set("Cache-Control", "private, no-store")
	http.Error(w, "This page is temporarily unavailable. Try again shortly.", http.StatusServiceUnavailable)
}

func (a *App) catalog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy(true))
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	genre := r.URL.Query().Get("genre")
	if len(search) > 100 || len(genre) > 40 {
		http.Error(w, "Search is too long.", http.StatusBadRequest)
		return
	}
	items, err := a.Q.Catalog(r.Context(), store.CatalogParams{Search: search, Genre: genre})
	if err != nil {
		a.failure(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60, s-maxage=300, stale-while-revalidate=60")
	w.Header().Add("Vary", "Datastar-Request")
	if r.Header.Get("Datastar-Request") == "true" {
		a.render(w, r, catalogResults(items))
		return
	}
	a.render(w, r, catalogPage(items, search, genre))
}

func (a *App) series(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy(true))
	s, err := a.Q.Series(r.Context(), r.PathValue("slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}
	chapters, err := a.Q.Chapters(r.Context(), s.ID)
	if err != nil {
		a.failure(w, r, err)
		return
	}
	items, err := a.Q.Catalog(r.Context(), store.CatalogParams{})
	if err != nil {
		a.failure(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60, s-maxage=300, stale-while-revalidate=60")
	a.render(w, r, seriesPage(s, chapters, items))
}

func (a *App) currentUser(r *http.Request) (store.User, error) {
	id := a.Sessions.GetInt64(r.Context(), "user_id")
	if id == 0 {
		return store.User{}, pgx.ErrNoRows
	}
	u, err := a.Q.UserByID(r.Context(), id)
	if err != nil {
		return u, err
	}
	if u.Disabled || int(u.AuthVersion) != a.Sessions.GetInt(r.Context(), "auth_version") {
		return store.User{}, pgx.ErrNoRows
	}
	return u, nil
}
func (a *App) requireUser(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	u, err := a.currentUser(r)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "Sign in to continue.", http.StatusUnauthorized)
		return u, false
	}
	if err != nil {
		a.failure(w, r, err)
		return u, false
	}
	return u, true
}
func (a *App) loginPage(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, loginView(nosurf.Token(r), ""))
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form.", http.StatusBadRequest)
		return
	}
	email, _ := auth.Email(r.PostForm.Get("email"))
	password := r.PostForm.Get("password")
	if len(password) > 72 || len(email) == 0 {
		password = "invalid"
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	for _, key := range []struct {
		value string
		limit int32
	}{{"account:" + email, 10}, {"ip:" + ip, 100}} {
		hash := sha256.Sum256([]byte(key.value))
		n, err := a.Q.LoginAttempt(r.Context(), hex.EncodeToString(hash[:]))
		if err != nil {
			a.failure(w, r, err)
			return
		}
		if n > key.limit {
			w.Header().Set("Retry-After", "900")
			http.Error(w, "Too many sign-in attempts. Try again in 15 minutes.", http.StatusTooManyRequests)
			return
		}
	}
	select {
	case a.verifiers <- struct{}{}:
		defer func() { <-a.verifiers }()
	default:
		w.Header().Set("Retry-After", "2")
		http.Error(w, "Sign-in is busy. Try again shortly.", http.StatusTooManyRequests)
		return
	}
	u, err := a.Q.UserByEmail(r.Context(), email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		a.failure(w, r, err)
		return
	}
	valid := auth.Verify(u.PasswordHash, []byte(password))
	if !valid || u.ID == 0 || u.Disabled {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		a.render(w, r, loginView(nosurf.Token(r), "Email or password is incorrect."))
		return
	}
	if err = a.Sessions.RenewToken(r.Context()); err != nil {
		a.failure(w, r, err)
		return
	}
	a.Sessions.Put(r.Context(), "user_id", u.ID)
	a.Sessions.Put(r.Context(), "auth_version", int(u.AuthVersion))
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if err := a.Sessions.Destroy(r.Context()); err != nil {
		a.failure(w, r, err)
		return
	}
	http.Redirect(w, r, "/catalog", http.StatusSeeOther)
}
func (a *App) account(w http.ResponseWriter, r *http.Request) {
	u, err := a.currentUser(r)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}
	var removals []store.MangaDexGroupRemovalsRow
	if u.Role == "admin" {
		var err error
		removals, err = a.Q.MangaDexGroupRemovals(r.Context())
		if err != nil {
			a.failure(w, r, err)
			return
		}
	}
	a.render(w, r, accountView(u, nosurf.Token(r), removals))
}

func chapterID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("chapter"), 10, 64)
	if id < 1 {
		err = errors.New("invalid chapter")
	}
	return id, err
}
func (a *App) reader(w http.ResponseWriter, r *http.Request) {
	id, err := chapterID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	c, err := a.Q.Chapter(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}
	pages, err := a.Q.Pages(r.Context(), id)
	if err != nil {
		a.failure(w, r, err)
		return
	}
	if len(pages) == 0 {
		http.NotFound(w, r)
		return
	}
	u, err := a.currentUser(r)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		a.failure(w, r, err)
		return
	}
	a.render(w, r, readerView(c, pages, nosurf.Token(r), u.ID))
}
func (a *App) getProgress(w http.ResponseWriter, r *http.Request) {
	u, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	id, err := chapterID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err = a.Q.Chapter(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		a.failure(w, r, err)
		return
	}
	p, err := a.Q.GetProgress(r.Context(), store.GetProgressParams{UserID: u.ID, ChapterID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		p = 0
	} else if err != nil {
		a.failure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int32{"page": p})
}
func (a *App) saveProgress(w http.ResponseWriter, r *http.Request) {
	u, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	id, err := chapterID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Use JSON.", http.StatusUnsupportedMediaType)
		return
	}
	var body struct {
		Page int32 `json:"page"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err = d.Decode(&body); err != nil {
		http.Error(w, "Invalid page.", http.StatusBadRequest)
		return
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		http.Error(w, "Invalid JSON.", http.StatusBadRequest)
		return
	}
	if _, err = a.Q.Chapter(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		a.failure(w, r, err)
		return
	}
	pages, err := a.Q.Pages(r.Context(), id)
	if err != nil {
		a.failure(w, r, err)
		return
	}
	if body.Page < 1 || int(body.Page) > len(pages) {
		http.Error(w, "Page is outside this chapter.", http.StatusBadRequest)
		return
	}
	if err = a.Q.SaveProgress(r.Context(), store.SaveProgressParams{UserID: u.ID, ChapterID: id, Page: body.Page}); err != nil {
		a.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

const maxProgressImportItems = 99

type progressImportItem struct {
	ChapterID int64 `json:"chapter"`
	Page      int32 `json:"page"`
}

func (a *App) importProgress(w http.ResponseWriter, r *http.Request) {
	u, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Use JSON.", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var body struct {
		Items []progressImportItem `json:"items"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&body); err != nil || len(body.Items) == 0 || len(body.Items) > maxProgressImportItems {
		http.Error(w, "Invalid progress import.", http.StatusBadRequest)
		return
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		http.Error(w, "Invalid JSON.", http.StatusBadRequest)
		return
	}

	seen := make(map[int64]struct{}, len(body.Items))
	for _, item := range body.Items {
		if item.ChapterID < 1 || item.Page < 1 {
			http.Error(w, "Invalid progress import.", http.StatusBadRequest)
			return
		}
		if _, duplicate := seen[item.ChapterID]; duplicate {
			http.Error(w, "Duplicate chapter in progress import.", http.StatusBadRequest)
			return
		}
		seen[item.ChapterID] = struct{}{}
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		a.failure(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.Q.WithTx(tx)
	for _, item := range body.Items {
		if _, err = q.Chapter(r.Context(), item.ChapterID); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			a.failure(w, r, err)
			return
		}
		pages, pageErr := q.Pages(r.Context(), item.ChapterID)
		if pageErr != nil {
			a.failure(w, r, pageErr)
			return
		}
		if int(item.Page) > len(pages) {
			http.Error(w, "Page is outside this chapter.", http.StatusBadRequest)
			return
		}
	}

	imported := int32(0)
	for _, item := range body.Items {
		count, importErr := q.ImportProgress(r.Context(), store.ImportProgressParams{
			UserID: u.ID, ChapterID: item.ChapterID, Page: item.Page,
		})
		if importErr != nil {
			a.failure(w, r, importErr)
			return
		}
		imported += int32(count)
	}
	if err = tx.Commit(r.Context()); err != nil {
		a.failure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int32{
		"imported": imported,
		"existing": int32(len(body.Items)) - imported,
	})
}
func (a *App) upload(w http.ResponseWriter, r *http.Request) {
	u, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if u.Role != "admin" {
		http.Error(w, "Administrator access required.", http.StatusForbidden)
		return
	}
	// Parse in bounded memory; the global body limit also protects nosurf's form parsing.
	if err := r.ParseMultipartForm(media.MaxBytes + 1024); err != nil {
		http.Error(w, "Upload exceeds the allowed size or is invalid.", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Choose one image.", http.StatusBadRequest)
		return
	}
	defer f.Close()
	if len(r.MultipartForm.File["image"]) != 1 {
		http.Error(w, "Choose one image.", http.StatusBadRequest)
		return
	}
	id, err := a.Media.Accept(r.Context(), u.ID, f)
	if errors.Is(err, media.ErrImage) {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/admin/uploads/"+id)
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "state": "pending"})
}
func (a *App) uploadStatus(w http.ResponseWriter, r *http.Request) {
	u, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if u.Role != "admin" {
		http.Error(w, "Administrator access required.", http.StatusForbidden)
		return
	}
	v, err := a.Q.Upload(r.Context(), r.PathValue("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": v.ID, "state": v.State, "width": v.Width, "height": v.Height})
}

type response struct {
	http.ResponseWriter
	status int
}

func (w *response) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *response) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
func (w *response) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("Cache-Control", "private, no-store")
		idBytes := make([]byte, 12)
		_, _ = rand.Read(idBytes)
		id := hex.EncodeToString(idBytes)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy(false))
		if a.Config.Production() {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		r.Body = http.MaxBytesReader(w, r.Body, media.MaxBytes+64*1024)
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		rec := &response{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				a.Log.Error("request panic", "request_id", id)
				if rec.status == 0 {
					w.Header().Set("Cache-Control", "private, no-store")
					http.Error(rec, "This page is temporarily unavailable.", http.StatusInternalServerError)
				}
			}
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			a.Log.Info("request", "request_id", id, "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(rec, r)
	})
}

func contentSecurityPolicy(datastar bool) string {
	scripts := "'self'"
	if datastar {
		scripts += " 'unsafe-eval'"
	}
	return "default-src 'none'; script-src " + scripts + "; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'; object-src 'none'"
}
