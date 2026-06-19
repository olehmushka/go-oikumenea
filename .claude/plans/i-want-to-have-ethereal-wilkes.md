# Plan: RID-free company UI (search pickers + position code-gen)

## Context

The M21 company workspace (`web/src/app/(dashboard)/companies/page.tsx`) currently forces the
operator to paste **raw RIDs** into several forms, and to type a position `code` by hand. There are
five raw-RID entry points and one `window.prompt` (verified by exploration):

| Section | Field | Today |
|---|---|---|
| Locations | `locationId` | `<input placeholder="location RID">` |
| Fill position | `personId` | `window.prompt("Person RID to appoint?")` |
| Shareholding | `holderId` (person/company) | `<input placeholder="holder RID">` |
| Founding | `holderId` (person/company) | `<input placeholder="founder RID">` |
| Beneficiary (UBO) | `personId` | `<input placeholder="person RID">` |
| Position create | `code` | free-text (no auto-gen) |

Lists also render RID tails (`personId.slice(-6)`, `locationId.slice(-8)`), so operators see opaque
RIDs on screen too. (Branch/Succession already use a `<select>` of the in-memory company list — fine.)

Goal: replace every raw-RID input with a **searchable picker**, auto-generate position `code` the
same way unit positions and `CreateCompany` already do, and resolve the RID-tail displays to human
labels — removing visible RIDs from the company UI.

**Decisions taken (user):** (1) use **server-side search** for the pickers (debounced query to the
API, like `LanguagePicker`), not client-side filtering; (2) **resolve displays to labels** too.

## Backend changes (server-side search)

The picker hits a `?query=` substring filter. Status per module:

- **company** — *already done*. `GET /company/v1/companies` already has a `query` arg
  (`api/company.conjure.yml:457`) backed by an ILIKE on code/legal_name/short_name
  (`internal/company/adapters/queries/company.sql:72`). No change.

- **person** — *add `query`*. Person list has a **dual read-scope path** (must thread through both):
  - `api/person.conjure.yml` `listPersons`: add `query: optional<string>` query arg.
  - `internal/person/adapters/queries/person.sql` `ListPersons` (`:50`): add
    `AND (@query = '' OR display_name ILIKE '%'||@query||'%' OR code ILIKE '%'||@query||'%'
    OR given ILIKE '%'||@query||'%' OR surname ILIKE '%'||@query||'%')` — mirror the
    `ListCompanies`/`ListLanguages` (`internal/language/adapters/queries/language.sql:13`) shape.
  - Visible path: apply the same predicate to the read-scope union. Simplest server-side option:
    add a filtered variant used by `ListVisiblePersons` (hydration goes through `ListPersonsByIDs`),
    or filter the hydrated rows in Go with a case-insensitive `strings.Contains` on the same fields.
    Thread `query` through `internal/person/application/service.go` `ListPersons` + `ListVisiblePersons`
    and `internal/person/transport/service.go` `ListPersons` (`:146`, pass `derefOr(query,"")`).

- **location** — *add `query`* (text search, no spatial window). `location_locations` has **no `code`
  column**; search the address fields.
  - `api/location.conjure.yml` `listLocations`: add `query: optional<string>` query arg; relax the
    doc — when `query` is present a spatial window is **not** required.
  - `internal/geo/adapters/queries/location.sql`: add `SearchLocationsByText :many` — ILIKE over
    `locality`, `admin_area1`, `admin_area2`, `street`, `mgrs`, `raw_address`, no `ST_DWithin`/bbox,
    projecting geometry back with the existing `ST_Y(geom)/ST_X(geom)` select shape (copy the
    projection from `ListLocationsInBbox`), keyset/offset paginated.
  - `internal/geo/application/location.go`: add `SearchLocations(ctx, q, pageSize, offset)`.
  - `internal/geo/transport/...` `ListLocations`: if `query` non-empty → text search; else keep the
    existing radius/bbox branch and the `QueryWindowRequired` error.

**Regen after contract/SQL edits:** `./godelw conjure` (regenerates the `*api` Go server
interfaces + IR), `sqlc generate` (regenerates `*sql` query code), `scripts/gen-openapi.sh` then the
web type regen so `web/src/lib/api/schema.d.ts` picks up the new `query` params. Generated files are
never hand-edited. Watch the known sqlc gotchas (string/`pgtype.Text` overrides; `@query` arg naming).

## Frontend changes

