package survey

import (
	"cmp"
	"encoding/json"
	"maps"
	"path"
	"slices"
	"strings"

	"llocal.app/kit/recipe"
)

func (p *project) survey() {
	if r, err := recipe.Parse([]byte(p.text["llocal.json"])); err == nil {
		p.s.RecipeShape = r.Shape
	}
	p.readImports()
	p.manifests()
	p.languages()
	p.frameworks()
	p.builder()
	p.classify()
	p.routes()
	p.functions()
	p.serverPages()
	p.serverCode()
	p.envs()
	p.keys()
	p.stores()
	p.tables()
	p.login()
	p.services()
	p.attribute()
	p.schedules()
	p.webhooks()
	p.uploads()
	p.private()
	p.containers()
	p.unlisted()
	p.facts()
	p.finish()
}

var (
	jsImport   = compile(`(?:\bfrom|\bimport|\brequire\s*\(|\bimport\s*\()\s*["']([^"'\n]+)["']`)
	pyImport   = compile(`(?m)^[ \t]*(?:from[ \t]+([A-Za-z_][\w.]*)[ \t]+import\b|import[ \t]+([A-Za-z_][\w.]*(?:[ \t]*,[ \t]*[A-Za-z_][\w.]*)*))`)
	cdnPrefix  = []string{"npm:", "jsr:", "https://esm.sh/", "https://cdn.skypack.dev/", "https://esm.run/", "https://cdn.jsdelivr.net/npm/", "https://unpkg.com/"}
	reqLine    = compile(`(?m)^[ \t]*([A-Za-z0-9][A-Za-z0-9._-]*)`)
	pyDepsList = compile(`\bdependencies\s*=\s*\[((?:[^\]"']|"[^"]*"|'[^']*')*)\]`)
	pyDepName  = compile(`["']([A-Za-z0-9][A-Za-z0-9._-]*)`)
	tomlTable  = compile(`(?m)^\[(tool\.poetry\.dependencies|packages|dev-packages)\][ \t]*\n((?:[^\[][^\n]*\n?)*)`)
	devList    = compile(`(?i)dev|test|lint|doc`)
	tomlKey    = compile(`(?m)^([A-Za-z0-9][A-Za-z0-9._-]*)[ \t]*=`)
	goRequire  = compile(`(?m)^[ \t]*(?:require[ \t]+)?([a-z0-9.-]+\.[a-z]+/?[\w./-]*)[ \t]+v\d`)
)

// readImports records the packages each code file imports: npm names, or Python's
// top-level modules as written. Go's come from go.mod.
func (p *project) readImports() {
	p.imports = map[string][]string{}
	for _, n := range p.code {
		if isGo(n) {
			continue
		}
		if isPython(n) {
			for _, m := range pyImport.FindAllStringSubmatch(p.text[n], -1) {
				for _, mod := range strings.Split(m[1]+m[2], ",") {
					top, _, _ := strings.Cut(strings.TrimSpace(mod), ".")
					p.imports[n] = add(p.imports[n], top)
				}
			}
			continue
		}
		for _, m := range jsImport.FindAllStringSubmatch(p.text[n], -1) {
			if pkg := jsPackage(m[1]); pkg != "" {
				p.imports[n] = add(p.imports[n], pkg)
			}
		}
	}
}

// jsPackage is the npm package an import names, or "" for the app's own files and Node's.
func jsPackage(spec string) string {
	for _, pre := range cdnPrefix {
		spec = strings.TrimPrefix(spec, pre)
	}
	if spec == "" || strings.ContainsAny(spec[:1], "./~#") || strings.HasPrefix(spec, "@/") || strings.Contains(spec, ":") {
		return ""
	}
	parts := strings.Split(spec, "/")
	name := parts[0]
	if strings.HasPrefix(name, "@") && len(parts) > 1 {
		name += "/" + parts[1]
	}
	if i := strings.LastIndex(name, "@"); i > 0 {
		name = name[:i]
	}
	return name
}

var normalizer = strings.NewReplacer("_", "-", ".", "-")

// norm makes Python package names comparable: Flask_Login and flask-login are one package.
func norm(name string) string { return normalizer.Replace(strings.ToLower(name)) }

