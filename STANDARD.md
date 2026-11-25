# llocal's standard shape

This is how an app has to look to run on llocal, and how to get it there. It is written for
whoever does the work: the builder's own agent, llocal's agent, or a person. Follow it
exactly. If the app cannot be made to fit, say which rule it breaks and why, and stop.

The conversion needs a strong model (Claude Sonnet or better). A weaker one should stop
here and say so.

## The shape

A hosted app is:

- its own front end, built to plain files;
- one Go server built on llocal's kit (`llocal.app/kit`), or the app's own server, kept
  when porting it would be a big rewrite;
- one SQLite file at `$LLOCAL_DATA/app.db`, and uploaded files under `$LLOCAL_DATA/files`
  (`$LLOCAL_DATA` is `/data` on llocal);
- no login of its own: llocal signs people in and tells the app who is there;
- `llocal.json` at the root, saying how to build it, start it and what it needs.

There are three shapes, named in `llocal.json`:

| shape | what it is | use it when |
|-------|-----------|-------------|
| `static` | only files, no server | the app is a single HTML file or a front end that needs no server of its own |
| `standard` | a Go server on the kit, SQLite | everything else, unless porting the server is a big rewrite |
| `kept` | the app's own server, kept, with its data moved to SQLite | the server is large or unusual; say why it was kept |

The smallest standard app is `kit/starter`. Start conversions from it.

## Getting there

An existing app comes onto the shape in rounds: scan it, change what is left, check it,
upload it, and repeat.

1. **Scan.** The survey ("The survey", below) says what is left before the app fits, each
   thing with its `kind` and a sentence. llocal scans every upload the same way, for free,
   and says what is left and how much of what was left at the upload before is done.
2. **Change** what is left, the biggest first: the server and its data, then the login,
   keys, timers, webhooks and files, and `llocal.json` last. Make the smallest change that
   does each, by the rules and the examples below.
<!-- unpublished
3. **Check** it: until the kit is public, llocal runs `llocal-check` itself when it builds
   ("Check it", below).
-->
3. **Check** it with `llocal-check` ("Check it", below), and fix every `FAIL` line.
<!-- /unpublished -->
4. **Upload** it, and llocal scans it again. Once an upload has a valid `llocal.json`,
   llocal builds it, runs the same checker and starts it.

llocal's own AI can do the last mile on llocal's side, for free, once what is left is within
an app's free allowance for llocal's AI.

## Rules

1. **Listen on `$PORT` on all interfaces** (`0.0.0.0`, which `$HOST` also says). Never a
   fixed port, never `127.0.0.1` or `localhost`.
2. **Write only under `$LLOCAL_DATA` and `/tmp`.** The app's own folder is read-only on
   llocal, or thrown away on restart. A kept app that must write inside its folder lists
   that path under `persistent` and llocal links it into `/data`.
3. **One database: SQLite at `$LLOCAL_DATA/app.db`.** No Postgres, MySQL, Supabase,
   Firebase or MongoDB, from the server or from the browser. The old database's data is
   copied in once by llocal; the app never connects to it again.
4. **Tables are made by migrations.** Each change to the schema is a new numbered `.sql`
   file in `db/migrations` (`001_init.sql`, `002_add_due_date.sql`), applied once, in order,
   at start. Never edit a migration that has run; add one. Table names stay the same as in
   the old database, so the data copy can match them.
5. **No login screen, sign-up, password reset or session handling of its own.** Delete
   them. llocal signs people in before a request reaches the app. The app reads who is
   there (see "Who is signed in").
6. **Key people on the llocal id, never on email.** Email is a contact detail that can
   change.
7. **Outside services that are not the database stay** (payments, AI, accounting, text
   messages, mail). Their keys are secrets named in `llocal.json`, read from the
   environment. Never commit a key.
8. **No timers in the server.** Apps sleep when nobody uses them, so `setInterval`,
   node-cron, APScheduler and goroutine tickers stop. Scheduled work becomes a route plus a
   `schedules` entry; llocal sends a `POST` to the route on its timetable.
9. **Webhooks** (Stripe, Twilio, GitHub) get a route listed in `machine_paths`. The app
   still checks the sender's signature with its own secret.
