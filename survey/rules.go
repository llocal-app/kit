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
	expressRoute = compile("\\b(?:app|router|server|api|routes|\\w+Router)\\s*\\.\\s*(get|post|put|patch|delete|all|route)\\s*\\(\\s*[\"'`](/[^\"'`]*)[\"'`]")
	chainCall    = compile(`^\s*(?://[^\n]*\s*)*\.\s*(get|post|put|patch|delete|all)\s*\(`)
	expressParam = compile(`:(\w+)(?:\([^)]*\))?\??`)
	pyRoute      = compile(`@(\w+)\.(route|get|post|put|patch|delete|api_route)\(\s*f?["'](/[^"']*)["']([^)]*)\)`)
	pyRouter     = compile(`(\w+)\s*=\s*(?:APIRouter|Blueprint)\(([^)]*)\)`)
	pyPrefix     = compile(`\b(?:url_)?prefix\s*=\s*["']([^"']+)["']`)
	pyMethods    = compile(`methods\s*=\s*[\[(]([^\])]*)`)
	quotedWord   = compile(`["'](\w+)["']`)
	flaskParam   = compile(`<(?:\w+:)?(\w+)>`)
	typedParam   = compile(`\{(\w+):[^}]*\}`)
	goRoute      = compile(`\.(Get|Post|Put|Patch|Delete|GET|POST|PUT|PATCH|DELETE|HandleFunc|Handle)\(\s*"(?:(GET|POST|PUT|PATCH|DELETE) +)?(/[^"]*)"`)
)

func (p *project) routes() {
	next := p.s.fw("nextjs")
	mounted, pyMounted := p.mounts(), p.pyMounts()
	for _, n := range p.code {
		t := p.text[n]
		switch {
		case next && nextFile.MatchString(n):
			p.nextRoute(n, t)
		case isGo(n):
			for _, m := range goRoute.FindAllStringSubmatch(t, -1) {
				method := m[2]
				if method == "" && !strings.HasPrefix(m[1], "Handle") {
					method = strings.ToUpper(m[1])
				}
				if pth, ok := colonPath(strings.ReplaceAll(m[3], "...}", "}")); ok {
					p.route(n, method, pth)
				}
			}
		case isPython(n):
			p.pyRoutes(n, t, pyMounted[n])
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
				methods := []string{sub(t, loc, 1)}
				if methods[0] == "route" {
					// Read up to the next route, so an unclosed call cannot make the scan quadratic.
					end := len(t)
					if k+1 < len(locs) {
						end = locs[k+1][0]
					}
					methods = chained(t[loc[3]+strings.IndexByte(t[loc[3]:], '(') : end])
				}
				if pth, ok := colonPath(cmp.Or(strings.TrimSuffix(mounted[n]+sub(t, loc, 2), "/"), "/")); ok {
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

var (
	mount      = compile(`\.use\(\s*["'](/[^"']*)["']\s*,\s*(?:[\w.]+(?:\([^()]*\))?\s*,\s*)*(\w+)\s*\)`)
	importName = compile(`(?:\bimport\s+(\w+)\s+from|\b(?:const|let|var)\s+(\w+)\s*=\s*require\()\s*["'](\.[^"']*)["']`)
)

// mounts finds Express routers mounted under a prefix, app.use("/api", router), where the
// router comes from another of the app's files, and returns that file's prefix.
func (p *project) mounts() map[string]string {
	prefix := map[string]string{}
	for _, n := range p.code {
		if p.browser[n] || !expressFile.MatchString(p.text[n]) {
			continue
		}
		from := map[string]string{}
		for _, m := range importName.FindAllStringSubmatch(p.text[n], -1) {
			from[m[1]+m[2]] = path.Join(path.Dir(n), m[3])
		}
		for _, m := range mount.FindAllStringSubmatch(p.text[n], -1) {
			file := from[m[2]]
			if file == "" {
				continue
			}
			for _, f := range []string{"", ".ts", ".js", ".mjs", "/index.ts", "/index.js"} {
				if _, ok := p.text[file+f]; ok {
					prefix[file+f] = strings.TrimSuffix(m[1], "/")
				}
			}
		}
	}
	return prefix
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
	pyMount = compile(`\.(?:include_router|register_blueprint)\(\s*([\w.]+)\s*,[^)]*?\b(?:url_)?prefix\s*=\s*["']([^"']+)["']`)
	pyFrom  = compile(`(?m)^[ \t]*from[ \t]+([\w.]+)[ \t]+import[ \t]+([\w, \t]+)`)
)

// pyMounts finds FastAPI routers and Flask blueprints given a prefix where they are added,
// app.include_router(items.router, prefix="/api"), and returns the prefix of the file that
// defines them.
func (p *project) pyMounts() map[string]string {
	prefix := map[string]string{}
	for _, n := range p.files(isPython) {
		from := map[string]string{}
		for _, m := range pyFrom.FindAllStringSubmatch(p.text[n], -1) {
			for _, name := range strings.Split(m[2], ",") {
				from[strings.TrimSpace(name)] = strings.ReplaceAll(m[1], ".", "/")
			}
		}
		for _, m := range pyMount.FindAllStringSubmatch(p.text[n], -1) {
			name, _, _ := strings.Cut(m[1], ".")
			mod, ok := from[name]
			if !ok {
				continue
			}
			for _, base := range []string{path.Dir(n), "."} {
				for _, f := range []string{path.Join(base, mod, name+".py"), path.Join(base, mod+".py")} {
					if _, read := p.text[f]; read && prefix[f] == "" {
						prefix[f] = strings.TrimSuffix(m[2], "/")
					}
				}
			}
		}
	}
	return prefix
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

// pyRoutes reads Flask and FastAPI routes. A router's routes sit under its own prefix and
// the one it is added with (mount).
func (p *project) pyRoutes(n, t, mount string) {
	prefix := map[string]string{}
	for _, m := range pyRouter.FindAllStringSubmatch(t, -1) {
		prefix[m[1]] = mount
		if pm := pyPrefix.FindStringSubmatch(m[2]); pm != nil {
			prefix[m[1]] += strings.TrimSuffix(pm[1], "/")
		}
	}
	for _, m := range pyRoute.FindAllStringSubmatch(t, -1) {
		pth := prefix[m[1]] + m[3]
		if prefix[m[1]] != "" && m[3] == "/" {
			pth = prefix[m[1]]
		}
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
