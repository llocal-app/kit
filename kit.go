// Package kit is the standard shape of an app hosted on llocal: settings from the
// environment, SQLite at $LLOCAL_DATA/app.db with migrations from db/migrations, files
// under $LLOCAL_DATA/files, the front end from llocal.json's static folder, /healthz,
// and who is signed in from the gateway's identity token. See STANDARD.md.
package kit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"llocal.app/kit/identity"
	"llocal.app/kit/recipe"
)

type App struct {
	chi.Router
	DB    *sql.DB
	Files *os.Root
	// URL is the app's public address, like https://inventory.acme.llocal.app.
	URL string

	addr     string
	static   string
	keys     identity.Keys
	audience string
}

// User is the person signed in, as the gateway vouches for them. ID is stable and never
// reused; email can change, so key data on ID.
type User struct {
	ID      string   `json:"id"`
	Email   string   `json:"email"`
	Name    string   `json:"name"`
	Groups  []string `json:"groups"`
	Company string   `json:"company"`
	Guest   bool     `json:"guest"`
}

type userKey struct{}

// New reads the environment and llocal.json in the working directory, opens the database
// and applies migrations. Any problem stops the app.
func New() *App {
	a, err := open()
	if err != nil {
		log.Fatal(err)
	}
	return a
}

func open() (*App, error) {
	rec, err := recipe.Load("llocal.json")
	if err != nil {
		return nil, err
	}
	for _, k := range []string{"PORT", "LLOCAL_URL", "LLOCAL_DATA", "LLOCAL_JWKS"} {
		if os.Getenv(k) == "" {
			return nil, fmt.Errorf("%s is not set", k)
		}
	}
	u, err := url.Parse(os.Getenv("LLOCAL_URL"))
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("LLOCAL_URL %q must be an address like https://inventory.acme.llocal.app", os.Getenv("LLOCAL_URL"))
	}
	keys, err := identity.ParseJWKS([]byte(os.Getenv("LLOCAL_JWKS")))
	if err != nil {
		return nil, fmt.Errorf("LLOCAL_JWKS: %w", err)
	}
	data := os.Getenv("LLOCAL_DATA")
	filesDir := filepath.Join(data, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		return nil, err
	}
	files, err := os.OpenRoot(filesDir)
	if err != nil {
		return nil, err
	}
	db, err := openDB(filepath.Join(data, "app.db"))
	if err != nil {
		return nil, err
	}
	if err := migrate(db, "db/migrations"); err != nil {
		return nil, err
	}
	a := &App{
		Router:   chi.NewRouter(),
		DB:       db,
		Files:    files,
		URL:      u.String(),
		addr:     net.JoinHostPort(os.Getenv("HOST"), os.Getenv("PORT")),
		static:   rec.Static,
		keys:     keys,
		audience: u.Hostname(),
	}
	a.Use(middleware.Recoverer, a.identify)
	return a, nil
}

// Run adds /healthz and the front end after the app's own routes, then serves until
// SIGTERM or SIGINT.
func (a *App) Run() {
	a.mount()
	srv := &http.Server{Addr: a.addr, Handler: a}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	log.Printf("listening on %s", a.addr)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
	a.DB.Close()
}

func (a *App) mount() {
	a.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := a.DB.PingContext(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	if a.static != "" {
		a.Get("/*", spa(a.static))
	}
}

// CurrentUser is the person signed in, or nil on paths that machines or the public reach.
func CurrentUser(r *http.Request) *User {
	u, _ := r.Context().Value(userKey{}).(*User)
	return u
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (a *App) identify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(identity.Header)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		c, err := identity.Verify(a.keys, token, a.audience, time.Now())
		if err != nil {
			http.Error(w, "identity: "+err.Error(), http.StatusUnauthorized)
			return
		}
		u := &User{ID: c.Subject, Email: c.Email, Name: c.Name, Groups: c.Groups, Company: c.Company, Guest: c.Guest}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}

// spa serves the front end, and index.html for any path that is not a file so the
// front end's router can take it. Unmatched /api/ paths stay 404s.
func spa(dir string) http.HandlerFunc {
	fsys := os.DirFS(dir)
	files := http.FileServerFS(fsys)
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if fi, err := fs.Stat(fsys, strings.TrimPrefix(path.Clean(r.URL.Path), "/")); err != nil || fi.IsDir() {
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}
		files.ServeHTTP(w, r)
	}
}
