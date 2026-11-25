// Package survey reads a project folder and says what it is: its framework, pages, server
// routes, data, login, outside services and settings. That is survey.json, which
// the checker, the plan and the conversion all start from.
//
// It runs on hostile files: it only reads, never runs any of the project's code, never
// follows a symbolic link, and stops at fixed caps. Every rule is a pattern over file names
// and contents; nothing here asks an AI.
package survey

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
)

type Survey struct {
	Languages  []string `json:"languages"`
	Frameworks []string `json:"frameworks"`
	// Builder is the tool that made the project, like lovable, bolt, v0 or replit.
	Builder string `json:"builder,omitempty"`
	// RecipeShape is the shape a valid llocal.json at the root names: the app already fits.
	RecipeShape string   `json:"recipe_shape,omitempty"`
	Servers     []Server `json:"servers,omitempty"`
	// ServerPages is true when the server draws the pages itself (templates, Streamlit,
	// Next.js server rendering or server actions), so the front end cannot be split off.
	ServerPages bool        `json:"server_pages,omitempty"`
	Pages       []Route     `json:"pages"`
	Routes      []Route     `json:"routes"`
	Functions   []Function  `json:"functions,omitempty"`
	DataStores  []DataStore `json:"data_stores"`
	Login       *Login      `json:"login,omitempty"`
	Services    []Service   `json:"services,omitempty"`
	Env         []Env       `json:"env,omitempty"`
	Keys        []Key       `json:"keys,omitempty"`
	Schedules   []Schedule  `json:"schedules,omitempty"`
	Webhooks    []Webhook   `json:"webhooks,omitempty"`
	Uploads     []Upload    `json:"uploads,omitempty"`
	Private     []Address   `json:"private,omitempty"`
	Containers  []Container `json:"containers,omitempty"`
	// UnlistedPackages are the Python packages the code imports, when it has no
	// requirements file to say so.
	UnlistedPackages []string  `json:"unlisted_packages,omitempty"`
	Packages         []Package `json:"packages,omitempty"`
	Repeated         *Repeated `json:"repeated,omitempty"`
	Screens          *Screens  `json:"screens,omitempty"`
	// Leftovers are the folders and files whose names say they are old copies or backups.
	Leftovers []string `json:"leftovers,omitempty"`
	// Incomplete says why only part of the project was read, when a cap was reached.
	Incomplete string `json:"incomplete,omitempty"`
}

// Server is a program in the project that listens for requests. Dir is "." for the root.
// Files and Bytes are its own code: the code files in its folder that run on the server,
// leaving out pages, tests, config and the folders of other servers inside it.
type Server struct {
	Dir       string `json:"dir"`
	Framework string `json:"framework"`
	Files     int    `json:"files,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
}

// Route is a page or a server route. Path parameters are written {name}. An empty method
// means any. Removed is never set by the survey: whoever converts the app sets it, saying
// why, on a server route the person agreed should go.
type Route struct {
	Method  string `json:"method,omitempty"`
	Path    string `json:"path"`
	File    string `json:"file,omitempty"`
	Removed string `json:"removed,omitempty"`
}

// Function is server code called by name rather than by path: supabase-edge,
// server-action, server-function or netlify.
type Function struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
}

// DataStore is a database the app uses: supabase, postgres, mysql, mssql, mongodb,
// firebase or sqlite. Host is the address as it appears in the project. Browser is true
// when visitors' browsers talk to it directly.
type DataStore struct {
	Kind    string   `json:"kind"`
	Host    string   `json:"host,omitempty"`
	Browser bool     `json:"browser,omitempty"`
	Env     []string `json:"env,omitempty"`
	Files   []string `json:"files,omitempty"`
	Tables  []Table  `json:"tables"`
}

// Table is a table or collection. Defined says the survey read its definition (SQL, Prisma,
// SQLAlchemy or Drizzle), so Key, References, JSON and Lists say what that holds: whether it
// has a primary key, the tables it refers to, and its columns of JSON and of lists.
type Table struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns,omitempty"`
	File       string   `json:"file,omitempty"`
	Defined    bool     `json:"defined,omitempty"`
	Key        bool     `json:"key,omitempty"`
	References []string `json:"references,omitempty"`
	JSON       []string `json:"json,omitempty"`
	Lists      []string `json:"lists,omitempty"`
}

