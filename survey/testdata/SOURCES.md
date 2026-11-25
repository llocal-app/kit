# Where the fixtures come from

Checked 2026-11-20 against each tool's own documentation and public repositories those tools
created. The fixtures are small projects written for these tests with the same tell-tale files
and patterns; no code was copied. Every key and password in them is fake. `golden/` holds what
the survey and the plan say about each one (`go test ./survey -update` rewrites it).

## What was expected, and what we found

- **Single HTML file** (`html`), **Vite + React** (`vite-react`): as described.
- **Lovable before TanStack Start** (`lovable-vite`): Vite + React + react-router, `lovable-tagger`,
  README "Welcome to your Lovable project", `src/integrations/supabase/{client,types}.ts`,
  `supabase/config.toml` (`project_id`), `supabase/migrations`, `supabase/functions/<name>`.
  Differs: Supabase is not always there (many projects are front end only). The 2024 client
  hard-codes the URL and anon key; later ones read `VITE_SUPABASE_URL` and
  `VITE_SUPABASE_PUBLISHABLE_KEY`.
- **Lovable after** (`lovable-tanstack`): the default for new apps since 2026-05-13. TanStack
  Start (`@tanstack/react-start`, `@lovable.dev/vite-tanstack-config`, nitro), `.lovable/project.json`,
  `src/routes`, server functions (`createServerFn`, `src/lib/*.functions.ts`), server routes
  (`createFileRoute("/api/...")({ server: { handlers } })`, webhooks under `src/routes/api/public/`),
  `client.server.ts` with `SUPABASE_SERVICE_ROLE_KEY`, and since October 2026 `drizzle/migrations`.
  Differs: `.env` is committed on purpose in both eras, not only the old one; it builds for
  Cloudflare Workers by default, so "needs a JavaScript server" rather than "a Node server"
  (Node works with `NITRO_PRESET=node-server`); Google sign-in goes through
  `@lovable.dev/cloud-auth-js`; AI calls go to `ai.gateway.lovable.dev` with `LOVABLE_API_KEY`.
- **Bolt** (`bolt`): `.bolt/config.json` (`"template"`), `.bolt/prompt`, Vite + React + TypeScript,
  `src/lib/supabase.ts` with `VITE_SUPABASE_URL` and `VITE_SUPABASE_ANON_KEY`, migrations under
  `supabase/migrations` opening with a `/* ... */` summary, discarded ones under
  `.bolt/supabase_discarded_migrations` (ignored). Bolt's own database is a Supabase project, so it
  reads the same. Also: downloads seem to unzip into a `project/` folder (llocal's upload
  handles a root one level down); `.env` has been ignored by the starter since 2025-05 but
  still turns up.
- **v0 and Next.js** (`v0`, `nextjs`): Next.js App Router with shadcn, README "bootstrapped with
  [v0]", `.gitignore` starting `# v0 sandbox internal files`, package name `my-v0-project` (now
  mostly `my-project`). Since 2026-06-08 the default database and login is Neon + Drizzle + Better
  Auth; Supabase (`@supabase/ssr`, `NEXT_PUBLIC_SUPABASE_*`) is the other common shape. Server
  actions, route handlers, `proxy.ts` (Next 16) or `middleware.ts`, and AI through the AI Gateway
  (`ai`, `AI_GATEWAY_API_KEY` off Vercel) are usual, so most v0 apps keep their Next.js server.
- **Replit Agent** (`replit`, `replit-agent4`): the usual "Express serving a Vite build,
  DATABASE_URL, Replit's own login" is right only for projects made before Agent 4 (2026-03-11):
  `client/` `server/` `shared/`, `.replit`, `replit.md`, Drizzle on Neon, `server/replitAuth.ts`
  (`REPLIT_DOMAINS`, `REPL_ID`, `ISSUER_URL`). Since then: a pnpm workspace with
  `artifacts/api-server` (Express 5 under `/api`), `artifacts/<app>` (Vite, served by Replit, not by
  Express), `artifacts/mockup-sandbox` (ignored), `lib/db`, and Clerk as the default login (Replit
  Auth only on Enterprise). Replit's own login works only on Replit: the `host_login` case.