### 1. New reusable `web/src/components/SearchSelect.tsx`
A **server-query debounced typeahead** (200ms, min ~1–2 chars), modeled on `LanguagePicker.tsx`
(debounced fetch) + `EntitySelect.tsx` (portaled dropdown, keyboard nav, `name`-hidden-input **and**
`onChange` modes, selected-label persistence across refetch). A small registry keyed by kind:

- `person`   → `/person/v1/persons?query={q}&pageSize=20`, label `displayName||code`, hint `code`.
- `company`  → `/company/v1/companies?query={q}&pageSize=20`, label `pickLabel(legalName)||code`.
- `location` → `/location/v1/locations?query={q}&pageSize=20`, label `locality||mgrs||"lat,lng"`,
  hint `mgrs`.

Keep `EntitySelect.tsx` untouched (it's used in roles/orders/authorize/units/education) — a new
component avoids regressions in those flows while delivering server-side search.

### 2. Rewrite `web/src/app/(dashboard)/companies/page.tsx`
- **Locations form**: replace the `locationId` text input with `<SearchSelect kind="location"
  name="locationId" required />`.
- **Position create**: add the live-fill code-gen used by `CreateCompany` (same file, lines 76–96)
  and `PositionForms.tsx` (`:53–58`): `suffix = useRef(newSuffix())`, derive
  `codeValue = codeTouched ? code : slug ? \`${slug}-${suffix}\` : ""` from `slugify(title)` via
  `@/lib/code`; keep `code` editable but auto-filled from the title.
- **Fill position**: drop `window.prompt`. Make the `fill` button toggle an inline row with
  `<SearchSelect kind="person" onChange={setPersonId} />` + a confirm button, then
  `POST /positions/{id}/fill { personId }`.
- **Shareholding & Founding holders**: keep the `holderKind` `<select>`; render a controlled
  `<SearchSelect kind={holderKind==="company" ? "company" : "person"} onChange={setHolderId} />`
  next to it (re-key/reset on kind switch).
- **Beneficiary**: `<SearchSelect kind="person" name="personId" required />`.

### 3. Resolve RID-tail displays to labels
- **Position holder** and **Beneficiary** person tails (`personId.slice(-6/-8)`): reuse the exported
  `PersonLink` (`PositionForms.tsx:20`, module-cached `GET /person/v1/persons/{id}`).
- **Company location** tail (`locationId.slice(-8)`): add a small `LocationLabel` resolver
  (cached `GET /location/v1/locations/{id}` → `locality||mgrs`), mirroring `PersonLink`.
- Shareholders/founders/successions/branches already return `*Label` from the backend — leave as-is.

## Critical files
- Backend: `api/person.conjure.yml`, `internal/person/adapters/queries/person.sql`,
  `internal/person/application/service.go`, `internal/person/transport/service.go`;
  `api/location.conjure.yml`, `internal/geo/adapters/queries/location.sql`,
  `internal/geo/application/location.go`, `internal/geo/transport/*` (ListLocations).
- Frontend: `web/src/components/SearchSelect.tsx` (new),
  `web/src/app/(dashboard)/companies/page.tsx`, reuse `web/src/lib/code.ts` &
  `PersonLink` from `web/src/components/PositionForms.tsx`.
- Regenerated (do not hand-edit): `*api/*.go`, `*sql/*.go`, `docs/api/openapi/openapi.json`,
  `web/src/lib/api/schema.d.ts`.

## Verification
1. `./godelw conjure && sqlc generate && go build ./...` — backend compiles.
2. `go test ./internal/person/... ./internal/geo/... ./internal/company/...` — read-scope + geo green
   (add a test asserting `listPersons?query=` filters, and location text-search returns without a
   spatial window).
3. `curl` against a running server: `GET /person/v1/persons?query=<name>`,
   `GET /location/v1/locations?query=<locality>` (no lat/lng) return filtered pages, not
   `QueryWindowRequired`.
4. `cd web && npm run build` (type-check) — `schema.d.ts` has the new `query` params; page compiles.
5. Manual (`/run` the app): on `/companies` select a company → fill a vacant position by **searching**
   a person (no prompt); add a location by **searching** locality; add a shareholder with
   kind=person/company switching the picker; create a position and confirm the `code` auto-fills from
   the title. Confirm lists show **names**, not RID tails.

## Notes / out of scope
- No new migration — all three modules' tables already have the searched columns; this is contract +
  query + UI only.
- `docs/modules/company.md` is descriptive of behavior; update only if a sentence references the
  RID-paste flow. No stage-board change (M21 stays verified; this is UI/contract polish).
