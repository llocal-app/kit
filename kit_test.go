package kit

import (
	"crypto/ed25519"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"llocal.app/kit/identity"
)

func TestApp(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"llocal.json":                `{"shape": "standard", "base": "go", "start": "./server", "port": 8080, "static": "web"}`,
		"web/index.html":             "<p>index</p>",
		"web/app.js":                 "app()",
		"db/migrations/001_init.sql": "CREATE TABLE notes (id INTEGER PRIMARY KEY);",
	} {
		os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pub, key, _ := ed25519.GenerateKey(nil)
	t.Chdir(dir)
	t.Setenv("PORT", "8080")
	t.Setenv("LLOCAL_URL", "https://notes.acme.llocal.app")
	t.Setenv("LLOCAL_JWKS", string(identity.Keys{"k": pub}.JWKS()))
	t.Setenv("LLOCAL_DATA", filepath.Join(dir, "data"))

	a, err := open()
	if err != nil {
		t.Fatal(err)
	}
	a.Get("/api/me", func(w http.ResponseWriter, r *http.Request) { JSON(w, http.StatusOK, CurrentUser(r)) })
	a.Get("/api/fail", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "database is locked", http.StatusInternalServerError)
	})
	a.Route("/api/v1", func(r chi.Router) { r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {}) })
	a.mount()
	var logged strings.Builder
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	if _, err := a.DB.Exec("INSERT INTO notes DEFAULT VALUES"); err != nil {
		t.Fatalf("migration not applied: %v", err)
	}
	if f, err := a.Files.Create("a.txt"); err != nil {
		t.Fatal(err)
	} else {
		f.Close()
	}
	if _, err := a.Files.Create("../escape.txt"); err == nil {
		t.Error("Files let a name escape /data/files")
	}

	token := func(aud string) string {
		return identity.Sign("k", key, identity.Claims{Subject: "u_1", Audience: aud, Expires: time.Now().Add(time.Minute).Unix(), Name: "Dana", Groups: []string{"finance"}})
	}
	for _, tc := range []struct {
		path, token string
		status      int
		body        string
	}{
		{"/healthz", "", 200, "ok"},
		{"/api/me", "", 200, "null"},
		{"/api/me", token("notes.acme.llocal.app"), 200, `"id":"u_1"`},
		{"/api/me", token("payroll.acme.llocal.app"), 401, "identity: token is for"},
		{"/api/missing", "", 404, "not found"},
		{"/api/v1/missing", "", 404, "not found"},
		{"/api/fail", "", 500, "database is locked"},
		{"/app.js", "", 200, "app()"},
		{"/", "", 200, "<p>index</p>"},
		{"/notes/7", "", 200, "<p>index</p>"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if tc.token != "" {
			req.Header.Set(identity.Header, tc.token)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, req)
		body, _ := io.ReadAll(w.Body)
		if w.Code != tc.status || !strings.Contains(string(body), tc.body) {
			t.Errorf("GET %s: %d %q, want %d with %q", tc.path, w.Code, body, tc.status, tc.body)
		}
		if none := w.Header().Get("X-LLocal-Route") == "none"; none != strings.HasSuffix(tc.path, "/missing") {
			t.Errorf("GET %s: X-LLocal-Route none is %v", tc.path, none)
		}
	}
	if strings.Count(logged.String(), "answered") != 1 || !strings.HasSuffix(logged.String(), "GET /api/fail answered 500: database is locked\n") {
		t.Errorf("logged %q, want only the failure", logged.String())
	}
}

func TestOpenNeedsSettings(t *testing.T) {
	t.Chdir(t.TempDir())
	os.WriteFile("llocal.json", []byte(`{"shape": "standard", "base": "go", "start": "./server", "port": 8080}`), 0o644)
	if _, err := open(); err == nil || err.Error() != "PORT is not set" {
		t.Fatalf("err = %v", err)
	}
}