// manifests reads each manifest's dependencies. They are not capped like the other lists,
// so a long requirements file is read whole; a name listed twice does no harm.
// The packages the app runs on (runtime) leave out its development tools: devDependencies, a
// module only another needs (// indirect), a requirements file named for development or
// tests, and Pipfile's dev-packages.
func (p *project) manifests() {
	p.deps, p.runtime = map[string][]string{}, map[string][]string{}
	for _, n := range p.paths {
		text, ok := p.text[n]
		if !ok {
			continue
		}
		var run, dev []string
		switch base := path.Base(n); {
		case base == "package.json":
			var pkg struct {
				Dependencies    map[string]string `json:"dependencies"`
				DevDependencies map[string]string `json:"devDependencies"`
			}
			if json.Unmarshal([]byte(text), &pkg) != nil {
				continue
			}
			run, dev = slices.Sorted(maps.Keys(pkg.Dependencies)), slices.Sorted(maps.Keys(pkg.DevDependencies))
		case base == "go.mod":
			for _, line := range strings.Split(text, "\n") {
				if m := goRequire.FindStringSubmatch(line); m != nil && strings.Contains(line, "// indirect") {
					dev = append(dev, m[1])
				} else if m != nil {
					run = append(run, m[1])
				}
			}
		case isRequirements(base) && devList.MatchString(base):
			dev = normed(firsts(reqLine, text))
		case isRequirements(base):
			run = normed(firsts(reqLine, text))
		case base == "pyproject.toml" || base == "Pipfile":
			for _, list := range firsts(pyDepsList, text) {
				run = append(run, normed(firsts(pyDepName, list))...)
			}
			for _, table := range tomlTable.FindAllStringSubmatch(text, -1) {
				keys := normed(slices.DeleteFunc(firsts(tomlKey, table[2]), func(k string) bool { return k == "python" }))
				if table[1] == "dev-packages" {
					dev = append(dev, keys...)
				} else {
					run = append(run, keys...)
				}
			}
		default:
			continue
		}
		p.deps[n] = append(slices.Clone(run), dev...)
		slices.Sort(run)
		p.runtime[n] = slices.Compact(run)
	}
}

// normed is names, each as norm makes it.
func normed(names []string) []string {
	for i := range names {
		names[i] = norm(names[i])
	}
	return names
}

// dep says whether any manifest lists one of names.
func (p *project) dep(names ...string) bool {
	for _, deps := range p.deps {
		if slices.ContainsFunc(deps, func(d string) bool { return slices.Contains(names, d) }) {
			return true
		}
	}
	return false
}

func (p *project) languages() {
	count := map[string]int{}
	for _, n := range p.paths {
		switch path.Ext(n) {
		case ".ts", ".tsx", ".mts", ".cts":
			count["typescript"]++
		case ".js", ".jsx", ".mjs", ".cjs":
			count["javascript"]++
		case ".py":
			count["python"]++
		case ".html", ".htm":
			count["html"]++
		case ".go":
			count["go"]++
		case ".php":
			count["php"]++
		case ".rb":
			count["ruby"]++
		case ".cs":
			count["csharp"]++
		case ".java":
			count["java"]++
		}
	}
	if len(count) > 1 {
		delete(count, "html")
	}
	p.s.Languages = slices.SortedFunc(maps.Keys(count), func(a, b string) int {
		return cmp.Or(count[b]-count[a], strings.Compare(a, b))
	})
}

// frameworks fills Frameworks and Servers: a manifest folder that lists a server framework
// is a server, and so is the folder of a Python file that imports one no manifest lists. A
// Go module is a Go server.
func (p *project) frameworks() {
	found := map[string]bool{}
	for _, m := range slices.Sorted(maps.Keys(p.deps)) {
		var here []string
		if path.Base(m) == "go.mod" {
			found["go"], here = true, []string{"go"}
		}
		for _, d := range p.deps[m] {
			if f := frameworkDeps[d]; f != "" {
				found[f], here = true, append(here, f)
			}
		}
		if i := slices.IndexFunc(serverFrameworks, func(f string) bool { return slices.Contains(here, f) }); i >= 0 {
			p.addServer(path.Dir(m), serverFrameworks[i])
		}
	}
	for _, n := range p.files(isPython) {
		for _, mod := range p.imports[n] {
			if f := frameworkDeps[norm(mod)]; f != "" && !found[f] {
				found[f] = true
				p.addServer(path.Dir(n), f)
			}
		}
	}
	if len(found) == 0 && slices.ContainsFunc(p.paths, isHTML) {
		found["html"] = true
	}
	p.s.Frameworks = slices.Sorted(maps.Keys(found))
}

// serverCode counts each server's own code files and their bytes. A file belongs to the
// server whose folder holds it most closely.
func (p *project) serverCode() {
	for _, n := range p.code {
		if p.browser[n] || isHTML(n) || notRuntime.MatchString(n) {
			continue
		}
		in, closest := -1, -1
		for i, sv := range p.s.Servers {
			if prefix := strings.TrimPrefix(sv.Dir+"/", "./"); strings.HasPrefix(n, prefix) && len(prefix) > closest {
				in, closest = i, len(prefix)
			}
		}
		if in >= 0 {
			p.s.Servers[in].Files++
			p.s.Servers[in].Bytes += int64(len(p.text[n]))
		}
	}
}

