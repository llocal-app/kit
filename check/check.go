// Package check is llocal's checker: it says whether an app is in the standard shape, with
// a one-line reason a person and an agent can both act on for each failure.
//
// It runs the app's own commands with /bin/sh in the app folder and reads its files, so it
// must run where the app is built: as it is on a builder's machine, and on the server inside
// the app's sandbox, where the runner starts `llocal-check -json <dir>` and reads back the
// report. It never runs on the server's host, because the app's code is not trusted.
package check

import (
	"bytes"
	"cmp"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"llocal.app/kit/identity"
	"llocal.app/kit/recipe"
)

type Result struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

type Report struct {
	OK     bool     `json:"ok"`
	Checks []Result `json:"checks"`
	// IdleMemory is the app's resident memory after the checks' requests, in bytes. It is
	// left out where the machine has no ps.
	IdleMemory int64 `json:"idle_memory_bytes,omitempty"`
	// StartMillis is how long the app took to answer its health check.
	StartMillis int64 `json:"start_ms,omitempty"`
	// DiskBytes is the app's folder once built, installed packages and all.
	DiskBytes int64 `json:"disk_bytes,omitempty"`
}

// Survey is the part of survey.json the checker reads.
type Survey struct {
	Pages      []Route `json:"pages"`
	Routes     []Route `json:"routes"`
	DataStores []struct {
		Host   string  `json:"host"`
		Tables []Table `json:"tables"`
	} `json:"data_stores"`
}

// Route is a page or a server route. Path parameters are written {name}, like
// /api/orders/{id}. Method defaults to GET. Removed says why a server route the person
// agreed should go is gone.
type Route struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Removed string `json:"removed"`
}

// Table is a table of the old database, with the columns the survey read.
type Table struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

func LoadSurvey(file string) (Survey, error) {
	var s Survey
	data, err := os.ReadFile(file)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("%s: %w", file, err)
	}
	return s, nil
}

