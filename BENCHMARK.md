# One project, before and after

How the figures in the README were taken, and all of them. They are from one trial, on
2026-11-25, of one open-source project:
[fastapi/full-stack-fastapi-template](https://github.com/fastapi/full-stack-fastapi-template)
at commit 77e550a (MIT licence; not affiliated with llocal). Nothing here is claimed for any
other project.

## What was compared

- **Before:** the project as shipped: its React front end and its FastAPI server (Python
  3.14), built with its own `backend/Dockerfile` and run with its own command,
  `fastapi run --workers 4`, with `postgres:18` (its compose file's database) beside it.
  Before the first start, its `scripts/prestart.sh` ran its migrations and made its first
  superuser, as its deployment guide says.
- **Before, one worker:** the same image and code, started with `--workers 1`. As shipped,
  its four workers fill the 512 MB at rest, so this row shows the server without that.
- **After:** the same project with its back end rewritten on the kit by an AI agent (Claude
  Code), working from llocal's public instructions: one Go server (312 lines) at the same
  `/api/v1` paths, its data in SQLite, and the same front end. The agent took 10 minutes 18
  seconds on the clock, 6 minutes of it its own work and the rest waiting for the person's
  go-ahead. It was built as its `llocal.json` says (`npm ci`, then
  `npm run build && go build -o server .`) in a copy of llocal's `go` base image (Go 1.27 and
  Node 24), from what would be uploaded (`git archive HEAD`).

## Conditions

- A MacBook Pro with an Intel Core i9-9880H (8 cores, 16 threads) and 32 GB, under Docker
  Desktop 4.85.0 (engine 29.6.2, cgroup v2, plain runc), whose Linux VM had 16 CPUs and
  7.65 GiB. Other containers were running in the same VM, unpinned, in every run, before and
  after alike.
- Every container that ran an app's code had one CPU, 512 MB of memory and 512 MB of swap:
  `--cpus 1 --memory 512m --memory-swap 1024m`, pinned to one VM CPU. Swap was a disk file.
- A database the app needs (before only) ran beside it in its own container with the same
  limits, pinned to a CPU of its own. So before had two CPUs in all, and after one.
- The after app ran the way llocal runs an app: a read-only root, a 64 MB `/tmp`, an init, no
  capabilities, its data in `/data`, and llocal's environment (`PORT`, `HOST`, `LLOCAL_URL`,
  `LLOCAL_DATA`, `LLOCAL_JWKS`, `TZ`). The before apps ran as their own images run them.
- All containers of a run shared one Docker network with no route out. No host folder was
  mounted into any of them. The passwords and keys the apps demand were random values made
  for each run and thrown away.
- The load generator (oha 1.17.0) and the client that timed the start ran on VM CPUs 8 to
  15, outside both.
- llocal's own servers are ARM machines that run apps under gVisor. These figures are not
  from there, and llocal's sign-in gateway is not in them.

## One run

Three runs of each, each from nothing (fresh containers, an empty database):

1. Start the database (before only) and wait until it is ready.
2. **Start time:** a client asks for `GET /` every 20 ms; the app's container starts. Start
   time is the client's clock at the first `200` minus Docker's start time for the container.
