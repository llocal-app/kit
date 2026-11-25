package check

import (
	"context"
	"net"
	"os"
	"os/exec"
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

// notesApp is the starter's server with only GET /api/notes, which first does read, and
// GET /api/notes/{id}, which finds no note.
func notesApp(read string) string {
	return `package main

import (
	"net/http"

	"llocal.app/kit"
)

func main() {
	app := kit.New()
	app.Get("/api/notes", func(w http.ResponseWriter, r *http.Request) {
		` + read + `
		kit.JSON(w, http.StatusOK, []string{})
	})
	app.Get("/api/notes/{id}", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	app.Run()
}
`
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
	if got := strings.Join(names, " "); got != "recipe build links old-database login start pages routes reads writes tables data-copy" {
		t.Errorf("checks = %s", got)
	}
	if r.IdleMemory == 0 || r.StartMillis == 0 || r.DiskBytes == 0 {
		t.Errorf("numbers not recorded: %+v", r)
	}
}

// gVisor's /proc/meminfo counts its processes' memory by the pages they use; tmpfs, where
// the checker keeps the app's data, is in AnonPages and Shmem both.
func TestInUseReadsGVisorsMeminfo(t *testing.T) {
	meminfo := "MemTotal:        4194304 kB\nMemFree:         4000000 kB\nMemAvailable:    4000000 kB\nBuffers:               0 kB\n" +
		"Cached:            12000 kB\nAnonPages:         30000 kB\nMapped:             5000 kB\nShmem:              7000 kB\n"
	if got := inUse([]byte(meminfo)); got != (30000-7000+5000)<<10 {
		t.Fatalf("in use %d", got)
	}
}

func TestABuiltAppIsCheckedWithoutBuildingIt(t *testing.T) {
	dir := starter(t, nil)
	build := exec.Command("go", "build", "-o", "server", ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	// Built: the server binary already there is what starts, so a broken source doesn't matter.
	write(t, dir, map[string]string{"broken.go": "package main\n\nfunc init() { undefined() }\n"})
	r := Built(context.Background(), dir, survey(t))
	if !r.OK || slices.ContainsFunc(r.Checks, func(c Result) bool { return c.Name == "build" }) {
		t.Fatalf("built check: %+v", r.Checks)
	}
}

// A route the app no longer has fails, unless it was the old login's or was marked removed.
// A route's own 404 for a record an empty database doesn't have is not a route gone.
func TestRoutesThatMayBeGone(t *testing.T) {
	s := Survey{Routes: []Route{
		{Method: "GET", Path: "/api/notes/"},
		{Method: "POST", Path: "/api/notes", Removed: "the person agreed notes are read-only"},
		{Method: "POST", Path: "/api/login/access-token"},
		{Method: "GET", Path: "/api/notes/{id}"},
		{Method: "GET", Path: "/api/gone/{id}"},
	}}
	r := Run(context.Background(), starter(t, map[string]string{"main.go": notesApp("")}), s)
	i := slices.IndexFunc(r.Checks, func(c Result) bool { return c.Name == "routes" })
	if i < 0 || r.Checks[i].OK || !strings.HasPrefix(r.Checks[i].Reason, "GET /api/gone/1 answered 404, as where the app has no route") || strings.Contains(r.Checks[i].Reason, "more)") {
		t.Fatalf("checks: %+v", r.Checks)
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

// The checker builds what llocal would take in, not the working folder: a binding built by
// hand in node_modules, which a clean install doesn't make, can't pass the build here when
// it can't on llocal. The same with git, which leaves out what it ignores.
func TestTheCheckBuildsACleanCopy(t *testing.T) {
	for _, withGit := range []bool{false, true} {
		dir := t.TempDir()
		write(t, dir, map[string]string{
			"llocal.json": `{"shape": "kept", "base": "python", "build": "test -f node_modules/binding.node && test -f local.cfg",
				"start": "exec python3 -m http.server $PORT --bind 0.0.0.0", "port": ` + freePort(t) + `}`,
			"local.cfg": "made by hand",
		})
		os.Mkdir(filepath.Join(dir, "node_modules"), 0o755)
		write(t, dir, map[string]string{"node_modules/binding.node": "built by hand"})
		if withGit {
			write(t, dir, map[string]string{".gitignore": "local.cfg\n"})
			if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
		}
		r := Run(context.Background(), dir, Survey{})
		i := slices.IndexFunc(r.Checks, func(c Result) bool { return c.Name == "build" })
		if i < 0 || r.Checks[i].OK || !strings.Contains(r.Checks[i].Reason, "test -f node_modules/binding.node") {
			t.Fatalf("git %v: checks %+v", withGit, r.Checks)
		}
	}
}

// What llocal refuses, the checker refuses: a link in the project, which the upload refuses,
// and a link the build left that points outside the app, which llocal's build refuses.
func TestLinksLlocalRefusesFail(t *testing.T) {
	dir := starter(t, nil)
	if err := os.Symlink("index.html", filepath.Join(dir, "web", "home.html")); err != nil {
		t.Fatal(err)
	}
	r := Run(context.Background(), dir, survey(t))
	if r.OK || r.Checks[0].Name != "upload" || r.Checks[0].Reason != "web/home.html is a link: llocal takes only files and folders, so put the file itself there" {
		t.Fatalf("project link: %+v", r.Checks)
	}
	dir = starter(t, nil)
	edit(t, filepath.Join(dir, "llocal.json"), `"build": "go build -o server ."`, `"build": "go build -o server . && ln -s /etc/hosts hosts"`)
	r = Run(context.Background(), dir, survey(t))
	i := slices.IndexFunc(r.Checks, func(c Result) bool { return c.Name == "links" })
	if r.OK || i < 0 || !strings.HasPrefix(r.Checks[i].Reason, "hosts is a link to /etc/hosts, an absolute path") {
		t.Fatalf("build link: %+v", r.Checks)
	}
}

// A standard app's node_modules only builds its front end, so llocal doesn't keep it and
// its size on disk leaves it out.
func TestAStandardAppsDiskLeavesOutNodeModules(t *testing.T) {
	dir := starter(t, nil)
	edit(t, filepath.Join(dir, "llocal.json"), `"build"`, `"install": "mkdir -p node_modules && dd if=/dev/zero of=node_modules/big bs=1 count=0 seek=1073741824 2>/dev/null", "build"`)
	r := Run(context.Background(), dir, survey(t))
	if !r.OK || r.DiskBytes == 0 || r.DiskBytes >= 1<<30 {
		t.Fatalf("disk %d: %+v", r.DiskBytes, r.Checks)
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
		{"a route gone", "routes", "POST /api/notes answered 405, as where the app has no route", map[string]string{
			"main.go": notesApp(``)}},
		{"a read that writes", "reads", "GET /api/notes wrote to the database", map[string]string{
			"main.go": notesApp(`app.DB.ExecContext(r.Context(), "INSERT INTO notes (body, user_id, by_name) VALUES ('seen', ?, '')", kit.CurrentUser(r).ID)`)}},
		// The person saved again on every request, as it was: nothing changes, but SQLite's writer is taken.
		{"a read that writes nothing new", "reads", "GET /api/notes wrote to the database", map[string]string{
			"main.go": notesApp(`var id int64
		app.DB.QueryRowContext(r.Context(), "UPDATE notes SET by_name = ? WHERE user_id = ? RETURNING id", kit.CurrentUser(r).Name, kit.CurrentUser(r).ID).Scan(&id)`)}},
		{"a column the old table lacks", "data-copy", `can't fill table "notes": its column llocal_id is required and the old notes table has no llocal_id`, map[string]string{
			"db/migrations/001_notes.sql": "CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL, user_id TEXT NOT NULL, by_name TEXT NOT NULL, llocal_id TEXT NOT NULL);\n"}},
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