- **Express, Flask, FastAPI** (`express`, `flask`, `fastapi`): as described. Replit's Flask apps
  use `pyproject.toml` with uv and no `requirements.txt`.
- **Streamlit with no requirements file** (`streamlit`): confirmed (Replit's Streamlit projects
  also come with only `pyproject.toml`); the survey lists the packages the code imports.
- The cases named before building: `two-services`, `docker`, `office-db`, and `replit` for a
  login only the original host provides.

## What open-source apps showed the survey misreading

Checked 2026-11-24 by surveying popular open-source apps (FastAPI, Next.js, Express, Flask and
react-admin on Supabase), without running them. Each fixture below has the shape the survey
misread, in a small project of its own:

- `sqlmodel`: SQLModel classes without `table=True` are the shapes of requests and replies,
  not tables.
- `github-actions`: a cron under `.github/workflows` times the repository's checks, not the app.
- `service-password`: a password field in the settings for an outside service (a mail server
  here) is not the app's own login. One where the file speaks of signing in, or of a new,
  current or confirmed password, still is.
- `timeout-timer`: a function that, as one of its own statements, sets a timeout to call itself
  again is a timer; a retry that does it from a callback is not.
- `express-chained`: Express routes written `router.route("/x").get(...).post(...)`, with a call
  commented out between them; a `client/` folder the server renders first still runs in the
  browser.
- `sql-not-tables`: SQL in a test file and on a help page makes no table; a table a migration
  renames is counted once, under its new name.
- `react-admin`: react-admin's Supabase data provider (`ra-supabase-core`) reaches Supabase from
  the browser.
- `cloudinary`: an upload posted straight to Cloudinary's API is an outside service.
- `python-node`: a kept Python server whose pages are built with Node. llocal's bases are `go`
  (Go and Node), `node` and `python`, so none builds both.

## Sources

- Lovable: docs.lovable.dev/changelog (2025-10-06 Cloud; 2026-05-18 TanStack Start default from
  2026-05-13; 2026-07-28 upgrade; 2026-10-07 Drizzle), lovable.dev/blog/building-apps-using-tanstack-start
  (2026-06-01), docs.lovable.dev/features/upgrade-to-tanstack-start, /tips-tricks/external-deployment-hosting,
  /features/secrets, /features/cloud.
- Bolt: support.bolt.new (/cloud/database, /integrations/supabase, /bolt-cloud/database/server-functions,
  /bolt-cloud/hosting, /integrations/netlify, /integrations/git, /managing-projects/projects-files),
  supabase.com/blog/bolt-cloud-launch (2025-09-30), github.com/stackblitz/starters (commits
  2025-05-06, 2025-09-26, 2026-10-20).
- v0: v0.app/docs (/github, /databases, /deployments, /ai-models), v0.app/changelog (2025-10-22 to
  2026-09-01), neon.com/docs/guides/vercel-managed-integration, supabase.com/docs/guides/integrations/vercel-marketplace,
  vercel.com/docs/vercel-blob/server-upload, vercel.com/docs/ai-gateway/authentication.
- Replit: docs.replit.com (/features/auth-and-identity/overview and /clerk-auth,
  /features/data-and-storage/development-and-production (2025-12-04) and /connection-details,
  /features/projects-and-artifacts/updating-projects-for-multiple-artifacts (2026-03-11),
  /features/sdks/object-storage-javascript-sdk, /replit-workspace/configuring-repl).
- Frameworks, through Context7: Next.js (App Router file conventions, static export limits),
  TanStack Start (server functions, server routes, and SPA mode, which is why a new Lovable app
  can become standard), Streamlit (multipage apps, st.secrets, st.login).

## Not confirmed

What a zip download from Lovable, Bolt or Replit contains beyond what their GitHub repos show;
whether Lovable's Google sign-in works once the app leaves Lovable Cloud (treated as a login llocal
replaces, not as host-only); whether a builder can get a `LOVABLE_API_KEY` outside Lovable.
