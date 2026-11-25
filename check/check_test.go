package check

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// starter copies kit/starter to a temp folder, made a module on this kit the way a
// builder's go get makes it (the kit's requirements and checksums), on a free port, and
// writes files over it.
func starter(t *testing.T, files map[string]string) string {
	kit, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "app")
	if err := os.CopyFS(dir, os.DirFS(filepath.Join(kit, "starter"))); err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(kit, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(kit, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	_, requires, _ := strings.Cut(string(mod), "\nrequire")
	write(t, dir, map[string]string{
		"go.mod": "module starter\n\ngo 1.24.0\n\nrequire llocal.app/kit v0.0.0\n\nreplace llocal.app/kit => " + kit + "\n\nrequire" + requires,
		"go.sum": string(sum),
	})
	edit(t, filepath.Join(dir, "llocal.json"), "8080", freePort(t))
	write(t, dir, files)
	return dir
}

func freePort(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

// write writes files into dir; a path mapped to "" is removed.
func write(t *testing.T, dir string, files map[string]string) {
	for name, body := range files {
		p := filepath.Join(dir, name)
		var err error
		if body == "" {
			err = os.Remove(p)
		} else {
			err = os.WriteFile(p, []byte(body), 0o644)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func edit(t *testing.T, file, old, new string) {
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(b), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func survey(t *testing.T) Survey {
	s, err := LoadSurvey("testdata/survey.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStarterPasses(t *testing.T) {
	r := Run(context.Background(), starter(t, nil), survey(t))
	if !r.OK {
		t.Fatalf("starter failed: %+v", r.Checks)
	}
	var names []string
	for _, c := range r.Checks {
		names = append(names, c.Name)
	}
	if got := strings.Join(names, " "); got != "recipe build old-database login start pages routes writes tables" {
		t.Errorf("checks = %s", got)
	}
	if r.IdleMemory == 0 || r.StartMillis == 0 || r.DiskBytes == 0 {
		t.Errorf("numbers not recorded: %+v", r)
	}
}

func TestABuiltAppIsCheckedWithoutBuildingIt(t *testing.T) {
	dir := starter(t, nil)
	if r := Run(context.Background(), dir, survey(t)); !r.OK {
		t.Fatalf("starter failed: %+v", r.Checks)
	}
	// Built: the server binary that Run's build left is what starts, so a broken source doesn't matter.
	write(t, dir, map[string]string{"broken.go": "package main\n\nfunc init() { undefined() }\n"})
	r := Built(context.Background(), dir, survey(t))
	if !r.OK || slices.ContainsFunc(r.Checks, func(c Result) bool { return c.Name == "build" }) {
		t.Fatalf("built check: %+v", r.Checks)
	}
}

func TestOtherShapesPass(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"static": {"llocal.json": `{"shape": "static", "base": "static", "static": "."}`, "index.html": "<p>hi</p>"},
		"kept": {"llocal.json": `{"shape": "kept", "base": "python", "start": "exec python3 -m http.server $PORT --bind 0.0.0.0", "port": ` +
			freePort(t) + `}`},
	} {
		dir := t.TempDir()
		write(t, dir, files)
		if r := Run(context.Background(), dir, Survey{}); !r.OK {
			t.Errorf("%s failed: %+v", name, r.Checks)
		}
	}
}

func TestBrokenVariantsFail(t *testing.T) {
	startTimeout = 3 * time.Second
	for _, tc := range []struct {
		name, check, reason string
		files               map[string]string
	}{
		{"wrong port", "start", "nothing answered on port", map[string]string{
			"port.go": "package main\n\nimport \"os\"\n\nfunc init() { os.Setenv(\"PORT\", \"0\") }\n"}},
		{"loopback only", "start", "answers on 127.0.0.1 only", map[string]string{
			"host.go": "package main\n\nimport \"os\"\n\nfunc init() { os.Setenv(\"HOST\", \"127.0.0.1\") }\n"}},
		{"writes beside its code", "writes", "it wrote cache.json beside its code", map[string]string{
			"cache.go": "package main\n\nimport \"os\"\n\nfunc init() { os.WriteFile(\"cache.json\", []byte(\"{}\"), 0o644) }\n"}},
		{"still calls the old database", "old-database", "abcd.supabase.co appears in web/old.js", map[string]string{
			"web/old.js": "fetch('https://abcd.supabase.co/rest/v1/notes')\n"}},
		{"missing migration", "tables", `nothing creates table "notes"`, map[string]string{
			"db/migrations/001_notes.sql": ""}},
		{"login screen of its own", "login", "a password field in web/login.html", map[string]string{
			"web/login.html": "<form><input type=\"password\" name=\"pw\"></form>\n"}},
		{"build fails", "build", "go build -o server . failed", map[string]string{
			"broken.go": "package main\n\nfunc init() { undefined() }\n"}},
		{"bad recipe", "recipe", "a standard app is a Go server on the kit", map[string]string{
			"llocal.json": `{"shape": "standard", "base": "node", "start": "node server.js", "port": 8080}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Run(context.Background(), starter(t, tc.files), survey(t))
			if r.OK {
				t.Fatal("passed")
			}
			for _, c := range r.Checks {
				if c.Name == tc.check {
					if c.OK || !strings.Contains(c.Reason, tc.reason) || strings.Contains(c.Reason, "\n") {
						t.Fatalf("%s: ok=%v reason=%q, want one line with %q", c.Name, c.OK, c.Reason, tc.reason)
					}
					t.Log(c.Reason)
					return
				}
			}
			t.Fatalf("no %s check in %+v", tc.check, r.Checks)
		})
	}
}
