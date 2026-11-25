package survey

import (
	"maps"
	"slices"
	"testing"
)

// TestFacts: the packages an app runs on and which go unused, what its table definitions
// say, code repeated across files, old copies, and what its screens' markup shows.
func TestFacts(t *testing.T) {
	dir := t.TempDir()
	block := "const total = items.reduce((sum, item) => sum + item.price, 0)\n" +
		"if (total > limit) { throw new Error('over the limit') }\nconst tax = total * rate\nconst due = total + tax\n" +
		"console.log('total', total)\nconsole.log('tax', tax)\nconsole.log('due', due)\nreturn { total, tax, due }\n"
	write(t, dir, map[string]string{
		"package.json":      `{"dependencies": {"express": "4", "pg": "8", "moment": "2", "tailwindcss": "3"}, "devDependencies": {"eslint": "9"}}`,
		"postcss.config.js": `module.exports = { plugins: { tailwindcss: {} } }`,
		"server.js":         "import express from 'express'\nimport pg from 'pg'\nfunction a() {\n" + block + "}\n",
		"lib/billing.js":    "function b() {\n" + block + "}\n",
		"old/server.js":     "// the first version\n",
		"src/App.old.jsx":   "export default 1\n",
		"schema.sql": `CREATE TABLE customers (id integer PRIMARY KEY, name text, tags text[]);
CREATE TABLE orders (id integer, customer_id integer REFERENCES customers (id), items jsonb);`,
		"src/Page.jsx": `export default () => <main>
  <h1>Orders</h1><h3>Today</h3>
  <img src="/logo.png">
  <input name="q" />
  <button onClick={go}><svg viewBox="0 0 1 1" /></button>
  <button aria-label="Close"><XIcon /></button>
  <p style={{ color: '#999999', background: '#ffffff' }}>Every order placed today</p>
</main>`,
	})
	s := run(t, dir)
	var unused []string
	for _, p := range s.Packages {
		if p.Unused {
			unused = append(unused, p.Name)
		}
	}
	if len(s.Packages) != 4 || !slices.Equal(unused, []string{"moment"}) {
		t.Errorf("packages %+v, want 4 with only moment unused", s.Packages)
	}
	tables := map[string]Table{}
	for _, tb := range s.DataStores[0].Tables {
		tables[tb.Name] = tb
	}
	if c, o := tables["customers"], tables["orders"]; !c.Defined || !c.Key || !slices.Equal(c.Lists, []string{"tags"}) ||
		!o.Defined || o.Key || !slices.Equal(o.References, []string{"customers"}) || !slices.Equal(o.JSON, []string{"items"}) {
		t.Errorf("tables %+v", tables)
	}
	if r := s.Repeated; r == nil || r.Lines != 2*repeatAt || !slices.Equal(r.Files, []string{"lib/billing.js", "server.js"}) {
		t.Errorf("repeated %+v, want the 8 lines in both files", r)
	}
	if !slices.Equal(s.Leftovers, []string{"old", "src/App.old.jsx"}) {
		t.Errorf("leftovers %v", s.Leftovers)
	}
	sc := s.Screens
	if sc == nil || sc.Files != 1 || sc.Controls != 3 || sc.Viewport || sc.Colors != 2 {
		t.Fatalf("screens %+v", sc)
	}
	signs := map[string]int{}
	for _, sg := range sc.Signs {
		signs[sg.Kind] = sg.Count
	}
	if want := map[string]int{"image_no_alt": 1, "field_no_label": 1, "icon_button": 1, "heading_skip": 1, "low_contrast": 1}; !maps.Equal(signs, want) {
		t.Errorf("signs %v, want %v", signs, want)
	}
	if c := contrast("000", "ffffff"); c < 20.9 || c > 21.1 {
		t.Errorf("contrast of black on white %v, want 21", c)
	}
}
