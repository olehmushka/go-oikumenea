# Links module

> **Status: review-2026-09 Phase 15 — verified.** Binding decision: **D-LinkTraversal**
> ([decisions.md](../architecture/decisions.md)), building on **D-VisibilityScope**. Resolves review
> finding **R‑27** (no backend "links for object X" / search-around) and closes **R‑32**'s
> fix-sketch item 2 (declaring the polymorphic ends for generic traversal) —
> [review-2026-09.md](../architecture/review-2026-09.md). The web console's universal object page
> (`/o/[rid]`) and graph explorer (`/graph`) are its first consumers, replacing the client-side
> per-collection fan-out over the hand-authored `registry.ts` `links[]`.

## Purpose

One generic answer to **"what links does object X have?"** over the whole registry — the Gotham-style
object-explorer / search-around primitive. `LinkService.getObjectLinks` fans in the **existing
reified link tables** (kind=link RID PK, two endpoint columns, per-endpoint partial index): no graph
database, no new edge/join table, no client fan-out. The module hosts the composition-time
**link-descriptor registry** — the Go counterpart of the console's hand-authored `links[]`, but its
link types are validated against the drift-proof **pkg/rid** link-type registry (R‑28) and its
coverage is boot-asserted, so a link table a new milestone adds surfaces in the console **without
editing `web/`**, or fails boot until wired. It is the **links facet** of the same cross-type
registry seam D-UnifiedSearch introduced (one registry, three facets: provider, visibility, links).

## Entities

None owned. The module owns **no tables and mints no RIDs** (no RID service number): every link and
every neighbor is another module's Object/Link, identified by its self-describing RID
(D-ResourceIdentifiers). `LinkRow`/`LinkGroup` are projections, not ontology entities — nothing to
register in [ontology-mapping.md](../ontology-mapping.md).

## Data model

None. No migration; the reified link tables and their endpoint indexes already exist (per module,
migrations `0003`–`0034`). The engine runs one **keyset query per incident link arm** over those
tables — the one place raw dynamic SQL is justified (a union over a runtime-registered set of tables
is not expressible in sqlc): identifiers come from the compile-time descriptor registry and pass
through `pgx.Identifier.Sanitize`; the queried RID, the polymorphic discriminator, the keyset cursor
and the limit are bound value params.

## Conjure endpoint sketch

`api/links.conjure.yml` — `LinkService`, base path `/links/v1`:

- `getObjectLinks(rid, linkTypes?, pageSize?, pageToken?) → ObjectLinks` — `GET
  /objects/{rid}/links`. Returns the object's links grouped by `(linkType, direction, targetType)`;
  a `LinkRow` is `{linkRid, targetRid, targetType, targetLabel?, direction, attrs?}`. `linkTypes` is
  a CSV of bare pkg/rid link-type names (default all incident); `pageSize` default 50, clamped
  `[1,200]`. `rid` that decodes to no registered object type → `Links:UnknownObjectType`.
- `searchAround(rid, depth?, linkTypes?, pageSize?, pageToken?) → Neighborhood` — `GET
  /objects/{rid}/search-around`. The same engine flattened to a neighbor list (the graph shape).
  `depth` is 1 (default) or 2 (clamped); at **depth 2** each direct neighbor's own neighbors are also
  returned, each such `LinkRow` tagged `hop:2` and carrying `viaRid` (the intermediate node) — so "any
  path between these two objects?" is one request. Per-hop authorization is identical to depth-1.
- **Pagination** is a composite keyset token (`Links:InvalidPageToken`): base64url JSON
  `{"v":1,"c":{"<linkName>/<side>":"<lastLinkRID>"}}` — one cursor per non-exhausted arm, keysetting
  over the link table's RID PK. Cursors advance over **raw** pre-trim rows, so a visibility-trimmed
  page may run short but never skips a row.

## Dependencies

- **authorization** — `pep.SubjectAuthority` (subject + admin flag, zero request-path queries),
  `pep.AllowedAnywhere` (the non-erroring per-arm permission probe), and the
  `internal/authorization/scope` Visibility adapters (D-VisibilityScope).
- **The shared pool** — the engine runs the generic per-arm queries directly (unlike search, which
  delegates to module services).
- **Descriptor wiring** (`cmd/oikumenea/link_descriptors.go`, composition-time): the pool, the
  membership person-scope batch probe (`SubjectReadablePersonsAmong`), and the authorization
  `FilterVisibleUnits` shadow gate for the unit scope. The engine imports no other module — table
  and column identifiers are supplied as descriptor data.

## Authorization touchpoints

The endpoint requires only an **authenticated subject** (no dedicated `links.*` permission code):
authorization is entirely per-arm gate + per-row trim —

- **Arm gate:** a link arm is **skipped** (not failed) unless the subject holds the descriptor's read
  permission somewhere (`person.read` for the person-owned links, `membership.read`, `education.read`,
  `company.read`, `finance.read`, `vehicle.read`, `religion.read`, `language.read`, `unit.read`). A
  subject with no relevant grants gets an empty result.
- **Neighbor trim (D-VisibilityScope):** each arm's neighbor rows pass through the neighbor object
  type's registered `Visibility` — **person** → person-scope (membership semi-join); **unit** →
  unit-scope (owning-unit identity map + shadow flags + the shadow gate `FilterVisibleUnits`); every
  other neighbor type → catalog-scope (identity; differential-equal to the owning module's coarse
  read-permission gate). Registering a neighbor type without a visibility fails boot.
- Reads are **not audited** (matches every read path).

## Patterns

- **Fan-in, not graph DB:** federates the existing reified link tables; each arm stays on its own
  endpoint partial index. Explicitly NOT: a graph database, an edge/EAV table, a client fan-out
  (review-2026-09 "NOT recommended").
