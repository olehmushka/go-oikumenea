# Extracting the external-API connectors into standalone repos

This is a **runbook**, not a binding decision doc — it records how to pull hermenea's
outbound third-party/government-API clients out of this monorepo into their own git
repos, to be executed by hand. Nothing in this file has been executed yet: no new repos
exist, and no code in this repo has changed as a result of writing it.

## Why these particular five

All of go-oikumenea's outbound calls to third-party/government/public-registry APIs
already live in one place — `internal/hermenea/**`, the companion ingestion service
that is deliberately the "external-coupling boundary" (D-Hermenea): the oikumenea core
never calls out, hermenea does, over its own HTTP client, and hands oikumenea
already-mapped data via `POST /import/{objectType}`.

Five real connectors exist there today, all sharing the same `domain.Fetcher` /
`domain.StreamingFetcher` / `domain.Mapper` shape but each hand-rolling its own
`*http.Client`, retries-none discipline, and header/error handling — there is no shared
`pkg/`-style HTTP kernel for them today, only a shared `UserAgent()` helper
(`internal/hermenea/fetcher/fetcher.go:241-250`).

## Boundary design: ports stay, adapters delegate out

`internal/hermenea/domain` keeps owning `Fetcher` / `StreamingFetcher` / `Mapper` /
`PagedMapper` / `watchlist.Provider` — hermenea's own orchestration abstractions, and
the thing every other fetcher/mapper in hermenea already conforms to. The extracted
repos expose plain, hermenea-agnostic Go APIs instead (e.g.
`Client.ScreenName(ctx, name) ([]Notice, error)`). hermenea's existing adapter files
(`interpol.go`, `factbook.go`, the `WOFSQLite` fetcher, `wikidataorgs/mapper.go`)
shrink to thin wrappers that call the extracted client and still satisfy hermenea's
existing interfaces — so `internal/hermenea/module.go`'s wiring barely changes.

This keeps hermenea's domain-specific mapping (e.g. turning a Wikidata SPARQL row into
an `ExternalOrg`) inside hermenea, while the "talk to the real external API" part —
genuinely reusable outside this project — moves out.

## Target repo/package layout

All new repos: `github.com/olehmushka/go-<name>`, created as siblings of
`go-oikumenea/` (i.e. `../go-govapi-core/`, `../go-interpol-client/`, …), one `go.mod`
each — **not** nested modules like `clients/go/`. This repo has no precedent for a
sibling-repo split; its only prior art (`clients/go/go.mod`, see
[`releasing.md`](../releasing.md)) is a nested module still inside the same repo and
git history. A sibling repo is a new pattern here, so lean on `releasing.md`'s lessons
below rather than re-deriving them.

