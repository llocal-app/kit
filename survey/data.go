package survey

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"path"
	"slices"
	"strings"

	"llocal.app/kit/recipe"
)

var (
	// envReads are the ways code reads a variable, one rule each: a pattern that starts with
	// its own literal is scanned many times faster than one with alternatives.
	envReads = []rule{
		compile(`process\.env\.([A-Z_][A-Z0-9_]*)`), compile(`process\.env\[\s*["']([A-Z_][A-Z0-9_]*)["']\s*\]`),
		compile(`import\.meta\.env\.([A-Z_][A-Z0-9_]*)`), compile(`Deno\.env\.get\(\s*["'](\w+)["']`),
		compile(`os\.environ(?:\[|\.get\()\s*["'](\w+)["']`), compile(`os\.getenv\(\s*["'](\w+)["']`),
		compile(`st\.secrets(?:\[|\.get\()\s*["'](\w+)["']`), compile(`env\(\s*["']([A-Z_][A-Z0-9_]*)["']\s*\)`),
		compile(`os\.(?:Getenv|LookupEnv)\(\s*"(\w+)"`),
	}
	envLine      = compile(`(?m)^[ \t]*(?:export[ \t]+)?([A-Za-z_][A-Za-z0-9_]*)[ \t]*=[ \t]*(.*?)[ \t]*$`)
	viteBuiltins = []string{"MODE", "DEV", "PROD", "SSR", "BASE_URL"}
)

func IsEnvFile(name string) bool {
	return strings.HasPrefix(path.Base(name), ".env") || strings.HasSuffix(name, ".streamlit/secrets.toml")
}

// isTemplate is a .env file of placeholders, like .env.example.
func isTemplate(name string) bool {
	return slices.ContainsFunc([]string{".example", ".sample", ".template", ".dist", ".defaults"}, func(s string) bool {
		return strings.HasSuffix(name, s)
	})
}

