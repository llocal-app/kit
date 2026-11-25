package survey

import (
	"cmp"
	"encoding/json"
	"maps"
	"path"
	"slices"
	"strings"
)

// Facts about how a project is put together: the packages it runs on and whether its code
// uses them, what its table definitions say, code repeated across files, and old copies left
// in. They are counted, never judged.

// Package is one a manifest lists for the app to run, its development tools left out. Unused
// says no file the survey read imports or names it, no other package listed needs it, and it
// is not a command the app is started with.
type Package struct {
	Name     string `json:"name"`
	Manifest string `json:"manifest"`
	Unused   bool   `json:"unused,omitempty"`
}

// Repeated is the code that appears in more than one file: Lines are the lines, in all the
// files together, that are part of a run of repeatAt or more found in another file too.
type Repeated struct {
	Lines int      `json:"lines"`
	Files []string `json:"files"`
}

// repeatAt is the fewest lines in a row, blank lines, comments and imports left out, that
// count as repeated code.
const repeatAt = 8

// facts are the ones read in passes of their own; what a table's definition says is read with
// the definition (tables).
func (p *project) facts() {
	p.packages()
	p.repeats()
	p.leftovers()
	p.screens()
}

// ORMs are the packages that put a layer between an app's code and its database, which load
// the database's own package themselves.
var ORMs = []string{"drizzle-orm", "knex", "sequelize", "typeorm", "kysely", "@mikro-orm/core", "prisma", "@prisma/client", "mongoose",
	"sqlalchemy", "flask-sqlalchemy", "sqlmodel", "django", "peewee", "tortoise-orm", "gorm.io/gorm"}

var (
	setupFile = compile(`(^|/)[\w.-]*(config|rc)[\w.-]*\.[cm]?[jt]s$|(^|/)settings\.py$|(^|/)Dockerfile|\.toml$|^llocal\.json$|^\.github/workflows/`)
	word      = compile(`[@\w][\w@./-]*`)
	// commands are packages an app is started or built with rather than imported.
	commands = []string{"gunicorn", "uvicorn", "hypercorn", "waitress", "daphne", "nodemon", "pm2", "concurrently", "cross-env",
		"tsx", "ts-node", "serve", "http-server", "prisma", "vite", "next", "typescript"}
	// drivers are the database packages an ORM or query builder loads itself.
	drivers = []string{"pg", "postgres", "pg-hstore", "mysql", "mysql2", "sqlite3", "better-sqlite3", "tedious", "@libsql/client",
		"@neondatabase/serverless", "psycopg2", "psycopg2-binary", "psycopg", "asyncpg", "pymysql", "mysqlclient", "aiosqlite"}
	// neededBy are packages another one the app lists loads itself.
	neededBy = map[string][]string{"react": {"react-dom", "next"}, "react-dom": {"react", "next"}, "python-multipart": {"fastapi"}, "sharp": {"next"},
		"@prisma/client": {"prisma"}, "email-validator": {"pydantic", "fastapi"}}
)

// firsts are the first groups of every match of r in t.
func firsts(r rule, t string) []string {
	var out []string
	for _, m := range r.FindAllStringSubmatch(t, -1) {
		out = append(out, m[1])
	}
	return out
}

// packages lists the packages each manifest lists for running the app (manifests), and
// whether anything uses them.
func (p *project) packages() {
	imported, named := map[string]bool{}, map[string]bool{}
	for _, mods := range p.imports {
		for _, m := range mods {
			imported[m], imported[norm(m)] = true, true
		}
	}
	for _, n := range p.files(func(n string) bool { return path.Base(n) == "package.json" || setupFile.MatchString(n) }) {
		text := p.text[n]
		if path.Base(n) == "package.json" {
			var pkg struct {
				Scripts map[string]string `json:"scripts"`
			}
			json.Unmarshal([]byte(text), &pkg)
			text = strings.Join(slices.Collect(maps.Values(pkg.Scripts)), "\n")
		}
		for _, w := range word.FindAllString(text, -1) {
			named[w] = true
			for _, part := range strings.FieldsFunc(w, func(r rune) bool { return r == '.' || r == '/' }) {
				named[part] = true
			}
		}
	}
	var goCode []string
	for _, n := range p.files(isGo) {
		goCode = append(goCode, p.text[n])
	}
	listed := map[string]bool{}
	var all []Package
	for _, m := range slices.Sorted(maps.Keys(p.runtime)) {
		for _, d := range p.runtime[m] {
			listed[d] = true
			all = append(all, Package{Name: d, Manifest: m})
		}
	}
	lists := func(names []string) bool { return slices.ContainsFunc(names, func(n string) bool { return listed[n] }) }
	for i := range all {
		d := &all[i]
		used := slices.ContainsFunc(modules(d.Name), func(m string) bool { return imported[m] || named[m] }) ||
			slices.Contains(commands, d.Name) || slices.Contains(drivers, d.Name) && lists(ORMs) || lists(neededBy[d.Name])
		if path.Base(d.Manifest) == "go.mod" {
			used = slices.ContainsFunc(goCode, func(t string) bool { return strings.Contains(t, `"`+d.Name) })
		}
		d.Unused = !used
	}
	p.s.Packages = capped(&p.s, all)
}

