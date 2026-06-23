# Run hermenea imports (languages + countries) and drive them from the web UI

## Context

The goal is to run the hermenea ingestion pipeline for **countries** (`geo-countries`) and
**languages** (`language-scheme` + `language-scripts`), and to **trigger / watch progress / check
status from the web console** — not via `curl`.

Today that is impossible from the UI: the Next.js console (`web/`) is a BFF that proxies **only** to
oikumenea (`https://localhost:8443`) using the signed-in user's OIDC bearer. Hermenea is a **separate
service** on `https://localhost:9443`, authenticated by a *different*, static operator secret
(`OIKUMENEA_HERMENEA_TOKEN`). The hermenea Conjure API types are already generated into
`web/src/lib/api/schema.d.ts` (`/hermenea/v1/sources|runs|jobs|sync/{source}`,
`ImportSource`/`ImportRun`/`WorkerJob`), but there is **no proxy route and no page** that use them.

So this work is two parts: **(A)** build a small "Imports" admin surface in the console, and **(B)** a
runbook to bring the stack up locally and run the two imports through it.

Chosen options (from the user): **run mode = local host processes** (not docker compose);
**language source = live upstream fetch** (the seeded `http-files` connector pulling Glottolog
CLDF + CLDR/SIL — needs network).

## Confirmed facts (from exploration)

- Hermenea control/read API (`api/hermenea.conjure.yml`, base-path `/hermenea/v1`, context-path `""`):
  - `POST /hermenea/v1/sync/{source}` → enqueue a job, returns `JobRef {jobId, status}`
  - `GET  /hermenea/v1/sources` → `ImportSource[]` (code, name, objectType, connectorType, locator, cron, enabled)
  - `GET  /hermenea/v1/runs` → `ImportRun[]` (sourceCode, sourceVersion, status `running|succeeded|failed`, created/updated/skipped, startedAt/finishedAt, error)
  - `GET  /hermenea/v1/jobs` → `WorkerJob[]` (jobType, sourceCode, status `queued|running|succeeded|failed|dead`, attempts/maxAttempts, runAfter, lastError)
- Seeded source codes we care about (`var/conf/hermenea-install.yml`): `geo-countries-iso3166`
  (file, bundled, network-free), `glottolog-languoids` (http-files, live), `cldr-language-scripts`
  (http-files, live). **Order matters**: `language-scheme` before `language-scripts` (scripts resolve
  languoids that must already exist); countries are independent.
- **Scheduler gotcha** (`internal/hermenea/domain/hermenea.go` `ScheduleDue`): a never-enqueued source
  is "due" immediately, so on boot the scheduler tick enqueues **every enabled source**. The local
  `var/conf/hermenea-install.yml` enables ~50 `wof-geo-*` `geo-places` sources (large `.db.bz2`
  downloads). These must be disabled for this task.
- Tokens (local dev): `HERMENEA_OIKUMENEA_TOKEN` (hermenea→oikumenea import calls) and
  `OIKUMENEA_HERMENEA_TOKEN` (push trigger into hermenea `/sync`). Hermenea local config:
  `address 0.0.0.0:9443`, `oikumenea base-url https://localhost:8443`.

## Part A — Build the Imports UI (web/)

Reuse the existing BFF + page conventions; mirror `server.ts`/`browser.ts`/`Nav.tsx`.

1. **Hermenea server helper** — new `web/src/lib/api/hermenea.ts` (mirrors `web/src/lib/api/server.ts`):
   - `HERMENEA_BASE_URL = process.env.HERMENEA_BASE_URL ?? "https://localhost:9443"`.
   - `hermeneaForward/Get/Send` like `apiForward/apiGet/apiSend`, but inject the **static**
     `process.env.OIKUMENEA_HERMENEA_TOKEN` as the bearer (server-only secret) instead of the OIDC
     session token. Still require a signed-in session (call `auth()`; 401 if none) so only console
     users can reach it.
2. **Hermenea BFF proxy** — new `web/src/app/api/hermenea/[...path]/route.ts`, a near-copy of
   `web/src/app/api/oikumenea/[...path]/route.ts` but calling `hermeneaForward`. Exposes GET+POST so
   the browser hits `/api/hermenea/hermenea/v1/...` and never holds the operator token.
3. **Browser client** — extend `web/src/lib/api/browser.ts` with `hermeneaGet`/`hermeneaPost` hitting
   `/api/hermenea{path}` (mirror of `bffGet`), for client-side polling + trigger.