func (p *project) addServer(dir, framework string) {
	if !slices.ContainsFunc(p.s.Servers, func(s Server) bool { return s.Dir == dir }) && slices.Contains(serverFrameworks, framework) {
		p.s.Servers = append(p.s.Servers, Server{Dir: dir, Framework: framework})
	}
}

func (p *project) builder() {
	readme := p.text["README.md"] + p.text["readme.md"]
	prefixed := func(dir string) bool {
		return slices.ContainsFunc(p.paths, func(n string) bool { return strings.HasPrefix(n, dir) })
	}
	switch {
	case p.dep("lovable-tagger", "@lovable.dev/vite-tanstack-config", "@lovable.dev/cloud-auth-js") ||
		prefixed(".lovable/") || strings.Contains(readme, "lovable.dev/projects"):
		p.s.Builder = "lovable"
	case prefixed(".bolt/"):
		p.s.Builder = "bolt"
	case strings.Contains(p.text["package.json"], `"my-v0-project"`) || prefixed(".v0/") || strings.Contains(p.text[".gitignore"], "# v0 sandbox") ||
		strings.Contains(readme, "(https://v0.app") || strings.Contains(readme, "(https://v0.dev"):
		p.s.Builder = "v0"
	case slices.Contains(p.paths, ".replit") || slices.Contains(p.paths, "replit.md"):
		p.s.Builder = "replit"
	}
}

var (
	serverPath = compile(`(^|/)(server|api-server|backend|db)/|^(api|supabase|netlify|functions)/|(^|/)(app|pages|routes)/api/|(^|/)server\.[cm]?[jt]s$|\.(server|functions)\.[cm]?[jt]sx?$|(^|/)[\w-]+\.config\.[cm]?[jt]s$`)
	frontPath  = compile(`(^|/)(src|client|app|components|hooks|lib|pages|public|web)/`)
	clientPath = compile(`^(?:src/)?(client|public)/`)
	useClient  = compile(`(?m)^\s*["']use client["']`)
)

// classify decides which code files run in visitors' browsers. A client or public folder at
// the top does, even when the server draws its pages first.
func (p *project) classify() {
	spa := slices.ContainsFunc(spaFrameworks, p.s.fw)
	p.browser = map[string]bool{}
	for _, n := range p.code {
		t := p.text[n]
		switch {
		case isHTML(n):
			p.browser[n] = true
		case isPython(n) || isGo(n) || serverPath.MatchString(n) || strings.Contains(t, "createServerFn"):
		case useClient.MatchString(t) || strings.Contains(t, "import.meta.env.VITE_") || strings.Contains(t, "createBrowserClient"):
			p.browser[n] = true
		default:
			p.browser[n] = spa && frontPath.MatchString(n) || clientPath.MatchString(n)
		}
	}
}

func isHTML(name string) bool { return path.Ext(name) == ".html" || path.Ext(name) == ".htm" }

var (
	nextFile     = compile(`^(?:src/)?(app|pages)/(.*)\.(?:[jt]sx?|mdx)$`)
	nextMethod   = compile(`export\s+(?:async\s+)?(?:function|const)\s+(GET|POST|PUT|PATCH|DELETE)\b`)
	fileRoute    = compile("\\bcreate(File|ServerFile|APIFile)Route\\(\\s*[\"'`]([^\"'`]*)[\"'`]\\s*\\)")
	handlerKey   = compile(`\b(GET|POST|PUT|PATCH|DELETE)\s*:`)
	routerPage   = compile(`<Route\b[^>]*?\bpath=["']([^"']+)["']|\bpath\s*:\s*["'](/[^"']*)["']`)
	expressFile  = compile(`\b(express|fastify|hono|koa)\b|\bRouter\(\)`)
	expressRoute = compile("\\b(app|router|server|api|routes|\\w+Router)\\s*\\.\\s*(get|post|put|patch|delete|all|route)\\s*\\(\\s*[\"'`](/[^\"'`]*)[\"'`]")
	chainCall    = compile(`^\s*(?://[^\n]*\s*)*\.\s*(get|post|put|patch|delete|all)\s*\(`)
	expressParam = compile(`:(\w+)(?:\([^)]*\))?\??`)
	pyRoute      = compile(`@(\w+)\.(route|get|post|put|patch|delete|api_route)\(\s*f?["'](/[^"']*)["']([^)]*)\)`)
	pyRouter     = compile(`(\w+)\s*=\s*(?:APIRouter|Blueprint)\(([^)]*)\)`)
	pyPrefix     = compile(`\b(?:url_)?prefix\s*=\s*(?:(f?)["']([^"']*)["']|([A-Za-z_][\w.]*))`)
	pyMethods    = compile(`methods\s*=\s*[\[(]([^\])]*)`)
	quotedWord   = compile(`["'](\w+)["']`)
	flaskParam   = compile(`<(?:\w+:)?(\w+)>`)
	typedParam   = compile(`\{(\w+):[^}]*\}`)
	goRoute      = compile(`(\w*)\.(Get|Post|Put|Patch|Delete|GET|POST|PUT|PATCH|DELETE|HandleFunc|Handle)\(\s*(?:"(?:(GET|POST|PUT|PATCH|DELETE) +)?(/[^"]*)"|(\w+)\s*,)`)
	goLoop       = compile(`\bfor\s+_\s*,\s*(\w+)\s*:?=\s*range\s+\[\]string\s*\{([\s,]*(?:"[^"\n]*"[\s,]*)*)\}\s*\{`)
	goPath       = compile(`"(/[^"]*)"`)
	goGroup      = compile(`(\w+)\s*:?=\s*(\w+)\.(?:Group|PathPrefix)\(\s*"(/[^"]*)"`)
	goMount      = compile(`(\w+)\.Mount\(\s*"(/[^"]*)"\s*,\s*(\w+)\s*\)`)
	goBlock      = compile(`(\w*)\.Route\(\s*"(/[^"]*)"\s*,\s*func\s*\(\s*\w+\s+[\w.*]+\s*\)\s*\{`)
)

