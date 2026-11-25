// Package recipe reads llocal.json, the file at an app's root that says how to build it,
// start it and what it needs. llocal.schema.json describes the same fields for editors;
// Parse is the authority.
package recipe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

// StartWait is how long an app has, once started, to answer its health check: the runner
// waits this long for a real start and llocal's checker for its own, so they agree.
const StartWait = time.Minute

type Recipe struct {
	Schema        string     `json:"$schema,omitempty"`
	Shape         string     `json:"shape"`
	Base          string     `json:"base"`
	Install       string     `json:"install,omitempty"`
	Build         string     `json:"build,omitempty"`
	Start         string     `json:"start,omitempty"`
	Migrate       string     `json:"migrate,omitempty"`
	Port          int        `json:"port,omitempty"`
	Static        string     `json:"static,omitempty"`
	Persistent    []string   `json:"persistent,omitempty"`
	BuildSettings []string   `json:"build_settings,omitempty"`
	Secrets       []string   `json:"secrets,omitempty"`
	Services      []string   `json:"services,omitempty"`
	Schedules     []Schedule `json:"schedules,omitempty"`
	MachinePaths  []string   `json:"machine_paths,omitempty"`
}

type Schedule struct {
	Path string `json:"path"`
	Cron string `json:"cron"`
}

var (
	publicPrefixes = []string{"VITE_", "NEXT_PUBLIC_", "PUBLIC_", "REACT_APP_"}
	reserved       = []string{"PORT", "HOST", "TZ", "PATH", "HOME", "TMPDIR"}
	envName        = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	cronField      = regexp.MustCompile(`^[0-9*,/-]+$`)
)

func Load(path string) (*Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("llocal.json: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*Recipe, error) {
	var r Recipe
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("llocal.json: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("llocal.json: more than one JSON value")
	}
	if err := r.validate(); err != nil {
		return nil, fmt.Errorf("llocal.json: %w", err)
	}
	return &r, nil
}

func (r *Recipe) validate() error {
	switch {
	case !slices.Contains([]string{"standard", "kept", "static"}, r.Shape):
		return fmt.Errorf("shape must be standard, kept or static, not %q", r.Shape)
	case !slices.Contains([]string{"go", "node", "python", "static"}, r.Base):
		return fmt.Errorf("base must be go, node, python or static, not %q", r.Base)
	case r.Shape == "standard" && r.Base != "go":
		return fmt.Errorf("a standard app is a Go server on the kit, so base must be go")
	case r.Base == "static" && (r.Install != "" || r.Build != ""):
		return fmt.Errorf("base static has no toolchain: remove install and build, or pick base node")
	case r.Shape == "static" && r.Static == "":
		return fmt.Errorf("a static app needs static, the folder its files are in")
	case r.Shape == "static" && (r.Start != "" || r.Migrate != "" || r.Port != 0 ||
		len(r.Persistent)+len(r.Secrets)+len(r.Schedules)+len(r.MachinePaths) > 0):
		return fmt.Errorf("a static app has no server: remove start, migrate, port, persistent, secrets, schedules and machine_paths")
	case r.Shape != "static" && r.Start == "":
		return fmt.Errorf("start is required: the command that runs the server")
	case r.Shape != "static" && (r.Port < 1 || r.Port > 65535):
		return fmt.Errorf("port is required: the port the server listens on, which it gets in $PORT")
	case r.Static != "" && !fs.ValidPath(r.Static):
		return fmt.Errorf("static %q must be a folder inside the app, like dist", r.Static)
	case r.Shape == "standard" && r.Static == ".":
		return fmt.Errorf("a standard app's static folder cannot be the app's root, which would serve its code and settings: build the front end into a folder like dist")
	}
	for _, p := range r.Persistent {
		if p == "." || !fs.ValidPath(p) {
			return fmt.Errorf("persistent path %q must be a folder or file inside the app, like uploads", p)
		}
	}
	for _, n := range r.BuildSettings {
		if !envName.MatchString(n) || PublicPrefix(n) == "" {
			return fmt.Errorf("build setting %q must start with one of %s; anything else is a secret", n, strings.Join(publicPrefixes, ", "))
		}
	}
	for _, n := range r.Secrets {
		switch {
		case !envName.MatchString(n):
			return fmt.Errorf("secret %q must be an environment variable name like STRIPE_KEY", n)
		case PublicPrefix(n) != "":
			return fmt.Errorf("secret %q has a public prefix, so a front-end build would put it in the browser", n)
		case slices.Contains(reserved, n) || strings.HasPrefix(n, "LLOCAL_"):
			return fmt.Errorf("secret %q is a name llocal sets itself", n)
		}
	}
	for _, s := range r.Schedules {
		f := strings.Fields(s.Cron)
		if !strings.HasPrefix(s.Path, "/") || len(f) != 5 || slices.ContainsFunc(f, func(x string) bool { return !cronField.MatchString(x) }) {
			return fmt.Errorf("schedule %q %q: path must start with / and cron must be five numeric fields, like \"0 6 * * *\"", s.Path, s.Cron)
		}
	}
	for _, p := range r.MachinePaths {
		if !strings.HasPrefix(p, "/") {
			return fmt.Errorf("machine path %q must start with /", p)
		}
	}
	return nil
}

// HealthPath is the path that answers below 500 once the app is up.
func (r *Recipe) HealthPath() string {
	if r.Shape == "standard" {
		return "/healthz"
	}
	return "/"
}

// PublicPrefix is the public prefix a setting's name starts with, like VITE_, which puts its
// value in the pages a front-end build makes, or "" for a name the browser never sees.
func PublicPrefix(name string) string {
	i := slices.IndexFunc(publicPrefixes, func(p string) bool { return strings.HasPrefix(name, p) })
	if i < 0 {
		return ""
	}
	return publicPrefixes[i]
}