10. **Keep the app's API paths.** If the front end called `/api/orders`, the new server
    answers `/api/orders`, so the front end changes as little as possible. Server routes
    live under `/api/`. `/_llocal/` and `/healthz` are taken.

## llocal.json

```json
{
  "shape": "standard",
  "base": "go",
  "install": "npm ci",
  "build": "npm run build && go build -o server .",
  "start": "./server",
  "port": 8080,
  "static": "dist",
  "build_settings": ["VITE_MAPS_KEY"],
  "secrets": ["STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET"],
  "services": ["stripe"],
  "schedules": [{ "path": "/api/tasks/send-reminders", "cron": "0 7 * * 1-5" }],
  "machine_paths": ["/api/webhooks/stripe"]
}
```

| field | meaning |
|-------|---------|
| `shape` | `standard`, `kept` or `static`. Required. A static app has no server, so no `start`, `migrate`, `port`, `persistent`, `secrets`, `schedules` or `machine_paths`. |
| `base` | the stock image it is built and run in: `go` (also has Node, for front-end builds), `node`, `python`, or `static` (no toolchain: no install or build). A standard app is `go`. Required. |
| `install` | installs dependencies, like `npm ci` or `pip install -r requirements.txt`. |
| `build` | builds the app. |
| `start` | runs the server. Required unless static. |
| `migrate` | brings the database up to date before each start. Kit apps leave it out: the kit migrates at start. |
| `port` | the port the server listens on. llocal sets `$PORT` to it. Required unless static. |
| `static` | the folder of built front-end files, like `dist`, `build` or `out` (`.` for a static app that is a single `index.html` at the root). Required for static apps. A kit app serves it, with `index.html` for any path that is not a file. |
| `persistent` | files or folders inside a kept app that must survive restarts, like `uploads`. |
| `build_settings` | public settings the front-end build bakes in. They must start with `VITE_`, `NEXT_PUBLIC_`, `PUBLIC_` or `REACT_APP_`. Anyone can read them in the browser. |
| `secrets` | names of secrets the server reads from its environment. Never a public prefix, and never a name llocal sets itself (`PORT`, `HOST`, `TZ`, `PATH`, `HOME`, `TMPDIR` or one starting `LLOCAL_`). |
| `services` | outside services still used, like `stripe`, `openai`, `quickbooks`, `twilio`. |
| `schedules` | `{ "path", "cron" }`: llocal sends a `POST` to the path on a five-field cron timetable (numbers only) in the app's time zone. |
| `machine_paths` | paths machines may call with an access key, like webhooks. A path also covers the paths below it. |

Commands run with `/bin/sh -c` in the app's root folder. The full schema is
`kit/recipe/llocal.schema.json`; `llocal-check` applies the rules.

## What the app gets from llocal

Environment, at start:

| name | value |
|------|-------|
| `PORT` | the `port` from `llocal.json` |
| `HOST` | `0.0.0.0` |
| `LLOCAL_URL` | the app's public address, like `https://inventory.acme.llocal.app` |
| `LLOCAL_DATA` | `/data`: the database and files live here |
| `LLOCAL_JWKS` | the public keys that sign the identity token |
| `TZ` | the company's time zone, like `America/Denver` |
| each name in `secrets` | the value the builder saved |

Build settings are passed to `install` and `build` only. Secrets never reach the build.

## The kit (Go)

```go
package main

import (
	"net/http"

	"llocal.app/kit"
)

func main() {
	app := kit.New() // reads the environment and llocal.json, opens app.db, runs migrations
	app.Get("/api/orders", listOrders(app))
	app.Post("/api/orders", addOrder(app))
	app.Run() // adds /healthz and the front end, serves until stopped
}

func listOrders(app *kit.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := app.DB.QueryContext(r.Context(), `SELECT id, title FROM orders ORDER BY id DESC`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		// scan rows, then:
		kit.JSON(w, http.StatusOK, orders)
	}
}
```

- `kit.New()` stops the app with a plain message if anything is missing or a migration fails.
- `app` is a chi router: `app.Get`, `app.Post`, `app.Route`, `app.Use` (call `Use` before
  adding routes).