func (p *project) routes() {
	next := p.s.fw("nextjs")
	mounted, pyMounted := p.mounts(), p.pyPrefixes()
	for _, n := range p.code {
		t := p.text[n]
		switch {
		case next && nextFile.MatchString(n):
			p.nextRoute(n, t)
		case isGo(n):
			p.goRoutes(n, t)
		case isPython(n):
			p.pyRoutes(n, t, pyMounted)
		case fileRoute.MatchString(t):
			p.fileRoutes(n, t)
		case p.browser[n] && (strings.Contains(t, "react-router") || strings.Contains(t, "wouter")):
			for _, m := range routerPage.FindAllStringSubmatch(t, -1) {
				if pth, ok := colonPath(m[1] + m[2]); ok {
					p.page(n, "/"+strings.TrimPrefix(pth, "/"))
				}
			}
		case !p.browser[n] && expressFile.MatchString(t):
			locs := expressRoute.FindAllStringSubmatchIndex(t, -1)
			for k, loc := range locs {
				methods := []string{sub(t, loc, 2)}
				if methods[0] == "route" {
					// Read up to the next route, so an unclosed call cannot make the scan quadratic.
					end := len(t)
					if k+1 < len(locs) {
						end = locs[k+1][0]
					}
					methods = chained(t[loc[5]+strings.IndexByte(t[loc[5]:], '(') : end])
				}
				if pth, ok := colonPath(cmp.Or(strings.TrimSuffix(mounted.of(ref{n, sub(t, loc, 1)})+sub(t, loc, 3), "/"), "/")); ok {
					for _, method := range methods {
						if method == "all" {
							method = ""
						}
						p.route(n, strings.ToUpper(method), pth)
					}
				}
			}
		}
	}
	if len(p.s.Pages) > 0 {
		return
	}
	switch {
	case p.s.fw("html"):
		for _, n := range p.files(isHTML) {
			if path.Base(n) == "index.html" {
				p.page(n, strings.TrimSuffix("/"+path.Dir(n), ".")+"/")
			} else {
				p.page(n, "/"+n)
			}
		}
	case slices.ContainsFunc(spaFrameworks, p.s.fw) || slices.ContainsFunc(pageFrameworks, p.s.fw):
		p.page("", "/")
	}
}

// ref is a router: a variable in a file, or, with no name, the router a file exports.
type ref struct{ file, name string }

// mountPoint is where a router is added: to another router, under a prefix. A Flask
// blueprint registered with a prefix loses the one it was made with (replace).
type mountPoint struct {
	by      ref
	prefix  string
	replace bool
}

// prefixes are routers' own prefixes, APIRouter(prefix="/items"), where each is added, and
// each router's whole prefix once worked out.
type prefixes struct {
	own   map[ref]string
	under map[ref]mountPoint
	memo  map[ref]string
}

func newPrefixes() prefixes {
	return prefixes{own: map[ref]string{}, under: map[ref]mountPoint{}, memo: map[ref]string{}}
}

// maxPrefix is the longest prefix kept for a router, so a hostile file's routes stay short.
const maxPrefix = 1 << 10

// of is router r's whole prefix: its own, after those of the routers it is added to, in
// turn. A router not added itself is under whatever adds its file's export. A loop of
// routers stops at maxDepth.
func (x prefixes) of(r ref) string {
	p, ok := x.memo[r]
	if !ok {
		p = x.walk(r, 0)
		x.memo[r] = p
	}
	return p
}

