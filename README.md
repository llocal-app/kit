# llocal's kit

A small framework for the apps a team builds for itself: an inventory list, a job board, a
quote calculator. An app on the kit is one Go server, one SQLite file and a front end of
plain files. It has no login of its own: something in front of it signs people in and tells
it who they are.

The kit is what [llocal](https://llocal.app) hosts. Everything here also runs without llocal.

## One project, before and after

[fastapi/full-stack-fastapi-template](https://github.com/fastapi/full-stack-fastapi-template)
(MIT licence; not affiliated with llocal) is a React front end on a FastAPI server with
Postgres. llocal measured it as shipped, and again with its back end rewritten on the kit
by an AI agent (Claude Code, from llocal's public instructions) in about ten minutes: one
Go server and SQLite, the same front end.

| | as shipped (4 workers) | as shipped, 1 worker | rewritten on the kit |
|---|---|---|---|
| start to first good answer | 8,736 ms | 1,904 ms | 226 ms |
| memory at rest (RSS) | 599.7 MB, and Postgres 89.7 MB | 120.6 MB, and Postgres 89.5 MB | 16.0 MB |
| list of 50 items, 1 connection | 140 a second, median 6.7 ms | 139 a second, median 6.9 ms | 1,122 a second, median 0.8 ms |
| list of 50 items, 10 connections | 152 a second, median 62.8 ms | 192 a second, median 49.5 ms | 1,613 a second, median 5.7 ms |
| processor time per list request | 6.57 ms, and Postgres 0.90 ms | 5.21 ms, and Postgres 1.01 ms | 0.62 ms |
| processor time per new item | 6.14 ms, and Postgres 1.24 ms | 4.63 ms, and Postgres 1.34 ms | 0.90 ms |

Each app had one CPU and 512 MB of memory (and 512 MB of swap), in Docker on an Intel Mac.
Each figure is the median of three runs. Before you read anything into it:

- **The original's database had a processor of its own.** Postgres ran beside it on a
  second CPU; the rewrite did everything on one.
- **The rewrite was slower at the slow end for writes under load.** Creating items over 10
  connections, the slowest 1% took over 636 ms (the original: 96 ms), and over 50, 1,839 ms
  (601 ms); 8 of 64,913 creates failed with a server error, where the original had none. The
  app didn't log why. The likely causes: the rewritten server wrote to the database on every
  request, reads included, and the kit let writes poll SQLite's lock, which under load makes
  a few wait past its limit and fail. Both are fixed since: the instructions and the checker
  now forbid a read that writes, and the kit queues writes in order. This was not measured
  again.
- **The agent removed features and asked about them.** Besides the login screens llocal
  replaces, it removed the admin screen for managing users and the superuser's view of
  everyone's items, and asked the person to confirm. Its new users table also couldn't take
  the old rows, which the checker now catches.
- The original's four workers share memory that RSS counts in each (by PSS, 495.9 MB). The
  original checks its own sign-in token against Postgres on each request; the rewrite checks
  a signed header. llocal's own servers, ARM machines that run apps under gVisor, may give
  other figures.

That is one project, and nothing here is claimed for any other. The method, every figure
and their ranges are in [BENCHMARK.md](BENCHMARK.md).

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