- `app.DB` is the SQLite database (WAL, foreign keys on, writers wait their turn). Use `?`
  placeholders.
- `app.Files` is an `*os.Root` at `$LLOCAL_DATA/files`: names cannot escape it.
  `app.Files.Create(id)`, `app.Files.Open(id)`, `app.Files.Remove(id)`, and
  `http.ServeFileFS(w, r, app.Files.FS(), id)` to send one. Name files by id, not by the
  name the person uploaded.
- `kit.CurrentUser(r)` is the person signed in: `ID`, `Email`, `Name`, `Groups`, `Company`,
  `Guest`. It is nil on machine paths and scheduled calls, and on a page open to the public
  when the visitor is not signed in.
- `kit.JSON(w, status, v)` writes JSON.
- `app.URL` is the app's public address, for links in emails.

<!-- unpublished
The module's `go.mod`, at the app's root, needs `require llocal.app/kit`. The kit isn't public
yet, so it can't be downloaded and a server on it can't be built on the builder's computer;
llocal's builds supply it, whatever version `go.mod` names.
-->
The module's `go.mod`, at the app's root, needs `require llocal.app/kit` (`go get llocal.app/kit`).
<!-- /unpublished -->

## Who is signed in

llocal signs people in before their request reaches the app, and checks who may open
which page. The app never sees a password.

- **Kit apps:** `kit.CurrentUser(r)`. The kit checks the signed `X-LLocal-Identity` token on
  every request.
- **Kept apps:** read the headers llocal adds. llocal removes any that a visitor sends.
  - `X-LLocal-User-Id`: the stable llocal id. Key data on this.
  - `X-LLocal-User-Email`
  - `X-LLocal-User-Name`: percent-encoded (decode with `decodeURIComponent` or
    `urllib.parse.unquote`).
  - `X-LLocal-Groups`: comma-separated, each one percent-encoded.
  - `X-LLocal-Identity`: the same facts as a signed token (an EdDSA JWT, keys in
    `$LLOCAL_JWKS`), for apps that want to check it themselves.
- **The front end:** `GET /_llocal/me` answers
  `{"id", "email", "name", "groups", "company", "guest"}`. "Sign out" is a link to
  `/_llocal/logout`.

### People and their old accounts

The app's old users table stays, as profile data. Add a `llocal_id` column. The first time
someone signs in, find their old row by email and record their llocal id; after that, use
only the llocal id.

```sql
-- db/migrations/002_llocal_id.sql
ALTER TABLE users ADD COLUMN llocal_id TEXT;
CREATE UNIQUE INDEX users_llocal_id ON users (llocal_id);
```

