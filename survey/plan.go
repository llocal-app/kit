package survey

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"llocal.app/kit/recipe"
)

// Plan is the one screen a builder sees after the survey: what we found, what llocal will
// change and why, what stays outside, and what we need from them. It is plain text for the
// front end to lay out, written for an office manager rather than a developer.
type Plan struct {
	// Shape is static (nothing to convert), standard (converted to a Go server on the kit)
	// or kept (the app's own server stays and only the data moves).
	Shape string   `json:"shape"`
	Why   string   `json:"why"`
	Found []string `json:"found"`
	// Left is what stands between the app and the standard shape, empty when it fits.
	Left     []Left    `json:"left"`
	Changes  []Item    `json:"changes"`
	Outside  []Item    `json:"outside,omitempty"`
	Needs    []Need    `json:"needs,omitempty"`
	Blockers []Blocker `json:"blockers,omitempty"`
	Warnings []string  `json:"warnings,omitempty"`
	Settings []Setting `json:"settings,omitempty"`
	// Dropped are the settings llocal no longer needs once it has made its changes.
	Dropped []string `json:"dropped,omitempty"`
	// Tell is "Tell your other services about the new address".
	Tell []Tell `json:"tell,omitempty"`
	// IfNotConverted is for the result screen when the app goes live before it is
	// converted, while visitors' browsers still talk to its database.
	IfNotConverted string `json:"if_not_converted,omitempty"`
}

type Item struct {
	Text   string `json:"text"`
	Detail string `json:"detail,omitempty"`
}

// Need is something we need from the builder; Setting names it when it is a setting.
type Need struct {
	Text    string `json:"text"`
	Setting string `json:"setting,omitempty"`
}

