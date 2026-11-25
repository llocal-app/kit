package survey

import (
	"bytes"
	"encoding/json"
	"flag"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"llocal.app/kit/check"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the fixtures")

// fixtures are the sample projects under testdata, one folder each.
func fixtures(t *testing.T) []string {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "golden" {
			names = append(names, e.Name())
		}
	}
	return names
}

func run(t *testing.T, dir string) Survey {
	s, err := Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestFixtures surveys every sample with no manual input and compares the survey and the
// plan with testdata/golden. Run with -update after a deliberate change, and read the diff.
func TestFixtures(t *testing.T) {
	for _, name := range fixtures(t) {
		t.Run(name, func(t *testing.T) {
			s := run(t, filepath.Join("testdata", name))
			got, err := json.MarshalIndent(map[string]any{"survey": s, "plan": s.Plan("https://app.acme.llocal.app", "203.0.113.7")}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", "golden", name+".json")
			if *update {
				if err := os.WriteFile(golden, append(got, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bytes.TrimSpace(want), got) {
				t.Errorf("survey or plan of %s changed; run go test ./survey -update and check the diff of %s", name, golden)
			}
		})
	}
}

func TestExportShapes(t *testing.T) {
	for _, tc := range []struct {
		fixture, builder, shape string
		servers                 []string
	}{
		{"html", "", "static", nil},
		{"vite-react", "", "static", nil},
		{"lovable-vite", "lovable", "standard", nil},
		{"lovable-tanstack", "lovable", "standard", []string{". tanstack-start"}},
		{"bolt", "bolt", "standard", nil},
		{"v0", "v0", "kept", []string{". nextjs"}},
		{"nextjs", "", "standard", []string{". nextjs"}},
		{"replit", "replit", "standard", []string{". express"}},
		{"replit-agent4", "replit", "standard", []string{"artifacts/api-server express"}},
		{"express", "", "standard", []string{". express"}},
		{"flask", "", "kept", []string{". flask"}},
		{"fastapi", "", "standard", []string{". fastapi"}},
		{"streamlit", "", "kept", []string{". streamlit"}},
	} {
		s := run(t, filepath.Join("testdata", tc.fixture))
		if p := s.Plan("", ""); s.Builder != tc.builder || p.Shape != tc.shape || !slices.Equal(servers(s), tc.servers) {
			t.Errorf("%s: builder %q shape %q servers %v, want %q %q %v", tc.fixture, s.Builder, p.Shape, servers(s), tc.builder, tc.shape, tc.servers)
		}
	}
}

// servers are the survey's servers as "dir framework".
func servers(s Survey) []string {
	var out []string
	for _, sv := range s.Servers {
		out = append(out, sv.Dir+" "+sv.Framework)
	}
	return out
}

// TestViteSupabase: the survey of a Vite + Supabase app lists its
// tables, its login and exactly the keys it needs.
func TestViteSupabase(t *testing.T) {
	for fixture, want := range map[string]struct {
		tables, keys []string
	}{
		"lovable-vite": {[]string{"profiles", "jobs"}, []string{"RESEND_API_KEY"}},
		"bolt":         {[]string{"quotes"}, []string{"OPENAI_API_KEY"}},
	} {
		s := run(t, filepath.Join("testdata", fixture))
		if len(s.DataStores) != 1 || s.DataStores[0].Kind != "supabase" || !s.DataStores[0].Browser {
			t.Fatalf("%s: data stores %+v, want one Supabase store the browser talks to", fixture, s.DataStores)
		}
		var tables []string
		for _, tb := range s.DataStores[0].Tables {
			tables = append(tables, tb.Name)
		}
		if !slices.Equal(tables, want.tables) {
			t.Errorf("%s: tables %v, want %v", fixture, tables, want.tables)
		}
		if s.Login == nil || s.Login.Provider != "supabase" || s.Login.UserTable != "auth.users" {
			t.Errorf("%s: login %+v, want supabase with auth.users", fixture, s.Login)
		}
		p := s.Plan("", "")
		var keys []string
		for _, st := range p.Settings {
			keys = append(keys, st.Name)
		}
		if !slices.Equal(keys, want.keys) {
			t.Errorf("%s: settings %v, want exactly %v", fixture, keys, want.keys)
		}
		if !slices.ContainsFunc(p.Needs, func(n Need) bool { return n.Setting == "" && strings.Contains(n.Text, "service_role key") }) {
			t.Errorf("%s: needs %+v, want the one-time read key for the data copy", fixture, p.Needs)
		}
	}
}

// TestNamedCases: each case llocal names before anything is built has its plain-words
// screen and its way out.
func TestNamedCases(t *testing.T) {
	for fixture, want := range map[string]string{
		"two-services": "two_services",
		"docker":       "container",
		"office-db":    "private_database",
		"replit":       "host_login",
		"python-node":  "two_languages",
		"empty":        "unknown",
	} {
		dir := filepath.Join("testdata", fixture)
		if fixture == "empty" {
			dir = t.TempDir()
		}
		p := run(t, dir).Plan("", "203.0.113.7")
		i := slices.IndexFunc(p.Blockers, func(b Blocker) bool { return b.Case == want })
		if i < 0 {
			t.Errorf("%s: blockers %+v, want %s", fixture, p.Blockers, want)
			continue
		}
		b := p.Blockers[i]
		for _, text := range []string{b.Title, b.Detail, b.WayOut} {
			if text == "" || strings.Contains(text, "\n") {
				t.Errorf("%s: %s has a missing or multi-line part: %+v", fixture, want, b)
			}
		}
		t.Logf("%s: %s. %s %s", fixture, b.Title, b.Detail, b.WayOut)
	}
}

func TestSettings(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"package.json": `{"dependencies": {"vite": "7", "react": "19", "@stripe/stripe-js": "7"}}`,
		".env": "VITE_STRIPE_PUBLISHABLE_KEY=pk_test_FAKEFAKEFAKE\nVITE_ADMIN_TOKEN=sk_test_FAKEFAKEFAKE\n" +
			"VITE_MAPS_ID=north-yard\nVITE_SUPABASE_SERVICE_ROLE_KEY=\n",
		"src/main.tsx": `import { loadStripe } from "@stripe/stripe-js"
const stripe = loadStripe(import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY)
fetch("/admin", { headers: { token: import.meta.env.VITE_ADMIN_TOKEN } })
const signing = "-----BEGIN RSA PRIVATE KEY-----"`,
	})
	s := run(t, dir)
	p := s.Plan("", "")
	got := map[string]Setting{}
	for _, st := range p.Settings {
		got[st.Name] = st
	}
	if st := got["VITE_STRIPE_PUBLISHABLE_KEY"]; st.Kind != "build_setting" || !strings.Contains(st.Note, "Public by design") {
		t.Errorf("publishable key: %+v, want a build setting that is public by design", st)
	}
	if st := got["VITE_MAPS_ID"]; st.Kind != "build_setting" || st.Note != "" || st.Warning != "" {
		t.Errorf("plain public setting: %+v", st)
	}
	if st := got["ADMIN_TOKEN"]; st.Kind != "secret" || !strings.Contains(st.Warning, "VITE_ADMIN_TOKEN looks like a secret key") {
		t.Errorf("secret key in the pages: %+v, want a secret named without VITE_, with a warning", st)
	}
	warned := strings.Join(p.Warnings, "\n")
	for _, want := range []string{"VITE_ADMIN_TOKEN", "VITE_SUPABASE_SERVICE_ROLE_KEY looks like a Supabase service_role key", "A private key is written into the pages in src/main.tsx"} {
		if !strings.Contains(warned, want) {
			t.Errorf("warnings %q, want %q", p.Warnings, want)
		}
	}
	if p.Shape != "standard" {
		t.Errorf("shape %s: pages holding a secret need a server to keep it", p.Shape)
	}
}

// TestAlreadyFits: an app already on the kit, with llocal.json, has nothing to convert: the
// starter, once a builder has made it a module.
func TestAlreadyFits(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "starter"))); err != nil {
		t.Fatal(err)
	}
	write(t, dir, map[string]string{"go.mod": "module starter\n\ngo 1.24.0\n\nrequire llocal.app/kit v0.0.0\n"})
	s := run(t, dir)
	p := s.Plan("", "")
	if s.RecipeShape != "standard" || !slices.Equal(servers(s), []string{". go"}) || len(s.Routes) != 2 ||
		len(s.DataStores) != 1 || s.DataStores[0].Tables[0].Name != "notes" {
		t.Errorf("survey of kit/starter: %+v", s)
	}
	if p.Shape != "standard" || len(p.Changes) != 1 || p.Changes[0].Text != "Nothing to convert." || len(p.Needs) != 0 || len(p.Left) != 0 {
		t.Errorf("plan for kit/starter: %+v", p)
	}
}

