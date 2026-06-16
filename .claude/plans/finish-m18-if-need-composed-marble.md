# Finish M18 — Language & writing systems

## Context

M18 (Languages & writing systems, **D-Languages**) is the first new consumer of the M16 hermenea
pipeline. Its core (Glottolog forest import, closure, `family_code`, country ties, read-only
`LanguageService`) was verified, but an **i18n consistency review (2026-06-15)** downgraded the
backend gate to 🚧 and cleared `verified` over two gaps, and the **UI gate was never started** (⬜).

**Key finding from exploration:** both backend gaps the verdict raised are *already coded on disk*
(uncommitted) and the project builds clean:
- **Gap 1 (name as `locale→text` map):** `internal/language/transport/service.go` already assembles
  `name` via `LocalizationService.NamesByID` for languoids + writing systems (mirrors rank/tenant).
- **Gap 2 (`i18n_locale_languages` dead wiring):** `ReconcileLocaleLanguages` SQL exists, is wired
  through repo→domain→app, and is invoked at the end of each `language-scheme` import
  (`internal/dataimport/application/service.go:277`); the unit test asserts it fires.

So the remaining work is: **(A)** re-verify the already-coded backend e2e and add a durable test,
**(B)** build the NEW backend endpoints the editor UI needs (the `person_languages` /
`tenant_unit_languages` tables exist but have *no API*), **(C)** build the web UI (language browser +
person/unit editors + locale display), **(D)** flip the docs. The user approved full scope
(browser + full editors) and the full 27k preset load, and authorized editing the migration + DB
reset if needed (likely *not* needed — the schema is complete).

---

## Part A — Re-verify backend (already coded) + durable artifact

1. **Add a language integration test** in `internal/dataimport/` (new
   `language_integration_test.go`, `//go:build integration`, modeled on the existing
   `dataimport_integration_test.go` geo tests + `newPool`/`newService` helpers). Register the two
   language handlers (`LanguageSchemeHandler`, `LanguageScriptsHandler`) and assert, against the real
   `oikumenea_test` DB:
   - parent-first scheme import → closure + `family_code` derived; country ties resolved;
   - `i18n_locale_languages` populated by reconcile (seed an `i18n_locales` row e.g. `eng`, confirm it
     links to the languoid with `iso639_3='eng'`);
   - idempotent re-run (same `source_version` → all skipped, no closure/reconcile churn);
   - `person_languages` composite FK: inserting a `level='language'` languoid OK, a `family` rejected.
2. **Add a language-module read test** proving Gap 1: via `internal/language` app+transport over the
   test DB (or extend the integration test), confirm `GetLanguage` returns `name` as a `locale→text`
   map, and that inserting an `i18n_translations` row for `entity_type='languoid'` surfaces an extra
   locale in the map.
3. **Run** against a fresh DB: `scripts/reset-dev-db.sh` / `scripts/setup-test-db.sh`, then
   `OIKUMENEA_TEST_DSN=... go test -tags integration ./internal/dataimport/... ./internal/language/...`.
4. **Full 27k preset load (e2e):** bring up the cross-service stack (mirror the M16 run —
   `docker-compose.dev.yml` + `var/conf/hermenea-install.yml`, isolated project name to avoid the
   `oik-pgdata` volume collision noted in memory) and load
   `deploy/language-presets/glottolog-5.3.json` + `cldr-scripts.json` via hermenea →
   `POST /import/{language-scheme,language-scripts}`. Confirm the verified-core figures (27,177
   languoids, closure, `family_code`, country/script ties, idempotent re-run) AND the new assertions:
   `i18n_locale_languages` has `ukr`→Ukrainian / `eng`→English, and `GET /language/v1/languages/{id}`
   returns `name` as a map.

> Migration edits are *not anticipated* — the schema is complete. Reserve the user's reset-and-edit
> permission only if verification surfaces a schema bug.

## Part B — New backend endpoints for the link editors

The editor UI needs APIs that don't exist yet. Build two sub-resource slices, each mirroring the
established **person social-accounts** / **membership positions** patterns (conjure → transport →
application → domain → adapters/queries (sqlc) → tests):

1. **Person SPEAKS** (`person_languages`, RID `6,2,8`) on `api/person.conjure.yml`:
   - `GET /persons/{personId}/languages` → list (`languageId`, `name` map via `NamesByID`,
     `cefrLevel?`, `isNative`);
   - `PUT|POST /persons/{personId}/languages` upsert (validate `language_id` resolves to a
     `level='language'` languoid via the composite FK; `cefr_level ∈ {A1..C2}`);
     `DELETE …/{languageId}` (soft-delete, respect `person_languages_active_idx`).
   - **Purge:** add `DeleteAllPersonLanguages` to the purge erasure block in
     `internal/person/adapters/queries/person.sql` (sibling of `DeleteAllSocialAccounts`) and call it
     in the person purge sweep — the migration promises purge-erasure and it is currently missing.
   - PEP: reuse the person-write permission the sibling sub-resources use.