4. **Imports page** — new `web/src/app/(dashboard)/imports/page.tsx` (server component): SSR-fetch
   `sources`, `runs`, `jobs` via `hermeneaGet` and render a client child
   `web/src/app/(dashboard)/imports/ImportsClient.tsx` (`"use client"`):
   - **Sources** table with a **Sync** button per source (`POST /hermenea/v1/sync/{code}`); surface
     the three relevant codes prominently. Disable button while its latest job is `queued|running`.
   - **Runs** + **Jobs** tables = the progress/status view (status badge, created/updated/skipped,
     timestamps, error/lastError).
   - **Live polling**: `setInterval` (~2s) calling `hermeneaGet` for `runs`+`jobs` while any job is
     `queued|running`; back off to idle when all settled. (Pattern: client component + `useState`;
     no new deps.)
5. **Nav + i18n** — add `{ href: "/imports", key: "nav.imports" }` to `TOOLS` in
   `web/src/components/Nav.tsx`, and add `nav.imports` (+ any page labels) to the message catalog
   `web/src/lib/messages.ts` for both `eng` and `ukr` (follow the existing key/locale shape).
6. **Web env** — add to `web/.env` (and document in `web/.env.example`):
   `HERMENEA_BASE_URL=https://localhost:9443`, `OIKUMENEA_HERMENEA_TOKEN=<local trigger secret>`
   (must equal hermenea's value), and ensure `NODE_TLS_REJECT_UNAUTHORIZED=0` (self-signed 9443).

Auth note: this gates hermenea behind "any authenticated console user." If operator-only is desired,
add an instance-admin permission check in `hermenea.ts`/`route.ts` — flagged as an open question, not
assumed.

## Part B — Operational runbook (local host-run)

From `/home/user18/projects/go-oikumenea`, `set -a; . ./.env; set +a` first.

1. **Trim hermenea config**: in `var/conf/hermenea-install.yml`, set `enabled: false` on all
   `wof-geo-*` sources (keep `geo-countries-iso3166`, `glottolog-languoids`, `cldr-language-scripts`
   enabled) so the boot scheduler does not flood the worker with WOF downloads. (Optional: also remove
   the `cron:` line on the three keepers to make them trigger-only, so the UI is the sole trigger;
   otherwise the scheduler also auto-fires them on boot — harmless/idempotent.)
2. **Dev infra**: `docker compose -f docker-compose.dev.yml up -d` (Postgres + Keycloak).
3. **Migrations**: `atlas migrate apply --env local` and `atlas migrate apply --env hermenea`
   (hermenea has its own DB + `migrations/hermenea/`).
4. **oikumenea** (terminal A): export `HERMENEA_OIKUMENEA_TOKEN`, then `./godelw run`. Ready check:
   `curl -sk https://localhost:8444/status/readiness`.
5. **hermenea** (terminal B, from repo root so bundled-file locators resolve): export
   `OIKUMENEA_HERMENEA_TOKEN`, then `go run ./cmd/hermenea`. Ready check:
   `curl -sk https://localhost:9444/status/readiness`.
6. **web** (terminal C): in `web/`, `npm install` if needed, `npm run dev` (with the Part A env).
   Sign in via Keycloak, open **/imports**.
7. **Run the imports from the UI**, in order: **Sync `geo-countries-iso3166`** → **Sync
   `glottolog-languoids`** (the ~27k-languoid live load takes ~tens of seconds; the closure rebuild is
   one transaction) → **Sync `cldr-language-scripts`**. Watch the Runs/Jobs tables update live.

## Verification

- **UI**: the Imports page shows each Sync producing a `WorkerJob` `queued → running → succeeded`
  and an `ImportRun` with non-zero `created` (first run) / non-zero `skipped` on a second click of the
  same source (idempotency), `status: succeeded`, no `error`.
- **Type/build**: `cd web && npm run lint && npx tsc --noEmit && npm run build` clean.
- **Data crosscheck** (optional, confirms the UI reflects reality):
  - `psql "$DATABASE_URL" -c "select count(*) from oikumenea.geo_countries where source is not null;"` (~32)
  - `psql "$DATABASE_URL" -c "select count(*) from oikumenea.language_languoids;"` (~27k)
  - `psql "$DATABASE_URL" -c "select count(*) from oikumenea.language_languoid_scripts;"` (>0)
- **Failure path**: with no network, `glottolog-languoids` job should land `failed` with a fetch error
  surfaced in the Jobs `lastError` / Run `error` column in the UI — confirming status reporting works.

## Open questions / risks

- **Auth scope** for the hermenea proxy: any authenticated user vs. instance-admin only (above).
- **Live fetch needs network** from the host running hermenea (GitHub raw + SIL). If it's flaky, the
  fallback is switching the two language sources to `connector-type: file` against the committed
  presets (`deploy/language-presets/glottolog-5.3.json`, `cldr-scripts.json`) — a config-only change.
- **Generated code is not hand-edited**: `schema.d.ts` already has the hermenea types; we only add
  hand-written proxy/page/client code around them.