func (p *project) envs() {
	vars := map[string]*Env{}
	get := func(name, file string) *Env {
		e := vars[name]
		if e == nil {
			e = &Env{Name: name, Browser: recipe.PublicPrefix(name) != ""}
			vars[name] = e
		}
		e.Files = add(e.Files, file)
		return e
	}
	for _, n := range p.files(func(n string) bool { return IsCode(n) || path.Ext(n) == ".prisma" }) {
		for _, read := range envReads {
			for _, m := range read.FindAllStringSubmatch(p.text[n], -1) {
				if !slices.Contains(viteBuiltins, m[1]) {
					get(m[1], n)
				}
			}
		}
	}
	for _, n := range p.files(IsEnvFile) {
		for _, m := range envLine.FindAllStringSubmatch(p.text[n], -1) {
			e := get(m[1], n)
			if v := strings.Trim(m[2], `"'`); v != "" && !isTemplate(n) {
				e.Set = true
				e.Looks = cmp.Or(Looks(m[1], v), e.Looks)
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(vars)) {
		e := vars[name]
		e.Looks = cmp.Or(e.Looks, Looks(name, ""))
		p.s.Env = append(p.s.Env, *e)
	}
}

// Secret says whether a key that looks like this (Looks) must stay on the server.
func Secret(looks string) bool { return looks != "" && looks != "publishable" }

// Looks says what a key looks like from its value, or failing that its name: service_role,
// secret_key, private_key, publishable, or "" for nothing it knows.
func Looks(name, value string) string {
	has := func(prefixes ...string) bool {
		return slices.ContainsFunc(prefixes, func(s string) bool { return strings.HasPrefix(value, s) })
	}
	switch {
	case strings.Contains(value, "PRIVATE KEY-----"):
		return "private_key"
	case has("eyJ") && jwtRole(value) == "service_role", has("sb_secret_"):
		return "service_role"
	case has("eyJ") && jwtRole(value) == "anon", has("sb_publishable_", "pk_live_", "pk_test_"):
		return "publishable"
	case has("sk-", "sk_live_", "sk_test_", "rk_live_", "rk_test_", "whsec_"):
		return "secret_key"
	case value != "":
		return ""
	case strings.Contains(name, "SERVICE_ROLE"):
		return "service_role"
	case strings.Contains(name, "PRIVATE_KEY"):
		return "private_key"
	case strings.Contains(name, "ANON_KEY"), strings.Contains(name, "PUBLISHABLE"):
		return "publishable"
	case strings.Contains(name, "SECRET"):
		return "secret_key"
	}
	return ""
}

func jwtRole(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	var claims struct {
		Role string `json:"role"`
	}
	if err != nil || json.Unmarshal(b, &claims) != nil {
		return ""
	}
	return claims.Role
}

var keyLiteral = compile("[\"'`](eyJ[\\w-]{10,}\\.eyJ[\\w-]{10,}\\.[\\w-]*|sk-[\\w-]{16,}|[sr]k_(?:live|test)_\\w{8,}|" +
	"sb_(?:secret|publishable)_[\\w-]{8,}|pk_(?:live|test)_\\w{8,}|whsec_\\w{8,}|-----BEGIN [A-Z ]*PRIVATE KEY-----)")

// keys finds keys written into the code. Their values are not recorded.
func (p *project) keys() {
	for _, n := range p.code {
		for _, m := range keyLiteral.FindAllStringSubmatch(p.text[n], -1) {
			if l := Looks("", m[1]); l != "" {
				p.s.Keys = add(p.s.Keys, Key{File: n, Looks: l, Browser: p.browser[n]})
			}
		}
	}
}

var (
	dbURL      = compile("\\b((?:postgres(?:ql)?|mysql|mariadb|mssql|sqlserver|mongodb|rediss?)(?:\\+[a-z0-9]+)?)://([^\\s\"'`<>]+)")
	supaRef    = compile(`\b([a-z0-9]{12,40})\.supabase\.co\b`)
	projectID  = compile(`(?m)^\s*project_id\s*=\s*"([a-z0-9]+)"`)
	odbcServer = compile(`(?i)\bSERVER\s*=\s*(?:tcp:)?([A-Za-z0-9_.-]+)`)
	fireHost   = compile(`\b([a-z0-9-]+\.firebaseapp\.com)\b`)
	prismaDB   = compile(`(?s)datasource\s+\w+\s*\{[^}]*?provider\s*=\s*"(\w+)"`)
	prismaKind = map[string]string{"postgresql": "postgres", "cockroachdb": "postgres", "mysql": "mysql", "sqlserver": "mssql", "mongodb": "mongodb", "sqlite": "sqlite"}
)

// stores finds the databases: from the packages that talk to them, and their addresses
// from connection strings in the code and .env files.
func (p *project) stores() {
	files := map[string][]string{}
	var kinds []string
	use := func(kind, file string) {
		kinds = add(kinds, kind)
		files[kind] = add(files[kind], file)
	}
	for _, n := range p.code {
		for _, imp := range p.imports[n] {
			if k := storeDeps[norm(imp)]; k != "" {
				use(k, n)
			}
		}
	}
	for _, m := range slices.Sorted(maps.Keys(p.deps)) {
		for _, d := range p.deps[m] {
			if k := storeDeps[d]; k != "" && len(files[k]) == 0 {
				use(k, m)
			}
		}
	}
	for _, n := range p.files(func(n string) bool { return path.Ext(n) == ".prisma" }) {
		if m := prismaDB.FindStringSubmatch(p.text[n]); m != nil && prismaKind[m[1]] != "" {
			use(prismaKind[m[1]], n)
		}
	}
	hosts := map[string][]string{}
	host := func(kind, h string) {
		kinds = add(kinds, kind)
		hosts[kind] = add(hosts[kind], h)
	}
	for _, n := range p.files(func(n string) bool { return IsCode(n) || IsEnvFile(n) || path.Ext(n) == ".toml" }) {
		t := p.text[n]
		for _, m := range dbURL.FindAllStringSubmatch(t, -1) {
			scheme, _, _ := strings.Cut(m[1], "+")
			if h := urlHost(m[2]); h != "" && !strings.Contains(h, ".supabase.") {
				host(schemeKinds[scheme], h)
			}
		}
		for _, m := range supaRef.FindAllStringSubmatch(t, -1) {
			host("supabase", m[1]+".supabase.co")
		}
		if n == "supabase/config.toml" {
			for _, m := range projectID.FindAllStringSubmatch(t, -1) {
				host("supabase", m[1]+".supabase.co")
			}
		}
		for _, h := range odbcHosts(t) {
			if h = urlHost(h); h != "" {
				host("mssql", h)
			}
		}
		for _, m := range fireHost.FindAllStringSubmatch(t, -1) {
			host("firebase", m[1])
		}
	}
	for _, k := range kinds {
		if len(hosts[k]) == 0 {
			hosts[k] = []string{""}
		}
		for _, h := range hosts[k] {
			p.s.DataStores = append(p.s.DataStores, DataStore{Kind: k, Host: h, Files: files[k], Tables: []Table{}})
		}
	}
}

// odbcHosts are the servers named in ODBC connection strings, like SQL Server's.
func odbcHosts(t string) []string {
	var hosts []string
	if strings.Contains(t, "DRIVER=") || strings.Contains(t, "Driver=") || strings.Contains(t, "driver=") {
		for _, m := range odbcServer.FindAllStringSubmatch(t, -1) {
			hosts = append(hosts, m[1])
		}
	}
	return hosts
}

// urlHost is the host[:port] at the start of what follows "scheme://", or "" for a
// placeholder like HOST or ${DB_HOST}.
func urlHost(rest string) string {
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	h, _, _ := strings.Cut(rest, ",")
	name, _, _ := strings.Cut(h, ":")
	lower := strings.ToLower(name)
	if lower != "localhost" && (!strings.Contains(name, ".") || strings.ContainsAny(name, "${}<>") ||
		strings.Contains(lower, "example") || strings.Contains(lower, "your")) {
		return ""
	}
	return h
}

// store is the first data store of kind, or for kind "" the store that SQL tables belong
// to.
func (p *project) store(kind string) *DataStore {
	kinds := []string{kind}
	if kind == "" {
		kinds = sqlKinds
	}
	for _, k := range kinds {
		if i := slices.IndexFunc(p.s.DataStores, func(d DataStore) bool { return d.Kind == k }); i >= 0 {
			return &p.s.DataStores[i]
		}
	}
	return nil
}

func (p *project) addTable(kind, name, file string, cols []string) {
	d := p.store(kind)
	if d == nil {
		return
	}
	i := slices.IndexFunc(d.Tables, func(t Table) bool { return strings.EqualFold(t.Name, name) })
	if i < 0 && len(d.Tables) > maxList {
		return
	}
	if i < 0 {
		d.Tables = append(d.Tables, Table{Name: name, File: file})
		i = len(d.Tables) - 1
	}
	d.Tables[i].Columns = add(d.Tables[i].Columns, cols...)
}

var (
	sqlComment   = compile(`(?s)/\*.*?\*/|--[^\n]*`)
	createTable  = compile("(?i)\\bcreate\\s+table\\s+(?:if\\s+not\\s+exists\\s+)?(?:[\"`\\[]?(\\w+)[\"`\\]]?\\.)?[\"`\\[]?(\\w+)[\"`\\]]?\\s*\\(")
	alterAdd     = compile("(?i)\\balter\\s+table\\s+(?:if\\s+exists\\s+)?(?:only\\s+)?(?:[\"`]?(\\w+)[\"`]?\\.)?[\"`]?(\\w+)[\"`]?\\s+add\\s+(?:column\\s+)?(?:if\\s+not\\s+exists\\s+)?[\"`]?(\\w+)")
	otherSchemas = []string{"auth", "storage", "cron", "extensions", "realtime", "vault", "graphql", "net"}
	notColumns   = []string{"constraint", "primary", "foreign", "unique", "check", "exclude", "key", "index", "like"}
	identifier   = compile(`^[A-Za-z_]\w*$`)
	drizzleTable = compile(`\b(pg|mysql|sqlite)Table\(\s*["'](\w+)["']\s*,\s*\{`)
	drizzleKind  = map[string]string{"pg": "postgres", "mysql": "mysql", "sqlite": "sqlite"}
	drizzleCol   = compile(`(?m)^\s*(\w+)\s*:\s*\w+\(\s*(?:["'](\w+)["'])?`)
	prismaModel  = compile(`(?m)^model\s+(\w+)\s*\{`)
	prismaMap    = compile(`@@?map\(\s*(?:name:\s*)?"(\w+)"`)
	prismaField  = compile(`^\s*(\w+)\s+(\w+)`)
	pyClass      = compile(`(?m)^class\s+(\w+)\s*\(([^)]*)\)\s*:`)
	pyTableName  = compile(`__tablename__\s*=\s*["'](\w+)["']`)
	pyColumn     = compile(`(?m)^[ \t]+(\w+)\s*(?::[^=\n]*)?=\s*(?:\w+\.)?(?:Column|mapped_column|Field)\(`)
	supaTypes    = compile(`(\w+):\s*\{\s*Row:\s*\{([^}]*)\}`)
	typeKey      = compile(`(?m)^\s*(\w+)\??\s*:`)
	supaFrom     = compile(`\.from\(\s*["'](\w+)["']\s*\)\s*\.\s*(?:select|insert|update|upsert|delete)\b`)
	collection   = compile(`\bcollection\(\s*\w+\s*,\s*["']([\w-]+)["']|\.collection\(\s*["']([\w-]+)["']`)
	mongoModel   = compile(`\bmodel\(\s*["'](\w+)["']\s*,`)
	camelHump    = compile(`([a-z0-9])([A-Z])`)
)

// tables finds each store's tables: definitions first (SQL, Drizzle, Prisma, SQLAlchemy,
// Supabase's generated types), then names used in queries.
func (p *project) tables() {
	for _, n := range p.files(func(n string) bool {
		return IsCode(n) || slices.Contains([]string{".sql", ".prisma"}, path.Ext(n))
	}) {
		t := p.text[n]
		kind := ""
		if strings.HasPrefix(n, "supabase/") {
			kind = "supabase"
		}
		if path.Ext(n) == ".sql" {
			t = sqlComment.ReplaceAllString(t, "")
		}
		for _, loc := range createTable.FindAllStringSubmatchIndex(t, -1) {
			schema, name := sub(t, loc, 1), sub(t, loc, 2)
			if !slices.Contains(otherSchemas, strings.ToLower(schema)) {
				p.addTable(kind, name, n, sqlColumns(balanced(t[loc[1]-1:], '(', ')')))
			}
		}
		for _, m := range alterAdd.FindAllStringSubmatch(t, -1) {
			if !slices.Contains(otherSchemas, strings.ToLower(m[1])) && !slices.Contains(notColumns, strings.ToLower(m[3])) {
				p.addTable(kind, m[2], n, []string{m[3]})
			}
		}
		for _, loc := range drizzleTable.FindAllStringSubmatchIndex(t, -1) {
			var cols []string
			for _, c := range drizzleCol.FindAllStringSubmatch(balanced(t[loc[1]-1:], '{', '}'), -1) {
				cols = append(cols, cmp.Or(c[2], c[1]))
			}
			p.addTable(drizzleKind[sub(t, loc, 1)], sub(t, loc, 2), n, cols)
		}
		if path.Ext(n) == ".prisma" {
			p.prismaTables(n, t)
		}
		if isPython(n) {
			p.pyTables(n, t)
		}
	}
	for _, n := range p.code {
		t := p.text[n]
		for _, block := range strings.Split(t, "Tables: {")[1:] {
			block, _, _ = strings.Cut(block, "Views: {")
			for _, m := range supaTypes.FindAllStringSubmatch(block, -1) {
				var cols []string
				for _, c := range typeKey.FindAllStringSubmatch(m[2], -1) {
					cols = append(cols, c[1])
				}
				p.addTable("supabase", m[1], n, cols)
			}
		}
	}
	for _, n := range p.code {
		t := p.text[n]
		for _, m := range supaFrom.FindAllStringSubmatch(t, -1) {
			p.addTable("supabase", m[1], n, nil)
		}
		for _, m := range collection.FindAllStringSubmatch(t, -1) {
			p.addTable("firebase", m[1]+m[2], n, nil)
		}
		for _, m := range mongoModel.FindAllStringSubmatch(t, -1) {
			p.addTable("mongodb", m[1], n, nil)
		}
	}
}

func sub(s string, loc []int, i int) string {
	if loc[2*i] < 0 {
		return ""
	}
	return s[loc[2*i]:loc[2*i+1]]
}

// balanced is what lies between s's first byte, open, and its matching close.
func balanced(s string, open, close byte) string {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case open:
			depth++
		case close:
			if depth--; depth == 0 {
				return s[1:i]
			}
		}
	}
	return ""
}