func (x prefixes) walk(r ref, depth int) string {
	m, ok := x.under[r]
	if !ok {
		m, ok = x.under[ref{r.file, ""}]
	}
	own := x.own[r]
	if !ok || depth == maxDepth {
		return cut(own)
	}
	if m.replace {
		own = ""
	}
	return cut(x.walk(m.by, depth+1) + m.prefix + own)
}

func cut(prefix string) string { return prefix[:min(len(prefix), maxPrefix)] }

// join is a route's path under its router's prefix; the router's root is the prefix itself.
func join(prefix, pth string) string {
	if prefix != "" && pth == "/" {
		return prefix
	}
	return prefix + pth
}

var (
	mount      = compile(`\b(\w+)\s*\.use\(\s*["'](/[^"']*)["']\s*,\s*(?:[\w.]+(?:\([^()]*\))?\s*,\s*)*(\w+)\s*\)`)
	importName = compile(`(?:\bimport\s+(\w+)\s+from|\b(?:const|let|var)\s+(\w+)\s*=\s*require\()\s*["'](\.[^"']*)["']`)
)

// mounts finds Express routers added under a prefix, app.use("/api", router), whether the
// router is the export of another of the app's files or a variable in the same one, and
// routers added to routers.
func (p *project) mounts() prefixes {
	x := newPrefixes()
	for _, n := range p.code {
		if p.browser[n] || !expressFile.MatchString(p.text[n]) {
			continue
		}
		from := map[string]string{}
		for _, m := range importName.FindAllStringSubmatch(p.text[n], -1) {
			from[m[1]+m[2]] = path.Join(path.Dir(n), m[3])
		}
		for _, m := range mount.FindAllStringSubmatch(p.text[n], -1) {
			at := mountPoint{by: ref{n, m[1]}, prefix: strings.TrimSuffix(m[2], "/")}
			file, imported := from[m[3]]
			if !imported {
				x.under[ref{n, m[3]}] = at
				continue
			}
			for _, f := range []string{"", ".ts", ".js", ".mjs", "/index.ts", "/index.js"} {
				if _, ok := p.text[file+f]; ok {
					x.under[ref{file + f, ""}] = at
				}
			}
		}
	}
	return x
}

// colonPath writes a route's :name parameters as {name}, and says false for a path with a
// wildcard, which names no one address.
func colonPath(pth string) (string, bool) {
	return expressParam.ReplaceAllString(pth, "{$1}"), !strings.Contains(pth, "*")
}

// chained is the methods called one after another on the call that opens s, as in
// router.route("/x").get(...).post(...), past the comments between them.
func chained(s string) []string {
	var methods []string
	for {
		s = s[min(len(s), len(balanced(s, '(', ')'))+2):]
		m := chainCall.FindStringSubmatchIndex(s)
		if m == nil {
			return methods
		}
		methods = append(methods, s[m[2]:m[3]])
		s = s[m[1]-1:]
	}
}

var (
	pyMount       = compile(`(\w+)\s*\.\s*(include_router|register_blueprint)\(\s*([\w.]+)`)
	pyFrom        = compile(`(?m)^[ \t]*from[ \t]+([\w.]+)[ \t]+import[ \t]+(\([\w,\s]*\)|[^\n#]+)`)
	pyConst       = compile(`(?m)^[ \t]*(\w+)[ \t]*(?::[^=\n]*)?=[ \t]*["'](/[^"'\n]*)["']`)
	pyPlaceholder = compile(`\{([\w.]+)\}`)
)

// pyPrefixes finds the prefix of each FastAPI router and Flask blueprint: its own, under
// those it is added with along the chain, app.include_router(api_router,
// prefix=settings.API_V1_STR) then api_router.include_router(items.router). A prefix may be
// a constant set in any of the project's Python files, or an f-string of them.
func (p *project) pyPrefixes() prefixes {
	x := newPrefixes()
	files := p.files(isPython)
	consts := map[string]string{}
	// modules is each Python file by every end of its path, since imports start from a
	// folder inside the project; a file's whole path is always itself.
	modules := map[string]string{}
	for _, n := range files {
		for _, m := range pyConst.FindAllStringSubmatch(p.text[n], -1) {
			if _, ok := consts[m[1]]; !ok {
				consts[m[1]] = m[2]
			}
		}
		modules[n] = n
		for tail := n; strings.Contains(tail, "/"); {
			_, tail, _ = strings.Cut(tail, "/")
			if _, ok := modules[tail]; !ok {
				modules[tail] = n
			}
		}
	}
	constant := func(name string) string { return consts[name[strings.LastIndexByte(name, '.')+1:]] }
	prefix := func(args string) (string, bool) {
		m := pyPrefix.FindStringSubmatch(args)
		switch {
		case m == nil:
			return "", false
		case m[3] != "":
			m[2] = constant(m[3])
		case m[1] != "":
			m[2] = pyPlaceholder.ReplaceAllStringFunc(m[2], func(s string) string { return constant(s[1 : len(s)-1]) })
		}
		return strings.TrimSuffix(m[2], "/"), true
	}
	for _, n := range files {
		t := p.text[n]
		for _, m := range pyRouter.FindAllStringSubmatch(t, -1) {
			x.own[ref{n, m[1]}], _ = prefix(m[2])
		}
		from := pyImports(n, t)
		locs := pyMount.FindAllStringSubmatchIndex(t, -1)
		for k, loc := range locs {
			// Read up to the next call, so an unclosed one cannot make the scan quadratic.
			end := len(t)
			if k+1 < len(locs) {
				end = locs[k+1][0]
			}
			call := t[loc[0]:end]
			at, given := prefix(balanced(call[strings.IndexByte(call, '('):], '(', ')'))
			if r := pyRef(n, sub(t, loc, 3), from, modules); r.file != "" {
				x.under[r] = mountPoint{ref{n, sub(t, loc, 1)}, at, given && sub(t, loc, 2) == "register_blueprint"}
			}
		}
	}
	return x
}