3. **Rest:** 30 seconds with no requests, then memory: for every process in the container,
   the sum of resident memory (RSS) and of proportional memory (PSS, which splits pages that
   processes share, such as forked workers', between them).
4. Sign in and make 50 items. Before: the superuser signs in with the app's own login
   (`POST /api/v1/login/access-token`) and sends its bearer token on each request, which the
   server checks and loads the user for from Postgres. After: each request carries the signed
   identity header llocal's gateway adds for a signed-in person, which the kit checks.
5. Check each request once; warm each up for 3 s at 10 connections, discarded.
6. **Load:** each request for 15 s at 1, 10 and 50 connections (`oha -z 15s -w -c <n>`, with
   keep-alive), reading each container's CPU time before and after.
7. Memory again, including the peak, then remove everything.

**Throughput** is good (2xx) answers a second. **Latency** is over all answers: the median,
and p99, the time 99 in 100 answers came within. **Processor
time per request** is the growth of the container's cgroup CPU time over the 15 s, divided by
its good answers, for the app and for Postgres separately. Each figure below is the median of
the three runs, with the lowest and highest in brackets.

The requests, the same before and after, are what the app's own front end calls:

- page: `GET /`, the front end's HTML
- list: `GET /api/v1/items/?skip=0&limit=100` (the 50 items)
- create: `POST /api/v1/items/`

## Before, as shipped (four workers)

| measure | median (lowest-highest) |
|---|---|
| start to first good answer | 8736 ms (7492-10868) |
| memory at rest, app: RSS | 599.7 MB (598.8-600.1) |
| memory at rest, app: PSS | 495.9 MB (495.8-496.2) |
| peak memory during the run, app | 512.0 MB (512.0-512.0) |
| peak swap during the run, app | 31.2 MB (30.8-38.3) |
| memory at rest, Postgres: RSS | 89.7 MB (87.9-89.9) |
| memory at rest, Postgres: PSS | 35.1 MB (34.9-35.1) |
| peak memory during the run, Postgres | 179.3 MB (178.7-179.8) |

| request | conns | good/s | median ms | p99 ms | processor time per good answer | not 2xx (all runs) |
|---|---|---|---|---|---|---|
| page | 1 | 779 (774-810) | 1.2 (1.2-1.2) | 2.4 (2.4-2.8) | app 1.28 ms (1.23-1.29); Postgres 0.00 ms | 0 |
| page | 10 | 691 (589-749) | 13.4 (9.0-16.0) | 36.4 (28.6-49.1) | app 1.45 ms (1.34-1.70); Postgres 0.00 ms | 0 |
| page | 50 | 724 (690-818) | 65.3 (59.0-71.5) | 112.4 (93.3-117.6) | app 1.38 ms (1.22-1.45); Postgres 0.00 ms | 0 |
| list | 1 | 140 (139-164) | 6.7 (5.9-6.7) | 14.4 (9.9-14.7) | app 6.10 ms (5.09-6.16); Postgres 1.05 ms (0.89-1.05) | 0 |
| list | 10 | 152 (140-175) | 62.8 (55.2-68.9) | 117.0 (95.5-121.4) | app 6.57 ms (5.70-7.11); Postgres 0.90 ms (0.76-0.97) | 0 |
| list | 50 | 155 (140-161) | 312.5 (292.6-336.6) | 673.3 (583.7-728.0) | app 6.47 ms (6.19-7.11); Postgres 1.31 ms (1.21-1.40) | 0 |
| create | 1 | 142 (121-157) | 6.9 (6.0-8.0) | 10.4 (9.9-14.5) | app 5.25 ms (4.72-6.06); Postgres 1.40 ms (1.24-1.60) | 0 |
| create | 10 | 163 (152-183) | 61.1 (52.5-65.5) | 96.1 (91.3-100.2) | app 6.14 ms (5.47-6.58); Postgres 1.24 ms (1.11-1.35) | 0 |
| create | 50 | 153 (140-166) | 312.3 (287.3-339.0) | 601.0 (568.2-637.1) | app 6.51 ms (6.01-7.12); Postgres 1.95 ms (1.80-2.12) | 0 |

As shipped, it sits at its memory limit at rest and keeps about 31 MB in swap. With no swap,
every sign-in was killed for want of memory (its password check needs more than is left).

## Before, one worker

| measure | median (lowest-highest) |
|---|---|
| start to first good answer | 1904 ms (1822-2490) |
| memory at rest, app: RSS | 120.6 MB (120.5-120.9) |
| memory at rest, app: PSS | 120.6 MB (120.5-120.9) |
| peak memory during the run, app | 167.5 MB (167.3-180.8) |
| memory at rest, Postgres: RSS | 89.5 MB (89.1-90.4) |
| memory at rest, Postgres: PSS | 35.1 MB (35.1-35.2) |
| peak memory during the run, Postgres | 113.4 MB (112.7-129.7) |

| request | conns | good/s | median ms | p99 ms | processor time per good answer | not 2xx (all runs) |
|---|---|---|---|---|---|---|
| page | 1 | 813 (675-854) | 1.1 (1.1-1.4) | 2.2 (2.1-2.8) | app 1.23 ms (1.17-1.48); Postgres 0.00 ms | 0 |
| page | 10 | 1023 (961-1041) | 9.5 (9.4-10.0) | 15.9 (15.0-16.9) | app 0.98 ms (0.96-1.04); Postgres 0.00 ms | 0 |
| page | 50 | 1059 (972-1074) | 46.1 (45.6-48.1) | 65.3 (62.4-97.1) | app 0.94 ms (0.93-1.03); Postgres 0.00 ms | 0 |
| list | 1 | 139 (135-162) | 6.9 (5.9-6.9) | 12.2 (9.2-14.8) | app 6.08 ms (5.15-6.21); Postgres 1.05 ms (0.89-1.09) | 0 |
| list | 10 | 192 (182-202) | 49.5 (46.9-52.0) | 101.7 (94.4-113.0) | app 5.21 ms (4.94-5.49); Postgres 1.01 ms (0.95-1.02) | 0 |
| list | 50 | 206 (203-207) | 236.1 (234.0-240.7) | 447.3 (430.1-458.1) | app 4.85 ms (4.82-4.92); Postgres 0.73 ms (0.72-0.74) | 0 |
| create | 1 | 157 (151-158) | 6.1 (6.0-6.4) | 10.1 (9.4-10.7) | app 4.74 ms (4.67-4.93); Postgres 1.25 ms (1.23-1.29) | 0 |
| create | 10 | 216 (212-228) | 42.8 (41.6-43.7) | 91.0 (85.7-91.7) | app 4.63 ms (4.39-4.71); Postgres 1.34 ms (1.31-1.36) | 0 |
| create | 50 | 238 (234-240) | 191.1 (188.3-193.1) | 509.9 (506.5-516.5) | app 4.20 ms (4.17-4.27); Postgres 0.99 ms (0.97-1.00) | 0 |

## After, rewritten on the kit

| measure | median (lowest-highest) |
|---|---|
| start to first good answer | 226 ms (180-260) |
| memory at rest, app: RSS | 16.0 MB (15.9-16.0) |
| memory at rest, app: PSS | 16.0 MB (15.9-16.0) |
| peak memory during the run, app | 426.7 MB (324.9-436.0) |

| request | conns | good/s | median ms | p99 ms | processor time per good answer | not 2xx (all runs) |
|---|---|---|---|---|---|---|
| page | 1 | 3003 (2362-3328) | 0.3 (0.3-0.4) | 0.7 (0.6-0.8) | app 0.25 ms (0.23-0.31) | 0 |
| page | 10 | 5227 (3576-5334) | 1.8 (1.8-2.7) | 5.1 (4.7-8.0) | app 0.19 ms (0.19-0.28) | 0 |
| page | 50 | 5053 (3598-5269) | 9.6 (9.3-13.9) | 23.8 (21.4-30.2) | app 0.20 ms (0.19-0.28) | 0 |
| list | 1 | 1122 (712-1253) | 0.8 (0.7-1.1) | 2.4 (2.0-4.9) | app 0.77 ms (0.70-1.19) | 0 |
| list | 10 | 1613 (959-1720) | 5.7 (5.5-9.4) | 11.8 (10.9-22.6) | app 0.62 ms (0.58-1.04) | 0 |
| list | 50 | 1438 (1361-1750) | 21.5 (17.6-22.5) | 704.0 (576.4-757.6) | app 0.69 ms (0.57-0.73) | 0 |
| create | 1 | 497 (389-507) | 1.9 (1.9-2.4) | 3.4 (3.1-4.7) | app 0.86 ms (0.78-1.04) | 0 |
| create | 10 | 492 (387-503) | 1.9 (1.9-2.5) | 636.0 (535.8-735.3) | app 0.90 ms (0.88-1.17) | 1 |
| create | 50 | 561 (360-575) | 1.8 (1.7-3.2) | 1838.5 (1838.4-2543.6) | app 0.87 ms (0.85-1.47) | 7 |

- The peak memory during the run is mostly the kernel's cache of SQLite's files, which counts
  against a container's limit until the kernel takes it back. The app's own memory after the
  load was 25 MB (RSS 38 MB).
- 8 of 64,913 creates answered 500. The app returned the error without logging it, so the
  cause was not captured; a 20-second burst of 11,154 creates afterwards had none.
- The rewritten server wrote to the database on every API request, reads included (it saved
  the person's row each time). With SQLite's one writer, that is why its median write is fast
  and its slowest 1% slow.

Since then, and not measured again: llocal's instructions say a request that only reads never
writes, and `llocal-check` fails an app whose reads write; the kit queues writes for SQLite's
one writer in the order they come, where before they polled for its lock and, under load, a
few could wait past its 5-second limit and fail; and the kit logs every answer of 500 or more.

## What must be installed

- Before: the app's image, 1.47 GB (its folder 309 MB, of it Python packages 307 MB), and
  `postgres:18`, 474 MB.
- After: the app's folder as built, 604 MB, of it `node_modules` 585 MB, needed only to build
  the front end. What runs is 17.7 MB (the server 16.8 MB, the pages 0.9 MB, migrations), on
  llocal's `go` base image (1.09 GB, shared by every app).

## What the rewrite changed

- **Kept:** the dashboard, items and settings screens; adding, editing and deleting an item
  and renaming yourself, through the screens, with no console errors or failed API calls
  (the same steps were run before and after in a browser).
- **Removed, as llocal's shape asks:** the app's own login, sign-up, password recovery and
  reset, change-password and delete-account screens; llocal signs people in.
- **Removed, and the agent asked the person to confirm:** the admin screen for managing users,
  and the superuser's view of everyone's items. The one rule it wrote is "people see and
  change only their own items".
- **Removed at the person's word:** sending e-mail.
- **The old data:** the person said not to copy it. Copied by name into the new schema anyway,
  the old users failed (the new users table required a column the old one didn't have), and
  their items, which point at them, failed too or arrived with nobody able to see them.
  llocal's checker now tries that copy and fails such a table.

llocal's checker (`llocal-check@v0.1.2`) passed the rewrite: recipe, install, build,
old-database, login, start, pages, routes, writes and tables, with 17.1 MB idle, a 203 ms
start and 540.2 MB on disk. Its survey had missed the `/api/v1` prefix of the routes, so the
routes check asked for paths that don't exist; with the prefix, the real routes also passed.
The survey now reads such prefixes, and the checker fails a route that is gone.