| Repo | Scope | Depends on | Extracted from |
|---|---|---|---|
| `go-govapi-core` | Shared HTTP kernel: timeout-bound `*http.Client` builder (no-retry-by-default, matching `connectorcall`'s discipline), configurable `UserAgent()`, generic single-URL JSON fetch, generic multi-file fetch, shared error wrapping | — (zero deps on the others; same "never imports a leaf" rule as `pkg/`) | `internal/hermenea/fetcher/fetcher.go:62-90` (`HTTP`), `:169-224` (`HTTPFiles`), `:241-250` (`UserAgent`); `internal/connectorcall/connectorcall.go:21-58` (deadline discipline) |
| `go-interpol-client` | INTERPOL Red Notices API client + response DTOs | `go-govapi-core` | `internal/hermenea/watchlist/interpol.go:20-107` |
| `go-factbook-client` | CIA World Factbook (GitHub-mirror tree walk + raw content fetch) client + DTOs | `go-govapi-core` | `internal/hermenea/fetcher/factbook.go:1-165` |
| `go-wof-client` | Who's-On-First gazetteer: per-country `.db.bz2` download/decompress + SQLite query API | `go-govapi-core` | `internal/hermenea/fetcher/fetcher.go:114-163` (`WOFSQLite`) |
| `go-wikidata-client` | Generic Wikidata SPARQL query client (returns raw SPARQL JSON bindings — not oikumenea's `ExternalOrg` shape) | `go-govapi-core` | `internal/hermenea/wikidataorgs/mapper.go:1-105` (split: HTTP part moves, entity-mapping part stays) |

**Not extracted:**
- Glottolog/CLDR fetching stays as hermenea-side config over `go-govapi-core`'s
  generic multi-file fetch — it was already generic (`HTTPFiles`), not
  source-specific, so no separate repo earns its keep.
- The sanctions stub (`internal/hermenea/watchlist/interpol.go:123-131`, OFAC/EU/UN)
  stays in hermenea — it has no real external endpoint to extract yet.

**Dependency rule** to enforce (in review, or a small CI check per repo): **core has no
`require` on any leaf module; every leaf module `require`s core and nothing else.** No
leaf depends on another leaf.

## Per-repo runbook (apply once per repo, `core` first)

1. `mkdir ../go-<name> && cd ../go-<name> && git init`
2. `go mod init github.com/olehmushka/go-<name>` — pin the same Go version as
   go-oikumenea's `go.mod` (`go 1.26.3`) unless there's a specific reason to diverge.
   **Double-check the module path matches where the repo actually lives before the
   first tag** — `releasing.md` records a real incident here: `clients/go/go.mod` once
   declared `github.com/olegamysk/…` while the repo lived at `github.com/olehmushka/…`,
   so the declared path 404'd. A module is fetched by its `go.mod` path, not by the repo
   that was tagged; catching this after a version is tagged means the version number is
   spent with nothing to roll back.
3. Copy the source file(s) listed in the table above into the new repo; strip:
   - any import of `internal/hermenea/domain` or other oikumenea/hermenea packages —
     replace hermenea's `domain.Fetcher`/`Mapper` types with the new package's own
     plain request/response types.
   - hermenea-specific config plumbing (`internal/hermenea/config`) — replace with a
     small `Config` struct owned by the new package (base URL, timeout, API key if
     any), constructed by the caller.
   - inline `&http.Client{Timeout: ...}` construction — build it via
     `go-govapi-core`'s builder instead.
4. Port the existing tests for that connector (check for a `*_test.go` next to each
   source file above) — adapt fixtures, drop any hermenea-domain assertions in favor of
   asserting on the new package's own DTOs.
5. Add a `README.md` (what API, auth, rate limits, an example call) and a license file.
6. Give the new repo its own `scripts/release.sh`, modeled on this repo's (see
   [`releasing.md`](../releasing.md)) but with only what a single-module repo needs:
   `require_clean_tree`, `require_semver`, refuse to re-tag a byte-identical tree, and —
   worth keeping verbatim — the module-path-resolves check from step 2, run again at
   release time so a path drift is caught even if it was fine at `go mod init` time.
7. Tag `v0.1.0`, push. A Go module release **is** the tag — nothing else to upload;
   `proxy.golang.org` serves it directly.

## Wiring hermenea back up to the new packages

1. In `go-oikumenea/go.mod`, add each new module as a `require`. During local
   development, before a repo is pushed/tagged, point at the sibling checkout with a
   `replace` directive:
   ```
   replace github.com/olehmushka/go-govapi-core => ../go-govapi-core
   replace github.com/olehmushka/go-interpol-client => ../go-interpol-client
   ```
   Remove each `replace` line once a real tagged version exists and `go get` that
   version instead — don't ship a dev-only `replace` in a committed `go.mod`.
2. Shrink the existing adapter files to thin wrappers:
   - `internal/hermenea/watchlist/interpol.go` — `Interpol` keeps implementing
     `watchlist.Provider` (`Name()`/`Screen()`, interface at
     `internal/hermenea/watchlist/watchlist.go:32-36`); `Screen`'s body now
     constructs a `go-interpol-client` and maps its DTOs to hermenea's
     `watchlist.Hit`.
   - `internal/hermenea/fetcher/factbook.go` — `Factbook` keeps implementing
     `domain.Fetcher`/`domain.StreamingFetcher`, delegating to `go-factbook-client`.
   - `internal/hermenea/fetcher/fetcher.go`'s `WOFSQLite` (`:114-163`) — delegates to
     `go-wof-client`.
   - `internal/hermenea/wikidataorgs/mapper.go` — keeps implementing `domain.Mapper`
     (interface at `internal/hermenea/domain/hermenea.go:150`); the HTTP/SPARQL part
     delegates to `go-wikidata-client`, the entity-mapping part (SPARQL row →
     `ExternalOrg`) stays here unchanged.
3. `internal/hermenea/module.go:86` (wikidataorgs wiring) and `:100-109` (watchlist
   wiring) — constructor calls change to build the new clients and inject them, but the
   registered interface types (`domain.Mapper`, `watchlist.Provider`) don't change, so
   the rest of `module.go` is untouched.
4. Config keys (`internal/hermenea/config/config.go:35-51`,
   `var/conf/hermenea-install.yml`) stay as-is; they now flow into the new clients'
   `Config` structs instead of being read directly inside the adapter.
5. `go build ./... && go test ./...` at the go-oikumenea root; also re-run any existing
   hermenea integration tests covering watchlist/wikidataorgs/factbook/wof.

## Suggested order of execution

1. **`go-govapi-core` first** — nothing else compiles against it until it exists.
2. **`go-interpol-client` next** — the smallest connector (single endpoint, no
   decompression/paging). Proves the whole pattern — repo scaffold, release script,
   hermenea rewiring, tests green — before repeating it three more times.
3. `go-factbook-client`, `go-wof-client`, `go-wikidata-client` — same recipe, any
   order.

## Verification

- Each new repo: `go build ./... && go test ./...` standalone, with no go-oikumenea
  checkout present — proves it's actually decoupled.
- go-oikumenea: `go build ./... && go vet ./... && go test ./...` at root after
  **each** connector is rewired, not just at the end — catches a broken adapter before
  four more are stacked on top of it.
- `go list -m all` in go-oikumenea shows no `replace` directives left once real tags
  exist.
- Grep `internal/hermenea` for `net/http` / `http.Client{` after rewiring to confirm no
  inline client construction remains outside `go-govapi-core`'s builder.
