package recipe

import (
	"encoding/json"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, ok := range []string{
		`{"shape": "standard", "base": "go", "build": "go build -o server .", "start": "./server", "port": 8080, "static": "web"}`,
		`{"shape": "static", "base": "static", "static": "."}`,
		`{"shape": "static", "base": "node", "install": "npm ci", "build": "npm run build", "static": "dist", "build_settings": ["VITE_API"]}`,
		`{"shape": "kept", "base": "python", "install": "pip install -r requirements.txt", "start": "gunicorn -b 0.0.0.0:$PORT app:app", "port": 8000,
		  "persistent": ["uploads"], "secrets": ["STRIPE_KEY"], "services": ["stripe"],
		  "schedules": [{"path": "/tasks/nightly", "cron": "0 6 * * *"}], "machine_paths": ["/webhooks/stripe"]}`,
	} {
		if _, err := Parse([]byte(ok)); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for want, bad := range map[string]string{
		"shape must be":       `{"shape": "docker", "base": "go"}`,
		"base must be go":     `{"shape": "standard", "base": "node", "start": "x", "port": 1}`,
		"no toolchain":        `{"shape": "static", "base": "static", "build": "make", "static": "."}`,
		"has no server":       `{"shape": "static", "base": "node", "static": "dist", "port": 3000}`,
		"start is required":   `{"shape": "kept", "base": "node", "port": 3000}`,
		"port is required":    `{"shape": "kept", "base": "node", "start": "node ."}`,
		"inside the app":      `{"shape": "static", "base": "static", "static": "../x"}`,
		"cannot be the app's": `{"shape": "standard", "base": "go", "start": "./server", "port": 1, "static": "."}`,
		"persistent path":     `{"shape": "kept", "base": "node", "start": "x", "port": 1, "persistent": ["/etc"]}`,
		"anything else":       `{"shape": "static", "base": "node", "static": "dist", "build_settings": ["API_URL"]}`,
		"public prefix":       `{"shape": "kept", "base": "node", "start": "x", "port": 1, "secrets": ["VITE_KEY"]}`,
		"llocal sets":         `{"shape": "kept", "base": "node", "start": "x", "port": 1, "secrets": ["LLOCAL_URL"]}`,
		"five numeric fields": `{"shape": "kept", "base": "node", "start": "x", "port": 1, "schedules": [{"path": "/t", "cron": "@daily"}]}`,
		"must start with /":   `{"shape": "kept", "base": "node", "start": "x", "port": 1, "machine_paths": ["hook"]}`,
		"unknown field":       `{"shape": "static", "base": "static", "static": ".", "image": "ubuntu"}`,
		"more than one":       `{"shape": "static", "base": "static", "static": "."} {}`,
	} {
		if _, err := Parse([]byte(bad)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", bad, err, want)
		}
	}
}

func TestSchemaMatchesRecipe(t *testing.T) {
	data, err := os.ReadFile("llocal.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	var fields []string
	for _, f := range reflect.VisibleFields(reflect.TypeFor[Recipe]()) {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		fields = append(fields, name)
	}
	props := slices.Sorted(maps.Keys(schema.Properties))
	slices.Sort(fields)
	if !slices.Equal(fields, props) {
		t.Errorf("schema properties %v, Recipe fields %v", props, fields)
	}
}