func TestMountedRoutes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"requirements.txt": "fastapi\n",
		"main.py":          "from fastapi import FastAPI\nfrom routers import items\napp = FastAPI()\napp.include_router(items.router, prefix=\"/api\")\n",
		"routers/items.py": "from fastapi import APIRouter\nrouter = APIRouter(prefix=\"/items\")\n\n@router.get(\"/\")\ndef all(): ...\n\n@router.get(\"/{id}\")\ndef one(id: int): ...\n",
	})
	var got []string
	for _, r := range run(t, dir).Routes {
		got = append(got, r.Method+" "+r.Path)
	}
	if want := []string{"GET /api/items", "GET /api/items/{id}"}; !slices.Equal(got, want) {
		t.Errorf("routes %v, want %v", got, want)
	}
}

// TestRenamedTables: a table rebuilt under a new name and renamed back is one table, and so
// is one moved out of the way and made again under its own name.
func TestRenamedTables(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"package.json":       `{"dependencies": {"better-sqlite3": "11"}}`,
		"migrations/001.sql": "CREATE TABLE jobs (id integer, title text);\nCREATE TABLE crews (id integer);\n",
		"migrations/002.sql": "CREATE TABLE new_jobs (id integer, title text, due text);\nINSERT INTO new_jobs SELECT id, title, NULL FROM jobs;\n" +
			"DROP TABLE jobs;\nALTER TABLE new_jobs RENAME TO jobs;\n",
		"migrations/003.sql": "ALTER TABLE crews RENAME TO crews_old;\nCREATE TABLE crews (id integer, lead text);\nDROP TABLE crews_old;\n",
	})
	var got []string
	for _, tb := range run(t, dir).DataStores[0].Tables {
		got = append(got, tb.Name+" "+strings.Join(tb.Columns, ","))
	}
	if want := []string{"jobs id,title,due", "crews id,lead"}; !slices.Equal(got, want) {
		t.Errorf("tables %q, want %q", got, want)
	}
}

