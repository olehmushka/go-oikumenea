# Plan: Person ↔ Religion relations (M23 clergy credentials + M24 lay affiliation) + UI

## Context

The user wants relations between **persons** and **religions**, with UI. The repo already
*designs* exactly this as two binding decisions, deferred behind the shipped M22 taxonomy/org slice:

- **M23 / D-ClergyCredential** — a person's ordination/standing (bishop, imam, rabbi, bhikkhu…)
  within an organization unit. A **public directory fact** (never an authz input, parallel to
  D-Rank). Reified Link `link__clergy_credential` + per-tradition ordered grade catalog.
- **M24 / D-ReligiousAffiliation** — a person's **lay** faith/membership (adherent, baptized,
  shahada, bar/bat-mitzvah…). **GDPR Art. 9 `pii:special`** → must be **envelope-encrypted +
  blind-indexed + crypto-erased on purge** (D-SpecialPII).

Decision (per user): build **both M23 and M24**, with **full encryption per the binding decision**,
and surface the UI in **both** the person detail page and the religion workspace.

Both relations are owned by the **religion module** (RID service `16`), with endpoints on
`ReligionService` — *not* the person module. The crypto seam already exists
(`pkg/crypto`, `crypto.Cipher` with `Seal`/`Open`/`BlindIndex`); the `document` module is the
working precedent (personal codes, `pii:sensitive`). Extending to `pii:special` is "extend unchanged"
(D-SpecialPII) — no new mechanism.

> **Open seam (match existing reality):** the `PersonPurged` event is **not yet wired** — even
> `document.ErasePersonRecords` is "exercised directly today", with the bus subscriber left as an
> open seam. So M24's crypto-erase ships as a directly-callable `ErasePersonAffiliations` method
> (mirroring document), and auto-wiring it to a real `PersonPurged` event stays the same shared
> follow-up seam document already documents. Do **not** balloon scope by inventing the event bus
> wiring here.

## RID allocation (add in BOTH places — boot asserts equality, kind≠3)

`migrations/<new>.sql` `platform_rid_types` **and** `pkg/rid/registry.go` (service `SvcReligion=16`):

- Objects: `(16,1,7) grade_category`, `(16,1,8) clergy_grade`, `(16,1,9) office_type`,
  `(16,1,10) affiliation_type`
- Links: `(16,2,2) clergy_credential`, `(16,2,3) affiliated_with`

Existing religion registry block is `pkg/rid/registry.go:154-161`; migration block is
`migrations/20260601000023_religion.sql:42-49`.

## Migrations (two new, expand-only; M23 then M24)

Next numbers after the untracked `0023`. Run `atlas migrate hash` after writing each, then apply to
the dev DB (`postgres`) and the test DB (`oikumenea_test` via `scripts/setup-test-db.sh`). These are
**new** files — no DROP-SCHEMA reset needed.

### `migrations/20260601000024_religion_clergy.sql` (M23)
- `religion_grade_categories` (16,1,7) — `id`, optional `tradition_taxon_id → religion_taxa`, `code`,
  `name`, `ordinal`, `status`, `sort_order`, timestamps, soft-delete.
- `religion_clergy_grades` (16,1,8) — `id`, optional `tradition_taxon_id → religion_taxa`,
  `grade_category_id → religion_grade_categories`, `code` (unique among active within a tradition),
  `name`, `ordinal` (seniority **within a tradition** — no cross-tradition comparator, DS-43 parked),
  `status`, timestamps, soft-delete.
- `religion_office_types` (16,1,9) — `id`, optional `tradition_taxon_id`, `code`, `name`, timestamps,
  soft-delete. *(Catalog only; offices themselves stay `membership.Position` — not built here.)*
- `religion_clergy_credentials` (16,2,2) — the reified Link. `id` (RID-shape CHECK 16/2/2),
  `person_id → person_persons ON DELETE RESTRICT`, `clergy_grade_id → religion_clergy_grades`,
  `org_unit_id → tenant_units`, `granted_on DATE`, `conferred_by_person_id → person_persons
  ON DELETE SET NULL`, `status CHECK (active|suspended|revoked) DEFAULT active`,
  `effective_from`/`effective_to`, `source`, `confidence`, timestamps, soft-delete.
  **Indelible where sacramental** → revocation is a status flip, never hard delete. RLS backstop on
  `org_unit_id` (`app.readable_units`/`app.writable_units`, like `religion_org_classifications`).
