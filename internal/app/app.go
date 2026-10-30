// Package app is the DesignHub application: HTTP API, permissions and the
// services behind each screen.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"designhub/internal/auth"
	"designhub/internal/config"
	"designhub/internal/db"
	"designhub/internal/repo"
)

// App holds the shared services.
type App struct {
	Cfg     *config.Config
	DB      *pgxpool.Pool
	Repo    *repo.Repo
	Files   *FileStore
	Events  *Hub
	Limiter *auth.Limiter
	Web     fs.FS // built frontend, nil in API-only tests

	settingsMu sync.RWMutex
	settings   map[string]json.RawMessage

	rolesMu sync.RWMutex
	roles   map[string]*Role // by id

	indexMu sync.Mutex // serializes index rebuilds
}

// SystemAuthor is used for commits made by the system itself.
var SystemAuthor = repo.Author{Name: "DesignHub", Email: "system@designhub.local"}

// New connects the services. The caller runs migrations first.
func New(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, web fs.FS) (*App, error) {
	rp, err := repo.Open(cfg.RepoDir, SystemAuthor)
	if err != nil {
		return nil, err
	}
	fstore, err := NewFileStore(cfg.FilesDir)
	if err != nil {
		return nil, err
	}
	a := &App{
		Cfg: cfg, DB: pool, Repo: rp, Files: fstore, Events: NewHub(),
		Limiter: auth.NewLimiter(5, 15*time.Minute), Web: web,
	}
	if err := a.syncBuiltInRoles(ctx); err != nil {
		return nil, fmt.Errorf("roles: %w", err)
	}
	if err := a.loadSettings(ctx); err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	if err := a.RebuildIndex(ctx); err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	return a, nil
}

// ---------------------------------------------------------------------------
// HTTP plumbing

// Error is an API error with an HTTP status and a message for the user.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code,omitempty"`
	Message string `json:"error"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string { return e.Message }

func errf(status int, format string, args ...any) *Error {
	return &Error{Status: status, Message: fmt.Sprintf(format, args...)}
}

var (
	errNotFound     = &Error{Status: http.StatusNotFound, Code: "not_found", Message: "Not found."}
	errForbidden    = &Error{Status: http.StatusForbidden, Code: "forbidden", Message: "You do not have permission to do this."}
	errUnauthorized = &Error{Status: http.StatusUnauthorized, Code: "signed_out", Message: "Please sign in."}
)

func badRequest(format string, args ...any) *Error {
	return &Error{Status: http.StatusBadRequest, Code: "invalid", Message: fmt.Sprintf(format, args...)}
}

func conflict(msg string, data any) *Error {
	return &Error{Status: http.StatusConflict, Code: "conflict", Message: msg, Data: data}
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

// handle turns a handler that returns an error into an http.HandlerFunc.
func (a *App) handle(fn handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err == nil {
			return
		}
		var ae *Error
		switch {
		case errors.As(err, &ae):
		case errors.Is(err, db.ErrNotFound), errors.Is(err, pgx.ErrNoRows), errors.Is(err, repo.ErrNotFound):
			ae = errNotFound
		case errors.Is(err, repo.ErrConflict):
			ae = conflict("This record changed since you opened it.", nil)
		default:
			var pe *pgconn.PgError
			if errors.As(err, &pe) && pe.Code == "23505" {
				ae = &Error{Status: http.StatusConflict, Code: "duplicate", Message: "That already exists."}
			} else {
				log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
				ae = &Error{Status: http.StatusInternalServerError, Message: "Something went wrong. The error has been logged."}
			}
		}
		writeJSON(w, ae.Status, ae)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, v any) error {
	if v == nil {
		v = map[string]bool{"ok": true}
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}

// decode reads a JSON body of at most 8 MB into v.
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 8<<20))
	if err := dec.Decode(v); err != nil {
		return badRequest("Invalid request: %v", err)
	}
	return nil
}

func clientIP(r *http.Request) string {
	// The proxy is the only client of the application container, so the
	// first X-Forwarded-For entry is the browser's address.
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		ip, _, _ := strings.Cut(f, ",")
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// ---------------------------------------------------------------------------
// Middleware

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; font-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// hostCheck refuses requests whose Host is not in the allowed addresses
// setting. An empty setting allows any host.
func (a *App) hostCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		allowed := a.AllowedHosts()
		if len(allowed) > 0 {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			host = strings.Trim(strings.ToLower(host), "[]")
			okHost := host == "localhost" || host == "127.0.0.1" || host == "::1"
			for _, h := range allowed {
				if strings.EqualFold(h, host) {
					okHost = true
				}
			}
			if !okHost {
				http.Error(w, "Host not allowed", http.StatusMisdirectedRequest)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// sameOrigin refuses state-changing requests from other sites.
func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		// Browsers always send Origin on cross-site POST; a missing Origin
		// is a same-site request or a non-browser client, which still needs
		// the CSRF token.
		return true
	}
	o = strings.TrimPrefix(strings.TrimPrefix(o, "https://"), "http://")
	return strings.EqualFold(o, r.Host)
}

// Handler returns the full HTTP handler.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := a.DB.Ping(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	a.routes(mux)
	mux.Handle("/", a.spa())
	return a.securityHeaders(a.hostCheck(mux))
}

// ---------------------------------------------------------------------------
// Database helpers

// tx runs fn in a transaction.
func (a *App) tx(ctx context.Context, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, a.DB, fn)
}

// querier is satisfied by the pool and by transactions.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