- **Registry-derived-from-pkg/rid:** a `Descriptor`'s `(service, code)` must be a real kind=link RID
  type (validated at `Register`), and `MustBeBound` fails boot unless **every** kind=link type is
  registered **or** explicitly exempt — the R‑27 drift guard, pairing R‑28. A migration adding a link
  type without wiring it here fails boot (and the integration `TestLinkCoverage`).
- **Raw-cursor advance:** an arm's cursor advances over its raw (pre-trim) rows — a trimmed page runs
  short, but the walk never skips or duplicates.
- **Polymorphic ends declared:** the F-014 `holder_kind`/`holder_id` (finance held_by, vehicle
  registered_to, company founded/owns_stake) ends declare one target per discriminator (person
  `(6,1,1)` / company = tenant org `(4,1,6)`), so generic traversal — which cannot discover a
  no-FK text edge from the schema — includes them (closes R‑32 item 2).
- **Descriptor `NoSoftDelete` / `FilterCol`:** a descriptor over an **append-only** table (no
  `deleted_at`) sets `NoSoftDelete` so the arm query omits the `deleted_at IS NULL` clause
  (`parent_of`→`tenant_unit_edges`, `written_in`→`language_writing_systems`); a descriptor may also set
  an equality `FilterCol`/`FilterVal` (bound param) both to scope the graph to *current* edges and to
  **match a partial index**. `member_of` sets `status='active'`, matching the membership partial indexes
  (`…WHERE status='active' AND deleted_at IS NULL`) so a unit's members are read from the index rather
  than a seq scan of the 1M-row table — the depth-2 gate measurement made this load-bearing.

## Invariants

- A link/neighbor set is **never wider** than what the owning module's endpoints would serve the same
  subject (R‑30). Fail closed: no registered visibility → the neighbor type can't be registered;
  unmapped unit-scope candidate → dropped.
- Every kind=link RID type is either **registered** or **exempt** (boot-asserted) — the console never
  silently under-reports a relationship because a link table was added without wiring.
- The page token only ever names registered arms; a token naming anything else is rejected.
- A RID that does not decode to a registered **object** type is rejected (`UnknownObjectType`).

## Open seams / future

- **Depth-2 search-around — delivered.** `searchAround(rid, depth=2, …)` walks the two-hop
  neighborhood exhaustively, staying Postgres-over-the-link-tables (no graph DB). Two sequential keyset
  phases share the page budget: (1) drain the origin's hop-1 arms (the depth-1 engine, unchanged); (2)
  enumerate the trimmed hop-1 neighbors as a **frontier in neighbor-RID order** (a distinct-neighbor
  keyset query per origin arm, fetched a **batch** at a time so a wide frontier is not re-scanned per
  node) and expand each with an inner hop-2 `collect`. The arm gate + neighbor trim run at **every
  hop** (reusing depth-1's primitives), so an unreadable node is neither returned nor expanded; the
  backtrack edge to the origin is dropped, genuine alternate 2-paths kept (rows are *edges*, not deduped
  nodes). A distinct **v2** keyset token (origin cursors + scalar frontier high-water mark + current
  node cursors) makes the walk resumable; v1/v2 tokens never cross. The review gate ("< 1 s, 50-neighbor
  node, 2-hop, M49 scale") is met — ~0.4–0.5 s for 767 neighbors on the 1M-person seed-scale dataset
  (`TestSearchAroundDepth2Scale`). Clearing it hardened the shared descriptor layer (see Patterns).
- **Server-side neighbor labelers — delivered.** Each neighbor object type registers a batch labeler
  (`RegisterLabeler`) that resolves its RIDs to a `targetLabel` **locale→text map** (D-i18n: all
  locales, no negotiation): person from `display_name` + per-locale name variants, everything else via
  an `overlayLabeler` that reads the neighbor's base `name`/`title` and overlays the `i18n_translations`
  store through `localization.NamesByID` (types with no translation rows degrade to a single
  default-locale entry). Comprehensive across all named neighbor types; the RID-tail fallback survives
  only for the genuinely nameless (`curriculum_version`, `vehicle`, `account`).
- **Per-link-type permission codes — delivered (D-LinkPermissions).** A relationship that is its own
  disclosure now carries its own read code, and that code gates **both** the owning module's dedicated
  list endpoint and this engine's arm — so the bespoke page and the object graph never disagree. Landed
  for the person relationship graph (`person.partnership.read`, `person.kinship.read`,
  `person.guardianship.read`, `person.sponsorship.read`, `person.next_of_kin.read`,
  `person.association.read`, `person.address.read`) and the ownership links (`finance.holder.read`,
  `vehicle.registration.read`), composing the additive `person-relationship-reader` /
  `finance-graph-reader` / `vehicle-graph-reader` base roles. Two classes deliberately keep the coarse
  module read: **aggregate-embedded** links (`holds_rank`, `speaks` — returned inside `getPerson`, so a
  separate arm code would be incoherent) and **structural/reference** links (`parent_of`, `written_in`,
  `unit_language`, `curriculum_item`, `manufactured_by`, `has_industry`, … — no personal subject, so a
  per-link code restricts nothing). Remaining modules follow the same pattern with no engine change.
- **Currently exempt link types** (8): `locale_language` (text-code end), `has_ethnicity` /
  `party_membership` / `government_position` / `lobbying_rel` (encrypted / free-text / untyped
  polymorphic ends), `has_role` (three-way assignment), `instance_admin` (no neighbor),
  `affiliated_with` (multi-ended optional affiliation) — each becomes traversable if/when it gains a
  cleanly-typed RID neighbor.
