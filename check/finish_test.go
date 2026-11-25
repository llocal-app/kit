package check

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// tree writes files and links into a new folder: a path mapped to "-> target" is a link.
func tree(t *testing.T, files map[string]string) string {
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		var err error
		if to, ok := strings.CutPrefix(body, "-> "); ok {
			err = os.Symlink(to, p)
		} else {
			err = os.WriteFile(p, []byte(body), 0o755)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func links(t *testing.T, dir string) []string {
	var found []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type() == fs.ModeSymlink {
			found = append(found, p)
		}
		return err
	})
	return found
}

func output(t *testing.T, file string) string {
	out, err := exec.Command(file).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v %s", file, err, out)
	}
	return strings.TrimSpace(string(out))
}

// An npm workspace links the workspace into node_modules, and npm links each command into
// node_modules/.bin: they become a copy and scripts, and the commands still run their files.
func TestFinishTurnsLinksInTheAppIntoWhatTheyPointTo(t *testing.T) {
	dir := tree(t, map[string]string{
		"llocal.json":                                 `{"shape": "kept", "base": "node", "install": "npm ci", "start": "node server.js", "port": 3000}`,
		"frontend/package.json":                       `{"name": "frontend"}`,
		"frontend/node_modules/vite/bin/vite.js":      "#!/bin/sh\necho vite in $(dirname $0)\n",
		"frontend/node_modules/.bin/vite":             "-> ../vite/bin/vite.js",
		"node_modules/frontend":                       "-> ../frontend",
		"node_modules/tool/cli.sh":                    "#!/bin/sh\necho tool\n",
		"node_modules/.bin/tool":                      "-> ../tool/cli.sh",
		"node_modules/.pnpm/tool@1/node_modules/tool": "-> ../../../tool",
		"node_modules/alias":                          "-> .pnpm/tool@1/node_modules/tool",
	})
	if err := Finish(dir); err != nil {
		t.Fatal(err)
	}
	if l := links(t, dir); len(l) > 0 {
		t.Fatalf("links left: %v", l)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "node_modules/frontend/package.json")); err != nil || string(b) != `{"name": "frontend"}` {
		t.Fatalf("the workspace wasn't copied: %q %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "node_modules/alias/cli.sh")); err != nil || !strings.Contains(string(b), "echo tool") {
		t.Fatalf("a link through links wasn't followed: %q %v", b, err)
	}
	if got := output(t, filepath.Join(dir, "node_modules/.bin/tool")); got != "tool" {
		t.Errorf("tool printed %q", got)
	}
	for _, bin := range []string{"frontend/node_modules/.bin/vite", "node_modules/frontend/node_modules/.bin/vite"} {
		if got := output(t, filepath.Join(dir, bin)); !strings.HasSuffix(got, "frontend/node_modules/vite/bin") {
			t.Errorf("%s ran from %q, not where its file is", bin, got)
		}
	}
}

// A standard app's node_modules goes, links and all, once what links into it is copied.
func TestFinishDropsAStandardAppsNodeModules(t *testing.T) {
	dir := tree(t, map[string]string{
		"llocal.json":                   `{"shape": "standard", "base": "go", "start": "./server", "port": 8080, "static": "web"}`,
		"node_modules/lib/dist/lib.css": "body {}",
		"node_modules/.bin/x":           "-> /usr/bin/env",
		"web/lib.css":                   "-> ../node_modules/lib/dist/lib.css",
	})
	if err := Finish(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "node_modules")); err == nil {
		t.Error("node_modules is still there")
	}
	if b, err := os.ReadFile(filepath.Join(dir, "web/lib.css")); err != nil || string(b) != "body {}" {
		t.Errorf("web/lib.css: %q %v", b, err)
	}
}

// A static app keeps only its static folder, so only links there count.
func TestFinishOfAStaticAppLooksOnlyInItsFolder(t *testing.T) {
	dir := tree(t, map[string]string{
		"llocal.json":       `{"shape": "static", "base": "node", "build": "vite build", "static": "dist"}`,
		"node_modules/x":    "-> /usr/bin/env",
		"src/logo.svg":      "<svg/>",
		"dist/index.html":   "<p>hi</p>",
		"dist/img/logo.svg": "-> ../../src/logo.svg",
	})
	if err := Finish(dir); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "dist/img/logo.svg")); err != nil || string(b) != "<svg/>" {
		t.Errorf("dist/img/logo.svg: %q %v", b, err)
	}
}

func TestFinishRefusesLinksItCantKeep(t *testing.T) {
	recipe := `{"shape": "kept", "base": "node", "start": "node server.js", "port": 3000}`
	maxCopy = 1 << 20
	defer func() { maxCopy = 1 << 30 }()
	for _, tc := range []struct {
		name   string
		files  map[string]string
		reason string
	}{
		{"absolute", map[string]string{"db": "-> /var/lib/llocal/global/auth.db"},
			"db is a link to /var/lib/llocal/global/auth.db, an absolute path: llocal copies in what a link points to only inside the app, by a relative path, so put the file itself there"},
		{"outside", map[string]string{"lib/shared": "-> ../../shared"},
			"lib/shared is a link to ../../shared, outside the app: llocal copies in what a link points to only inside the app, so put the file itself there"},
		{"to nothing", map[string]string{"node_modules/.bin/gone": "-> ../gone/cli.js"},
			"node_modules/.bin/gone is a link to ../gone/cli.js, which isn't there: remove the link, or have the build make what it points to"},
		{"through a file", map[string]string{"a.txt": "a", "b": "-> a.txt/c"},
			"b is a link to a.txt/c, which isn't there"},
		{"to its own folder", map[string]string{"pkg/index.js": "", "pkg/self": "-> ."},
			"pkg/self is a link to ., a folder that holds the link, so a copy would never end: remove the link"},
		{"to the app", map[string]string{"pkg/up": "-> .."},
			"pkg/up is a link to .., a folder that holds the link"},
		{"between two folders", map[string]string{"a/to-b": "-> ../b", "b/to-a": "-> ../a"},
			"a folder that holds the link"},
		{"round in a circle", map[string]string{"a": "-> b", "b": "-> a"},
			"a is a link to b, which leads round in a circle: remove the link"},
		{"too much to copy", map[string]string{"big/blob": strings.Repeat("x", 600<<10), "c1": "-> big", "c2": "-> big"},
			"the links the build left point to more than 1 MB in all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.files["llocal.json"] = recipe
			err := Finish(tree(t, tc.files))
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("got %v, want %q", err, tc.reason)
			}
		})
	}
}