// pyImports is where each name a Python file imports comes from: the module's path in
// slashes (a relative one from the file's folder), and the name there.
func pyImports(n, t string) map[string][2]string {
	from := map[string][2]string{}
	for _, m := range pyFrom.FindAllStringSubmatch(t, -1) {
		mod := strings.TrimLeft(m[1], ".")
		dir := ""
		if dots := len(m[1]) - len(mod); dots > 0 {
			dir = path.Dir(n)
			for range dots - 1 {
				dir = path.Dir(dir)
			}
		}
		mod = path.Join(dir, strings.ReplaceAll(mod, ".", "/"))
		for _, name := range strings.Split(strings.Trim(m[2], "() \t\r\n"), ",") {
			switch f := strings.Fields(name); len(f) {
			case 1:
				from[f[0]] = [2]string{mod, f[0]}
			case 3:
				from[f[2]] = [2]string{mod, f[0]}
			}
		}
	}
	return from
}

// pyRef is the router expr names in file n: a variable there, a name imported from a
// module, or one in a module imported by name (items.router).
func pyRef(n, expr string, from map[string][2]string, modules map[string]string) ref {
	head, attr, dotted := strings.Cut(expr, ".")
	imp, ok := from[head]
	switch {
	case !ok:
		return ref{n, expr}
	case dotted:
		return ref{pyModule(path.Join(imp[0], imp[1]), modules), attr}
	}
	return ref{pyModule(imp[0], modules), imp[1]}
}

func pyModule(mod string, modules map[string]string) string {
	for _, f := range []string{mod + ".py", mod + "/__init__.py"} {
		if n, ok := modules[f]; ok {
			return n
		}
	}
	return ""
}

// goRoutes reads a Go server's routes, under the prefixes of the chi Route blocks around
// them and of the routers they are added to: r.Route("/api", func(r chi.Router) {...}),
// api := r.Group("/api"), r.PathPrefix("/api").Subrouter(), r.Mount("/api", api). A route
// added in a loop over a list of paths, for _, p := range []string{...}, is each of them.
func (p *project) goRoutes(n, t string) {
	x := newPrefixes()
	for _, m := range goGroup.FindAllStringSubmatch(t, -1) {
		x.under[ref{n, m[1]}] = mountPoint{by: ref{n, m[2]}, prefix: strings.TrimSuffix(m[3], "/")}
	}
	for _, m := range goMount.FindAllStringSubmatch(t, -1) {
		x.under[ref{n, m[3]}] = mountPoint{by: ref{n, m[1]}, prefix: strings.TrimSuffix(m[2], "/")}
	}
	s := scopes{t: t, blocks: map[int]opened{}}
	for _, loc := range goBlock.FindAllStringSubmatchIndex(t, -1) {
		s.blocks[loc[1]-1] = opened{prefix: cut(x.of(ref{n, sub(t, loc, 1)}) + strings.TrimSuffix(sub(t, loc, 2), "/"))}
	}
	for _, loc := range goLoop.FindAllStringSubmatchIndex(t, -1) {
		paths := firsts(goPath, sub(t, loc, 2))
		s.blocks[loc[1]-1] = opened{name: sub(t, loc, 1), paths: paths[:min(len(paths), maxLoop)]}
	}
	for _, loc := range goRoute.FindAllStringSubmatchIndex(t, -1) {
		if len(p.s.Routes) > maxList {
			return
		}
		s.to(loc[0])
		method := sub(t, loc, 3)
		if f := sub(t, loc, 2); method == "" && !strings.HasPrefix(f, "Handle") {
			method = strings.ToUpper(f)
		}
		paths := []string{sub(t, loc, 4)}
		if v := sub(t, loc, 5); v != "" {
			paths = s.paths(v)
		}
		prefix := s.scope() + x.of(ref{n, sub(t, loc, 1)})
		for _, pth := range paths {
			if pth, ok := colonPath(join(prefix, strings.ReplaceAll(pth, "...}", "}"))); ok {
				p.route(n, method, pth)
			}
		}
	}
}

