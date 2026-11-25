# llocal's kit

A small framework for the apps a team builds for itself: an inventory list, a job board, a
quote calculator. An app on the kit is one Go server, one SQLite file and a front end of
plain files. It has no login of its own: something in front of it signs people in and tells
it who they are.

The kit is what [llocal](https://llocal.app) hosts. Everything here also runs without llocal.

## The shape

- **A static front end**, built to plain files (React, Vue, plain HTML, anything that builds
  to a folder). The server sends it, with `index.html` for any path that is not a file, so
  the front end's own router works.
- **One Go server** on the kit. Its routes live under `/api/`. It reads its settings from the
  environment and `llocal.json`, applies the database's migrations at start, and answers
  `/healthz`.
- **One SQLite file**, `$LLOCAL_DATA/app.db`, with uploaded files beside it in
  `$LLOCAL_DATA/files`. Tables are made by numbered `.sql` files in `db/migrations`.
- **No login screen.** The kit checks a signed token on each request and hands the route the
  person: `kit.CurrentUser(r)`, with a stable id, email, name, groups and company.
- **`llocal.json`** at the root says how to build and start it and what it needs: secrets by
  name, outside services, schedules, webhook paths.

[STANDARD.md](STANDARD.md) is the whole of it, with the rules an app keeps and how to move an
existing app onto it (Supabase, Firebase, Postgres, Express, Flask, Next.js).

## Why it stays fast on little

- One process. There is no database server to run beside the app: SQLite is a file the
  server opens itself, so a query never crosses a network.
- The database runs in WAL mode, so reading never waits for writing, and writers queue for
  their turn instead of failing.
- No sign-in code, sessions or user tables to run: sign-in happens once, in front of every
  app.
- The front end is files. The server only answers the API.
- The server is one binary, and the SQLite driver is pure Go, so there is no C toolchain to
  build with and nothing else to install where it runs.
- No timers inside the server. Scheduled work is a route that something calls on a
  timetable, so an app nobody is using can be stopped and started again when someone is.

## Start an app

You need Go (1.24 or newer) and macOS, Linux or WSL.

```bash
git clone https://github.com/llocal-app/kit
cp -r kit/starter notes && cd notes
go mod init notes
go get llocal.app/kit
go run llocal.app/kit/cmd/llocal-dev
```

Open http://localhost:8000. The starter is a page of notes: `main.go` has its two routes,
`db/migrations/001_notes.sql` its table and `web/` its front end. Build yours from there.
Keep `LLOCAL-KIT-LICENSE`, which covers the code that came from the kit; the rest of the app
is yours.

`llocal-dev` is for your own computer only. It builds and starts the app as `llocal.json`
says, sets `PORT`, `HOST`, `LLOCAL_URL`, `LLOCAL_DATA` and `LLOCAL_JWKS` (a key set of its
own), and puts a stand-in for sign-in in front of it at http://localhost:8000 that signs
every request in as one test person. `TZ` and the app's secrets come from your shell. Anyone
who can reach that address is that person, so it listens on this computer only.

```bash
go run llocal.app/kit/cmd/llocal-dev -name "Dana Ruiz" -email dana@example.com -groups finance
```

Its data stays between runs, in a folder named after the app's folder that `llocal-dev`
prints; `-data` picks another.

## Check it

```bash
go run llocal.app/kit/cmd/llocal-check .
```

The checker runs the app the way llocal does and prints one line per check, each failure
with what to do about it: it builds, starts on `$PORT` on every interface, writes only under
`$LLOCAL_DATA`, makes its tables, has no login screen of its own, and its pages and routes
answer without errors.

To bring an existing app, `go run llocal.app/kit/cmd/llocal-survey .` describes it (its
framework, pages, routes, database, login and the keys it reads, never their values), and
`-plan` says in plain words what is left before it fits the shape, and what has to change.

## Run it yourself

Build it with its `build` command (`go build -o server .` for the starter), then start it
with:

| setting | what it is |
|---------|-----------|
| `PORT`, `HOST` | where it listens, like `8080` and `0.0.0.0` |
| `LLOCAL_URL` | its public address, like `https://notes.example.com` |
| `LLOCAL_DATA` | the folder for its database and files |
| `LLOCAL_JWKS` | the public keys it trusts to say who is signed in |
| `TZ` | its time zone, like `America/Denver` |
| each name in `secrets` | that secret's value |

What it needs in front of it is sign-in. The kit trusts a request's person only from the
`X-LLocal-Identity` header: a token signed with Ed25519 by a key in `LLOCAL_JWKS`, made for
the app's host name, with an expiry ([identity](identity/identity.go) has the
format, and `identity.Sign` makes one). A request without it reaches the app with nobody
signed in. So, before anyone else can reach the app, put a sign-in proxy in front of it that:

- signs people in,
- removes any `X-LLocal-*` header a visitor sends,
- adds the token for the person, signed with its own key, whose public half is the app's
  `LLOCAL_JWKS`,
- answers `/_llocal/me` (who is signed in, for the front end) and `/_llocal/logout`.

Or change `identify` in [kit.go](kit.go) to trust your proxy's own header. An app that keeps
its own server instead of the kit's reads the plain headers `X-LLocal-User-Id`,
`X-LLocal-User-Email`, `X-LLocal-User-Name` and `X-LLocal-Groups`, which the proxy sets the
same way.

## On llocal

llocal hosts apps in this shape at an address like `inventory.acme.llocal.app`, with sign-in
in front: people sign in before they reach the app, and the app is told who they are. Its
owners choose who can open it: chosen people, groups, or everyone at the company. The
company's IT sees every app, who owns it, who can open it and the audit log, in one place.
See [llocal.app](https://llocal.app).

## For AI agents

[STANDARD.md](STANDARD.md) is written for the agent doing the work, and
[agent/](agent) has what each AI tool is given: the steps to host on llocal, a Claude Code
plugin, a Cursor rule and an `AGENTS.md` snippet. In Claude Code:

```bash
claude plugin marketplace add llocal-app/kit
claude plugin install llocal@llocal
```

## Versions

Each version is a tag, like `v0.1.0`. `go get llocal.app/kit@latest` takes the newest;
`go get llocal.app/kit@v0.1.0` a given one.

## License

MIT, Copyright (c) 2026 Genfit LLC. See [LICENSE](LICENSE).
