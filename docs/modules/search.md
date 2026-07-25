# Search module

> **Status: review-2026-09 Phase 14 — verified.** Binding decisions: **D-UnifiedSearch** +
> **D-VisibilityScope** ([decisions.md](../architecture/decisions.md)). Resolves review findings
> **R‑26** (no unified search) and **R‑30** (bespoke per-module read visibility) —
> [review-2026-09.md](../architecture/review-2026-09.md). The web console's ⌘K palette is its first
> consumer, replacing the client-side first-page-per-type fan-out cache.

## Purpose

One cross-type object search over the registry — the Gotham-style entry point ("search anything,
then navigate"). `SearchService.searchObjects` fans in the **existing per-module trigram search
queries** (D-PersonSearch + the R‑21 search generalization): no global index table, no external
search infrastructure — each per-type arm keeps its proven `search_text` STORED column + `pg_trgm`
GIN index. The module also hosts the composition-time **provider registry** that pairs each
searchable object type with its **D-VisibilityScope** adapter — the same registry the R‑27 generic
link/graph API will extend with a links facet.

## Entities

None owned. The module owns **no tables and mints no RIDs** (no RID service number): every hit is
another module's Object, identified by its self-describing RID (D-ResourceIdentifiers). `SearchHit`
is a projection, not an ontology entity — nothing to register in
[ontology-mapping.md](../ontology-mapping.md).

## Data model

None. No migration; the per-type trigram indexes already exist (person migration `0005`, the rest
migration `0011_infra`).

## Conjure endpoint sketch

`api/search.conjure.yml` — `SearchService`, base path `/search/v1`:

- `searchObjects(query, types?, perTypeLimit?, pageSize?, pageToken?) → SearchResultPage` —
  `GET /objects`. `query` is a case-insensitive substring (min 2 chars — `Search:QueryTooShort`);
  `types` is a CSV of object-type tokens (`Search:UnknownObjectType` on an unregistered token);
  `perTypeLimit` (default 5) caps each type's contribution per page, `pageSize` (default 25) the
  page total. A hit is `{rid, objectType, label, snippet?}` — `objectType` is the ontology registry
  token (the `pkg/rid` / generated-web-mirror vocabulary), so no per-type response shapes exist.
- **Pagination** is a composite keyset token (`Search:InvalidPageToken`): base64url JSON
  `{"v":1,"c":{"<objectType>":"<providerCursor>"}}` — one cursor per non-exhausted provider, each
  cursor opaque and provider-owned (person: its encoded page token; languoid: the glottocode; the
  others: the last row's RID). Hits are grouped by type in **fixed lexicographic type order**
  (trigram search has no relevance rank — deterministic order instead, D-UnifiedSearch).

## Dependencies

- **authorization** — `pep.SubjectAuthority` (subject + admin flag, zero request-path queries),
  `pep.AllowedAnywhere` (the non-erroring per-provider permission probe), and the
  `internal/authorization/scope` Visibility adapters (D-VisibilityScope).
- **Provider modules** (composition-time closures in `cmd/oikumenea/search_providers.go`, the
  late-bound-seam posture): person (`ListVisiblePersons` / `ListPersons`), membership (the
  person-scope batch probe `SubjectReadablePersonsAmong`), language (`ListLanguoidsPage`), geo
  (`SearchLocations`), education (`ListInstitutions` / `ListPublications` / `ListScholarships`),
  company (`ListCompanies`). The engine itself imports none of them.

## Authorization touchpoints

The endpoint requires only an **authenticated subject** (no dedicated `search.*` permission code):
authorization is entirely per-provider + per-row —

- **Provider gate:** a provider is **skipped** (not failed) unless the subject holds its read
  permission somewhere (`person.read`, `language.read`, `location.read`, `education.read`,
  `company.read`). A subject with no read grants gets an empty page.
- **Row trim (D-VisibilityScope):** each provider's raw rows pass through its registered
  `Visibility` — person-scope (membership semi-join), unit-scope (owning-unit map + shadow gate;
  built for R‑27, no searchable type uses it yet), catalog-scope (identity; the gate is the whole
  decision). The **person provider is `PreTrimmed`**: its search runs the D-PersonReadScope
  visibility semi-join in SQL (`VisiblePersonIDsForSubjectSearch`), so the engine skips the
  post-trim; its person-scope Visibility is registered regardless (the R‑27 link facet consumes it).
- Reads are **not audited** (matches every read path).

## Patterns

- **Fan-in, not index:** federates the existing per-type queries; each arm stays on its own GIN
  index (EXPLAIN-verified under R‑21). Explicitly NOT: EAV store, Elasticsearch, graph DB
  (review-2026-09 "NOT recommended").
- **Register-then-assert:** providers register at composition (`Register(provider, visibility)` —
  duplicate or visibility-less registration errors at boot) and the engine joins main.go's
  `MustBeBound` seam loop (review-2026-07 R-11), so a type can never ship searchable-but-untrimmed.
- **Raw-cursor advance:** a provider's cursor advances over its raw (pre-trim) rows — a
  visibility-trimmed page may run short, but the walk never skips or duplicates a row.
- **Differential correctness:** the trim contract is equality with the owning module's own list
  endpoint for the same subject/query — enforced by the integration tests (person ≡
  `ListVisiblePersons`, catalog ≡ the module list) and, for the person reach predicate itself, by
  membership's randomized `TestReachDifferential` (section b2).

## Invariants

- A hit set is **never wider** than what the owning module's endpoints would serve the same
  subject (R‑30). Fail closed: no registered visibility → no registration; unmapped unit-scope
  candidate → dropped.
- Provider order (and therefore hit grouping) is deterministic — lexicographic by object type.
- The page token only ever names registered object types; a token naming anything else is rejected.
- `query` < 2 chars is rejected, mirroring the palette's gate.

## Open seams / future

- **R‑27 (Phase 15):** the links facet on the same registry — `getObjectLinks` + `searchAround`;
  the unit-scope Visibility and the person-scope registration are already in place for it.
- **Relevance ranking:** trigram similarity scoring / cross-type interleaving (today: grouped,
  keyset order). Revisit only with an analyst-workload measurement.
- **Locale-map labels:** hits carry a single default-locale `label`; a `locales=` projection
  (R-19 style) returning `locale→text` maps is additive.
- **More providers:** units (needs a `search_text` on `tenant_units` first), external
  organizations, religion taxa — each is one `Register` call + a provider closure.
- **Console consolidation:** `EntitySelect` and the `explore/[type]` search boxes still use
  per-type fetches; migrating them onto `searchObjects` (with `types=` filters) retires the last
  client-side search paths.
