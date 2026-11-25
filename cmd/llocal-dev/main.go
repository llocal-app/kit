// llocal-dev runs an app on this computer for development, behind a stand-in for llocal's
// sign-in that signs every request in as one test person. Anyone who can reach its address
// is that person, so it listens on this computer only and is never a way to run an app for
// real. In the app's folder, with its dependencies installed, it runs llocal.json's build,
// migrate and start with the settings llocal gives (a key set of its own in LLOCAL_JWKS),
// and serves the app at -addr with /_llocal/me answered.
//
//	llocal-dev [-addr localhost:8000] [-data dir] [-email a@b.c] [-name "Dev Person"] [-groups a,b]
package main

import (
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"llocal.app/kit/identity"
	"llocal.app/kit/recipe"
)

func main() {
	addr := flag.String("addr", "localhost:8000", "where to serve the app")
	data := flag.String("data", "", "the app's data folder (default: llocal-dev/<folder name> in your cache folder)")
	email := flag.String("email", "dev@example.com", "the test person's email")
	name := flag.String("name", "Dev Person", "the test person's name")
	groups := flag.String("groups", "", "the test person's groups, comma-separated")
	flag.Parse()

	rec, err := recipe.Load("llocal.json")
	if err != nil {
		log.Fatal(err)
	}
	if rec.Shape == "static" {
		log.Fatal("a static app has no server to run: open its files in a browser, or serve its folder with any web server")
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		log.Fatal(err)
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		log.Fatalf("-addr %q: llocal-dev signs in everyone who reaches it, so it listens only on this computer, like localhost:8000", *addr)
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	if *data == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			log.Fatal(err)
		}
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		*data = filepath.Join(cache, "llocal-dev", filepath.Base(wd))
	}
	if err := os.MkdirAll(*data, 0o755); err != nil {
		log.Fatal(err)
	}

	pub, key, _ := ed25519.GenerateKey(nil)
	steps := slices.DeleteFunc([]string{"set -e", rec.Build, rec.Migrate, rec.Start}, func(s string) bool { return s == "" })
	app := exec.Command("/bin/sh", "-c", strings.Join(steps, "\n"))
	app.Stdout, app.Stderr = os.Stdout, os.Stderr
	app.Env = append(os.Environ(),
		"PORT="+strconv.Itoa(rec.Port),
		"HOST=127.0.0.1",
		"LLOCAL_URL=http://"+*addr,
		"LLOCAL_DATA="+*data,
		"LLOCAL_JWKS="+string(identity.Keys{"dev": pub}.JWKS()),
	)
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
	go func() {
		app.Wait()
		log.Fatalf("the app stopped: %v", app.ProcessState)
	}()

	person := identity.Claims{Subject: "u_dev", Audience: host, Email: *email, Name: *name, Groups: []string{}, Company: "c_dev"}
	if *groups != "" {
		person.Groups = strings.Split(*groups, ",")
	}
	to := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(rec.Port))}
	mux := http.NewServeMux()
	mux.Handle("/", &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(to)
		c, now := person, time.Now()
		c.IssuedAt, c.Expires = now.Unix(), now.Add(time.Minute).Unix()
		identity.Set(r.Out.Header, "dev", key, c)
	}})
	mux.HandleFunc("/_llocal/", http.NotFound)
	mux.HandleFunc("GET /_llocal/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": person.Subject, "email": person.Email, "name": person.Name, "groups": person.Groups, "company": person.Company, "guest": person.Guest})
	})
	mux.HandleFunc("/_llocal/logout", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("llocal-dev keeps everyone signed in as " + person.Name + ". On llocal, this signs the person out.\n"))
	})
	log.Printf("the app is at http://%s, signed in as %s <%s>, with its data in %s (development only)", *addr, person.Name, person.Email, *data)
	log.Fatal(http.Serve(ln, mux))
}