func sqlColumns(body string) []string {
	var cols []string
	depth, start := 0, 0
	for i := 0; i <= len(body); i++ {
		if i < len(body) {
			switch body[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if body[i] != ',' || depth > 0 {
				continue
			}
		}
		f := strings.Fields(body[start:i])
		start = i + 1
		if len(f) > 0 {
			name := strings.Trim(f[0], "\"`[]")
			if identifier.MatchString(name) && !slices.Contains(notColumns, strings.ToLower(name)) {
				cols = append(cols, name)
			}
		}
	}
	return cols
}

func (p *project) prismaTables(n, t string) {
	locs := prismaModel.FindAllStringSubmatchIndex(t, -1)
	models := map[string]bool{}
	for _, loc := range locs {
		models[sub(t, loc, 1)] = true
	}
	kind := ""
	if m := prismaDB.FindStringSubmatch(t); m != nil {
		kind = prismaKind[m[1]]
	}
	for _, loc := range locs {
		name := sub(t, loc, 1)
		var cols []string
		for _, line := range strings.Split(balanced(t[loc[1]-1:], '{', '}'), "\n") {
			f, mapped := prismaField.FindStringSubmatch(line), prismaMap.FindStringSubmatch(line)
			switch {
			case strings.HasPrefix(strings.TrimSpace(line), "@@map") && mapped != nil:
				name = mapped[1]
			case f == nil || models[f[2]]:
			case mapped != nil:
				cols = append(cols, mapped[1])
			default:
				cols = append(cols, f[1])
			}
		}
		p.addTable(kind, name, n, cols)
	}
}

// pyTables reads SQLAlchemy and SQLModel classes. Flask-SQLAlchemy names a table after its
// class in snake case unless __tablename__ says otherwise.
func (p *project) pyTables(n, t string) {
	locs := pyClass.FindAllStringSubmatchIndex(t, -1)
	for i, loc := range locs {
		bases := sub(t, loc, 2)
		table := strings.Contains(bases, "table=True") ||
			!strings.Contains(bases, "BaseModel") && (strings.Contains(bases, "Model") || strings.Contains(bases, "Base"))
		if !table {
			continue
		}
		end := len(t)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		body := t[loc[1]:end]
		var cols []string
		for _, c := range pyColumn.FindAllStringSubmatch(body, -1) {
			cols = append(cols, c[1])
		}
		if len(cols) == 0 {
			continue
		}
		name := strings.ToLower(camelHump.ReplaceAllString(sub(t, loc, 1), "${1}_${2}"))
		if m := pyTableName.FindStringSubmatch(body); m != nil {
			name = m[1]
		}
		p.addTable("", name, n, cols)
	}
}

var logins = []struct {
	provider string
	re       rule
}{
	{"replit", compile(`replit\.com/oidc|\bREPLIT_DOMAINS\b|X-Replit-User|@replit/repl-auth`)},
	{"netlify-identity", compile(`netlify-identity-widget|gotrue-js`)},
	{"lovable", compile(`@lovable\.dev/cloud-auth-js|\blovable\.auth\.`)},
	{"clerk", compile(`["']@clerk/`)},
	{"auth0", compile(`["']@auth0/`)},
	{"better-auth", compile(`["']better-auth\b`)},
	{"nextauth", compile(`["']next-auth\b|\bNextAuth\(|["']@auth/`)},
	{"supabase", compile(`\.auth\s*\.\s*(signInWithPassword|signUp|signInWithOtp|signInWithOAuth|signInWithIdToken|resetPasswordForEmail)\b`)},
	{"firebase", compile(`\b(signInWithEmailAndPassword|createUserWithEmailAndPassword|signInWithPopup|signInWithRedirect)\b`)},
	{"flask-login", compile(`\bflask_login\b|\bLoginManager\(`)},
	{"fastapi", compile(`\bOAuth2PasswordBearer\b|\bfastapi_users\b`)},
	{"streamlit", compile(`\bst\.login\(|\bstreamlit_authenticator\b`)},
	{"passport", compile(`\bpassport\.(use|authenticate)\(`)},
	{"own", compile(`type\s*=\s*\{?["']password["']|\bbcrypt\b|\bcheck_password_hash\b`)},
}

var authPath = compile(`(?i)replit_?auth|replit-auth|replit_integrations/auth`)

func (p *project) login() {
	var l *Login
	for _, lg := range logins {
		for _, n := range p.code {
			if lg.re.MatchString(p.text[n]) || lg.provider == "replit" && authPath.MatchString(n) {
				if l == nil {
					l = &Login{Provider: lg.provider}
				}
				l.Files = add(l.Files, n)
			}
		}
	}
	if l == nil {
		return
	}
	if l.Provider == "supabase" || l.Provider == "lovable" {
		l.UserTable = "auth.users"
	}
	for _, d := range p.s.DataStores {
		for _, t := range d.Tables {
			if l.UserTable == "" && slices.Contains([]string{"users", "user", "accounts", "account", "members", "profiles"}, strings.ToLower(t.Name)) {
				l.UserTable = t.Name
			}
		}
	}
	slices.Sort(l.Files)
	p.s.Login = l
}

// services finds the outside services: from the files that import them or name their
// hosts, failing that from a manifest that lists them or a variable named for them.
func (p *project) services() {
	manifests := slices.Sorted(maps.Keys(p.deps))
	for _, sv := range services {
		if sv.fallback && slices.ContainsFunc(p.s.Services, func(s Service) bool { return s.Kind == sv.kind }) {
			continue
		}
		uses := func(names []string) bool {
			return slices.ContainsFunc(names, func(d string) bool { return slices.Contains(sv.packages, norm(d)) || slices.Contains(sv.packages, d) })
		}
		var files []string
		for _, n := range p.code {
			if uses(p.imports[n]) || slices.ContainsFunc(sv.hosts, func(h string) bool { return strings.Contains(p.text[n], h) }) {
				files = append(files, n)
			}
		}
		if i := slices.IndexFunc(manifests, func(m string) bool { return uses(p.deps[m]) }); len(files) == 0 && i >= 0 {
			files = manifests[i : i+1]
		}
		if i := slices.IndexFunc(p.s.Env, func(e Env) bool { return sv.owns(e.Name) }); len(files) == 0 && i >= 0 {
			files = p.s.Env[i].Files
		}
		if len(files) == 0 {
			continue
		}
		browser := slices.ContainsFunc(files, func(n string) bool { return p.browser[n] }) ||
			slices.ContainsFunc(p.s.Env, func(e Env) bool { return e.Browser && sv.owns(e.Name) })
		p.s.Services = append(p.s.Services, Service{Name: sv.name, Kind: sv.kind, Browser: browser, Files: files})
	}
}

var (
	loginEnv  = compile(`^(SESSION_SECRET|AUTH_SECRET|AUTH_URL|AUTH_TRUST_HOST|JWT_SECRET|REPLIT_DOMAINS|REPL_ID|ISSUER_URL)$|NEXTAUTH|BETTER_AUTH|CLERK|AUTH0`)
	loginFile = compile(`(?i)auth|login|passport|session`)
	// platformEnv are set by llocal or only mean something on the old host. CRON_SECRET is
	// how Vercel and Lovable prove a timed call is theirs; llocal's own timetable replaces it.
	platformEnv = compile(`^(PORT|HOST|HOSTNAME|NODE_ENV|TZ|PATH|HOME|TMPDIR|CI|PYTHONPATH|PYTHONUNBUFFERED|NEXT_RUNTIME)$|` +
		`^(LLOCAL|VERCEL|NETLIFY|REPLIT|REPL|V0|BOLT|RAILWAY|RENDER|FLY)_|(^|_)CRON_SECRET$`)
	storeEnv = []struct {
		token, kind string
	}{{"SUPABASE", "supabase"}, {"FIREBASE", "firebase"}, {"MONGO", "mongodb"}, {"REDIS", "redis"}, {"KV_REST_API", "redis"},
		{"UPSTASH", "redis"}, {"MYSQL", "mysql"}, {"MSSQL", "mssql"}, {"SQLSERVER", "mssql"}}
	sqlEnv = compile(`DATABASE|(^|_)DB(_|$)|POSTGRES|^PG[A-Z]+$|^NEON_`)
)

// attribute ties each variable to what it is for, and each store to its variables.
func (p *project) attribute() {
	main := ""
	if d := p.store(""); d != nil {
		main = d.Kind
	}
	for i := range p.s.Env {
		p.s.Env[i].Service = p.owner(p.s.Env[i], main)
	}
	for i := range p.s.DataStores {
		d := &p.s.DataStores[i]
		for _, e := range p.s.Env {
			if e.Service == d.Kind {
				d.Env = append(d.Env, e.Name)
				d.Browser = d.Browser || e.Browser
			}
		}
		d.Browser = d.Browser || slices.ContainsFunc(d.Files, func(n string) bool { return p.browser[n] })
	}
}

// owner is what a variable belongs to, by the first rule that matches. main is the store SQL
// tables belong to.
func (p *project) owner(e Env, main string) string {
	for _, se := range storeEnv {
		if strings.Contains(e.Name, se.token) {
			return se.kind
		}
	}
	switch {
	case loginEnv.MatchString(e.Name):
		return "login"
	case platformEnv.MatchString(e.Name):
		return "platform"
	}
	for _, sv := range p.s.Services {
		if known(sv.Name).owns(e.Name) {
			return sv.Name
		}
	}
	switch {
	case sqlEnv.MatchString(e.Name):
		return main
	case p.s.Login != nil && onlyIn(e.Files, p.s.Login.Files):
		return "login"
	}
	return ""
}

// onlyIn says whether every code file in files is one of the login files in, and is named
// for logging in, like pages/api/auth/[...nextauth].ts. A login inside the main server
// file says nothing about the other variables that file reads.
func onlyIn(files, in []string) bool {
	code := slices.DeleteFunc(slices.Clone(files), IsEnvFile)
	return len(code) > 0 && !slices.ContainsFunc(code, func(f string) bool { return !slices.Contains(in, f) || !loginFile.MatchString(f) })
}

var (
	cronLib     = compile("\\b(?:cron\\.schedule|scheduleJob|new CronJob)\\(\\s*[\"'`]([^\"'`]+)[\"'`]")
	pgCron      = compile(`\bcron\.schedule\(\s*'[^']*'\s*,\s*'([^']+)'`)
	pyScheduler = compile(`\b(?:BackgroundScheduler|AsyncIOScheduler|BlockingScheduler)\(|\.add_job\(|\bschedule\.every\(|@\w+\.scheduled_job\(`)
	ghCron      = compile(`\bcron:\s*["']([^"']+)["']`)
)

func (p *project) schedules() {
	for _, n := range p.paths {
		t, ok := p.text[n]
		if !ok {
			continue
		}
		put := func(cron, pth string) { p.s.Schedules = add(p.s.Schedules, Schedule{File: n, Cron: cron, Path: pth}) }
		switch {
		case path.Ext(n) == ".sql":
			for _, m := range pgCron.FindAllStringSubmatch(t, -1) {
				put(m[1], "")
			}
		case strings.HasPrefix(n, ".github/workflows/"):
			for _, m := range ghCron.FindAllStringSubmatch(t, -1) {
				put(m[1], "")
			}
		case path.Base(n) == "vercel.json":
			var v struct {
				Crons []struct{ Path, Schedule string }
			}
			if json.Unmarshal([]byte(t), &v) == nil {
				for _, c := range v.Crons {
					put(c.Schedule, c.Path)
				}
			}
		case IsCode(n) && !p.browser[n]:
			for _, m := range cronLib.FindAllStringSubmatch(t, -1) {
				put(m[1], "")
			}
			if strings.Contains(t, "setInterval(") || strings.Contains(t, "time.NewTicker(") || isPython(n) && pyScheduler.MatchString(t) {
				put("", "")
			}
		}
	}
}

var signature = compile(`\bconstructEvent(Async)?\(|\bconstruct_event\(|\bvalidateRequest\(|\bverifyWebhook\(|\bwebhooks\.verify\(`)

// webhooks are routes and functions outside services call: by name, or by the sender's
// signature being checked in a file that holds nothing else.
func (p *project) webhooks() {
	service := func(name, file string) string {
		for _, s := range p.s.Services {
			if strings.Contains(strings.ToLower(name), s.Name) || slices.Contains(s.Files, file) {
				return s.Name
			}
		}
		return ""
	}
	paths, signed := map[string][]string{}, map[string]bool{}
	for _, r := range p.s.Routes {
		paths[r.File] = add(paths[r.File], r.Path)
	}
	for _, f := range p.code {
		if _, ok := paths[f]; ok || slices.ContainsFunc(p.s.Functions, func(fn Function) bool { return fn.File == f }) {
			signed[f] = signature.MatchString(p.text[f])
		}
	}
	for _, r := range p.s.Routes {
		if strings.Contains(strings.ToLower(r.Path), "webhook") || len(paths[r.File]) == 1 && signed[r.File] {
			p.s.Webhooks = add(p.s.Webhooks, Webhook{Path: r.Path, Service: service(r.Path, r.File), File: r.File})
		}
	}
	for _, f := range p.s.Functions {
		if strings.Contains(strings.ToLower(f.Name), "webhook") || signed[f.File] {
			p.s.Webhooks = add(p.s.Webhooks, Webhook{Service: service(f.Name, f.File), File: f.File})
		}
	}
}

var uploadKinds = []struct {
	kind string
	re   rule
}{
	{"supabase-storage", compile(`\.storage\s*\.from\([^)]*\)\s*\.upload\(`)},
	{"firebase-storage", compile(`\buploadBytes(Resumable)?\(`)},
	{"vercel-blob", compile(`["']@vercel/blob["']`)},
	{"s3", compile(`\bPutObjectCommand\b|\.upload_fileobj\(|\.put_object\(`)},
	{"replit-storage", compile(`["']@replit/object-storage["']`)},
	{"server", compile(`\bmulter\b|\bformidable\b|\bbusboy\b|express-fileupload|\brequest\.files\b|\bUploadFile\b|\bst\.file_uploader\(|\.FormFile\(`)},
	{"form", compile(`type\s*=\s*\{?["']file["']`)},
}

func (p *project) uploads() {
	for _, n := range p.code {
		for _, u := range uploadKinds {
			if u.re.MatchString(p.text[n]) {
				p.s.Uploads = add(p.s.Uploads, Upload{Kind: u.kind, File: n})
			}
		}
	}
}

var (
	anyURLHost = compile(`\b[a-z][a-z0-9+.-]*://(?:[^\s@/"'<>]*@)?([A-Za-z0-9.-]+)(:\d+)?`)
	bareIP     = compile(`\b(10\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3})\b`)
	notRuntime = compile(`(^|/)(tests?|e2e|__tests__|docs?)/|\.(test|spec)\.[jt]sx?$|(^|/)[\w-]+\.config\.[cm]?[jt]s$|(^|/)test_\w+\.py$|_test\.go$`)
	privateTLD = []string{".local", ".lan", ".internal", ".corp", ".home.arpa", ".localdomain"}
)

// private finds addresses on a private network in code the app runs, and localhost in
// code that runs in the browser. A server naming localhost means itself, which the checker
// covers; a database on localhost is in its store's host.
func (p *project) private() {
	for _, n := range p.files(func(n string) bool {
		return (IsCode(n) || IsEnvFile(n) && !isTemplate(n)) && !notRuntime.MatchString(n)
	}) {
		t := p.text[n]
		var found []string
		for _, m := range anyURLHost.FindAllStringSubmatch(t, -1) {
			if privateHost(m[1]) && (p.browser[n] || !loopback(m[1])) {
				found = append(found, m[1]+m[2])
			}
		}
		for _, m := range bareIP.FindAllStringSubmatch(t, -1) {
			found = append(found, m[1])
		}
		for _, h := range odbcHosts(t) {
			if privateHost(h) && !loopback(h) {
				found = append(found, h)
			}
		}
		for _, a := range found {
			if len(p.s.Private) <= maxList && !slices.ContainsFunc(p.s.Private, func(x Address) bool { return x.Address == a || strings.HasPrefix(x.Address, a+":") }) {
				p.s.Private = append(p.s.Private, Address{Address: a, File: n})
			}
		}
	}
}

// privateHost says whether a host name (without its port) is on localhost or a private
// network.
func privateHost(h string) bool {
	h = strings.ToLower(h)
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	return loopback(h) || slices.ContainsFunc(privateTLD, func(s string) bool { return strings.HasSuffix(h, s) })
}

func loopback(h string) bool {
	ip := net.ParseIP(h)
	return strings.EqualFold(h, "localhost") || ip != nil && ip.IsLoopback()
}

var (
	composeService = compile(`^([ \t]+)([\w.-]+):[ \t]*(#.*)?$`)
	systemInstall  = compile(`\b(?:apt-get|apt|apk|yum|dnf)\s+(?:install|add)\b([^\n&|;]*)`)
)

var composeNames = []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}