```go
// userID is the app's own id for the person signed in.
func userID(r *http.Request, db *sql.DB) (string, error) {
	u := kit.CurrentUser(r)
	var id string
	err := db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE llocal_id = ?`, u.ID).Scan(&id)
	if err != sql.ErrNoRows {
		return id, err
	}
	err = db.QueryRowContext(r.Context(), `UPDATE users SET llocal_id = ?
		WHERE llocal_id IS NULL AND lower(email) = lower(?) RETURNING id`, u.ID, u.Email).Scan(&id)
	if err != sql.ErrNoRows {
		return id, err
	}
	err = db.QueryRowContext(r.Context(), `INSERT INTO users (id, email, name, llocal_id)
		VALUES (?, ?, ?, ?) RETURNING id`, u.ID, u.Email, u.Name, u.ID).Scan(&id)
	return id, err
}
```

### Who may do what

Rules the old database enforced (Supabase or Postgres row-level security, Firebase rules)
become explicit checks in the server, on every route that needs them. Write each one down
in plain words for the builder to confirm, like "people see only the jobs assigned to
them" or "only the finance group can delete invoices"
(`slices.Contains(kit.CurrentUser(r).Groups, "finance")`).

## SQLite in place of Postgres or MySQL

| was | becomes |
|-----|---------|
| `serial`, `bigserial`, `identity` | `INTEGER PRIMARY KEY` |
| `uuid` | `TEXT` (keep the existing values) |
| `timestamptz`, `timestamp`, `date` | `TEXT` in ISO 8601 UTC, like `2026-11-20T15:04:05Z` |
| `now()` | `strftime('%Y-%m-%dT%H:%M:%SZ', 'now')` |
| `boolean` | `INTEGER` 0 or 1 |
| `json`, `jsonb` | `TEXT` holding JSON (`json_extract` reads it) |
| `numeric(10,2)` for money | `INTEGER` cents |
| arrays | a child table, or `TEXT` holding a JSON array |
| enums | `TEXT` with `CHECK (status IN ('open', 'done'))` |
| `$1`, `:name` placeholders | `?` |
| `ILIKE` | `LIKE` (case-insensitive for ASCII) |
| `RETURNING` | `RETURNING` (works) |

## The data copy

llocal copies the old database's data in once, after the converted app first starts and its
migrations have made its tables. Nobody writes code for it and the app never connects to the
old database: llocal reads it with the address and read key the builder gave (used for the
copy, then thrown away), or from data files in the upload (`.csv`, one per table, or a
SQLite file).

- Every table and column keeps its name, and llocal copies rows by name, with the columns
  both have. A table nothing uses any more (an old login's sessions) still keeps its
  migration; the checker looks for every table the old database had.
- Values arrive as SQLite takes them: timestamps as ISO 8601 UTC text, booleans as 0 or 1,
  JSON and arrays as JSON text, numbers as they were.
- When a table or column changes its name or form, `db/import.sql` says how to fill it. llocal
  runs it in the app's database during the copy, with the old data attached as `old`, and
  copies by name only the tables it leaves empty. Supabase's sign-in users come as
  `old.auth_users` (`id`, `email`, `name`, `created_at`).

```sql
-- db/import.sql
INSERT INTO users (id, email, name) SELECT id, email, name FROM old.auth_users;
INSERT INTO orders (id, customer_id, total_cents)
  SELECT id, customer_id, CAST(round(total * 100) AS INTEGER) FROM old.orders;
```

llocal shows the builder each table's row count before and after. The copy can run again
until the app goes live, always into empty tables; after that llocal's database is the live
one.

## Before and after

### Supabase called from the browser (Vite + React)

Before: the browser talks to Supabase, and row-level security decides what each person sees.

```ts
// src/notes.ts
import { createClient } from '@supabase/supabase-js'
const supabase = createClient(import.meta.env.VITE_SUPABASE_URL, import.meta.env.VITE_SUPABASE_ANON_KEY)

export async function listNotes() {
  const { data, error } = await supabase.from('notes').select('*').order('created_at', { ascending: false })
  if (error) throw error
  return data
}