// TestFunctionWebhook: a webhook that is a function today has no address on the app yet, so
// the plan does not tell the service one.
func TestFunctionWebhook(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"package.json": `{"dependencies": {"vite": "7", "@supabase/supabase-js": "2"}}`,
		"supabase/functions/stripe-webhook/index.ts": `import Stripe from "npm:stripe@16"
Deno.serve(async (req) => { new Stripe(Deno.env.get("STRIPE_SECRET_KEY")!).webhooks.constructEvent(await req.text(), "", "") })`,
	})
	s := run(t, dir)
	if len(s.Webhooks) != 1 || s.Webhooks[0].Service != "stripe" {
		t.Fatalf("webhooks %+v", s.Webhooks)
	}
	for _, tl := range s.Plan("https://a.llocal.app", "").Tell {
		if tl.Service == "Stripe" {
			t.Errorf("told Stripe %q before the function has an address", tl.Value)
		}
	}
}

// TestNeedles: a rule skips only text that cannot match it.
func TestNeedles(t *testing.T) {
	for expr, want := range map[string][]string{
		`\bprocess\.env\.(\w+)`:            {"process.env."},
		`\bflask_login\b|\bLoginManager\(`: {"flask_login", "LoginManager("},
		`@(\w+)\.(get|post)\(`:             {"get", "post"},
		`a(bc)?d+`:                         {"a"},
		`(?i)create\s+table`:               nil,
		`x|\w+`:                            nil,
	} {
		if got := compile(expr).needles; !slices.Equal(got, want) {
			t.Errorf("%s: needles %q, want %q", expr, got, want)
		}
	}
}

// TestNoValues: survey.json holds names, never the values of keys and passwords.
func TestNoValues(t *testing.T) {
	for _, name := range fixtures(t) {
		s := run(t, filepath.Join("testdata", name))
		out, _ := json.Marshal(map[string]any{"survey": s, "plan": s.Plan("", "")})
		for _, secret := range []string{"eyJhbGci", "sk-proj-", "sb_publishable_", "hunter2", "SG.fake", "dev-not-secret", "postgres:postgres@"} {
			if bytes.Contains(out, []byte(secret)) {
				t.Errorf("%s: the survey or plan contains %q", name, secret)
			}
		}
	}
}