func isDockerfile(base string) bool {
	return strings.HasPrefix(base, "Dockerfile") || base == "Containerfile" || strings.HasSuffix(base, ".dockerfile")
}

func (p *project) containers() {
	for _, n := range p.paths {
		base := path.Base(n)
		switch {
		case isDockerfile(base):
			c := Container{File: n}
			for _, m := range systemInstall.FindAllStringSubmatch(strings.ReplaceAll(p.text[n], "\\\n", " "), -1) {
				for _, pkg := range strings.Fields(m[1]) {
					if !strings.HasPrefix(pkg, "-") {
						c.Installs = add(c.Installs, pkg)
					}
				}
			}
			p.s.Containers = append(p.s.Containers, c)
		case slices.Contains(composeNames, base):
			p.s.Containers = append(p.s.Containers, Container{File: n, Services: composeServices(p.text[n])})
		}
	}
}

// composeServices lists the keys directly under a compose file's services:.
func composeServices(text string) []string {
	var names []string
	in, indent := false, ""
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
		case !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t"):
			in = strings.HasPrefix(line, "services:")
		case in:
			if m := composeService.FindStringSubmatch(line); m != nil && (indent == "" || m[1] == indent) {
				indent = m[1]
				names = append(names, m[2])
			}
		}
	}
	return names
}