var (
	startTimeout  = recipe.StartWait
	pathParam     = regexp.MustCompile(`\{[^/]+\}`)
	surveyFile    = regexp.MustCompile(`^survey([._-][^/]*)?\.json$`)
	passwordField = regexp.MustCompile("(?i)\\btype\\s*[=:]\\s*[\"'`]?password\\b")
	client        = &http.Client{
		Transport:     &http.Transport{},
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
)

// The person the checker signs requests as, the way the gateway would.
const (
	checkID    = "u_llocal_check"
	checkEmail = "check@llocal.app"
	checkName  = "llocal check"
)

type checker struct {
	dir      string
	root     *os.Root
	survey   Survey
	rec      *recipe.Recipe
	report   Report
	key      ed25519.PrivateKey
	base     string
	audience string
	built    bool
}

// Run checks the app in dir. The survey is optional: without one, the checker requests
// only / and cannot look for the old database or its tables.
func Run(ctx context.Context, dir string, s Survey) Report {
	return run(ctx, &checker{dir: dir, survey: s})
}

// Built checks an app that is already built and installed, as llocal's deploys are: it
// skips install and build, and checks what will run.
func Built(ctx context.Context, dir string, s Survey) Report {
	return run(ctx, &checker{dir: dir, survey: s, built: true})
}

func run(ctx context.Context, c *checker) Report {
	c.run(ctx)
	c.report.OK = !slices.ContainsFunc(c.report.Checks, func(r Result) bool { return !r.OK })
	return c.report
}

func (c *checker) failed(name string, err error) bool {
	if err != nil {
		c.add(name, err)
	}
	return err != nil
}

func (c *checker) add(name string, err error) bool {
	r := Result{Name: name, OK: err == nil}
	if err != nil {
		r.Reason = err.Error()
	}
	c.report.Checks = append(c.report.Checks, r)
	return err == nil
}

func (c *checker) run(ctx context.Context) {
	var err error
	if c.root, err = os.OpenRoot(c.dir); c.failed("recipe", err) {
		return
	}
	defer c.root.Close()
	if c.rec, err = recipe.Load(filepath.Join(c.dir, "llocal.json")); !c.add("recipe", err) {
		return
	}
	for _, step := range [][2]string{{"install", c.rec.Install}, {"build", c.rec.Build}} {
		if step[1] != "" && !c.built && !c.add(step[0], c.sh(ctx, step[1], os.Environ())) {
			return
		}
	}
	if c.report.DiskBytes, err = c.disk(); c.failed("disk", err) {
		return
	}
	if hosts := c.hosts(); len(hosts) > 0 {
		c.add("old-database", c.oldDatabase(hosts))
	}
	if c.rec.Shape == "static" {
		_, err := fs.Stat(c.root.FS(), path.Join(c.rec.Static, "index.html"))
		if err != nil {
			err = fmt.Errorf("static folder %q has no index.html: the build must put the site there", c.rec.Static)
		}
		c.add("static", err)
		return
	}
	if c.rec.Shape == "standard" && c.rec.Static != "" {
		c.add("login", c.login())
	}
	c.serve(ctx)
}

// serve runs the app the way the runner would and checks what it does.
func (c *checker) serve(ctx context.Context) {
	before, err := c.snapshot()
	if c.failed("writes", err) {
		return
	}
	data, err := os.MkdirTemp("", "llocal-data-")
	if c.failed("start", err) {
		return
	}
	defer os.RemoveAll(data)
	tmp, err := os.MkdirTemp("", "llocal-tmp-")
	if c.failed("start", err) {
		return
	}
	defer os.RemoveAll(tmp)
	ip, err := outsideIP()
	if c.failed("start", err) {
		return
	}
	pub, key, _ := ed25519.GenerateKey(nil)
	c.key, c.audience = key, ip
	c.base = "http://" + net.JoinHostPort(ip, strconv.Itoa(c.rec.Port))
	env := c.env(data, tmp, identity.Keys{"check": pub}.JWKS())

	if c.rec.Migrate != "" && !c.add("migrate", c.sh(ctx, c.rec.Migrate, env)) {
		return
	}
	if answers(c.loopback()) {
		c.add("start", fmt.Errorf("port %d is already in use on this machine: stop what is using it and check again", c.rec.Port))
		return
	}
	started := time.Now()
	p, err := startProcess(c.dir, c.rec.Start, env)
	if c.failed("start", err) {
		return
	}
	defer p.stop()
	defer client.CloseIdleConnections()
	if !c.add("start", c.healthy(ctx, p)) {
		return
	}
	c.report.StartMillis = time.Since(started).Milliseconds()
	pages := c.survey.Pages
	if len(pages) == 0 && c.rec.Static != "" {
		pages = []Route{{Path: "/"}}
	}
	if len(pages) > 0 {
		c.add("pages", c.pages(ctx, pages))
	}
	if len(c.survey.Routes) > 0 {
		c.add("routes", c.routes(ctx))
	}
	db := filepath.Join(data, "app.db")
	if c.rec.Shape == "standard" {
		c.add("reads", c.reads(ctx, db, pages))
	}
	time.Sleep(time.Second)
	c.report.IdleMemory = p.memory()
	p.stop()
	c.add("writes", c.writes(before))
	tables := c.tables()
	if len(tables) > 0 && c.add("tables", c.missingTables(db, tables)) && slices.ContainsFunc(tables, func(t Table) bool { return len(t.Columns) > 0 }) {
		c.add("data-copy", c.dataCopy(ctx, db, tables))
	}
}

func (c *checker) env(data, tmp string, jwks []byte) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"PORT=" + strconv.Itoa(c.rec.Port),
		"HOST=0.0.0.0",
		"LLOCAL_URL=" + c.base,
		"LLOCAL_DATA=" + data,
		"LLOCAL_JWKS=" + string(jwks),
		"TMPDIR=" + tmp,
		"TZ=America/Denver",
		// .pyc files are a cache, not data the app loses when llocal throws its folder away.
		"PYTHONDONTWRITEBYTECODE=1",
	}
	for _, s := range c.rec.Secrets {
		env = append(env, s+"="+cmp.Or(os.Getenv(s), "llocal-check-placeholder"))
	}
	return env
}

func (c *checker) loopback() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(c.rec.Port))
}