// Login is the app's own login. Provider is supabase, replit, lovable, clerk, nextauth,
// firebase, auth0, netlify-identity, passport, flask-login, fastapi, streamlit or own.
type Login struct {
	Provider  string   `json:"provider"`
	UserTable string   `json:"user_table,omitempty"`
	Files     []string `json:"files"`
}

// Service is an outside service that is not the database, like stripe (payments).
type Service struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Browser bool     `json:"browser,omitempty"`
	Files   []string `json:"files"`
}

// Env is an environment variable the code reads or a .env file sets. Values are never
// recorded. Looks says what a value (or failing that, the name) looks like: service_role,
// secret_key, private_key or publishable. Service is what it belongs to: a service or data
// store name, login, or platform for names the old host or llocal sets.
type Env struct {
	Name    string   `json:"name"`
	Files   []string `json:"files"`
	Browser bool     `json:"browser,omitempty"`
	Set     bool     `json:"set,omitempty"`
	Looks   string   `json:"looks,omitempty"`
	Service string   `json:"service,omitempty"`
}

// Key is a key written into the code itself.
type Key struct {
	File    string `json:"file"`
	Looks   string `json:"looks"`
	Browser bool   `json:"browser,omitempty"`
}

// Schedule is timed work: a timer or cron library in the code, or a host's cron entry.
type Schedule struct {
	File string `json:"file"`
	Cron string `json:"cron,omitempty"`
	Path string `json:"path,omitempty"`
}

// Webhook is a route or function an outside service calls.
type Webhook struct {
	Path    string `json:"path,omitempty"`
	Service string `json:"service,omitempty"`
	File    string `json:"file"`
}

// Upload is a place people upload files: server, form, supabase-storage, firebase-storage,
// s3, vercel-blob or replit-storage.
type Upload struct {
	Kind string `json:"kind"`
	File string `json:"file"`
}

// Address is an address on localhost or a private network.
type Address struct {
	Address string `json:"address"`
	File    string `json:"file"`
}

// Container is a Dockerfile, with the system packages it installs, or a compose file, with
// its services.
type Container struct {
	File     string   `json:"file"`
	Installs []string `json:"installs,omitempty"`
	Services []string `json:"services,omitempty"`
}

// What one survey reads at most, so a hostile project cannot make it run long or use much
// memory, and how long each list in survey.json may grow.
var (
	maxFiles           = 20000
	maxDepth           = 16
	maxFileBytes int64 = 1 << 20
	maxReadBytes int64 = 64 << 20
	maxList            = 200
)

// Folders that hold installed packages, build output or caches, not the app's own code.
// mockup-sandbox is the design scratch space Replit Agent adds beside every app. llocal's
// upload leaves the same folders out of what it commits.
var SkipDirs = []string{
	".git", "node_modules", "bower_components", "dist", "build", "out", ".next", ".nuxt", ".output", ".nitro", ".vercel",
	".netlify", ".wrangler", ".svelte-kit", ".turbo", ".cache", ".parcel-cache", ".expo", ".tanstack",
	"coverage", "__pycache__", ".venv", "venv", ".tox", ".mypy_cache", ".pytest_cache", ".ruff_cache",
	"site-packages", "vendor", "target", "mockup-sandbox",
}

var (
	codeExts = []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts", ".vue", ".svelte", ".astro", ".py", ".go", ".html", ".htm"}
	// The other files the rules read, besides code, .env, requirements, Docker and compose files.
	dataExts  = []string{".sql", ".prisma", ".toml"}
	readNames = []string{"package.json", "go.mod", "llocal.json", "vercel.json", "Pipfile", "README.md", "readme.md", ".gitignore"}
)