// unlisted finds the Python packages the code imports when there is no requirements file.
// With one, an import it leaves out is usually installed by a package it lists.
func (p *project) unlisted() {
	for m := range p.deps {
		if base := path.Base(m); base != "package.json" && base != "go.mod" {
			return
		}
	}
	local := map[string]bool{}
	for _, n := range p.paths {
		if isPython(n) {
			local[strings.TrimSuffix(path.Base(n), ".py")] = true
			for d := path.Dir(n); d != "."; d = path.Dir(d) {
				local[path.Base(d)] = true
			}
		}
	}
	for _, n := range p.files(isPython) {
		for _, mod := range p.imports[n] {
			if !pythonStdlib[mod] && !local[mod] && mod != "google" {
				p.s.UnlistedPackages = add(p.s.UnlistedPackages, norm(cmp.Or(pipNames[mod], mod)))
			}
		}
	}
	slices.Sort(p.s.UnlistedPackages)
}

// finish sorts the routes, caps each list so survey.json stays small, and leaves no
// required list null.
func (p *project) finish() {
	s := &p.s
	byPath := func(a, b Route) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), strings.Compare(a.Method, b.Method))
	}
	slices.SortFunc(s.Pages, byPath)
	slices.SortFunc(s.Routes, byPath)
	s.Languages = capped(s, s.Languages)
	s.Frameworks = capped(s, s.Frameworks)
	s.DataStores = capped(s, s.DataStores)
	s.Pages = capped(s, s.Pages)
	s.Routes = capped(s, s.Routes)
	s.Functions = capped(s, s.Functions)
	s.Env = capped(s, s.Env)
	s.Keys = capped(s, s.Keys)
	s.Schedules = capped(s, s.Schedules)
	s.Webhooks = capped(s, s.Webhooks)
	s.Uploads = capped(s, s.Uploads)
	s.Private = capped(s, s.Private)
	s.Containers = capped(s, s.Containers)
	s.UnlistedPackages = capped(s, s.UnlistedPackages)
	for i := range s.DataStores {
		s.DataStores[i].Tables = capped(s, s.DataStores[i].Tables)
		s.DataStores[i].Files = capped(s, s.DataStores[i].Files)
		s.DataStores[i].Env = capped(s, s.DataStores[i].Env)
	}
	for i := range s.Env {
		s.Env[i].Files = capped(s, s.Env[i].Files)
	}
	for i := range s.Services {
		s.Services[i].Files = capped(s, s.Services[i].Files)
	}
	if s.Login != nil {
		s.Login.Files = capped(s, s.Login.Files)
	}
}

// capped cuts list to maxList entries, saying so in Incomplete, and makes nil empty.
func capped[T any](s *Survey, list []T) []T {
	if len(list) > maxList {
		s.Incomplete = cmp.Or(s.Incomplete, fmt.Sprintf("it has more than %d of some things; each list stops at %d", maxList, maxList))
		return list[:maxList]
	}
	if list == nil {
		return []T{}
	}
	return list
}