// healthy waits for the app to answer its health check from outside, and says why not.
func (c *checker) healthy(ctx context.Context, p *process) error {
	health := c.rec.HealthPath()
	var unhealthy error
	deadline := time.After(startTimeout)
	for {
		a, err := c.do(ctx, http.MethodGet, health, false)
		if err == nil && a.status < 500 {
			return nil
		}
		if err == nil {
			unhealthy = fmt.Errorf("its health check GET %s answered %d: %s", health, a.status, a.line)
		}
		select {
		case <-p.done:
			return fmt.Errorf("it exited during start: %s", lastLine(p.out.b))
		case <-deadline:
			switch {
			case unhealthy != nil:
				return unhealthy
			case answers(c.loopback()):
				return errors.New("it answers on 127.0.0.1 only: listen on all interfaces (0.0.0.0, as $HOST says)")
			}
			return fmt.Errorf("nothing answered on port %d within %s: listen on the port in $PORT", c.rec.Port, startTimeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func answers(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err == nil {
		conn.Close()
	}
	return err == nil
}

// pages requests each page signed in, and fails on one that answers 400 or above.
func (c *checker) pages(ctx context.Context, pages []Route) error {
	var failed []string
	for _, r := range pages {
		p := pathParam.ReplaceAllString(r.Path, "1")
		a, err := c.do(ctx, http.MethodGet, p, true)
		switch {
		case err != nil:
			failed = append(failed, fmt.Sprintf("GET %s failed: %v", p, err))
		case a.status >= http.StatusBadRequest:
			failed = append(failed, fmt.Sprintf("GET %s answered %d: %s", p, a.status, a.line))
		}
	}
	return joined(failed)
}

// loginRoute is a path of the old login's, which goes: llocal signs people in.
var loginRoute = regexp.MustCompile(`(?i)(^|[/_-])(log-?in|log-?out|sign-?in|sign-?up|sign-?out|password|auth|oauth2?)([/_.-]|$)`)

// routes sends each server route in the survey signed in, and fails on one that answers 500
// or above, or that the app no longer has: a 405 to the route's own method, or the 404 the
// kit gives where it has no route. The old login's routes and the routes marked removed may
// be gone. Frameworks differ on a trailing slash, so the path's other form must be gone too.
func (c *checker) routes(ctx context.Context) error {
	var failed []string
	gone := func(r Route, a answer) bool {
		return a.status == http.StatusMethodNotAllowed && r.Method != "" || a.noRoute
	}
	for _, r := range c.survey.Routes {
		method := cmp.Or(strings.ToUpper(r.Method), http.MethodGet)
		p := pathParam.ReplaceAllString(r.Path, "1")
		a, err := c.do(ctx, method, p, true)
		switch {
		case err != nil:
			failed = append(failed, fmt.Sprintf("%s %s failed: %v", method, p, err))
		case a.status >= http.StatusInternalServerError:
			failed = append(failed, fmt.Sprintf("%s %s answered %d: %s", method, p, a.status, a.line))
		case gone(r, a) && r.Removed == "" && !loginRoute.MatchString(r.Path):
			other := strings.TrimSuffix(p, "/")
			if other == p {
				other += "/"
			}
			if b, err := c.do(ctx, method, other, true); err != nil || gone(r, b) {
				failed = append(failed, fmt.Sprintf(`%s %s answered %d, as where the app has no route: the old app had it (the survey lists it), so keep it, or, if the person agreed it goes, set "removed" on it in survey.json to why`, method, p, a.status))
			}
		}
	}
	return joined(failed)
}

// reads sends each page and GET route again, now that the test person has been seen once,
// and fails on one that writes to the database.
func (c *checker) reads(ctx context.Context, file string, pages []Route) error {
	db, err := sql.Open("sqlite", file+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	// data_version changes when another connection, here the app's, has written.
	version := func() (int64, error) {
		var v int64
		err := conn.QueryRowContext(ctx, "PRAGMA data_version").Scan(&v)
		return v, err
	}
	was, err := version()
	if err != nil {
		return err
	}
	for _, r := range slices.Concat(pages, c.survey.Routes) {
		if r.Method != "" && !strings.EqualFold(r.Method, http.MethodGet) {
			continue
		}
		p := pathParam.ReplaceAllString(r.Path, "1")
		if _, err := c.do(ctx, http.MethodGet, p, true); err != nil {
			continue
		}
		if now, err := version(); err != nil || now != was {
			return cmp.Or(err, fmt.Errorf("GET %s wrote to the database: a request that only reads must not write, since every write waits for SQLite's one writer; record a person on their first visit only, not on every request", p))
		}
	}
	return nil
}

func joined(failed []string) error {
	switch len(failed) {
	case 0:
		return nil
	case 1:
		return errors.New(failed[0])
	}
	return fmt.Errorf("%s (and %d more)", failed[0], len(failed)-1)
}

// answer is what the app answered: its status, the first line of its body, and whether
// the kit said it has no route there (X-LLocal-Route: none).
type answer struct {
	status  int
	line    string
	noRoute bool
}

func (c *checker) do(ctx context.Context, method, p string, signed bool) (answer, error) {
	var body io.Reader
	if method != http.MethodGet && method != http.MethodHead {
		body = strings.NewReader("{}")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+p, body)
	if err != nil {
		return answer{}, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if signed {
		now := time.Now()
		identity.Set(req.Header, "check", c.key, identity.Claims{
			Subject: checkID, Audience: c.audience, IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
			Email: checkEmail, Name: checkName, Groups: []string{}, Company: "c_llocal_check",
		})
	}
	resp, err := client.Do(req)
	if err != nil {
		return answer{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return answer{resp.StatusCode, firstLine(b), resp.Header.Get("X-LLocal-Route") == "none"}, nil
}

type stamp struct{ size, mod int64 }

func (c *checker) snapshot() (map[string]stamp, error) {
	files := map[string]stamp{}
	err := c.walk(".", func(p string, d fs.DirEntry) error {
		fi, err := d.Info()
		if err == nil {
			files[p] = stamp{fi.Size(), fi.ModTime().UnixNano()}
		}
		return err
	})
	return files, err
}

func (c *checker) writes(before map[string]stamp) error {
	after, err := c.snapshot()
	if err != nil {
		return err
	}
	persistent := func(p string) bool {
		return slices.ContainsFunc(c.rec.Persistent, func(pp string) bool { return p == pp || strings.HasPrefix(p, pp+"/") })
	}
	var wrote []string
	for p, s := range after {
		if b, ok := before[p]; ok && b == s || persistent(p) {
			continue
		}
		wrote = append(wrote, p)
	}
	if len(wrote) == 0 {
		return nil
	}
	slices.Sort(wrote)
	return fmt.Errorf("it wrote %s beside its code: keep data under $LLOCAL_DATA (/data on llocal) and scratch files under /tmp", strings.Join(wrote[:min(3, len(wrote))], ", "))
}

func (c *checker) tables() []Table {
	var tables []Table
	for _, ds := range c.survey.DataStores {
		tables = append(tables, ds.Tables...)
	}
	return tables
}

func (c *checker) missingTables(file string, want []Table) error {
	if _, err := os.Stat(file); err != nil {
		return errors.New("missing migration: there is no database at $LLOCAL_DATA/app.db; create the app's tables there with migrations (db/migrations on the kit)")
	}
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return err
	}
	defer db.Close()
	var missing []string
	for _, t := range want {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ? COLLATE NOCASE`, t.Name).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			missing = append(missing, strconv.Quote(t.Name))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing migration: nothing creates table %s in $LLOCAL_DATA/app.db; add a migration that does (db/migrations on the kit)", strings.Join(missing, ", "))
}

func (c *checker) hosts() []string {
	var hosts []string
	for _, ds := range c.survey.DataStores {
		if ds.Host != "" {
			hosts = append(hosts, ds.Host)
		}
	}
	return hosts
}

func (c *checker) oldDatabase(hosts []string) error {
	return c.scan(".", func(p string) bool {
		name := path.Base(p)
		return !strings.HasSuffix(name, ".md") && !strings.HasPrefix(name, ".env") && !surveyFile.MatchString(name)
	}, func(p string, data []byte) error {
		for _, h := range hosts {
			if bytes.Contains(data, []byte(h)) {
				return fmt.Errorf("it still uses the old database: %s appears in %s; move its data to SQLite and remove the old address", h, p)
			}
		}
		return nil
	})
}

func (c *checker) login() error {
	return c.scan(c.rec.Static, func(p string) bool {
		return slices.Contains([]string{".html", ".htm", ".js", ".mjs"}, path.Ext(p))
	}, func(p string, data []byte) error {
		if passwordField.Match(data) {
			return fmt.Errorf("it has a login screen of its own: a password field in %s; remove the login, llocal signs people in", p)
		}
		return nil
	})
}

// disk is the size of every regular file in the app but .git, installed packages included.
func (c *checker) disk() (int64, error) {
	var n int64
	err := fs.WalkDir(c.root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == ".git":
			return fs.SkipDir
		case !d.Type().IsRegular():
			return nil
		}
		fi, err := d.Info()
		if err == nil {
			n += fi.Size()
		}
		return err
	})
	return n, err
}

// walk hands fn each regular file under dir, skipping .git and node_modules.
func (c *checker) walk(dir string, fn func(p string, d fs.DirEntry) error) error {
	return fs.WalkDir(c.root.FS(), dir, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules"):
			return fs.SkipDir
		case !d.Type().IsRegular():
			return nil
		}
		return fn(p, d)
	})
}

// scan hands find the contents of each file under dir that want accepts, skipping files
// over 4 MB, and stops at the first error find returns.
func (c *checker) scan(dir string, want func(p string) bool, find func(p string, data []byte) error) error {
	return c.walk(dir, func(p string, d fs.DirEntry) error {
		if !want(p) {
			return nil
		}
		fi, err := d.Info()
		if err != nil || fi.Size() > 4<<20 {
			return err
		}
		data, err := fs.ReadFile(c.root.FS(), p)
		if err != nil {
			return err
		}
		return find(p, data)
	})
}

func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return clip(lines[len(lines)-1])
}

func firstLine(b []byte) string {
	line, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	return clip(line)
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