// modules are the names code may import or name a package by: npm's own name, or a Python
// package's module names, like dotenv for python-dotenv and PIL for pillow.
func modules(pkg string) []string {
	out := []string{pkg, strings.ReplaceAll(pkg, "-", "_"), strings.TrimPrefix(pkg, "python-"), strings.TrimSuffix(pkg, "-binary"), strings.TrimPrefix(pkg, "py")}
	for mod, name := range pipNames {
		if name == pkg {
			out = append(out, mod)
		}
	}
	for _, m := range slices.Clone(out) {
		out = append(out, norm(m))
	}
	return out
}

// What a table's definition says (Table.Defined): the parsers in data.go call these with each
// definition they read.
var (
	primaryKey  = compile(`(?i)\bprimary\s+key\b`)
	sqlRefers   = compile("(?i)\\breferences\\s+(?:[\"`\\[]?\\w+[\"`\\]]?\\.)?[\"`\\[]?(\\w+)")
	jsonType    = compile(`(?i)^jsonb?$`)
	prismaTable = compile(`@@map\(\s*(?:name:\s*)?"(\w+)"`)
	prismaRel   = compile(`(?m)^\s*\w+\s+(\w+)\??\s.*@relation\([^)\n]*\bfields\s*:`)
	prismaJSON  = compile(`(?m)^\s*(\w+)\s+Json\b`)
	prismaList  = compile(`(?m)^\s*(\w+)\s+(?:String|Int|BigInt|Float|Decimal|Boolean|DateTime)\[\]`)
	pyKey       = compile(`primary_key\s*=\s*True`)
	pyRefers    = compile(`(?:ForeignKey\(\s*|foreign_key\s*=\s*)["'](\w+)\.`)
	pyJSON      = compile(`(?m)^[ \t]+(\w+)\s*(?::[^=\n]*)?=\s*(?:\w+\.)?(?:Column|mapped_column)\(\s*(?:\w+\.)?JSONB?\b`)
	pyList      = compile(`(?m)^[ \t]+(\w+)\s*(?::[^=\n]*)?=\s*(?:\w+\.)?(?:Column|mapped_column)\(\s*(?:\w+\.)?ARRAY\(`)
	drizzleVar  = compile(`\b(\w+)\s*=\s*(?:pg|mysql|sqlite)Table\(\s*["'](\w+)["']`)
	drizzleKey  = compile(`\.primaryKey\(\)|\bprimaryKey\(`)
	drizzleRefs = compile(`\.references\(\s*\(\)\s*(?::\s*\w+\s*)?=>\s*(\w+)\.`)
	drizzleJSON = compile(`(?m)^\s*(\w+)\s*:\s*jsonb?\(`)
	drizzleList = compile(`(?m)^\s*(\w+)\s*:\s*\w+\([^\n]*\.array\(\)`)
)

// sqlFacts reads the body of a CREATE TABLE: a key, references, and columns of JSON and lists.
func (t *Table) sqlFacts(body string) {
	t.Defined = true
	t.alterFacts(body)
	for _, def := range topLevel(body) {
		fields := strings.Fields(def)
		if len(fields) < 2 {
			continue
		}
		name := strings.Trim(fields[0], "\"`[]")
		switch typ := strings.ToLower(fields[1]); {
		case jsonType.MatchString(typ):
			t.JSON = add(t.JSON, name)
		case strings.HasSuffix(typ, "[]") || strings.HasPrefix(typ, "array"):
			t.Lists = add(t.Lists, name)
		}
	}
}

// alterFacts reads a key or a reference in SQL that makes or changes the table.
func (t *Table) alterFacts(sql string) {
	t.Key = t.Key || primaryKey.MatchString(sql)
	t.References = add(t.References, firsts(sqlRefers, sql)...)
}

// prismaFacts reads a Prisma model's body: a key is @id or @@id, a reference a field of another
// model (tables, by its table) with fields of its own, a Json field holds JSON, and a list of
// plain values a list.
func (t *Table) prismaFacts(body string, tables map[string]string) {
	t.Defined, t.Key = true, t.Key || strings.Contains(body, "@id")
	for _, m := range firsts(prismaRel, body) {
		if tables[m] != "" {
			t.References = add(t.References, tables[m])
		}
	}
	t.JSON = add(t.JSON, firsts(prismaJSON, body)...)
	t.Lists = add(t.Lists, firsts(prismaList, body)...)
}