// maxLoop is the most paths a loop adds a route at, so a hostile file's loops stay quick.
const maxLoop = 8

// opened is what a brace opens: a Route block's prefix, or a loop's variable and its paths.
type opened struct {
	prefix string
	name   string
	paths  []string
}

// scopes walks Go code once, keeping what each brace still open opened, from its opening
// brace to its closing one. Braces in strings, runes and line comments don't count, and
// braces deeper than maxDepth open nothing.
type scopes struct {
	t      string
	blocks map[int]opened // by its opening brace
	i      int
	open   []opened
}

// scope is the prefixes of the Route blocks open where the walk is.
func (s *scopes) scope() string {
	var b strings.Builder
	for _, o := range s.open[:min(len(s.open), maxDepth)] {
		b.WriteString(o.prefix)
	}
	return b.String()
}

// paths is what the loop variable name takes where the walk is.
func (s *scopes) paths(name string) []string {
	for k := min(len(s.open), maxDepth) - 1; k >= 0; k-- {
		if s.open[k].name == name {
			return s.open[k].paths
		}
	}
	return nil
}

func (s *scopes) to(end int) {
	for ; s.i < end; s.i++ {
		switch c := s.t[s.i]; c {
		case '{':
			var o opened
			if len(s.open) < maxDepth {
				o = s.blocks[s.i]
			}
			s.open = append(s.open, o)
		case '}':
			if k := len(s.open) - 1; k >= 0 {
				s.open = s.open[:k]
			}
		case '"', '\'':
			for s.i++; s.i < len(s.t) && s.t[s.i] != c && s.t[s.i] != '\n'; s.i++ {
				if s.t[s.i] == '\\' {
					s.i++
				}
			}
		case '`':
			s.skipPast("`")
		case '/':
			if strings.HasPrefix(s.t[s.i:], "//") {
				s.skipPast("\n")
			}
		}
	}
}

// skipPast moves the walk to the next end, or to the end of the code.
func (s *scopes) skipPast(end string) {
	if k := strings.Index(s.t[s.i+1:], end); k >= 0 {
		s.i += k + 1
	} else {
		s.i = len(s.t)
	}
}

// page and route stop one past maxList, like add.
func (p *project) page(file, pth string) {
	pth = strings.ReplaceAll(pth, "//", "/")
	if len(p.s.Pages) <= maxList && !slices.ContainsFunc(p.s.Pages, func(r Route) bool { return r.Path == pth }) {
		p.s.Pages = append(p.s.Pages, Route{Path: pth, File: file})
	}
}

func (p *project) route(file, method, pth string) {
	r := Route{Method: method, Path: pth, File: file}
	if len(p.s.Routes) <= maxList && !slices.ContainsFunc(p.s.Routes, func(x Route) bool { return x.Method == r.Method && x.Path == r.Path }) {
		p.s.Routes = append(p.s.Routes, r)
	}
}

// nextRoute reads a page or route handler from its place under app/ or pages/.
func (p *project) nextRoute(n, t string) {
	m := nextFile.FindStringSubmatch(n)
	dir, base := path.Dir(m[2]), path.Base(m[2])
	if m[1] == "pages" {
		dir, base = m[2], "page"
		if slices.Contains([]string{"_app", "_document", "_error", "404", "500"}, m[2]) {
			return
		}
		if strings.HasPrefix(m[2], "api/") {
			base = "api"
		}
	}
	var segs []string
	for _, s := range strings.Split(dir, "/") {
		switch {
		case s == "." || s == "index" || s == "" || strings.HasPrefix(s, "@") || strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")"):
		case strings.HasPrefix(s, "_") || strings.HasPrefix(s, "("):
			return
		case strings.HasPrefix(s, "["):
			segs = append(segs, "{"+strings.Trim(s, "[].")+"}")
		default:
			segs = append(segs, s)
		}
	}
	pth := "/" + strings.Join(segs, "/")
	switch base {
	case "page":
		p.page(n, pth)
	case "api":
		p.route(n, "", pth)
	case "route":
		p.methodRoutes(n, pth, nextMethod.FindAllStringSubmatch(t, -1))
	}
}

// methodRoutes adds a route for each method a handler file exports, or one for any method.
func (p *project) methodRoutes(n, pth string, methods [][]string) {
	if len(methods) == 0 {
		p.route(n, "", pth)
	}
	for _, m := range methods {
		p.route(n, m[1], pth)
	}
}