- Seed per-tradition grade/office catalogs (Christianity bishop/presbyter/deacon; Islam
  imam/mufti/sheikh; Judaism rabbi/cantor; Buddhism bhikkhu/lama; Hinduism pujari/swami), keyed to
  the seeded `tradition`-rank taxa from 0023. Embed directly (mirrors 0023's embedded seed), and
  extend `deploy/religion-presets/gen-presets.py` to emit these waves reproducibly.

### `migrations/20260601000025_religion_affiliation.sql` (M24, `pii:special`)
- `religion_affiliation_types` (16,1,10) — `id`, optional `tradition_taxon_id`, `code`, `name`,
  timestamps, soft-delete. Seed: generic adherent/member; Christianity catechumen/baptized/confirmed;
  Islam shahada; Judaism bar/bat-mitzvah.
- `religion_affiliations` (16,2,3) — the reified `pii:special` Link. `id` (RID-shape CHECK 16/2/3),
  `person_id → person_persons ON DELETE RESTRICT`, optional `religion_id → religion_taxa`
  (faith anchor), optional `tradition_unit_id`/`community_unit_id → tenant_units`,
  `affiliation_type_id → religion_affiliation_types`, the envelope columns
  `value_ciphertext BYTEA`, `wrapped_dek BYTEA`, `key_ref TEXT`, `value_blind_index BYTEA`
  (exactly the `document` personal-code shape), `status CHECK (active|lapsed|renounced) DEFAULT
  active`, `effective_from`/`effective_to`, `source`, `confidence`, timestamps, soft-delete.
  Person-scoped (instance-global) → **no unit RLS**, mirroring `person_languages` /
  `person_partnerships`.

## Permissions

`internal/authorization/domain/permissions.go` already has `religion.read`, `religionorg.manage`,
`religion.catalog.manage` (lines 121–141). **Add** `PermClergyManage = "clergy.manage"` and
`PermAffiliationManage = "affiliation.manage"` (named in the module doc's authorization touchpoints),
plus their catalog registration entries alongside the existing religion block.

## Conjure API — `api/religion.conjure.yml` (`ReligionService`, `/religion/v1`)

Add objects + request bodies + list/page types: `GradeCategory`, `ClergyGrade`, `OfficeType`,
`ClergyCredential`, `AffiliationType`, `Affiliation` (translatable `name` as `map<string,string>`
locale→text, per D-i18n / `NamesByID`). New endpoints:

- **M23 catalogs:** `GET·PUT /grade-categories`, `/clergy-grades`, `/office-types`
  (`religion.read` / `religion.catalog.manage`).
- **M23 credential (person-scoped read, unit-gated write):**
  `GET /persons/{personId}/clergy-credentials`, `GET /units/{unitId}/clergy-credentials`,
  `POST /persons/{personId}/clergy-credentials`, `PUT /clergy-credentials/{id}` (status flip /
  effective-dating). No DELETE (indelible). Writes gate `clergy.manage` on the `org_unit_id`
  (canonical graph).
- **M24 catalog:** `GET·PUT /affiliation-types`.
- **M24 affiliation (`pii:special`):** `GET /persons/{personId}/affiliations` (decrypted value,
  projected through D-PersonReadScope), `POST /persons/{personId}/affiliations` (encrypts value),
  `PUT /affiliations/{id}` (status/value), `DELETE /affiliations/{id}` (soft-delete). Gate
  `affiliation.manage`.

Then regenerate: gödel/conjure build → `scripts/gen-openapi.sh` → `web/scripts/gen-api-types.sh`
(`schema.d.ts`). Conjure/sqlc-generated code is never hand-edited.

## Go layers — `internal/religion/` (raw pgx, mirrors the M22 slice)

- **`domain/religion.go`** — add `GradeCategory`, `ClergyGrade`, `OfficeType`, `ClergyCredential`,
  `AffiliationType`, `Affiliation` structs + `Validate()` + sentinels (`ErrGradeNotFound`,
  `ErrCredentialNotFound`, `ErrAffiliationTypeNotFound`, `ErrAffiliationNotFound`, reuse `ErrConflict`/
  `ErrInvalid`/`ErrInUse`).
- **`application/service.go`** — add `cipher *crypto.Cipher` to the `Service` struct + `NewService`.
  Methods (each wrapped in existing `inTx()` + audited via `s.record(...)`, system actor, service 16
  kind 3):
  - catalog CRUD for grade-categories/clergy-grades/office-types/affiliation-types;
  - `ListPersonClergyCredentials`, `ListUnitClergyCredentials`, `AddClergyCredential`,
    `UpdateClergyCredential` (status flip);
  - `ListPersonAffiliations` (calls `cipher.Open` to decrypt), `AddAffiliation` /`UpdateAffiliation`
    (calls `cipher.Seal` + `cipher.BlindIndex` — copy `internal/document/application/service.go:433`
    `encrypt`/`decrypt` helpers), `DeleteAffiliation` (soft-delete);
  - `ErasePersonAffiliations(ctx, personID)` — crypto-erase (NULL the ciphertext + drop wrapped DEK),
    audited, mirroring `document.ErasePersonRecords`.
- **`adapters/repository.go`** — raw-pgx insert/list/get/update/soft-delete/erase for all new tables;
  reuse the existing `23505→ErrConflict` / `23503→ErrInvalid` mapping.
- **`transport/service.go`** — Conjure handlers, permission gating (`clergy.manage` /
  `affiliation.manage` / `religion.read` / `religion.catalog.manage`), locale-map assembly for
  catalog `name`s (reuse the existing localization path), error→Conjure mapping. Affiliation reads
  return the decrypted value gated/projected per D-PersonReadScope.
- **`module.go` + `cmd/oikumenea/main.go`** — extend `religion.Register(...)` with `cipher
  *crypto.Cipher`; pass the already-built `cipher` (constructed `main.go:185`, used by `document`
  at `main.go:190`) into the `religion.Register` call at `main.go:241`.

## Web UI (both places)

API client: `mutate()` / `bffGet()` over `/api/oikumenea` (`web/src/lib/api/`). Add narrow projections
to `web/src/lib/api/types.ts` (`ClergyCredential`, `Affiliation`, `AffiliationType`, `ClergyGrade`…).

- **Person detail** (`web/src/app/(dashboard)/persons/[personId]/`): add two manager cards on
  `page.tsx` + components in `PersonForms.tsx`, mirroring `PersonLanguageManager`
  (`PersonForms.tsx:1187`):
  - `PersonClergyManager` — lists credentials (grade · org unit · status), add via a grade picker +
    `EntitySelect kind="unit"` for the org + `granted_on`; status flip via PUT.
  - `PersonAffiliationManager` — lists affiliations (faith/community + type + decrypted value), add via
    affiliation-type picker + optional taxon/unit pickers; soft-delete via `RowDelete`.
  - Pickers: client-side `ClergyGradePicker` / `AffiliationTypePicker` from the catalog endpoints;
    reuse the taxon search for the religion anchor and `EntitySelect` for units.
- **Religion workspace** (`web/src/app/(dashboard)/religion/page.tsx`): add catalog-management
  sections (grade-categories / clergy-grades / office-types / affiliation-types) and a per-org-unit
  **clergy roster** view (`GET /units/{unitId}/clergy-credentials`). Affiliations are `pii:special` —
  show only an aggregate/gated view there, not a raw roster.
- `web/src/components/Nav.tsx` already lists `/religion`; no nav change required.

## Docs (same-pass coherence — required by CLAUDE.md)

- `docs/milestones.md` **stage board** — advance M23 and M24 rows through the gates as each lands.
- `docs/modules/religion.md` — move the Clergy + Lay-affiliation sections from "designed/deferred" to
  built; update the Conjure API surface table and the M22→M23/M24 ontology-kinds note.
- `docs/ontology-mapping.md` — register the new Objects (`GradeCategory`/`ClergyGrade`/`OfficeType`/
  `AffiliationType`) + Links `link__clergy_credential` (16,2,2), `link__affiliated_with` (16,2,3).
- `docs/architecture/roadmap-decisions.md` — note D-ClergyCredential / D-ReligiousAffiliation /
  D-SpecialPII as landed for M23/M24 (now binding against code).
- `docs/glossary.md` — add clergy-credential / affiliation / pii:special-affiliation terms if new.
- Run the docs link-coherence check (CLAUDE.md snippet) after editing.

## Verification

1. **Migrate:** `atlas migrate hash`; apply 0024/0025 to dev (`postgres`) and test (`oikumenea_test`
   via `scripts/setup-test-db.sh`).
2. **Build/test:** `go build ./...`; extend `internal/religion/religion_integration_test.go` to:
   create a tradition taxon + clergy grade, add a credential, flip status active→suspended; add an
   affiliation and **assert at the DB level that `value_ciphertext`/`value_blind_index` are non-empty
   and the plaintext is absent**, then read it back decrypted (round-trip), and verify
   `ErasePersonAffiliations` drops the wrapped DEK. Run `go test ./internal/religion/...`.
3. **Codegen:** regenerate conjure → openapi → web `schema.d.ts`; `cd web && npm run build`
   (type-check).
4. **Live HTTP demo** (local server or `docker compose`): `POST` a clergy credential and an
   affiliation, query the DB to confirm the affiliation value is encrypted at rest, `GET` it back
   decrypted via the API, and exercise the status flip — then confirm both manager cards render on
   the person page and the religion workspace shows the clergy roster + catalog editors.
