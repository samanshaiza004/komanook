package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct{ Env, Addr, BaseURL, DatabaseURL, MediaDir string }

func Load() (Config, error) { return Parse(os.Getenv) }

func Parse(get func(string) string) (Config, error) {
	c := Config{Env: get("APP_ENV"), Addr: get("HTTP_ADDR"), BaseURL: get("BASE_URL"), DatabaseURL: get("DATABASE_URL"), MediaDir: get("MEDIA_DIR")}
	if c.Env == "" {
		c.Env = "development"
	}
	if c.Addr == "" {
		c.Addr = "127.0.0.1:8080"
	}
	if c.BaseURL == "" {
		c.BaseURL = "http://localhost:8080"
	}
	if c.MediaDir == "" {
		c.MediaDir = ".data/media"
	}
	if c.Env != "development" && c.Env != "test" && c.Env != "production" {
		return c, fmt.Errorf("invalid APP_ENV")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https") {
		return c, fmt.Errorf("BASE_URL must be an origin")
	}
	c.BaseURL = strings.TrimSuffix(c.BaseURL, "/")
	db, err := url.Parse(c.DatabaseURL)
	if err != nil || db.Host == "" || (db.Scheme != "postgres" && db.Scheme != "postgresql") {
		return c, fmt.Errorf("DATABASE_URL must be a PostgreSQL URL")
	}
	if c.Production() && (u.Scheme != "https" || db.Query().Get("sslmode") != "verify-full") {
		return c, fmt.Errorf("production requires HTTPS BASE_URL and database sslmode=verify-full")
	}
	return c, nil
}
func (c Config) Production() bool { return c.Env == "production" }