// project is the folder being surveyed: every regular file's path, the text of the ones
// the rules read, and what the rules have learned so far.
type project struct {
	fsys  fs.FS
	paths []string
	text  map[string]string
	read  int64
	s     Survey

	code    []string            // the code files read, in walk order
	deps    map[string][]string // manifest -> dependency names
	runtime map[string][]string // manifest -> the dependencies the app runs on, sorted
	imports map[string][]string // file -> packages it imports
	browser map[string]bool     // file -> runs in visitors' browsers
}

// Run surveys the project in dir.
func Run(dir string) (Survey, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Survey{}, err
	}
	defer root.Close()
	p := &project{fsys: root.FS(), text: map[string]string{}}
	if err := p.load(); err != nil {
		return Survey{}, err
	}
	p.code = p.files(IsCode)
	p.survey()
	return p.s, nil
}

var errFull = errors.New("full")

func (p *project) load() error {
	err := fs.WalkDir(p.fsys, ".", func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && name != "." && (slices.Contains(SkipDirs, d.Name()) || strings.Count(name, "/") >= maxDepth):
			return fs.SkipDir
		case !d.Type().IsRegular():
			return nil
		case len(p.paths) == maxFiles:
			p.s.Incomplete = fmt.Sprintf("it has more than %d files; only the first %d were read", maxFiles, maxFiles)
			return errFull
		}
		p.paths = append(p.paths, name)
		if !wanted(name) {
			return nil
		}
		fi, err := d.Info()
		if err != nil || fi.Size() > maxFileBytes {
			return err
		}
		return p.readFile(name)
	})
	if err == errFull {
		return nil
	}
	return err
}

func wanted(name string) bool {
	base := path.Base(name)
	if strings.HasSuffix(base, ".min.js") || strings.HasSuffix(base, ".d.ts") || inToolFolder(name) {
		return false
	}
	return IsCode(name) || IsEnvFile(name) || isRequirements(base) || isDockerfile(base) ||
		slices.Contains(dataExts, path.Ext(base)) || slices.Contains(readNames, base) || slices.Contains(composeNames, base)
}

// inToolFolder says whether a file is in a folder like .bolt, .lovable or .github, which
// hold a tool's own files rather than the app's. Their names still say which tool made the
// app.
func inToolFolder(name string) bool {
	dir := path.Dir(name)
	return (strings.HasPrefix(dir, ".") && dir != "." || strings.Contains(dir, "/.")) && !strings.Contains(name, ".streamlit/")
}

func isRequirements(base string) bool {
	return strings.HasPrefix(base, "requirements") && path.Ext(base) == ".txt"
}

// readFile keeps the text of one file, skipping a file that has grown past maxFileBytes
// since it was listed, and stopping all reading once maxReadBytes have been read.
func (p *project) readFile(name string) error {
	if p.read >= maxReadBytes {
		p.s.Incomplete = fmt.Sprintf("its files hold more than %d MB of code; the rest was not read", maxReadBytes>>20)
		return errFull
	}
	f, err := p.fsys.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil || int64(len(b)) > maxFileBytes {
		return err
	}
	p.read += int64(len(b))
	p.text[name] = string(b)
	return nil
}

// files returns the names of the files read whose name match accepts, in walk order.
func (p *project) files(match func(name string) bool) []string {
	var names []string
	for _, n := range p.paths {
		if _, ok := p.text[n]; ok && match(n) {
			names = append(names, n)
		}
	}
	return names
}

func IsCode(name string) bool { return slices.Contains(codeExts, path.Ext(name)) }

func isPython(name string) bool { return path.Ext(name) == ".py" }

func isGo(name string) bool { return path.Ext(name) == ".go" }

// add appends v to list unless it is there already, and stops one past maxList so finish
// can say the list was cut: checking a long list for each new entry would take a hostile
// project's survey from seconds to hours.
func add[T comparable](list []T, v ...T) []T {
	for _, x := range v {
		if len(list) <= maxList && !slices.Contains(list, x) {
			list = append(list, x)
		}
	}
	return list
}