// Blocker is named before anything is built, with its way out. Case is two_services,
// container, private_database, two_languages, host_login or unknown.
type Blocker struct {
	Case   string `json:"case"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	WayOut string `json:"way_out"`
}

// Setting is a value the builder gives llocal. A build_setting is public: the pages carry
// it, and changing it rebuilds the app. A secret stays on the server, sealed; changing it
// restarts the app.
type Setting struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Service string `json:"service,omitempty"`
	Note    string `json:"note,omitempty"`
	Warning string `json:"warning,omitempty"`
}

// Tell is one service to give the app's new address, the exact value, and where.
type Tell struct {
	Service string `json:"service"`
	Value   string `json:"value"`
	Where   string `json:"where"`
}

// maxPort is the number of server routes and functions above which a server is too big to
// rewrite safely, so it is kept.
const maxPort = 30

var (
	frameworkLabels = map[string]string{
		"nextjs": "Next.js", "tanstack-start": "TanStack Start", "remix": "Remix", "sveltekit": "SvelteKit",
		"nestjs": "NestJS", "express": "Express", "fastify": "Fastify", "hono": "Hono", "koa": "Koa",
		"django": "Django", "flask": "Flask", "fastapi": "FastAPI", "streamlit": "Streamlit",
		"gradio": "Gradio", "dash": "Dash", "go": "Go", "react": "React", "vue": "Vue", "svelte": "Svelte", "angular": "Angular",
	}
	storeLabels = map[string]string{
		"supabase": "Supabase", "postgres": "PostgreSQL", "mysql": "MySQL", "mssql": "SQL Server",
		"mongodb": "MongoDB", "firebase": "Firebase", "redis": "Redis", "sqlite": "SQLite",
	}
	builderLabels = map[string]string{"lovable": "Lovable", "bolt": "Bolt", "v0": "v0", "replit": "Replit"}
	loginLabels   = map[string]string{
		"supabase": "email and password through Supabase", "replit": "Replit's own login",
		"lovable": "Lovable Cloud's sign-in", "clerk": "Clerk", "auth0": "Auth0", "better-auth": "Better Auth",
		"nextauth": "NextAuth", "firebase": "Firebase's sign-in", "netlify-identity": "Netlify Identity",
		"passport": "email and password, checked by the app itself", "flask-login": "email and password, checked by the app itself",
		"fastapi": "email and password, checked by the app itself", "streamlit": "Streamlit's sign-in",
		"own": "email and password, checked by the app itself",
	}
	keyName = compile(`KEY|SECRET|TOKEN|PASSWORD|SID`)
	// hostLogins work only while the original host runs the app.
	hostLogins    = map[string]string{"replit": "Replit", "netlify-identity": "Netlify"}
	functionKinds = []struct{ kind, noun, where string }{
		{"supabase-edge", "function", " on Supabase"}, {"netlify", "function", " on Netlify"},
		{"server-action", "server action", ""}, {"server-function", "server function", ""},
	}
	uploadLabels = map[string]string{
		"supabase-storage": "kept in Supabase Storage", "firebase-storage": "kept in Firebase Storage",
		"vercel-blob": "kept in Vercel Blob", "s3": "kept in Amazon S3", "replit-storage": "kept in Replit's storage",
		"server": "kept by its server",
	}
	looksLabels = map[string]string{
		"service_role": "a Supabase service_role key, which can read and change every row",
		"secret_key":   "a secret key", "private_key": "a private key",
	}
	fast = "A Go server starts in a moment and needs little memory, so the app is quick and can sleep when nobody is using it."
)

// Plan makes the plan for the survey. url is the app's address on llocal and ip llocal's
// outbound address; either may be empty before they are known.
func (s Survey) Plan(url, ip string) Plan {
	url = cmp.Or(url, "https://<your app>.llocal.app")
	ip = cmp.Or(ip, "<llocal's outbound address>")
	p := Plan{Found: s.found(), Changes: []Item{}}
	p.Shape, p.Why = s.shape()
	p.settings(s)
	p.changes(s)
	p.outside(s)
	p.needs(s)
	p.blockers(s, ip)
	p.left(s)
	p.warnings(s)
	p.tell(s, url, ip)
	if stores := s.replaced(true); len(stores) > 0 {
		p.IfNotConverted = fmt.Sprintf("Until it is converted, the %s database is reached straight from each visitor's browser, "+
			"and llocal's sign-in does not stand in front of it: anyone who has its address and the key in the pages can try to read it.", stores)
	}
	return p
}

func (s Survey) shape() (string, string) {
	work, stores := len(s.Routes)+len(s.Functions), s.replaced(false)
	exposed := slices.ContainsFunc(s.Env, func(e Env) bool { return e.Browser && Secret(e.Looks) }) ||
		slices.ContainsFunc(s.Keys, func(k Key) bool { return k.Browser && Secret(k.Looks) })
	switch {
	case s.RecipeShape != "":
		return s.RecipeShape, "It already has llocal.json, which says how llocal builds and starts it, so it goes online as it is."
	case work == 0 && len(s.DataStores) == 0 && !s.ServerPages && !exposed:
		return "static", "It is only pages and needs no server of its own, so it goes online as it is."
	case len(s.Servers) == 0 && stores != "":
		return "standard", fmt.Sprintf("Its pages talk straight to %s, so llocal gives it a small server of its own, "+
			"written in Go, and keeps the data in SQLite.", stores)
	case len(s.Servers) == 0 && work > 0:
		return "standard", "Its pages call functions that run on another service, so llocal gives it a small server of its own, written in Go, to run them."
	case len(s.Servers) == 0 && exposed:
		return "standard", "Its pages hold a secret key, so llocal gives it a small server of its own, written in Go, to keep the key and make the calls that need it."
	case slices.ContainsFunc(s.Servers, func(sv Server) bool { return sv.Framework == "go" }):
		return "kept", "Its server is already written in Go, so it stays as it is and only the data moves."
	case s.ServerPages && slices.ContainsFunc(pageFrameworks, s.fw):
		return "kept", fmt.Sprintf("%s draws the pages in Python, so its server stays as it is and only the data moves.", s.serverLabel())
	case s.ServerPages:
		return "kept", "Its server draws the pages itself, so rewriting it would mean rewriting the app. The server stays as it is and only the data moves."
	case stores == "" && len(s.DataStores) > 0:
		return "kept", "It already keeps its data in SQLite, so its server stays as it is and the data file moves to llocal."
	case work > maxPort:
		return "kept", fmt.Sprintf("Its server has %d routes and functions, too many to rewrite safely, so it stays as it is and only the data moves.", work)
	}
	return "standard", fmt.Sprintf("Its %s server is small, so llocal rewrites it in Go on llocal's kit. %s", s.serverLabel(), fast)
}

func (s Survey) fw(name string) bool { return slices.Contains(s.Frameworks, name) }

// StoreLabel is a data store's kind as people know it, like PostgreSQL for postgres.
func StoreLabel(kind string) string { return cmp.Or(storeLabels[kind], kind) }

// PrivateHost says whether a host is on a private network or this computer, which llocal
// can't reach.
func PrivateHost(host string) bool { return privateHost(host) }

func (s Survey) serverLabel() string {
	return list(s.Servers, len(s.Servers), func(sv Server) string { return frameworkLabels[sv.Framework] })
}

// replaced names the databases that become SQLite, or only those the browser talks to.
func (s Survey) replaced(browserOnly bool) string {
	var names []string
	for _, d := range s.DataStores {
		if d.Kind != "sqlite" && (d.Browser || !browserOnly) {
			names = add(names, storeLabels[d.Kind])
		}
	}
	return joinAnd(names)
}

func (s Survey) serverList() string {
	return fmt.Sprintf("%d servers: %s", len(s.Servers), list(s.Servers, len(s.Servers), func(sv Server) string {
		return fmt.Sprintf("%s in %s", frameworkLabels[sv.Framework], sv.Dir)
	}))
}

func (s Survey) found() []string {
	var f []string
	what := "A project llocal could not identify"
	front := ""
	if i := slices.IndexFunc(frontFrameworks, s.fw); i >= 0 {
		front = frameworkLabels[frontFrameworks[i]]
	}
	switch {
	case len(s.Servers) > 1:
		what = "A project with " + s.serverList()
	case len(s.Servers) == 1 && front != "" && !slices.Contains(fullStackFrameworks, s.Servers[0].Framework):
		what = fmt.Sprintf("An app with its own %s server and %s front end", s.serverLabel(), aOrAn(front))
	case len(s.Servers) == 1:
		what = fmt.Sprintf("An app with its own %s server", s.serverLabel())
	case front != "":
		what = capital(aOrAn(front)) + " website"
	case s.fw("html") && len(s.Pages) == 1:
		what = "A single web page"
	case s.fw("html"):
		what = "A website of plain web pages"
	}
	if b := builderLabels[s.Builder]; b != "" {
		what += ", made with " + b
	}
	f = append(f, what+".")
	if len(s.Pages) > 0 {
		f = append(f, fmt.Sprintf("%s: %s.", count(len(s.Pages), "page"), list(s.Pages, 6, func(r Route) string { return r.Path })))
	}
	if len(s.Routes) > 0 {
		f = append(f, fmt.Sprintf("%s, like %s.", count(len(s.Routes), "server route"), list(s.Routes, 3, func(r Route) string {
			return strings.TrimSpace(r.Method + " " + r.Path)
		})))
	}
	for _, k := range functionKinds {
		if names := s.functionNames(k.kind); len(names) > 0 {
			f = append(f, fmt.Sprintf("%s%s: %s.", count(len(names), k.noun), k.where, list(names, 6, func(n string) string { return n })))
		}
	}
	for _, d := range s.DataStores {
		line := fmt.Sprintf("A %s database", storeLabels[d.Kind])
		if d.Kind == "sqlite" {
			line = "Its data is in SQLite already"
		}
		if d.Host != "" {
			line += " at " + d.Host
		}
		if len(d.Tables) > 0 {
			line += fmt.Sprintf(", with %s: %s", count(len(d.Tables), "table"), list(d.Tables, 8, func(t Table) string { return t.Name }))
		}
		line += "."
		if d.Browser {
			line += " Visitors' browsers talk to it directly."
		}
		f = append(f, line)
	}
	if l := s.Login; l != nil {
		line := "Its own login: " + loginLabels[l.Provider]
		if l.UserTable != "" {
			line += fmt.Sprintf(", with people kept in %s", l.UserTable)
		}
		f = append(f, line+".")
	}
	if len(s.Services) > 0 {
		f = append(f, "Outside services: "+list(s.Services, 10, func(sv Service) string { return fmt.Sprintf("%s (%s)", serviceLabel(sv.Name), sv.Kind) })+".")
	}
	if len(s.Uploads) > 0 {
		var where []string
		for _, u := range s.Uploads {
			if uploadLabels[u.Kind] != "" {
				where = add(where, uploadLabels[u.Kind])
			}
		}
		f = append(f, strings.TrimSuffix("People upload files, "+joinAnd(where), ", ")+".")
	}
	if len(s.Schedules) > 0 {
		var files []string
		for _, sc := range s.Schedules {
			files = add(files, sc.File)
		}
		f = append(f, fmt.Sprintf("%s, in %s.", count(len(s.Schedules), "timed job"), joinAnd(files)))
	}
	for _, w := range s.Webhooks {
		f = append(f, fmt.Sprintf("%s sends it events at %s.", cmp.Or(serviceLabel(w.Service), "An outside service"), cmp.Or(w.Path, w.File)))
	}
	if len(s.Containers) > 0 {
		f = append(f, "Docker files: "+list(s.Containers, 4, func(c Container) string { return c.File })+".")
	}
	if len(s.Private) > 0 {
		f = append(f, "Addresses on a private network or this computer: "+list(s.Private, 4, func(a Address) string {
			return fmt.Sprintf("%s (in %s)", a.Address, a.File)
		})+".")
	}
	if s.Incomplete != "" {
		f = append(f, "The project is large, so llocal read only part of it: "+s.Incomplete+".")
	}
	return f
}

// functionNames are the names of its functions of kind.
func (s Survey) functionNames(kind string) []string {
	var names []string
	for _, fn := range s.Functions {
		if fn.Kind == kind {
			names = append(names, fn.Name)
		}
	}
	return names
}

// fits: it already has llocal.json, and no database to move or login to take out.
func (s Survey) fits() bool { return s.RecipeShape != "" && s.replaced(false) == "" && s.Login == nil }

func (p *Plan) changes(s Survey) {
	stores := s.replaced(false)
	if s.fits() {
		p.Changes = append(p.Changes, Item{"Nothing to convert.", "It already follows llocal's standard shape, and llocal builds and starts it the way its llocal.json says."})
		return
	}
	if p.Shape == "static" {
		p.Changes = append(p.Changes, Item{"Nothing to convert.", "llocal puts the pages online as they are, behind sign-in."})
	}
	if stores != "" && p.Shape != "static" {
		p.Changes = append(p.Changes, Item{
			fmt.Sprintf("The %s data moves into one SQLite file on llocal, copied over once.", stores),
			"SQLite sits right next to the app, so it is fast, it is backed up with the app, and only people who can open the app reach the data.",
		})
	}
	if stores == "" && len(s.DataStores) > 0 {
		p.Changes = append(p.Changes, Item{"The SQLite file moves into llocal's data folder.", "There it survives updates and is backed up."})
	}
	if s.replaced(true) != "" && p.Shape == "standard" {
		p.Changes = append(p.Changes, Item{
			"The pages stop talking to the database directly and ask the app's own server instead.",
			"Today each visitor's browser reaches the database with a key anyone can read in the pages, and only the database's own rules stand in the way. " +
				"On llocal only people who have signed in reach the data, and only through the server, which checks the same rules.",
		})
	}
	if s.Login != nil {
		detail := "People sign in with llocal, and nobody keeps another password. You choose who can open the app."
		if s.Login.UserTable != "" {
			detail += fmt.Sprintf(" The list of people in %s stays, so everyone keeps their own records.", s.Login.UserTable)
		}
		p.Changes = append(p.Changes, Item{"The app's own login goes, and llocal signs people in.", detail})
	}
	switch {
	case p.Shape == "standard" && len(s.Servers) == 0:
		text := "llocal adds a small server, written in Go, that does the work the pages need from it"
		if stores != "" {
			text = fmt.Sprintf("llocal adds a small server, written in Go, that does the work %s did for the pages", stores)
		}
		if n := len(s.Functions); n > 0 {
			noun := "function"
			if n > 1 {
				noun += "s"
			}
			text += fmt.Sprintf(", including the %s %s", list(s.Functions, 6, func(f Function) string { return f.Name }), noun)
		}
		p.Changes = append(p.Changes, Item{text + ".", fast})
	case p.Shape == "standard" && slices.ContainsFunc(s.Servers, func(sv Server) bool { return slices.Contains(fullStackFrameworks, sv.Framework) }):
		p.Changes = append(p.Changes, Item{"The pages are built once into plain files, and its server routes and functions are rewritten in Go, at the same addresses.", fast})
	case p.Shape == "standard":
		p.Changes = append(p.Changes, Item{fmt.Sprintf("The %s server is rewritten in Go on llocal's kit, answering at the same addresses.", s.serverLabel()), fast})
	case p.Shape == "kept" && len(s.Servers) > 1:
		p.Changes = append(p.Changes, Item{fmt.Sprintf("Its own %s servers stay.", s.serverLabel()), p.Why})
	case p.Shape == "kept":
		p.Changes = append(p.Changes, Item{fmt.Sprintf("Its own %s server stays.", s.serverLabel()), p.Why})
	}
	if len(s.Schedules) > 0 {
		p.Changes = append(p.Changes, Item{"Timed jobs run on llocal's timetable instead of a timer inside the app.", "Apps sleep when nobody is using them, which would stop a timer."})
	}
	if len(s.Uploads) > 0 && p.Shape != "static" {
		p.Changes = append(p.Changes, Item{"Uploaded files move to llocal.", "They are kept and backed up with the app."})
	}
	if len(s.UnlistedPackages) > 0 {
		p.Changes = append(p.Changes, Item{
			"llocal writes down the Python packages it needs: " + joinAnd(s.UnlistedPackages) + ".",
			"The project has no complete list of them, and the app cannot be installed without one.",
		})
	}
}

// settings splits the variables the app reads into public build settings and secrets, and
// drops the ones llocal's changes make unnecessary.
func (p *Plan) settings(s Survey) {
	stores := map[string]bool{}
	for _, d := range s.DataStores {
		stores[d.Kind] = p.Shape != "static" && d.Kind != "sqlite"
	}
	have := map[string]bool{}
	for _, e := range s.Env {
		if e.Service == "platform" || stores[e.Service] || e.Service == "login" && s.Login != nil {
			p.Dropped = append(p.Dropped, e.Name)
			continue
		}
		st := Setting{Name: e.Name, Kind: "secret", Service: e.Service}
		switch {
		case !e.Browser:
		case Secret(e.Looks):
			st.Name = strings.TrimPrefix(e.Name, recipe.PublicPrefix(e.Name))
			st.Note = fmt.Sprintf("Was %s, in the pages. llocal moves what uses it to the server, so it can stay secret.", e.Name)
			st.Warning = secretInPages(e)
		case e.Looks == "publishable":
			st.Kind = "build_setting"
			st.Note = "Public by design: a publishable or anon key is meant to be in the pages."
		default:
			st.Kind = "build_setting"
		}
		have[e.Service] = true
		p.Settings = append(p.Settings, st)
	}
	for _, sv := range s.Services {
		if def := known(sv.Name); def.key != "" && !have[sv.Name] {
			p.Settings = append(p.Settings, Setting{Name: def.key, Kind: "secret", Service: sv.Name})
		}
	}
}

func secretInPages(e Env) string {
	return fmt.Sprintf("%s looks like %s, and anything named %s... is copied into the pages every visitor downloads, where anyone can read it. "+
		"Make a new one at the service and stop using this one; llocal keeps the new one on the server as a secret.", e.Name, looksLabels[e.Looks], recipe.PublicPrefix(e.Name))
}

func (p *Plan) outside(s Survey) {
	for _, sv := range s.Services {
		var keys []string
		for _, st := range p.Settings {
			if st.Service == sv.Name {
				keys = append(keys, st.Name)
			}
		}
		detail := "It needs " + joinAnd(keys) + "."
		if len(keys) == 0 {
			detail = "No setting holds its key, so the key stays where it is in the code, if it has one."
		}
		if sv.Browser {
			detail += " Visitors' browsers talk to it directly."
		}
		p.Outside = append(p.Outside, Item{fmt.Sprintf("%s (%s) stays connected.", serviceLabel(sv.Name), sv.Kind), detail})
	}
}

func (p *Plan) needs(s Survey) {
	for _, st := range p.Settings {
		text := fmt.Sprintf("The value of %s", st.Name)
		switch sv := known(st.Service); {
		case sv.label != "" && keyName.MatchString(st.Name):
			text = fmt.Sprintf("Your %s key, saved as %s", sv.label, st.Name)
		case sv.label != "":
			text = fmt.Sprintf("Your %s setting %s", sv.label, st.Name)
		}
		if st.Kind == "build_setting" {
			text += " (public: it is part of the pages)"
		}
		if slices.ContainsFunc(s.Env, func(e Env) bool { return e.Name == st.Name && e.Set }) {
			text += ". One is in the upload; check it is the one to use"
		}
		p.Needs = append(p.Needs, Need{Text: text + ".", Setting: st.Name})
	}
	if p.Shape == "static" {
		return
	}
	for _, d := range s.DataStores {
		name, at, like := storeLabels[d.Kind], "", ""
		if d.Host != "" {
			at = " (" + d.Host + ")"
		}
		if len(d.Env) > 0 {
			like = ", like the one in " + d.Env[0]
		}
		host, _, _ := strings.Cut(d.Host, ":")
		var text string
		switch {
		case d.Kind == "sqlite" || privateHost(host) && !loopback(host):
			continue
		case loopback(host):
			text = fmt.Sprintf("The %s database's address in the code, %s, points at your own computer. If the real data is there, "+
				"export it to a file (a backup, or a spreadsheet per table) and add it to the upload. If it lives somewhere else, "+
				"give llocal that address and a user that can read every table.", name, d.Host)
		case d.Kind == "supabase":
			text = fmt.Sprintf("So llocal can copy your data once: a key that can read every table of your Supabase project%s. "+
				"The service_role key (Supabase > Project Settings > API Keys) or the database password will do. It is used for the copy, then thrown away.", at)
		case d.Kind == "firebase":
			text = fmt.Sprintf("So llocal can copy your data once: a service account key for your Firebase project%s "+
				"(Firebase console > Project settings > Service accounts > Generate new private key). It is used for the copy, then thrown away.", at)
		default:
			text = fmt.Sprintf("So llocal can copy your data once: the full address of your %s database%s%s, "+
				"with a user that can read all of it. It is used for the copy, then thrown away.", name, at, like)
		}
		p.Needs = append(p.Needs, Need{Text: text})
	}
}

func (p *Plan) blockers(s Survey, ip string) {
	if len(s.Servers) > 1 {
		p.Blockers = append(p.Blockers, Blocker{"two_services", "This folder holds more than one app",
			fmt.Sprintf("It holds %s. llocal runs one server for each app.", s.serverList()),
			"Upload each one as its own app, or tell llocal which one this is. If they work together, llocal can merge them into one server when it converts the app."})
	}
	if len(s.Containers) > 0 {
		detail := fmt.Sprintf("It has %s. llocal does not run Docker: it builds the app from its own code, in llocal's standard setup for Node, Python or Go.",
			list(s.Containers, 4, func(c Container) string { return c.File }))
		for _, c := range s.Containers {
			if len(c.Installs) > 0 {
				detail += fmt.Sprintf(" %s installs %s, which the app may need.", c.File, joinAnd(c.Installs))
			}
			if len(c.Services) > 0 {
				detail += fmt.Sprintf(" %s starts %s: %s.", c.File, count(len(c.Services), "service"), joinAnd(c.Services))
			}
		}
		p.Blockers = append(p.Blockers, Blocker{"container", "It comes with Docker files", detail,
			"If they only install and start the app, there is nothing to do: llocal leaves them out. A database in them becomes SQLite on llocal. " +
				"If the app needs other programs installed, or another service from them, tell llocal which, or upload that service as its own app."})
	}
	for _, d := range s.DataStores {
		if privateStore(d) {
			p.Blockers = append(p.Blockers, Blocker{"private_database", "Its database is on a private network",
				fmt.Sprintf("The %s database at %s can only be reached from inside your network, so llocal cannot copy the data from it.", storeLabels[d.Kind], d.Host),
				fmt.Sprintf("Export the data to a file (a backup, or a spreadsheet per table) and add it to the upload. "+
					"Or ask whoever runs your network to let llocal's address, %s, reach the database once, for the copy.", ip)})
		}
	}
	if p.Shape == "kept" && s.RecipeShape == "" && len(s.Servers) == 1 && slices.Contains(pythonFrameworks, s.Servers[0].Framework) &&
		(slices.ContainsFunc(spaFrameworks, s.fw) || slices.ContainsFunc(frontFrameworks, s.fw)) {
		p.Blockers = append(p.Blockers, Blocker{"two_languages", "Its pages and its server need two different setups",
			fmt.Sprintf("Its pages are built with Node, and its %s server runs on Python. llocal builds and runs an app in one setup: Go (which has Node too), Node or Python. None has both Node and Python.", s.serverLabel()),
			"Build the pages on your computer and add the built files to the upload in a folder named static (llocal leaves out folders like dist and build), for the server to hand out. " +
				"Or have the server rewritten in Go on llocal's kit, whose setup builds the pages too."})
	}
	if l := s.Login; l != nil && hostLogins[l.Provider] != "" {
		host := hostLogins[l.Provider]
		p.Blockers = append(p.Blockers, Blocker{"host_login", "Its login only works on " + host,
			fmt.Sprintf("The app signs people in with %s, which works only while %s runs the app.", loginLabels[l.Provider], host),
			"llocal's sign-in takes its place when the app is converted, so people sign in with llocal. Until then the app cannot go live as it is."})
	}
	if len(s.Frameworks) == 0 && len(s.Pages) == 0 {
		p.Blockers = append(p.Blockers, Blocker{"unknown", "llocal could not tell what this app is",
			"There is no package.json, requirements file or web page in the folder.",
			"Check that the folder holds the app's own files, not a shortcut or a download page. Or paste the line for your AI agent, which can get the app ready."})
	}
}

func (p *Plan) warnings(s Survey) {
	for _, e := range s.Env {
		if e.Browser && Secret(e.Looks) {
			p.Warnings = append(p.Warnings, secretInPages(e))
		}
	}
	for _, k := range s.Keys {
		switch {
		case k.Looks == "publishable":
		case k.Browser:
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s is written into the pages in %s, where anyone who opens the app can read it and use it. "+
				"Make a new one at the service and stop using this one; llocal keeps the new one on the server as a secret.", capital(looksLabels[k.Looks]), k.File))
		default:
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s is written into the code in %s. llocal moves it into a secret, so it is no longer part of the code.",
				capital(looksLabels[k.Looks]), k.File))
		}
	}
	if lan := s.lan(); len(lan) > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("It calls addresses on a private network that llocal cannot reach: %s. Those parts will not work on llocal.", joinAnd(lan)))
	}
	if w := s.envSecrets(); w != "" {
		p.Warnings = append(p.Warnings, w)
	}
}

// envSecrets warns of the secrets a .env file holds in full, but those in the pages, which
// have a warning of their own.
func (s Survey) envSecrets() string {
	var names, files []string
	for _, e := range s.Env {
		if e.Set && Secret(e.Looks) && !e.Browser {
			names = append(names, e.Name)
			for _, f := range e.Files {
				if IsEnvFile(f) && !isTemplate(f) {
					files = add(files, f)
				}
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	holds, what, it, renew := "holds", fmt.Sprintf("a secret, %s, written out in full", names[0]), "it", "a new one at the service and stop using this one"
	if len(files) > 1 {
		holds = "hold"
	}
	if len(names) > 1 {
		what, it, renew = fmt.Sprintf("%s written out in full: %s", count(len(names), "secret"), joinAnd(names)), "them", "new ones at the service and stop using these"
	}
	return fmt.Sprintf("%s %s %s. Anyone with a copy of the project can use %s. llocal leaves %s out of the app's code; "+
		"if the project was ever shared or pushed to GitHub, make %s.", joinAnd(files), holds, what, it, joinAnd(files), renew)
}

// lan are the addresses on a private network the app calls, but its databases'.
func (s Survey) lan() []string {
	var lan []string
	for _, a := range s.Private {
		host, _, _ := strings.Cut(a.Address, ":")
		if !loopback(host) && !slices.ContainsFunc(s.DataStores, func(d DataStore) bool { return d.Host == a.Address }) {
			lan = append(lan, fmt.Sprintf("%s (in %s)", a.Address, a.File))
		}
	}
	return lan
}

// privateStore says whether a database is on a private network, not this computer.
func privateStore(d DataStore) bool {
	host, _, _ := strings.Cut(d.Host, ":")
	return privateHost(host) && !loopback(host)
}

// tell lists the services to give the new address. A webhook that is a function today has no
// address on the app until it is converted; the survey of the converted app gives its line.
func (p *Plan) tell(s Survey, url, ip string) {
	hooks := slices.DeleteFunc(slices.Clone(s.Webhooks), func(w Webhook) bool { return w.Path == "" })
	for _, sv := range s.Services {
		def := known(sv.Name)
		value := ""
		switch def.calls {
		case "webhook":
			for _, w := range hooks {
				if w.Service == sv.Name {
					p.Tell = append(p.Tell, Tell{def.label, url + w.Path, def.tell})
				}
			}
		case "redirect":
			value = url
			for _, r := range s.Routes {
				if slices.Contains(sv.Files, r.File) && strings.Contains(r.Path, "callback") {
					value = url + r.Path
				}
			}
		case "origin":
			if sv.Browser {
				value = url
			}
		}
		if value != "" {
			p.Tell = append(p.Tell, Tell{def.label, value, def.tell})
		}
	}
	for _, w := range hooks {
		if w.Service == "" {
			p.Tell = append(p.Tell, Tell{"The service that sends events to " + w.Path, url + w.Path, "wherever that service asks for your app's address"})
		}
	}
	if len(s.Services) > 0 && p.Shape != "static" {
		p.Tell = append(p.Tell, Tell{"Any of these services that only accepts calls from listed addresses", ip, "its list of allowed IP addresses"})
	}
}

func known(name string) service {
	i := slices.IndexFunc(services, func(s service) bool { return s.name == name })
	if i < 0 {
		return service{}
	}
	return services[i]
}

func serviceLabel(name string) string { return cmp.Or(known(name).label, name) }

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// list names the first max items, and how many more there are.
func list[T any](items []T, max int, name func(T) string) string {
	var names []string
	for _, it := range items[:min(max, len(items))] {
		names = append(names, name(it))
	}
	if len(items) > max {
		return strings.Join(names, ", ") + fmt.Sprintf(" and %d more", len(items)-max)
	}
	return joinAnd(names)
}

func joinAnd(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func capital(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

func aOrAn(word string) string {
	if strings.ContainsAny(word[:1], "AEIOU") {
		return "an " + word
	}
	return "a " + word
}