2. **Unit official/working language** (`tenant_unit_languages`, RID `4,2,2`) on
   `api/tenant.conjure.yml` (or `membership` if unit sub-resources live there — follow where
   `units/{id}/positions` is defined):
   - `GET /units/{unitId}/languages`, `PUT`/`POST` upsert (`isOfficial`), `DELETE …/{languageId}`.
   - PEP: reuse the unit-write permission.
3. **Locale → language:** *read-only* — it is import-reconciled (`ON CONFLICT (locale) DO UPDATE`), so
   a manual editor would be overwritten on the next import. Expose it as a field/echo on the existing
   locale read (or a `GET /localization/v1/locales/{code}/language`) for display only. No write
   endpoint.
4. Regenerate Conjure (`./scripts/gen-openapi.sh` / gödel conjure) and sqlc; wire the new services in
   `cmd/oikumenea/main.go`. Add domain/app unit tests with fakes (mirror existing sub-resource tests).

## Part C — Web console UI (`web/`)

1. **Registry entries** in `web/src/lib/ontology/registry.ts` — add `languoid` and `writing_system`
   `ObjectTypeDef`s (model on the `rank`/`rank_system`/`locale` entries): `list` →
   `/language/v1/languages` (parse `languoids`) and `/language/v1/writing-systems`; `get` →
   `/language/v1/languages/{id}`; columns (`code` mono, `name` via `loc()`, `level` pill, `familyCode`,
   `iso6393`); `writing_system` columns (`code`, `name`, `scriptType`). This gives the explorer,
   ⌘K search, object view, and ontology browser for free.
2. **`/languages` browser page** (`web/src/app/(dashboard)/languages/page.tsx`) — server component
   using `apiGet` (model on `ranks/page.tsx`): a filterable languoid list (level/family/query, the
   API's existing filters) + writing-systems list. Add a **Nav.tsx** entry.
3. **Person language editor** — extend `web/src/app/(dashboard)/persons/[personId]/PersonForms.tsx`
   (the established multi-channel editor) with a "Languages spoken" manager (add/remove languoid,
   `cefrLevel`, `isNative`) hitting the new Part B person endpoints, plus a languoid picker (search
   over `/language/v1/languages?query=`).
4. **Unit language editor** — add an official/working-language manager to the unit detail surface
   (mirror the positions editor), hitting the new tenant endpoints.
5. **Locale language display** — show each locale's canonical language (read-only) on the
   `localization` page.
6. **Types:** extend `web/src/lib/api/types.ts` (and regenerate `schema.d.ts` from the OpenAPI if that
   is the generated source). `cd web && npm run build` + typecheck must pass.

## Part D — Docs & bookkeeping (same-pass coherence)

- **`docs/milestones.md` stage board:** M18 Backend 🚧→✅, **UI** ⬜→✅, **Verified** ⬜→✅; update the
  Stage cell to `verified`. Rewrite the **M18 Verdict** block to "resolved" (both gaps closed + how
  verified). Update the M18 `Status:` line and drop the "**Deferred:** web console UI" bullet.
- **`docs/modules/language.md`:** flip the status header to verified; in *Open seams* remove the
  resolved items (web UI, i18n name wiring pending, locale↔languoid as future) and document the new
  person/unit language endpoints + the read-only locale display.
- **`docs/ontology-mapping.md`:** confirm `link__speaks` (6,2,8), `link__unit_language` (4,2,2),
  `link__locale_language` (2,2,1) are registered (add if missing now that they have an API).
- **`docs/modules/person.md` / `tenant.md`:** add the new language sub-resource endpoints + purge note.
- Run the **docs link-coherence** check from `CLAUDE.md` after edits.

## Verification

1. `go build ./...` and `go vet ./...` clean.
2. Unit tests: `go test ./internal/language/... ./internal/dataimport/... ./internal/person/... ./internal/tenant/...`.
3. Integration: reset DB, `go test -tags integration ./internal/dataimport/... ./internal/language/...`.
4. Full e2e: docker-compose cross-service run + 27k preset load; curl `GET /language/v1/languages/{id}`
   (name is a map), confirm `i18n_locale_languages` rows, and exercise the new person/unit language
   endpoints (add a language with CEFR+native; add a unit official language; purge a person and
   confirm `person_languages` erased).
5. Web: `cd web && npm run build`; manually drive the `/languages` browser, the person language
   editor, and the unit language editor against the running server.
6. Docs link-coherence script passes.

## Memory update (after landing)

Update `project-implementation-progress.md` + `MEMORY.md`: M18 fully verified (backend gaps were
already coded; re-verified e2e + 27k load), new person/tenant language link endpoints + purge sweep,
language browser + editor UI shipped, UI gate flipped.