export const signIn = (email: string, password: string) => supabase.auth.signInWithPassword({ email, password })
```

```sql
create policy "own notes" on notes for select using (auth.uid() = user_id);
```

After: the browser calls the app's own server; the policy is a `WHERE` clause.

```ts
// src/notes.ts
export async function listNotes() {
  const res = await fetch('/api/notes')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
```

```go
// main.go, beside package.json
app.Get("/api/notes", func(w http.ResponseWriter, r *http.Request) {
	me, err := userID(r, app.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Was the policy "own notes": people see only their own notes.
	rows, err := app.DB.QueryContext(r.Context(),
		`SELECT id, body, created_at FROM notes WHERE user_id = ? ORDER BY created_at DESC`, me)
	// scan rows, then kit.JSON(w, http.StatusOK, notes)
})
```

```sql
-- db/migrations/001_init.sql
CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL, name TEXT NOT NULL DEFAULT '');
CREATE TABLE notes (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users (id),
  body TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
```

```json
{ "shape": "standard", "base": "go", "install": "npm ci", "build": "npm run build && go build -o server .",
  "start": "./server", "port": 8080, "static": "dist" }
```

Also: delete the login and sign-up pages and `@supabase/supabase-js`, drop
`VITE_SUPABASE_URL` and `VITE_SUPABASE_ANON_KEY`, and show the name from `/_llocal/me`.
Supabase's `auth.users` becomes the app's `users` table (id and email), so old rows keep
their owner. Supabase Storage files move under `$LLOCAL_DATA/files`.

### Express with Postgres

Before:

```js
const pool = new Pool({ connectionString: process.env.DATABASE_URL })
app.use(session({ secret: process.env.SESSION_SECRET }))
app.use(passport.session())

app.get('/api/jobs', requireLogin, async (req, res) => {
  const { rows } = await pool.query('SELECT * FROM jobs WHERE assigned_to = $1 ORDER BY due_at', [req.user.id])
  res.json(rows)
})

app.listen(3000)
```

After, ported to the kit:

```go
app.Get("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
	me, err := userID(r, app.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, err := app.DB.QueryContext(r.Context(),
		`SELECT id, title, due_at FROM jobs WHERE assigned_to = ? ORDER BY due_at`, me)
	// scan rows, then kit.JSON(w, http.StatusOK, jobs)
})
```

Passport, sessions, `SESSION_SECRET`, the login routes and `DATABASE_URL` go. The tables
become SQLite migrations (see the table above).

If the server is too big to port, keep it: swap `pg` for `better-sqlite3` opened at
`process.env.LLOCAL_DATA + '/app.db'`, take the person from `X-LLocal-User-Id` in place
of `req.user`, listen with `app.listen(process.env.PORT, '0.0.0.0')`, and use:

```json
{ "shape": "kept", "base": "node", "install": "npm ci", "start": "node server.js", "port": 3000,
  "migrate": "node migrate.js" }
```

### Flask with SQLAlchemy

Before:

```python
app.config["SQLALCHEMY_DATABASE_URI"] = os.environ["DATABASE_URL"]
login_manager = LoginManager(app)

@app.route("/orders")
@login_required
def orders():
    return render_template("orders.html", orders=Order.query.filter_by(owner_id=current_user.id).all())

if __name__ == "__main__":
    app.run(port=5000)
```

After, kept (Flask stays, the data moves):

```python
app.config["SQLALCHEMY_DATABASE_URI"] = "sqlite:///" + os.environ["LLOCAL_DATA"] + "/app.db"

def me():
    """The app's own User for the person llocal signed in (linked by email the first time)."""
    llocal_id = request.headers["X-LLocal-User-Id"]
    user = User.query.filter_by(llocal_id=llocal_id).first()
    if user is None:
        email = request.headers["X-LLocal-User-Email"]
        user = User.query.filter(func.lower(User.email) == email.lower(), User.llocal_id.is_(None)).first() or User(email=email)
        user.llocal_id = llocal_id
        db.session.add(user)
        db.session.commit()
    return user

@app.route("/orders")
def orders():
    return render_template("orders.html", orders=Order.query.filter_by(owner_id=me().id).all())
```

```json
{ "shape": "kept", "base": "python", "install": "pip install -r requirements.txt gunicorn",
  "migrate": "flask db upgrade", "start": "gunicorn -b 0.0.0.0:$PORT app:app", "port": 8000 }
```

Flask-Login, the login and register views and templates, and `DATABASE_URL` go. Uploads
save under `os.environ["LLOCAL_DATA"] + "/files"`. A small Flask app with a handful of
routes is better ported to the kit, the same way as Express above.

### Next.js API routes with Prisma

Before:

```ts
// pages/api/tasks.ts
export default async function handler(req, res) {
  const session = await getServerSession(req, res, authOptions)
  if (!session) return res.status(401).end()
  res.json(await prisma.task.findMany({ where: { ownerId: session.user.id } }))
}
```

After, standard: the pages become a static export and the API routes move to Go at the
same paths.

```js
// next.config.js
module.exports = { output: 'export' }
```

```go
app.Get("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
	me, err := userID(r, app.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, err := app.DB.QueryContext(r.Context(), `SELECT id, title, done FROM tasks WHERE owner_id = ?`, me)
	// scan rows, then kit.JSON(w, http.StatusOK, tasks)
})
```

```json
{ "shape": "standard", "base": "go", "install": "npm ci", "build": "npx next build && go build -o server .",
  "start": "./server", "port": 8080, "static": "out" }
```

To write the first migration, set the Prisma datasource to `provider = "sqlite"` and run
`npx prisma migrate diff --from-empty --to-schema-datamodel prisma/schema.prisma --script > db/migrations/001_init.sql`,
then remove Prisma. NextAuth and its pages go. `next/font` works in a static export.

If the app needs a Node server (server-side rendering, middleware, server actions), keep
it: Prisma `provider = "sqlite"`, start with
`DATABASE_URL=file:$LLOCAL_DATA/app.db npx next start -p $PORT -H 0.0.0.0`, migrate with
`DATABASE_URL=file:$LLOCAL_DATA/app.db npx prisma migrate deploy`, and list `.next/cache`
under `persistent`.

## The survey

Before converting, describe the project as it is in `survey.json`: its framework, pages and
server routes, its databases and their tables, its login, the outside services it uses and the
settings it reads (names only, never values). `llocal-survey` writes it by rules, and `-plan`
prints, in plain words, what is left before the app fits this shape (`left`: each thing's
`kind`, like `login`, `database` or `server`, and a sentence; empty when it fits), what llocal
will change and what it needs. An agent may correct or fill the same file; its schema is
`kit/survey/survey.schema.json`. The checker reads its pages, routes, database hosts and tables.

<!-- unpublished
Until the kit is public, `llocal-survey` can't be installed. llocal runs the same survey on
every upload, and an agent connected to llocal gets its plan with `get_plan`, or llocal's plan
for a `survey.json` it wrote with `survey_checklist`.
-->
```bash
go run llocal.app/kit/cmd/llocal-survey@latest . > survey.json
go run llocal.app/kit/cmd/llocal-survey@latest -plan .
```
<!-- /unpublished -->

## Check it

<!-- unpublished
Until the kit is public, `llocal-check` can't be installed: llocal runs it itself on every
version of an app with a server, before it starts, and says what failed.
-->
```bash
go run llocal.app/kit/cmd/llocal-check@latest -survey survey.json .
```
<!-- /unpublished -->

It runs the app the way llocal will and prints one line per check. Every `FAIL` line says
what is wrong and what to do. llocal runs the same checker on every version of an app with a
server before it starts, and after two failed checks, while the app is within its free
allowance, llocal's AI offers to do the conversion itself.

| check | passes when |
|-------|-------------|
| `recipe` | `llocal.json` follows the rules above |
| `install`, `build` | the commands finish |
| `old-database` | no file mentions the old database's host (from the survey) |
| `static` | a static app's folder has `index.html` |
| `login` | a standard app's front end has no password field |
| `migrate` | the migrate command works on an empty database |
| `start` | the app answers its health check on `$PORT` on all interfaces within a minute, as a real start must (`/healthz` for kit apps, `/` otherwise) |
| `pages` | each page in the survey (or `/`) answers below 400 |
| `routes` | each server route in the survey answers below 500 |
| `writes` | the app wrote nothing inside its own folder, outside `persistent` and `node_modules` |
| `tables` | every table in the survey exists in `$LLOCAL_DATA/app.db` |

It also prints the app's memory at idle, how long it took to start, and its size on disk.
Requests are sent signed in as a test person, the way llocal sends them: `GET` for pages, and
each route's method with the JSON body `{}`, which a route answers with 400 when it is missing
something, never 500. Each secret is passed from your environment if set, or a
placeholder. The checker needs macOS or Linux (on Windows, use WSL).

## Keep it in shape

Add this to the project's `CLAUDE.md` or `AGENTS.md`:

```markdown
## Hosting (llocal)

This app runs on llocal in its standard shape; keep it that way (llocal.app/agent.md).
- The server listens on $PORT on 0.0.0.0. Data lives only in SQLite at $LLOCAL_DATA/app.db
  and files under $LLOCAL_DATA/files.
- Schema changes are new numbered files in db/migrations. Never edit one that has run.
- No login code: llocal signs people in. Who is signed in: kit.CurrentUser(r) on the
  server, /_llocal/me in the browser. Key data on the llocal id, never on email.
- New secrets and build settings go in llocal.json by name, never in the code.
- No timers in the server: scheduled work is a route plus a "schedules" entry.
- Before pushing, check it as llocal.app/agent.md says ("4. Check").
```