// fileRoutes reads TanStack Router and Start routes: createFileRoute("/jobs/$id") is a
// page, unless it has server handlers, which make it a server route.
func (p *project) fileRoutes(n, t string) {
	for _, m := range fileRoute.FindAllStringSubmatch(t, -1) {
		if strings.HasPrefix(path.Base(m[2]), "_") {
			continue // a layout with no address of its own
		}
		var segs []string
		for _, s := range strings.Split(m[2], "/") {
			s = strings.TrimSuffix(s, "_")
			switch {
			case s == "" || strings.HasPrefix(s, "_") || strings.HasPrefix(s, "(") || strings.HasPrefix(s, "{-"):
			case s == "$":
				segs = append(segs, "{splat}")
			case strings.HasPrefix(s, "$"):
				segs = append(segs, "{"+s[1:]+"}")
			default:
				segs = append(segs, s)
			}
		}
		pth := "/" + strings.Join(segs, "/")
		if m[1] == "File" && !strings.Contains(t, "handlers") {
			p.page(n, pth)
		} else {
			p.methodRoutes(n, pth, handlerKey.FindAllStringSubmatch(t, -1))
		}
	}
}

// pyRoutes reads Flask and FastAPI routes, each under its router's whole prefix.
func (p *project) pyRoutes(n, t string, x prefixes) {
	for _, m := range pyRoute.FindAllStringSubmatch(t, -1) {
		pth := join(x.of(ref{n, m[1]}), m[3])
		pth = typedParam.ReplaceAllString(flaskParam.ReplaceAllString(pth, "{$1}"), "{$1}")
		methods := []string{strings.ToUpper(m[2])}
		if m[2] == "route" || m[2] == "api_route" {
			methods = []string{"GET"}
			if mm := pyMethods.FindStringSubmatch(m[4]); mm != nil {
				methods = nil
				for _, q := range quotedWord.FindAllStringSubmatch(mm[1], -1) {
					methods = append(methods, strings.ToUpper(q[1]))
				}
			}
		}
		for _, method := range methods {
			p.route(n, method, pth)
		}
	}
}

var (
	edgeFunction    = compile(`^supabase/functions/([^/_.][^/]*)/index\.[jt]s$`)
	netlifyFunction = compile(`^netlify/functions/([^/]+?)(?:/index)?\.[cm]?[jt]s$`)
	fileUseServer   = compile(`^(?:\s*//[^\n]*\n)*\s*["']use server["']`)
	exportedAsync   = compile(`export\s+async\s+function\s+(\w+)`)
	serverFn        = compile(`\b(?:const|let)\s+(\w+)\s*=\s*createServerFn\b`)
)

func (p *project) functions() {
	for _, n := range p.code {
		t := p.text[n]
		if m := edgeFunction.FindStringSubmatch(n); m != nil {
			p.s.Functions = add(p.s.Functions, Function{Name: m[1], Kind: "supabase-edge", File: n})
		}
		if m := netlifyFunction.FindStringSubmatch(n); m != nil {
			p.s.Functions = add(p.s.Functions, Function{Name: m[1], Kind: "netlify", File: n})
		}
		if fileUseServer.MatchString(t) {
			for _, m := range exportedAsync.FindAllStringSubmatch(t, -1) {
				p.s.Functions = add(p.s.Functions, Function{Name: m[1], Kind: "server-action", File: n})
			}
		}
		for _, m := range serverFn.FindAllStringSubmatch(t, -1) {
			p.s.Functions = add(p.s.Functions, Function{Name: m[1], Kind: "server-function", File: n})
		}
	}
}

var (
	nextServerOnly = compile(`["']use server["']|\bgetServerSideProps\b|from\s+["']next/headers["']|export\s+const\s+(dynamic\s*=\s*["']force-dynamic|revalidate\s*=)|\bcreateServerClient\(`)
	nextAsyncPage  = compile(`^(?:src/)?app/(?:.*/)?(?:page|layout)\.[jt]sx?$`)
	serverTemplate = compile(`\brender_template\(|\bJinja2Templates\(|\bres\.render\(`)
	rootMiddleware = compile(`^(?:src/)?(?:middleware|proxy)\.[jt]s$`)
)

// serverPages decides whether the server draws the pages, which keeps the server.
func (p *project) serverPages() {
	next := p.s.fw("nextjs")
	p.s.ServerPages = slices.ContainsFunc(pageFrameworks, p.s.fw) || slices.ContainsFunc(p.code, func(n string) bool {
		t := p.text[n]
		return serverTemplate.MatchString(t) || next && (nextServerOnly.MatchString(t) || rootMiddleware.MatchString(n) ||
			nextAsyncPage.MatchString(n) && strings.Contains(t, "export default async function"))
	})
}