// TestHostile: the survey never follows a link out of the folder, skips files over the cap
// and says when it stopped early.
func TestHostile(t *testing.T) {
	outside := t.TempDir()
	write(t, outside, map[string]string{"secret.ts": `const k = process.env.OUTSIDE_SECRET`, "lib/x.ts": `process.env.OUTSIDE_DIR`})
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"index.html": "<p>hi</p>",
		"big.js":     "process.env.BIG_FILE\n" + strings.Repeat("x", int(maxFileBytes)),
		"bare.js":    `import a from "npm:"; import b from "https://esm.sh/"; import c from ""`,
	})
	for link, target := range map[string]string{"linked.ts": filepath.Join(outside, "secret.ts"), "lib": filepath.Join(outside, "lib"), "up.ts": "../../etc/passwd"} {
		if err := os.Symlink(target, filepath.Join(dir, link)); err != nil {
			t.Fatal(err)
		}
	}
	s := run(t, dir)
	for _, e := range s.Env {
		t.Errorf("read %s from %v, which it should have skipped", e.Name, e.Files)
	}
	if s.Incomplete != "" {
		t.Errorf("incomplete %q for a small folder", s.Incomplete)
	}

	defer func(n int) { maxFiles = n }(maxFiles)
	maxFiles = 2
	write(t, dir, map[string]string{"a.html": "a", "b.html": "b"})
	if s := run(t, dir); !strings.Contains(s.Incomplete, "more than 2 files") {
		t.Errorf("incomplete %q, want the file cap", s.Incomplete)
	}
}

// TestCheckerReadsSurvey: llocal-check reads survey.json's pages, routes, hosts and tables.
func TestCheckerReadsSurvey(t *testing.T) {
	s := run(t, filepath.Join("testdata", "lovable-tanstack"))
	data, _ := json.Marshal(s)
	file := filepath.Join(t.TempDir(), "survey.json")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := check.LoadSurvey(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Pages) != len(s.Pages) || len(c.Routes) != 1 || c.Routes[0].Method != "POST" || c.Routes[0].Path != "/api/public/stripe-webhook" ||
		c.DataStores[0].Host != "qrstuvwxyzabcdefghij.supabase.co" || c.DataStores[0].Tables[1].Name != "invoices" {
		t.Errorf("checker read %+v", c)
	}
}

// TestSchemaMatchesSurvey keeps survey.schema.json and the Survey type in step, at every
// level, so an agent that fills the file from the schema writes what Run would.
func TestSchemaMatchesSurvey(t *testing.T) {
	data, err := os.ReadFile("survey.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema node
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	compare(t, "survey", reflect.TypeFor[Survey](), schema, schema.Defs)
}

type node struct {
	Ref        string          `json:"$ref"`
	Properties map[string]node `json:"properties"`
	Items      *node           `json:"items"`
	Defs       map[string]node `json:"$defs"`
	Required   []string        `json:"required"`
}

func compare(t *testing.T, at string, typ reflect.Type, n node, defs map[string]node) {
	if n.Ref != "" {
		n = defs[strings.TrimPrefix(n.Ref, "#/$defs/")]
	}
	switch typ.Kind() {
	case reflect.Pointer:
		compare(t, at, typ.Elem(), n, defs)
	case reflect.Slice:
		if n.Items == nil {
			t.Errorf("%s: schema has no items", at)
			return
		}
		compare(t, at+"[]", typ.Elem(), *n.Items, defs)
	case reflect.Struct:
		fields := map[string]reflect.Type{}
		var required []string
		for _, f := range reflect.VisibleFields(typ) {
			name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			fields[name] = f.Type
			if opts != "omitempty" {
				required = append(required, name)
			}
		}
		props := slices.Sorted(maps.Keys(n.Properties))
		if names := slices.Sorted(maps.Keys(fields)); !slices.Equal(names, props) {
			t.Errorf("%s: schema properties %v, Go fields %v", at, props, names)
			return
		}
		slices.Sort(required)
		if got := slices.Sorted(slices.Values(n.Required)); !slices.Equal(got, required) {
			t.Errorf("%s: schema requires %v, Go always writes %v", at, got, required)
		}
		for name, ft := range fields {
			compare(t, at+"."+name, ft, n.Properties[name], defs)
		}
	}
}

func write(t *testing.T, dir string, files map[string]string) {
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