// pyFacts reads the body of a SQLAlchemy or SQLModel class.
func (t *Table) pyFacts(body string) {
	t.Defined, t.Key = true, t.Key || pyKey.MatchString(body)
	t.References = add(t.References, firsts(pyRefers, body)...)
	t.JSON = add(t.JSON, firsts(pyJSON, body)...)
	t.Lists = add(t.Lists, firsts(pyList, body)...)
}

// drizzleVars are the tables Drizzle's variables in t hold, by variable, when t has any of its
// tables (n).
func drizzleVars(t string, n int) map[string]string {
	vars := map[string]string{}
	if n > 0 {
		for _, m := range drizzleVar.FindAllStringSubmatch(t, -1) {
			vars[m[1]] = m[2]
		}
	}
	return vars
}

// drizzleFacts reads a Drizzle table: def is all of it, its extra argument with a key included,
// and body its columns, whose references name the other table's variable.
func (t *Table) drizzleFacts(def, body string, vars map[string]string) {
	t.Defined, t.Key = true, t.Key || drizzleKey.MatchString(def)
	for _, v := range firsts(drizzleRefs, body) {
		t.References = add(t.References, cmp.Or(vars[v], v))
	}
	t.JSON = add(t.JSON, firsts(drizzleJSON, body)...)
	t.Lists = add(t.Lists, firsts(drizzleList, body)...)
}

// merge takes o's columns, and what its definitions said, into t.
func (t *Table) merge(o Table) {
	t.Columns = add(t.Columns, o.Columns...)
	t.Defined, t.Key = t.Defined || o.Defined, t.Key || o.Key
	t.References, t.JSON, t.Lists = add(t.References, o.References...), add(t.JSON, o.JSON...), add(t.Lists, o.Lists...)
}

// lineHash is a line's FNV-1a hash with its runs of spaces and tabs as one space, and false
// for a line too short, a comment or an import, which never count as repeated.
func lineHash(l string) (uint64, bool) {
	l = strings.TrimSpace(l)
	if len(l) < 8 || slices.ContainsFunc([]string{"//", "#", "*", "/*", "import ", "from ", "} from "}, func(s string) bool { return strings.HasPrefix(l, s) }) {
		return 0, false
	}
	h, space := uint64(14695981039346656037), false
	for i := 0; i < len(l); i++ {
		c := l[i]
		if c == ' ' || c == '\t' {
			space = true
			continue
		}
		if space {
			h = (h ^ ' ') * 1099511628211
			space = false
		}
		h = (h ^ uint64(c)) * 1099511628211
	}
	return h, true
}

// repeats finds the runs of repeatAt or more lines the app's own code has in more than one
// file, leaving out pages, tests and config.
func (p *project) repeats() {
	var files []string
	var windows [][]uint64 // each file's run of repeatAt lines from each line on
	first, shared := map[uint64]int{}, map[uint64]bool{}
	for _, n := range p.code {
		if isHTML(n) || notRuntime.MatchString(n) {
			continue
		}
		var hs []uint64
		for _, l := range strings.Split(p.text[n], "\n") {
			if h, ok := lineHash(l); ok {
				hs = append(hs, h)
			}
		}
		f := len(files)
		var ws []uint64
		for i := 0; i+repeatAt <= len(hs); i++ {
			var w uint64
			for _, h := range hs[i : i+repeatAt] {
				w = w*1099511628211 ^ h
			}
			if in, ok := first[w]; !ok {
				first[w] = f
			} else if in != f {
				shared[w] = true
			}
			ws = append(ws, w)
		}
		files, windows = append(files, n), append(windows, ws)
	}
	r := Repeated{}
	for f, ws := range windows {
		lines, covered := 0, 0 // covered: the lines before it are counted
		for i, w := range ws {
			if shared[w] {
				lines += i + repeatAt - max(i, covered)
				covered = i + repeatAt
			}
		}
		if lines > 0 {
			r.Lines += lines
			r.Files = add(r.Files, files[f])
		}
	}
	if r.Lines > 0 {
		r.Files = capped(&p.s, r.Files)
		p.s.Repeated = &r
	}
}

var oldCopy = compile(`(?i)^(old|backups?|bak|archived?|deprecated|unused|legacy|trash)$|[._ -](old|bak|backup|copy|orig)$|^(old|copy of)[ _-]|\(\d\)$`)

// leftovers are the folders and files whose names say they are old copies or backups: a folder
// counts once, not each file in it.
func (p *project) leftovers() {
	for _, n := range p.paths {
		parts := strings.Split(n, "/")
		for i, part := range parts {
			name := part
			if i == len(parts)-1 {
				name = strings.TrimSuffix(part, path.Ext(part))
			}
			if oldCopy.MatchString(name) || oldCopy.MatchString(part) {
				p.s.Leftovers = add(p.s.Leftovers, strings.Join(parts[:i+1], "/"))
				break
			}
		}
	}
	p.s.Leftovers = capped(&p.s, p.s.Leftovers)
}
