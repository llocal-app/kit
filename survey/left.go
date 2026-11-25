package survey

import (
	"cmp"
	"fmt"
	"slices"
)

// Left is one thing that stands between the app and llocal's standard shape, in a sentence
// for someone who doesn't program. Kind is what it is, for an agent to act on: unknown,
// servers, docker, private, build, login, database, browser, server, functions, secrets,
// schedules, webhooks, uploads, packages or recipe.
type Left struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// say adds one thing left, its sentence made as fmt.Sprintf makes one.
type say func(kind, format string, a ...any)

// left lists what stands between the app and the standard shape, what the kit can't take
// first. An app that fits has nothing left.
func (p *Plan) left(s Survey) {
	p.Left = []Left{}
	if s.fits() {
		return
	}
	put := func(kind, format string, a ...any) { p.Left = append(p.Left, Left{kind, fmt.Sprintf(format, a...)}) }
	for _, b := range p.Blockers {
		switch b.Case {
		case "unknown":
			put("unknown", "llocal couldn't tell what this app is: the folder has no package.json, requirements file or web page.")
		case "two_services":
			put("servers", "It holds %s, and llocal runs one server for each app.", s.serverList())
		case "container":
			put("docker", "It comes with Docker files, which llocal doesn't run: anything they install or start besides the app has to go, or move into the app.")
		case "two_languages":
			put("build", "Its pages are built with Node and its %s server runs on Python, and no llocal setup has both: the pages are built before they are uploaded, or the server is rewritten in Go.", s.serverLabel())
		}
	}
	for _, d := range s.DataStores {
		if privateStore(d) {
			put("private", "Its %s database at %s is on a private network, which llocal can't reach to copy the data.", storeLabels[d.Kind], d.Host)
		}
	}
	if lan := s.lan(); len(lan) > 0 {
		put("private", "It calls %s on a private network, which llocal can't reach.", joinAnd(lan))
	}
	if l := s.Login; l != nil && hostLogins[l.Provider] != "" {
		put("login", "It signs people in with %s, which works only while %s runs the app. llocal's sign-in takes its place.", loginLabels[l.Provider], hostLogins[l.Provider])
	} else if l != nil {
		put("login", "It has its own login (%s), which goes: llocal signs people in with their work account.", loginLabels[l.Provider])
	}
	for _, d := range s.DataStores {
		switch {
		case d.Kind == "sqlite":
			put("database", "Its SQLite file moves into llocal's data folder, where it survives updates and is backed up.")
		case len(d.Tables) > 0:
			put("database", "Its data is in %s (%s) and moves into one SQLite file beside the app.", storeLabels[d.Kind], count(len(d.Tables), "table"))
		default:
			put("database", "Its data is in %s and moves into one SQLite file beside the app.", storeLabels[d.Kind])
		}
	}
	if stores := s.replaced(true); stores != "" {
		put("browser", "Visitors' browsers talk to %s directly. Those calls go through the app's own server instead.", stores)
	}
	p.leftServer(s, put)
	leftSecrets(s, put)
	if n := len(s.Schedules); n > 0 {
		put("schedules", "%s moves to llocal's timetable: apps sleep when nobody uses them, which would stop a timer inside the app.", capital(count(n, "timed job")))
	}
	for _, w := range s.Webhooks {
		if w.Path != "" {
			put("webhooks", "%s sends it events at %s, which is set apart from sign-in so they still get through.", cmp.Or(serviceLabel(w.Service), "An outside service"), w.Path)
		}
	}
	var kept []string
	for _, u := range s.Uploads {
		if uploadLabels[u.Kind] != "" {
			kept = add(kept, uploadLabels[u.Kind])
		}
	}
	if len(kept) > 0 && p.Shape != "static" {
		put("uploads", "Files people upload are %s, and move into the app's own storage on llocal.", joinAnd(kept))
	}
	if len(s.UnlistedPackages) > 0 {
		put("packages", "The Python packages it needs aren't written down anywhere (%s), and llocal needs that list to install them.", joinAnd(s.UnlistedPackages))
	}
	if s.RecipeShape == "" {
		put("recipe", "It has no llocal.json yet: the short file that tells llocal how to build and start it.")
	}
}

// leftServer says what becomes of the app's server: one in Go on the kit where it has none
// or one in another language. A kept server stays as it is.
func (p *Plan) leftServer(s Survey, put say) {
	if p.Shape != "standard" || slices.ContainsFunc(s.Servers, func(sv Server) bool { return sv.Framework == "go" }) {
		return
	}
	switch stores := s.replaced(false); {
	case len(s.Servers) == 0 && stores != "":
		put("server", "It has no server of its own, and needs a small one in Go, on llocal's kit, for the work %s does for its pages.", stores)
	case len(s.Servers) == 0 && len(s.Functions) > 0:
		put("server", "It has no server of its own, and needs a small one in Go, on llocal's kit, for the functions it runs on another service.")
	case len(s.Servers) == 0:
		put("server", "It has no server of its own, and needs a small one in Go, on llocal's kit, to keep its secret key out of the pages.")
	case slices.ContainsFunc(s.Servers, func(sv Server) bool { return slices.Contains(fullStackFrameworks, sv.Framework) }):
		put("server", "Its %s server code (%s) is rewritten in Go on llocal's kit, and its pages are built once into plain files.", s.serverLabel(), s.code())
	default:
		put("server", "Its %s server (%s) is rewritten in Go on llocal's kit.", s.serverLabel(), s.code())
	}
	for _, k := range functionKinds {
		switch names := s.functionNames(k.kind); len(names) {
		case 0:
		case 1:
			put("functions", "Its 1 %s%s (%s) moves into the app's own server.", k.noun, k.where, names[0])
		default:
			put("functions", "Its %s%s (%s) move into the app's own server.", count(len(names), k.noun), k.where, list(names, 6, func(n string) string { return n }))
		}
	}
}

// code is how big the server is: its routes, and its files and kilobytes when the survey
// counted them.
func (s Survey) code() string {
	files := 0
	for _, sv := range s.Servers {
		files += sv.Files
	}
	if files == 0 {
		return count(len(s.Routes), "route")
	}
	return fmt.Sprintf("%s in %s, %d KB", count(len(s.Routes), "route"), count(files, "file"), s.ServerKB())
}

// ServerKB is the size of the servers' own code in kilobytes, at least 1 when they have any.
func (s Survey) ServerKB() int64 {
	var bytes int64
	for _, sv := range s.Servers {
		bytes += sv.Bytes
	}
	if bytes == 0 {
		return 0
	}
	return max(1, (bytes+512)>>10)
}

// leftSecrets are the secret keys in its pages, which anyone can read, and the keys written
// into its code.
func leftSecrets(s Survey, put say) {
	var pages, code []string
	for _, e := range s.Env {
		if e.Browser && Secret(e.Looks) {
			pages = add(pages, e.Name)
		}
	}
	for _, k := range s.Keys {
		switch {
		case !Secret(k.Looks):
		case k.Browser:
			pages = add(pages, "a key in "+k.File)
		default:
			code = add(code, k.File)
		}
	}
	switch len(pages) {
	case 0:
	case 1:
		put("secrets", "A secret key sits in its pages, where anyone can read it (%s). It moves to the server, and needs replacing with a new one.", pages[0])
	default:
		put("secrets", "Secret keys sit in its pages, where anyone can read them (%s). They move to the server, and each needs replacing with a new one.", joinAnd(pages))
	}
	if len(code) > 0 {
		put("secrets", "A key is written into its code, in %s. It moves into a secret llocal keeps.", joinAnd(code))
	}
}
