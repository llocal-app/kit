package survey

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func routeList(s Survey) []string {
	var got []string
	for _, r := range s.Routes {
		got = append(got, r.Method+" "+r.Path)
	}
	slices.Sort(got)
	return got
}

// TestRoutePrefixes: a route's path carries the prefixes of the routers it sits under, all
// the way up, wherever they are added and however the prefix is written.
func TestRoutePrefixes(t *testing.T) {
	for fixture, want := range map[string][]string{
		// A constant prefix from the settings, added to a router that adds the others.
		"fastapi-prefix": {
			"DELETE /api/v1/items/{item_id}", "GET /api/v1/items", "GET /api/v1/items/{item_id}",
			"GET /api/v1/utils/health-check/", "POST /api/v1/items", "POST /api/v1/login/access-token",
		},
		// A prefix given when registering replaces the blueprint's own.
		"flask-prefix": {
			"GET /api/orders", "GET /api/orders/{order_id}", "GET /health/ping", "GET /reports/monthly", "POST /api/orders",
		},
		"express-prefix": {
			"GET /api/orders", "GET /api/orders/{id}", "GET /api/orders/{id}/lines", "GET /api/status", "POST /api/orders",
		},
		"go-prefix": {
			"DELETE /api/v1/parts/{id}", "GET /admin/users", "GET /api/v1/parts", "GET /api/v1/parts/{id}", "GET /healthz", "POST /api/v1/parts",
		},
	} {
		if got := routeList(run(t, filepath.Join("testdata", fixture))); !slices.Equal(got, want) {
			t.Errorf("%s: routes %q, want %q", fixture, got, want)
		}
	}
}

// TestPrefixesStayQuick: files near the size cap, full of calls and blocks left open, of
// routers added to each other in a loop, of a huge prefix and of tables inheriting from
// each other, are still read quickly, with short paths.
func TestPrefixesStayQuick(t *testing.T) {
	dir := t.TempDir()
	n := int(maxFileBytes) / 128
	write(t, dir, map[string]string{
		"requirements.txt": "fastapi\n",
		"main.py":          strings.Repeat("from a import (\nx.include_router(y, prefix=f\"{A}/a\"\ny.include_router(x)\n", n) + "@x.get(\"/p\")\ndef p(): ...\n",
		"main.go":          strings.Repeat("r.Route(\"/a\", func(r chi.Router) {\n\tr.Get(\"/b\", h)\n", n),
		"server.js":        "const express = require('express')\n" + strings.Repeat("app.use('/a', router)\nrouter.use('/b', app)\nrouter.get('/c', h)\n", n),
		"long.js": "const express = require('express')\napp.use('/" + strings.Repeat("x", 100<<10) + "', router)\nrouter.use('/b', app)\n" +
			strings.Repeat("router.get('/c', h)\n", n),
		"models.py": "from sqlmodel import SQLModel, Field\n" + chain(n/2),
	})
	start := time.Now()
	s := run(t, dir)
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("took %s", d)
	}
	for _, r := range s.Routes {
		if len(r.Path) > maxPrefix+100 {
			t.Errorf("route path of %d bytes", len(r.Path))
		}
	}
}

// chain is n SQLModel tables, each inheriting the columns of the one after it.
func chain(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "class M%d(M%d, table=True):\n    a%d: int = Field()\n    b%d: str\n", i, i+1, i, i)
	}
	return b.String()
}

// TestGoGroups: routes on a group or subrouter variable sit under its prefix.
func TestGoGroups(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"go.mod": "module shop\n\ngo 1.24.0\n",
		"gin.go": "package main\n\nfunc gin() {\n\tr := newEngine()\n\tv1 := r.Group(\"/v1\")\n\t{\n\t\tv1.GET(\"/orders/:id\", one)\n\t}\n\tadmin := v1.Group(\"/admin\")\n\tadmin.POST(\"/refunds\", refund)\n}\n",
		"mux.go": "package main\n\nfunc gorilla() {\n\tr := mux.NewRouter()\n\tapi := r.PathPrefix(\"/api/\").Subrouter()\n\tapi.HandleFunc(\"/stock\", stock).Methods(\"GET\")\n}\n",
	})
	if got, want := routeList(run(t, dir)), []string{" /api/stock", "GET /v1/orders/{id}", "POST /v1/admin/refunds"}; !slices.Equal(got, want) {
		t.Errorf("routes %q, want %q", got, want)
	}
}
