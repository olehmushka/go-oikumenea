# Implementation milestones

> Reads: [README](README.md) · [architecture/overview.md](architecture/overview.md) ·
> [architecture/decisions.md](architecture/decisions.md) · the [modules/](modules/) docs.

The design is complete at the architecture level ([README](README.md)); this file sequences it into
**buildable milestones**. It is a roadmap, **not** a binding decision — `decisions.md` still governs
*what* is built; this governs *in what order*. Each milestone is a vertical slice that **boots,
migrates, and demos** on its own, so the service is runnable at every step.

## Ground rules (hold for every milestone)

- **Platform first, then dependency order.** A module lands only after the modules it imports
  (queries are direct interface calls; mutations are events — [overview.md](architecture/overview.md)).
- **Contract-first.** Each module starts from its `*.conjure.yml`; server interfaces/clients are
  generated, never hand-edited (D-Conjure / D-Stack).
- **Audit-on-write from M1.** Every permission-sensitive write records in-transaction (D-Audit); no
  domain milestone ships an unaudited mutation.
- **Migrations are versioned & expand-only.** One repo-root `migrations/`, Atlas versioned, the
  `atlas migrate lint` destructive-change gate, boot-time schema-version check
  ([upgrade-safety.md](architecture/upgrade-safety.md), L-UpgradeSafe).
- **Ontology-shaped.** Every table is an Object / reified Link / Action with an RID PK
  (D-Ontology / D-ResourceIdentifiers); writes are audited Actions.
- **Generated OpenAPI** site updates as each module's Conjure lands.

## At a glance

| # | Milestone | Delivers | Depends on |
|---|---|---|---|
| **M0** | Platform & walking skeleton | witchcraft bootstrap, config, observability, DB pool, schema bootstrap (RID/`uuid_v7`/triggers/`geo_countries`), `pkg/` kernel, events bus, crypto seam | — |
| **M1** | Audit | `audit_log`, in-txn `Record()`, `AuditService` reads | M0 |
| **M2** | Localization | `i18n_locales` + `i18n_translations`, `LocalizationService`, locale→text assembly | M0 |
| **M3** | Tenant | units, graphs, edges, per-graph closure, visibility, lifecycle | M0–M2 |
| **M4** | Rank | the one rank scheme (category→type→rank) | M0–M2 |
| **M5** | Person | directory, CLDR names + variants, citizenships, residences | M0–M2, M4 |
| **M6** | Membership | positions (billets) + memberships (belonging/filling) | M3, M4, M5 |
| **M7** | Authorization + PDP | permission catalog, roles, assignments, the PDP, shadow gate, RLS backstop | M3, M5 |
| **M8** | Identity-federation + bootstrap | accounts, external identities, OIDC/JWKS middleware, first-admin bootstrap, recover-admin CLI | M5, M7 |
| **M9** | Document | papers + envelope-encrypted personal codes | M5 (+ M0 crypto) |
| **M10** | Order | наказ + items + event-driven effects on issue | M3–M6 (+ M0 events) |
| **M11** | Hardening & upgrade-safety | staged RLS enablement, lint gate, CI upgrade tests, decision-explain/time-bound polish, packaging | M7–M10 |
| **M12** | Person enrichment & expanded identity | person emails / phones / call signs; RU·BY·LATAM personal-ID schemes; per-document-type attribute schema; date of death | M5, M9 |
| **M13** | Social & messenger channels | platform catalog; messenger reachability over phones/emails; standalone social accounts with analytics-grade attribution (stable id, provenance+confidence, verification) | M12 |
| **M14** | Person↔person relationships | per-type reified self-links: partnership/marriage, kinship, guardianship, sponsorship, next-of-kin, association/COI (friend/follower social-link deferred) | M5 |
| **M15** | Rank systems, NATO grades & presets | a `rank_system` top level (multinational); standardized `grade_code` (NATO STANAG 2116) for cross-system comparability; bundled scheme presets + idempotent `/rank-scheme/import` | M4 |
| **M16** | Hermenea — ingestion & scheduler companion (**absorbs M17**) | a **second binary** `cmd/hermenea` with its **own Postgres**, HTTP-only coupling: connector (http/file/**wof-sqlite**) → raw staging → mapper (incl. a **paged** mapper) → oikumenea `POST /import/{objectType}` idempotent upsert; cron scheduler + `worker_jobs` queue; `import_runs` lineage; service-principal auth. First real connector = the **Who's-On-First geo gazetteer** (`geo_places` + PostGIS, **D-GeoPlaces**, supersedes D-GeoSubdivisions) — **supersedes D-Worker, folds D-DataIngestion; promotes DS-25** | M0, M1 |
| ~~**M17**~~ | ~~Data ingestion & connector framework~~ → **folded into M16** | the connector/mapper/scheduler pipeline now lives in the **hermenea** service (D-Hermenea); oikumenea keeps the generic import endpoint + per-row provenance | — |
| **M18** | Language & writing systems | full Glottolog 5.3 languoid forest + ISO-15924 writing systems; person/unit/locale language links; first M17 consumer | M2, M5, M17 (M3 for the unit tie) |
| **M19** | Location | standalone `location_locations`; PostGIS `GEOGRAPHY` + h3-pg, DB-derived MGRS/H3; structured address over `geo_countries` | M0 |
| **M20** | Education | institutions + structure tree + buildings (Location); enrollments, mentorship, groups, dorm stays; institution positions | M5, M14, M19 (M17 for registries) |
| **M21** | Companies | legal-entity registry: legal form + ownership, registration schemes (LEI), industry classes, positions, equity/UBO links, company↔company graph | M5, M19 (M17 for registries) |
| **M22** | Religion core (multi-faith) | faith-agnostic taxonomy catalogs (religions→traditions→sub-traditions); org nodes reuse tenant units in `canonical`/`tradition`/`affiliation` graphs; catalog-driven org kinds, profiles, policies | M3, M5 |
| **M23** | Clergy grades & credentials | per-tradition ordered clergy-grade catalog + reified credential link; offices as positions + role assignments; religion decree types | M22, M6, M7, M10 |
| **M24** | Religious affiliation & belief | `pii:special` envelope extended (D-SpecialPII); lay affiliation as a reified link; affiliation-type catalog | M22 (+ M0 crypto) |
| **M25** | Religious discovery | sites → Location, service schedules, aliases; closure + PostGIS search; site/service-type catalogs | M22, M19 |
| **M26** | Vehicles (+ subnational subdivisions) | shared `geo_subdivisions` ISO-3166-2 registry; vehicle brand/model/type taxonomy + the vehicle object (VIN); temporal brand↔Company manufacturer link; the ownership+plate registration link (polymorphic person\|company owner, plate region) | M5, M21 |

M1/M2 and M3/M4 are independent and may be built in parallel. Everything after M2 assumes audit + i18n exist.
M12 is **verified** — see its section below (D-PersonContactChannels, D-DocumentAttrSchema, expanded D-PersonalCodes, D-PersonBio amendment); additive person/document enrichments, proven end-to-end (integration suites + a live HTTP demo on the running server).
M13 and M14 are **delivered** — see their sections below (D-PersonSocialChannels, D-PersonRelationships). M14's scoped friend/follower `person_social_links` tie was **deferred — not built** (see decisions.md).
M15 is **delivered** — see its section below (D-RankSystems); it is additive over M4 and refines the L-OneRankScheme lock (one registry, multiple systems).

M16 is **verified** — **re-scoped to the `hermenea` companion service** ([D-Hermenea](architecture/roadmap-decisions.md), which **supersedes D-Worker** and **absorbs M17/D-DataIngestion**): ingestion + the job runtime move **out of process** into a second binary with its own Postgres, coupled to oikumenea **only over HTTP**. M17 is **folded into M16** (no longer a separate milestone). Verified end-to-end (fixture tests both sides + a real `docker compose` cross-service run): the geo-countries pipeline and the **full WOF Ukraine `geo-places` backfill** (35k places, country→region→county→locality, with `geo_countries` enrichment + idempotent re-run); the disputed-territory parent-resolution edge (Crimea) is fixed in the WOF mapper.

M18–M26 are **planned** (designed, not yet built) — a domain cluster derived from `todo.md`, binding once their decisions land: the hermenea ingestion framework (M16) is the foundation the registry-fed verticals ride; **M18** (D-Languages, full Glottolog) is its first real consumer; **M19** (D-Location, PostGIS), **M20** (D-Education), **M21** (D-Companies). **M19 is a foundation reused by M20, M21, and the religion discovery milestone M25**. The **M22–M25** cluster is the **multi-faith religion vertical** (D-Religion, D-ClergyCredential, D-ReligiousAffiliation, D-SpecialPII) — it **promotes DS-48** (Religion) off the parked list and reuses the tenant graph, person/membership/order/authorization, and the shared M19 Location rather than adding new hierarchy machinery. **M26** (D-Vehicles) is the last todo item — a vehicle registry on person + M21 Company; its plate-region FK targets the WOF `geo_places` gazetteer (D-GeoPlaces, built in M16 — which superseded the originally-planned D-GeoSubdivisions and pulled the PostGIS bootstrap forward). The M16–M26 decisions live in [roadmap-decisions.md](architecture/roadmap-decisions.md) (split out of the binding `decisions.md` so it reflects the built M0–M15 surface).

## Stage board

The **single scannable index of where every milestone sits** in the feature pipeline
([development-process.md](development-process.md) defines the gates). One row per `M#`, one column
per gate; the **Stage** column names the furthest gate passed (or the gate in progress). This board
is authoritative for *stage*; the per-milestone prose below is authoritative for *detail*. Every
`✅` corresponds to a real artifact (a `migrations/` file, a `web/` page, a `D-<Name>` block) —
never marked from memory.

Legend: `✅` done · `🚧` in progress · `⬜` not started · `➖` not applicable.

| # | Decided | Designed | Backend | Migrated | UI | Verified | Stage |
|---|:---:|:---:|:---:|:---:|:---:|:---:|---|
| **M0** | ✅ | ✅ | ✅ | ✅ | ➖ | ✅ | verified |
| **M1** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M2** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M3** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M4** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M5** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M6** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M7** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M8** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M9** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M10** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M11** | ✅ | ✅ | ✅ | ✅ | ➖ | ✅ | verified |
| **M12** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M13** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M14** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M15** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified |
| **M16** | ✅ | ✅ | ✅ | ✅ | ➖ | ✅ | verified (geo-countries + full WOF Ukraine geo-places backfill, 35k places, e2e in compose) |
| ~~**M17**~~ | — | — | — | — | — | — | folded into M16 (D-Hermenea) |
| **M18** | ✅ | 🚧 | ⬜ | ⬜ | ⬜ | ⬜ | decided |
| **M19** | ✅ | ✅ | ⬜ | ⬜ | ⬜ | ⬜ | designed |
| **M20** | ✅ | 🚧 | ⬜ | ⬜ | ⬜ | ⬜ | decided |
| **M21** | ✅ | 🚧 | ⬜ | ⬜ | ⬜ | ⬜ | decided |
| **M22** | ✅ | ✅ | ⬜ | ⬜ | ⬜ | ⬜ | designed |
| **M23** | ✅ | ✅ | ⬜ | ⬜ | ⬜ | ⬜ | designed |
| **M24** | ✅ | ✅ | ⬜ | ⬜ | ⬜ | ⬜ | designed |
| **M25** | ✅ | ✅ | ⬜ | ⬜ | ⬜ | ⬜ | designed |
| **M26** | ✅ | 🚧 | ⬜ | ⬜ | ⬜ | ⬜ | decided |

Notes on the planned tier (M16–M26): all have a landed `D-<Name>` decision (in
[roadmap-decisions.md](architecture/roadmap-decisions.md)), so all are at least
**decided**. **M16 is verified** as the **hermenea** companion service (D-Hermenea
supersedes D-Worker and **absorbs M17/D-DataIngestion**); its module doc
([hermenea.md](modules/hermenea.md)) exists, so its *Designed* gate is `✅`, and its UI gate is `➖`
(a headless companion service — no console surface). *Designed* `✅` means a dedicated module doc
exists — present for **M16** ([hermenea.md](modules/hermenea.md)), **M19**
([location.md](modules/location.md)) and **M22–M25** ([religion.md](modules/religion.md)); for
M18/M20/M21/M26 the module doc is still to be written, hence `🚧`. M15's backend is additive over the M4 rank migration
(`20260601000004_rank.sql`), not a separate file. M12 is now **verified**: its exit criteria are met
across the board — grounded in migrations `0012_person_contacts` + `0016_person_date_of_death`, the
`person`/`document` integration suites (`TestContactChannels`, `TestContactTypeCatalogs`,
`TestPurgeGate`, `TestDocumentAttrSchema`, `TestPersonalCodeValidationAndUniqueness`), the
`PersonForms.tsx` console managers, and a live HTTP demo against the running server (email
provider-derivation, phone E.164 + country-derivation, call-sign uniqueness `409`, `date_of_death`
round-trip).

---

## M0 — Platform & walking skeleton

**Goal.** A server that boots, connects to the operator DB, passes health/readiness, and round-trips a
trivial Conjure endpoint — the chassis every module bolts onto.

- **Delivers:** `cmd/oikumenea/main.go` composition root on `witchcraft.Server`; ECV install/runtime
  config (`pkg/refreshable`); observability (`svc1log`/`req2log`, `pkg/metrics`, tracing, health
  reporters incl. the **schema-version readiness gate**); the pgx pool + sqlc plumbing + per-txn RLS
  GUC seam; the gödel/`godel-conjure-plugin` build.
- **Schema bootstrap migration:** the `oikumenea` schema + `citext`; `uuid_v7()`, **`new_id()`** + the
  `rid_*` decoders + the `platform_rid_services`/`platform_rid_types` registries (D-ResourceIdentifiers,
  amended F-014), `set_updated_at()`, `reject_mutation()`; `schema_version`; the seeded
  **`geo_countries`** ISO-3166 registry (D-Geo).
- **`pkg/` kernel:** `id`, `errors` (werror↔Conjure mapping), `pagination`, **`events`** (in-process
  bus + outbox seam), `locale`, `config`, **`crypto`** (`KeyProvider` + `pkg/crypto`, `local-dev`
  backend; D-CryptoProvider), `personalcode` registry (D-PersonalCodes).
- **Implements:** D-Stack, D-Conjure, D-ResourceIdentifiers (packed UUIDv8 `new_id()`),
  L-UpgradeSafe scaffolding. See [platform](modules/platform.md).
- **Exit:** `serve` boots; migrations apply cleanly and re-apply idempotently; `/status/readiness`
  goes green only on a known schema; one demo endpoint returns a `SerializableError` correctly.

## M1 — Audit (cross-cutting)

**Goal.** The append-only Action ledger every later write commits into.

- **Delivers:** `audit_log` (append-only, `reject_mutation()` guard; PK = the Action RID — D-Ontology
  / D-Audit); the in-transaction `Record(ctx, entry)` application service; `AuditService` read
  endpoints (unit-scoped + shadow-gated once those exist). Actor-shape CHECK (`person` | `system`).
- **Implements:** D-Audit. See [audit](modules/audit.md).
- **Exit:** a write + its audit row share one transaction (commit/rollback together); reads paginate.

## M2 — Localization (cross-cutting)

**Goal.** The supported-locale registry + translation store so every label-bearing module can return
`locale → text` maps.

- **Delivers:** `i18n_locales` (seeded `ukr` + `eng`; exactly-one-default trigger) + the polymorphic
  `i18n_translations`; `LocalizationService`; `TranslationsFor(...)` batch assembly; delete/retire
  subscribers that purge orphaned translations.
- **Implements:** D-i18n (all locales in every response, no Accept-Language negotiation), D-Code
  (code vs translatable name). See [localization](modules/localization.md).
- **Exit:** an entity's `name` + its translation rows assemble into a locale-map response.

## M3 — Tenant (structural foundation)

**Goal.** The unit graph the PDP later reads.

- **Delivers:** `tenant_units` (RID PK, `code`, visibility, lifecycle `state`, `level`); `tenant_graphs`
  registry (seeded `command` default/undeletable + `operational`); `tenant_unit_edges`
  (Link `link__parent_of`, per graph); the maintained **per-graph closure** + on-demand
  `verify`/`rebuild` + `closure-drift` diagnostic reporter; `TenantService`.
- **Implements:** D-Graphs, D-DirectoryGraphs, D-ClosureIntegrity, D-ClosureDriftHealth, D-EdgePerms
  (permission *strings* defined here, enforced in M7). See [tenant](modules/tenant.md).
- **Exit:** build a multi-parent DAG; cycle attempts rejected per graph; closure answers
  ancestor/descendant in one lookup; lifecycle transitions recorded.

## M4 — Rank

**Goal.** The single system-wide seniority scheme persons point at.

- **Delivers:** `rank_categories` → `rank_types` → `rank_ranks` (ordered, code+translatable name);
  `RankService` (reads `rank.scheme.read`, writes instance-scope).
- **Implements:** L-OneRankScheme, D-Rank (directory attribute, never authz). See [rank](modules/rank.md).
- **Exit:** a seeded scheme reads as category→type→rank with seniority ordering.

## M5 — Person (the core aggregate)

**Goal.** The personnel directory — account-optional, instance-global.

- **Delivers:** `person_persons` (RID PK; **CLDR** structured names; `birthdate`, ISO-5218 `sex`,
  `country_of_birth`; optional `code`; `rank_id`; lifecycle incl. **purge**/crypto-erase tombstone);
  `person_name_variants` (transliteration); `person_citizenships` + `person_residences` (effective-dated);
  `PersonService`; the `PersonPurged` event.
- **Implements:** D-PersonGlobal, D-PersonNamesCLDR, D-PersonBio, D-Geo, D-PIITiers (per-column PII
  tiers + `werror` redaction). See [person](modules/person.md).
- **Exit:** create a person with no account/unit; multi-citizenship; purge NULLs PII but keeps the id
  tombstone; reads honor the (M7) read-scope rule once authz lands.

## M6 — Membership

**Goal.** People belonging to / filling billets in units.

- **Delivers:** `membership_positions` (unit-owned billets, vacant-first) + `membership_memberships`
  (Link `link__member_of`; effective-dated; optional `position_id`; `order_item_id` provenance seam);
  derived vacancy; `MembershipService`; subscribers for order auto-apply (wired in M10).
- **Implements:** D-Position, one-holder-per-billet. See [membership](modules/membership.md).
- **Exit:** fill/vacate a billet; plain belonging; roster reads (shadow gate active after M7).

## M7 — Authorization + PDP (the centerpiece)

**Goal.** The product differentiator: decisions over the unit graph with per-assignment scope.

- **Delivers:** the **code-defined permission catalog** + seeded **base roles** (D-BaseRoles);
  `authz_roles` (+ `authz_role_permissions`), `authz_role_assignments` (Link `link__has_role`; scope,
  graph, `expires_at`), `authz_instance_admins`; the **PDP algorithm** (union across graphs via
  closure), the **shadow-visibility gate**, and the **RLS backstop** policies + GUC wiring;
  `AuthorizationService` (`/authorize`, `/authorize/batch` with decision-explain).
- **Implements:** D-Inherit, D-InstanceAdmin, D-Graphs (PDP union), D-TimeBoundGrants, D-EdgePerms
  (now enforced), D-NoRLS + D-RLSDefenseInDepth (staged in M11). See [authorization](modules/authorization.md).
- **Exit:** `unit` vs `subtree` scope behave per spec; cross-graph union; shadow gate filters reads;
  no self-escalation; person/membership read-scope rules now enforced.

## M8 — Identity-federation + first-admin bootstrap

**Goal.** Turn an inbound IdP token into a PDP context, and seed the first admin.

- **Delivers:** `account_accounts` + `account_external_identities` (append-only `FEDERATES`);
  the **OIDC discovery + JWKS** validation middleware → PDP context; `IdentityFederationService`
  (`/whoami`, account/identity management); **first-admin bootstrap** (idempotent, from install
  config) + the **`recover-admin` CLI** break-glass path.
- **Implements:** L-AuthzOnly, D-Bootstrap, D-JIT (link-on-match only). See
  [identity-federation](modules/identity-federation.md).
- **Exit:** a valid token resolves to `(person, account)`; an unknown identity is rejected; a fresh
  install bootstraps exactly one instance admin from config.

## M9 — Document

**Goal.** Person-held papers and encrypted national-identifier codes.

- **Delivers:** `document_document_types` + `document_documents` (metadata only); the
  `document_personal_code_schemes` catalog + `document_personal_codes` (**envelope-encrypted** value +
  blind index); `DocumentService`; the `PersonPurged` subscriber that erases papers and **crypto-erases**
  codes.
- **Implements:** D-Documents, D-PersonalCodes, D-CryptoProvider (`pii:sensitive` envelope), the
  read-through-holder scope. See [document](modules/document.md).
- **Exit:** attach a paper; store a personal code as ciphertext + blind-index uniqueness; purge
  crypto-erases.

## M10 — Order

**Goal.** Administrative orders (наказ) as the legal basis for status changes, applied via events.

- **Delivers:** `order_order_types` (category + effect) + `order_orders` + `order_order_items`;
  `OrderService` with **`/issue`** emitting per-item effect events that membership/person subscribers
  apply **in the issue transaction** (all-or-nothing), citing `order_item_id` provenance; revoke as a
  legal-status flip.
- **Implements:** D-Orders, D-OrderApply. See [order](modules/order.md).
- **Exit:** issuing an appointment order fills the billet in the same txn; a failing effect rolls back
  the whole issue; `record-only` items stand alone.

## M11 — Hardening & upgrade-safety

**Status: delivered** (revision `0011_rls`). RLS backstop enabled+tightened in one revision (the
service is pre-release; see [decisions.md](architecture/decisions.md) D-RLSDefenseInDepth *Enablement
timing*), per-request reach GUCs on a pinned connection, the non-superuser `oikumenea_app` role,
`UPGRADING.md` revision log, CI workflows, Docker packaging, and PDP/closure benchmarks.

**Goal.** Tighten the cross-cutting guarantees and package for release.

- **Delivers:** **staged RLS enablement** (permissive→tightened per [upgrade-safety.md](architecture/upgrade-safety.md));
  the `atlas migrate lint` destructive-change CI gate + **CI upgrade tests** (old→new data-safe);
  finalize decision-explain / time-bound-grant ergonomics; Docker + docker-compose packaging; the
  generated **OpenAPI** reference site; load/perf pass on the PDP + closure.
- **Implements:** L-UpgradeSafe end-to-end, D-RLSDefenseInDepth (full enablement).
- **Exit:** an upgrade from the prior release applies non-destructively in CI; RLS backstop active
  without `BYPASSRLS`; image builds and runs from compose.

## M12 — Person enrichment & expanded identity

**Status: verified.** The open questions are resolved (see *Resolved scope* below) and the work is
binding via **D-PersonContactChannels** + **D-DocumentAttrSchema** (and the expanded **D-PersonalCodes**
scheme set, plus the **D-PersonBio** amendment for `date_of_death`) in
[decisions.md](architecture/decisions.md); the only newly parked seam is **DS-40** (phone carrier
lookup). A bundle of additive person/document enrichments — expand-only (new child tables, a new
nullable column, new seed rows, new compiled validators) — shipped end-to-end (migrations
`0012_person_contacts` + `0016_person_date_of_death`, the `person`/`document` integration suites, the
`PersonForms.tsx` console managers) and proven against the running server: a person carries
provider-derived emails, E.164 phones with a derived country, and uniquely-named call signs (duplicate
→ `409`), plus a round-tripping `date_of_death`; a document write is validated against its type's
`attr_schema`; and the expanded national codes validate via their compiled/regex schemes.

**Goal.** Richer contact + identity data on a person: structured emails, phone numbers, call signs, a
wider set of national personal-ID schemes, a per-document-type attribute schema for military papers, and
a **date of death** bio field.

**Resolved scope.**
- **Email/phone `kind`** → instance-admin **catalog tables** (`person_email_types`,
  `person_phone_types`, code + translatable name), not a CHECK enum.
- **Phone** → `github.com/nyaruka/phonenumbers` for E.164 normalization + derived `country`.
- **Email** → stored derived `provider` column (domain→provider map on write).
- **Call signs** → `pii:basic`, required value, **unique per person** among active, `is_primary`.
- **Military docs (D)** → **per-type attribute schema** (`document_document_types.attr_schema` +
  write-time validation), not country-specific typed columns.
- **ID schemes (C)** → RU (`ru-inn`,`ru-snils`), BY (`by-personal-number`), BR (`br-cpf`),
  AR (`ar-dni`,`ar-cuil`), MX (`mx-curp`,`mx-rfc`), CL (`cl-rut`), CO (`co-cedula`); checksum
  validators where well-known, regex/accept-warn otherwise.
- **Date of death (F)** → a single nullable `date_of_death DATE` on `person_persons` (a bio field, not a
  child table), amending **D-PersonBio**; full precision now, partial/approximate dates share the
  existing **DS-38** seam with `birthdate`.

The per-item notes below record the original open questions (now answered as above) for provenance.

**A. Person emails (multiple).** *Where:* new `person_emails` child table (mirrors
[person](modules/person.md)'s `person_citizenships`/`person_residences`: CASCADE to `person_persons`,
soft-delete, `is_primary`). *Shape:* `address` (`citext`, `pii:contact`), `kind` (personal/work/other),
optional derived `provider`. *Open:* provider extraction = map the domain → a known provider
(gmail.com → Google); store derived or compute on read? a closed provider vocabulary or free text?
validation/normalization rules; per-person uniqueness; relation to the login `account_accounts.email`
([identity-federation](modules/identity-federation.md)) — contact email ≠ login email, keep distinct.
`pii:contact`, erased on purge.

**B. Person phone numbers (multiple).** *Where:* new `person_phones` child table (same pattern).
*Shape:* E.164-normalized `number`, derived `country` (from the dial prefix → `geo_countries`), `kind`,
`is_primary`, all `pii:contact`. *Open:* country extraction needs an E.164/libphonenumber parser (pick
a Go lib or a minimal prefix table); **carrier/provider lookup is NOT statically derivable** (number
portability → needs an external HLR/lookup service) → likely **out of scope** or a parked
external-dependency seam; normalization + validation rules. Erased on purge.

**C. Expanded personal-ID schemes — RU, BY, LATAM.** *Where:* additional
`document_personal_code_schemes` seed rows + optional `pkg/personalcode` compiled validators (mirrors
`ua-rnokpp`/`us-ssn`/`pl-pesel`; precedence code-validator > regex > accept-warn, D-PersonalCodes; see
[document](modules/document.md)). Candidates: RU `ru-inn`/`ru-snils`; BY personal number; LATAM
`br-cpf`, `ar-dni`/`ar-cuil`, `mx-curp`/`mx-rfc`, `cl-rut`, `co-cedula`, … *Open:* exact country/scheme
list + `generic_category` mapping; which get a checksum validator vs regex-only; confirm every
`country_iso` is in the seeded `geo_countries` registry. Purely additive (a code change is still needed
for compiled validators, but no schema/decision change).

**D. Military documents — model depth (research item).** *Where:* [document](modules/document.md)
module. `military-id` already exists as a seeded `document_type`; the question is whether a UA military
card (військовий квиток) and analogues need **structured fields** (e.g. ВОС/VOS specialty code, fitness
category, mobilization category, issuing commissariat) promoted out of the `attributes` JSONB. *Open:*
enumerate the real fields per target country; decide generic-document-with-attributes vs typed columns
vs a per-type attribute schema; whether any field is `pii:sensitive`/`pii:special` (envelope-encryption
seam, DS-29). **Resolved → per-type attribute schema** (D-DocumentAttrSchema): a nullable
`document_document_types.attr_schema` declaring typed/validated `attributes`, validated on document
write; `military-id` ships a schema. Generic for all types, not country-specific columns. Genuinely
special-category fields still wait on DS-29.

**E. Call signs (позивний, multiple).** *Where:* new `person_call_signs` child table (same child
pattern). *Shape:* `call_sign` text, `is_primary`, soft-delete; a person may hold several. *Resolved →*
`pii:basic`, **NOT NULL**, **unique per person** among active rows (`UNIQUE (person_id, call_sign)
WHERE deleted_at IS NULL`), surfaced in person reads, erased on purge.

**F. Date of death (bio field).** *Where:* a single nullable column `date_of_death DATE` directly on
`person_persons` (alongside `birthdate`/`sex`/`country_of_birth`), **not** a child table — a person dies
once. *Shape:* full-precision `DATE`, `pii:basic`, returned in person reads as an ISO `YYYY-MM-DD` string
(the existing birthdate string↔`pgtype.Date` pattern). **Amends D-PersonBio** (the binding person-bio
decision). *Notes:* death is a **bio attribute, not a lifecycle state** — it does **not** imply
`deactivated`/`purged` (a deceased person stays an active directory record; status is orthogonal).
Partial/approximate death dates (year-only) ride the same **DS-38** seam as `birthdate`; gender-identity
/ special-category concerns do not apply. NULLed on purge with the other bio fields.

**Dependencies / notes.** All six are additive and depend only on the existing
[person](modules/person.md)/[document](modules/document.md) modules + the `geo_countries` registry.
Items A/B/E follow the existing effective-dated child-table pattern, and the person **purge** erasure
list must be extended to cover their `pii:contact`/`pii:basic` columns (D-PIITiers). When scoped,
update `decisions.md` (new decisions for the contact model + call signs), `glossary.md`,
`ontology-mapping.md` (new Link/Object kinds), and allocate DS-40+ in `open-questions.md`.

- **Exit:** a person carries multiple contact emails (with a derived `provider`), E.164 phones (with a
  derived country), and uniquely-named call signs, plus a `date_of_death` that round-trips and is NULLed
  on purge; a document write is rejected when its `attributes` violate the type's `attr_schema`
  (`military-id`); an `ru-inn` / `br-cpf` personal code validates via its compiled checksum and a
  regex-only scheme (`ar-dni`) accept-warns; purging the person erases the three contact tables.

---

## M13 — Social & messenger channels

**Status: delivered** (revision `0013_person_social_channels`). Binding via **D-PersonSocialChannels** in
[decisions.md](architecture/decisions.md) (promotes open-question DS-41). A purely additive
[person](modules/person.md) enrichment — new catalog, new child tables, new seed rows; no breaking
change. The `person_platforms` catalog + `person_messenger_links` / `person_social_accounts` /
`person_social_account_handles` tables, their `PersonService` sub-resource endpoints (+ `GET
/person/platforms`), holder-scoped reads, audited writes, and purge erasure all landed.

**Goal.** Record a person's **messenger reachability** and **social-network presence** at analytics
grade — borrowing the Palantir ontology practices that make digital-identity data queryable and
weightable (stable id ≠ handle, provenance + confidence on the attribution, platform-vs-operator
verification) — while staying inside the project's PII discipline.

- **Delivers:**
  - `person_platforms` — instance-admin catalog (`code`/translatable `name`, `category ∈
    messenger|social`); seeded `telegram`/`whatsapp`/`signal`/`viber` + `instagram`/`linkedin`/`x`/
    `facebook`; names join localization (`entity_type='platform'`).
  - `person_messenger_links` — reachability layer: XOR FK over `person_phones`/`person_emails` + a
    `messenger`-category platform, optional `verified_at` (Link `link__reachable_on`).
  - `person_social_accounts` — standalone accounts: immutable `platform_user_id` + mutable `handle`,
    `display_name`/`profile_url`/`language` (`pii:contact`), `platform_verified` vs
    `verified_by_operator_at`, and **`source` + `confidence`** attribution on the `HOLDS_ACCOUNT` link
    (Object `PersonSocialAccount`).
  - `person_social_account_handles` — handle-rename history (temporal) so a rename never breaks links.
  - Endpoints `/persons/{id}/messenger-links`, `/social-accounts` (+ account handle history) +
    `GET /person/platforms`; all reads holder-scoped (D-PersonReadScope), writes audited, all four
    tables erased on purge.
- **Implements:** D-PersonSocialChannels (extends D-PersonContactChannels). See
  [person](modules/person.md).
- **Excluded / gated:** **no** time-series social-graph metrics (out of scope outright); free-text
  `bio` + `self_declared_location` are `pii:sensitive` and **wait on the DS-29 envelope seam** (not
  stored now).
- **Exit:** attach a social account with a stable id + sourced/weighted attribution; rename the handle
  and the history records it without breaking the link; mark messenger reachability on an existing
  phone; purge erases all four tables.

## M14 — Person↔person relationships

**Status: delivered** (revision `0014_person_relationships`). Binding via **D-PersonRelationships** in
[decisions.md](architecture/decisions.md) (promotes open-question DS-42, expanded). Per-type reified
self-links, all additive — the `person_relation_types` catalog + **six** link tables
(`person_partnerships`/`_kinships`/`_guardianships`/`_sponsorships`/`_next_of_kin`/`_associations`),
their `PersonService` sub-resource endpoints (per-type `GET`/`PUT`, a polymorphic
`DELETE /persons/{id}/relationships/{id}`, and `GET /person/relation-types`), holder-scoped reads,
audited writes, and both-endpoint purge erasure all landed. The scoped seventh tie,
`person_social_links` (friend/follower), was **deferred — not built** (no consumer / no authoritative
source / redundant with `person_associations`; see decisions.md D-PersonRelationships).

**Goal.** Record family and social structure between people as **per-type reified `Person → Person`
links** (D-Ontology), each mirroring the `membership_memberships` temporal-link shape — covering the
army/church/university domains (kinship, marriage, godparent/advisor/mentor, guardianship, next-of-kin)
plus a Palantir-style generic **association** link for COI / prohibited-contact.

- **Delivers** (each a per-type table, instance-global, holder-scoped reads, audited writes,
  both-endpoint purge erasure):
  - `person_partnerships` (marriage+engagement folded; symmetric canonical pair; ≤1 active marriage/
    engagement; `link__partnered_with`).
  - `person_kinships` (directional `parent_of`, siblings derived; `link__kin_parent_of`).
  - `person_guardianships` (guardian→ward, distinct from blood kin; `link__guardian_of`).
  - `person_sponsorships` (godparent / advisor / mentor; `link__sponsor_of`).
  - `person_next_of_kin` (in-directory nomination + priority; `link__next_of_kin`).
  - `person_associations` (associate / COI / no-contact; `link__associated_with`).
  - `person_relation_types` — catalog for the open-ended relation vocabularies (sponsorship/association/
    next-of-kin labels).
  - *(deferred — not built)* `person_social_links` (friend/follower; `link__social_tie`) — cut on review
    (no consumer / no authoritative source / redundant with `person_associations`); see decisions.md.
- **Implements:** D-PersonRelationships (extends D-Ontology). See [person](modules/person.md).
- **Exit:** record a marriage with one active row per spouse; a `parent_of` kinship; an in-directory
  next-of-kin nomination; a COI association; purging either endpoint erases the link.

## M15 — Rank systems, NATO grades & presets

**Status: delivered** (folded into the rank migration `0004_rank`). Binding via **D-RankSystems** in
[decisions.md](architecture/decisions.md) (extends D-Rank, refines L-OneRankScheme; promotes
open-question DS-43). Additive over M4 — a new top-level table (`rank_systems`), a denormalized
`system_id` down the tree, the seeded `rank_grades` reference catalog (NATO STANAG 2116) + `rank_ranks.grade_code`,
the type tree, system CRUD, `GET /rank-grades`, and the idempotent `POST /rank-scheme/import` (with
bundled `deploy/rank-presets/{us,ua}-armed-forces.json` + `nato-generic.json`) all landed; `person`
untouched.

**Goal.** Let one directory carry **multiple national rank systems at once** (a coalition with US and
Ukrainian ranks), make ranks **comparable across systems**, and let admins **bootstrap a ladder from a
preset** instead of hand-entering every node.

- **Delivers:**
  - `rank_systems` (new top level — national/organizational ladder, optional `country` → `geo_countries`);
    `rank_categories.system_id` denormalized down onto `rank_types`/`rank_ranks`; the scheme becomes
    `rank_system → rank_category → rank_type` (tree) `→ rank`.
  - `rank_grades` — a **migration-seeded** reference catalog (NATO STANAG 2116: `OF-1..OF-10`/`OF(D)`,
    warrant, `OR-1..OR-9`; `tier` + `ordinal`); `rank_ranks.grade_code` optional FK. Cross-system
    **equivalence** = same grade; **seniority** = `tier` then `ordinal`; absent grade ⇒ incomparable.
  - Endpoints: `rank_systems` CRUD, `gradeCode` on rank create/edit, `GET /rank-grades`, and an
    idempotent **`POST /rank-scheme/import`** (code-keyed upsert, additive/non-destructive) applying a
    bundled preset (`deploy/rank-presets/*.json`, opt-in, never auto-seeded). `GET /rank-scheme` nests
    `systems → categories → types → ranks`.
- **Implements:** D-RankSystems (extends D-Rank). See [rank](modules/rank.md).
- **Excluded / parked:** non-military cross-system comparators (academic/ecclesiastical have no STANAG
  analog → `grade_code` stays NULL there) — **DS-43**. Reparenting / moving a node between systems stays
  an open seam.
- **Exit:** import the `us-armed-forces` and `ua-armed-forces` presets; a person holds a rank in either
  system; `OF-5` ranks compare equivalent across the two; re-importing a preset changes nothing
  (idempotent); a non-military system omits grades and simply has no cross-system comparison.

---

## M16 — Hermenea: ingestion & scheduler companion (absorbs M17)

**Status: verified.** Binding via **D-Hermenea** in
[roadmap-decisions.md](architecture/roadmap-decisions.md), which **supersedes D-Worker** (reverses
*in-process*) and **folds D-DataIngestion (M17) into M16**. The background-job runtime + the
reference-data pipeline are realized as a **second deployable, `hermenea`** (`cmd/hermenea`) — a
companion ETL + scheduler beside oikumenea, with its **own PostgreSQL** and its **own Atlas
migrations**, coupled to oikumenea **only over HTTP** (it never touches oikumenea's DB). Promotes the
long-parked **DS-25**; greenfield for hermenea, additive/expand-only for oikumenea. See the
[hermenea](modules/hermenea.md) module doc.

**Goal.** Ingest external reference datasets out of process — fetch → stage raw → map → load via
oikumenea's public import API — on a cron or a push trigger, with at-least-once execution, retry/backoff,
lineage, and hard service separation from the PDP core.

- **Delivers (hermenea, its own DB):**
  - **Connectors** — a `Connector` interface (`Fetch(ctx, source) → RawBatch`); **HTTP(S)** + the
    degenerate **`file`** connector; an `import_sources` registry (`type ∈ http|file`; `jdbc-sql`/
    `object-store` parked **DS-44**), credentials via the crypto seam.
  - **Raw staging** — `import_raw_batches` lands payloads verbatim (checksum, `source_version`,
    `fetched_at`), re-mappable without re-fetch.
  - **Mapper registry** — per object-type, raw records → a **canonical envelope** (`{objectType,
    source, sourceVersion, license, generatedAt, records[]}`).
  - **Scheduler + queue** — an in-process **cron scheduler** + a `worker_jobs` queue
    (`SELECT … FOR UPDATE SKIP LOCKED`), at-least-once with **idempotency keys**, **exponential
    backoff (per-job-type config)**, dead-letter after max attempts, witchcraft-managed **graceful
    drain**, a **job-health reporter**; `import_runs` lineage ledger.
  - **`POST /sync/{source}`** endpoint (the **push trigger** from an oikumenea admin action) + source/
    run/job read endpoints (`api/hermenea.conjure.yml`).
- **Delivers (oikumenea, additive):** the generic **`POST /import/{objectType}`** endpoint over an
  upsert registry (code-keyed, idempotent, non-destructive, one txn, audited as a **`system`** actor);
  **per-row provenance** (`source`/`source_version`/`imported_at`); the **`import.manage`** permission
  + the **`hermenea-importer` service principal** (shared-secret auth path, amends L-AuthzOnly); a thin
  push-trigger HTTP client to hermenea.
- **Two trust directions, two runtime secrets** — `HERMENEA_OIKUMENEA_TOKEN` (import) and
  `OIKUMENEA_HERMENEA_TOKEN` (trigger), ECV-refreshable, never stored.
- **First consumer:** **`geo-countries`** (ISO-3166) — an existing reference catalog needing no new
  oikumenea domain module. M15's `/rank-scheme/import` is **not** retrofitted (legacy one-off).
- **Implements:** D-Hermenea (supersedes D-Worker, folds D-DataIngestion). See
  [hermenea](modules/hermenea.md) + [platform](modules/platform.md).
- **Exit:** hermenea cron fires a `geo-countries` sync; an oikumenea push (`POST /sync/geo-countries`)
  also fires one; raw stage → map → `POST /import/geo-countries` idempotent upsert; **re-running a sync
  changes nothing**; `import_runs` records lineage; per-row provenance is stamped; a bad/missing service
  secret is rejected (401); a failing fetch/map surfaces in hermenea's health + an oikumenea `system`
  audit row; in-flight jobs drain cleanly on shutdown.
- **Verification (delivered).** Fixture-based integration tests cover both sides — `dataimport`
  (`internal/dataimport/dataimport_integration_test.go`: geo-countries create/skip/update idempotency,
  per-row provenance, one `system` audit Action per import, geo-places parent-first upsert + country
  enrichment + RESTRICT) and `hermenea`
  (`internal/hermenea/hermenea_integration_test.go`: trigger dedup, fetch→map→load(stub)→`import_runs`
  lineage, loader-failure → failed run) against dedicated test DBs (`scripts/setup-test-db.sh` now
  provisions `hermenea_test`); the WOF paged mapper is covered against a generated SQLite fixture
  (`internal/hermenea/wof/mappaged_test.go`). A full `docker compose up` ran the **real** cross-service
  pipeline: the bundled `geo-countries-iso3166` `file` source loaded over HTTP (idempotent re-run =
  all-skipped; bad trigger secret → 401; an early load failure retried with exponential backoff in
  `import_runs`), and the **real** `wof-geo-ua` source downloaded + bzip2-decompressed + staged the
  62 MB WOF Ukraine dist, then loaded the **full Ukraine gazetteer** into `geo_places` (**35,072 places**
  — 1 country / 25 regions / 782 counties / 34,264 localities) and **enriched `geo_countries.UA`**
  (`wof_id`/`geom`/`iso_a3`); a re-trigger over the loaded data was an idempotent no-op (all-skipped).
- **WOF parent resolution (resolved).** The first UA run surfaced a disputed-territory quirk — WOF
  parents **Crimea** to a `country_id` outside the Ukraine dist, so its hierarchy-derived `parent_id`
  was absent and the whole region page failed on `geo_places_parent_id_fkey` (SQLSTATE 23503). Fixed in
  the `geo-places` paged mapper ([wof/mapper.go](../internal/hermenea/wof/mapper.go)): it now tracks the
  `wof_id`s it has emitted and **drops a `parentId` whose target isn't in the imported set** (the place
  lands top-level / NULL parent), keeping oikumenea's `parent_id` RESTRICT FK strict as defence in depth.
  Verified: Crimea loads as a top-level region, its 17 counties still attach beneath it, and normal
  regions keep their country parent.

## ~~M17 — Data ingestion & connector framework~~ → folded into M16

**Status: folded into M16 (D-Hermenea).** The generic connector/mapper/scheduler pipeline that was M17
now lives in the **hermenea** companion service (see M16 above); oikumenea keeps only the generic
`POST /import/{objectType}` upsert endpoint + per-row provenance. The pipeline *shape* of
**D-DataIngestion** (sources → raw staging → mapper → canonical envelope → idempotent upsert → lineage)
is adopted unchanged, only **relocated** out of process. M17 is no longer a separate milestone; the id
is retained (append-only) for provenance.

## M18 — Language & writing systems

**Status: planned.** Binding via **D-Languages** in [roadmap-decisions.md](architecture/roadmap-decisions.md). The
**first real consumer of M17** — proves the framework end-to-end via a Glottolog-CLDF HTTP connector.
The Glottolog dataset rides the M17 connector, so it adds **no** parked seam of its own. Additive over
person/localization; the unit tie adds a tenant dependency.

**Goal.** Model the world's languages faithfully (the full **Glottolog 5.3** genealogy), their writing
systems (ISO 15924), and a person's language proficiency — at analytics grade, so language becomes a
queryable, linkable dimension.

- **Delivers:**
  - **`language_languoids`** — the recursive **Glottolog forest**, one table: PK `code` (glottocode);
    `level ∈ {family, language, dialect}`; translatable `name`; self-FK `parent_id` (father — strict
    tree); denormalized `family_code` (root family, derived in SQL via the closure); nullable **UNIQUE**
    `iso639_3`; `macroarea`; representative `latitude`/`longitude` (plain numeric — M18 precedes the
    PostGIS Location); AES `status ∈ {not_endangered…extinct}`; `glottolog_version` provenance.
  - **`language_languoid_closure`** — maintained transitive closure (mirrors the tenant closure) so
    "all languages under Indo-European" is one lookup.
  - **`language_languoid_countries`** — M:N → `geo_countries` (from CLDF `Country_IDs`).
  - **`writing_system_script_types`** catalog (seeded `logographic`/`syllabary`/`alphabet`/`abjad`/
    `abugida`/`featural`); **`writing_systems`** (PK `code` ISO 15924, translatable `name`,
    `script_type`); **`language_writing_systems`** M:N (`is_primary`).
  - **Language links:** `person_languages` (child of `person_persons`: `language_id` constrained to
    `level='language'`, `cefr_level ∈ {A1…C2}` nullable, `is_native`; `pii:basic`, purge-erased);
    `tenant_unit_languages` (unit official/working language); `i18n_locale_languages` (locale → canonical
    language).
  - **Population:** the bundled preset is the **full pinned Glottolog 5.3 CLDF snapshot**
    (`deploy/language-presets/glottolog-5.3.json`, ~26k languoids — opt-in asset, never a migration,
    CC-BY-4.0 attribution carried) loaded via the M17 `language-scheme` mapper; the HTTP connector can
    pull a newer CLDF release the operator points it at.
- **Implements:** D-Languages, D-i18n (translatable names), D-DataIngestion (first consumer). See
  [person](modules/person.md) + a new `language` module.
- **Exit:** import the Glottolog snapshot; query all languages under a family via the closure; a person
  speaks two languages with native + CEFR; a unit declares a working language; purge erases
  `person_languages`.

## M19 — Location

**Status: planned.** Binding via **D-Location** in [roadmap-decisions.md](architecture/roadmap-decisions.md). A new
shared **standalone** entity that M20 (education buildings/dorms) and M21 (company addresses) reference
by FK. Re-adopts geography/PostGIS/H3 — explicitly noted as *dropped from `drafts/`* in decisions.md,
now reversed here with rationale.

**Goal.** A reusable, analytics-grade place entity: a precise coordinate with derived spatial indexes
plus a structured postal address over the existing country registry, so anything with a location
(buildings, campuses, dormitories, company addresses) points at one shared, queryable record.

- **Delivers:**
  - **`location_locations`** — RID PK; **`geom GEOGRAPHY(POINT, 4326)` required** (PostGIS); derived
    **MGRS** string + **H3 indexes** (a small set of resolutions) computed by **DB functions/triggers**
    via the **PostGIS + h3-pg** extensions; structured address: `country_code` (NOT NULL →
    `geo_countries`), `admin_area_1`/`admin_area_2`, `locality`, `street`, `house_number`,
    `postal_code`, `raw_address`; soft-delete; spatial GIST index.
  - A `LocationService` (CRUD + radius/`ST_DWithin` query) and the schema-bootstrap additions for the
    PostGIS + h3-pg extensions (an operator-DB prerequisite, surfaced in the readiness gate).
- **Implements:** D-Location (reverses the `drafts/` geography drop). See a new `location` module +
  [platform](modules/platform.md) (extension bootstrap).
- **Exit:** create a location from a coordinate; MGRS + H3 are derived on write; a structured address
  with only `country_code` set is rejected (coordinate required); a radius query returns nearby
  locations.

## M20 — Education

**Status: planned.** Binding via **D-Education** in [roadmap-decisions.md](architecture/roadmap-decisions.md). A new
module over person + the M19 Location foundation. Institutions are modeled as **external reference
entities** (where a person studied/taught), **independent of companies** (no shared org foundation, per
decision) and distinct from the deploying org's tenant units.

**Goal.** Record the education domain richly enough for analytics and relationship graphs: institutions
of every level, their internal structure and buildings, and the full set of person bindings — who
studied where/when, under whom, in which group/department, and where they lived.

- **Delivers:**
  - **Reference catalogs:** `education_institution_kinds` (kindergarten/school/lyceum/college/institute/
    university/academy…), `education_unit_kinds` (campus/faculty/department/chair…), `education_degree_levels`
    (seeded **ISCED 2011** 0–8).
  - **Objects:** `education_institutions` (code, name, `kind`, `country`→`geo_countries`, founded/closed,
    lifecycle); `education_units` (recursive per-institution structure tree, typed, maintained closure,
    `link__education_unit_parent_of`); `education_buildings` (FK `location_id`→M19, kind incl.
    `dormitory`); `education_groups` (cohort under a unit, admission year).
  - **Person bindings:** `person_education_enrollments` (`link__studied_at` — institution + optional
    unit/group, ISCED `degree_level`, field/specialty, effective-dated, status, qualification awarded;
    mirrors the membership temporal link; `pii:basic`); **mentorship** = extends M14 `person_sponsorships`
    with an optional **education context** (enrollment ref + role ∈ professor/tutor/curator/advisor) —
    no new link type; `person_dormitory_stays` (`link__resided_in_dormitory` — dedicated stay: person ↔
    dorm building, room, period; `pii:contact`, purge-erased).
  - **Positions ("like a military"):** `education_positions` (institution/unit-owned billets —
    rector/dean/head-of-chair/professor/lecturer, vacant-first) + `education_appointments`
    (`link__holds_education_position`, one-holder, effective-dated) — mirrors the membership module.
  - National institution registries (e.g. UA EDBO) ride the M17 connector.
- **Implements:** D-Education (extends D-Ontology; reuses D-PersonRelationships for mentorship). See a
  new `education` module + [person](modules/person.md) (sponsorship extension).
- **Exit:** record a person's enrollment at a university faculty in a study group with a graduation
  qualification; attach their professor as an education-context sponsorship; record a dorm stay; fill a
  dean position; purge erases the person's enrollment/dorm/mentorship rows.

## M21 — Companies

**Status: planned.** Binding via **D-Companies** in [roadmap-decisions.md](architecture/roadmap-decisions.md). A new
legal-entity registry over person + the M19 Location foundation — **independent of education** (per
decision). Scoped to **structural** registry data (identity + affiliation + ownership graph) for
analysis and linking; volatile intelligence (financials/court/tax/sanctions) is parked.

**Goal.** Hold companies (private/public/state-owned/…) at registry grade — identity, legal form,
multi-jurisdiction registration, locations, positions, and the **ownership/affiliation graph** (founders,
shareholders, beneficial owners, parent/subsidiary, succession) — so people and companies link into one
queryable graph.

- **Delivers:**
  - **Reference catalogs:** `company_legal_forms` (per-country: ТОВ/ПАТ/ФОП, LLC/JSC/GmbH…),
    `company_registration_schemes` (mirrors `document_personal_code_schemes`: `ua-edrpou`/`vat`/`us-ein`/
    `duns`/**LEI** ISO 17442 global spine, validators per scheme), `company_industry_classes` (NACE/ISIC/
    KVED economic-activity classification).
  - **Objects:** `company_companies` (code, legal + short names, `legal_form`, `ownership_category ∈
    private|public|state_owned|municipal|foreign|mixed`, `country`, founded/dissolved, lifecycle);
    `company_registrations` (per-scheme identifiers + validation); `company_industry_assignments` (M:N,
    primary+secondary); `company_locations` (→M19, role ∈ registered/operating/branch).
  - **Positions:** `company_positions` + `company_appointments` (`link__holds_company_position` —
    CEO/director/employee billets, mirrors membership).
  - **Equity / ownership links:** `company_foundings` (`link__founded`, founder is a person **or** a
    company); `company_shareholdings` (`link__owns_stake`, polymorphic holder person|company, stake %,
    effective-dated — company-holder edges form the **ownership DAG**); `company_beneficiaries`
    (`link__beneficiary_of` — UBO: ultimate %, declared-vs-computed flag).
  - **Company↔company:** parent/subsidiary (via shareholdings), `company_successions`
    (`link__succeeded_by` — M&A/reorganization lineage), founder-company (via foundings),
    `company_branches` (`link__branch_of` — non-independent sub-units).
  - GLEIF LEI data / national registries (UA EDR) / OpenCorporates ride the M17 connector.
- **Implements:** D-Companies (extends D-Ontology). See a new `company` module.
- **Excluded / parked:** volatile intelligence — financials, court cases, tax debt, sanctions/PEP flags
  (**DS-45**, connector-fed); company web-domain/contact channels (**DS-46**); ownership-graph closure +
  computed-UBO traversal (**DS-47**).
- **Exit:** register a company with a legal form, ownership category, LEI + national number, and a
  registered address; appoint a CEO; record a person founder + a 60% corporate shareholder + a
  beneficial owner; link a subsidiary and a predecessor; query the ownership graph.

---

## M22 — Religion core (multi-faith taxonomy & organization structure)

**Status: planned.** Binding via **D-Religion** in [roadmap-decisions.md](architecture/roadmap-decisions.md), which
**reverses the `drafts/` religion drop** (re-adopted multi-faith, like D-Location re-adopted geography)
and **refines L-SingleDomain** (the single domain may be *religion*, holding many religions/traditions
as catalog data + units in graphs — no org-type discriminator in code). The first milestone of the
**M22–M25 religion vertical**, and the one that **promotes DS-48**. A faith-agnostic structure that
reuses the [tenant](modules/tenant.md) unit graph rather than adding new hierarchy machinery.

**Goal.** Model the organizational structure of **any religion** — religions → traditions → branches →
local worship communities — with a **catalog-driven** taxonomy that fits Christianity, Islam, Judaism,
Hinduism, Buddhism, Sikhism, Bahá'í, Shinto, traditional/indigenous, … with **no hard-coded faith
vocabulary**.

- **Delivers:**
  - **Taxonomy catalogs:** `religion_religions` (top level — Christianity/Islam/Judaism/Hinduism/
    Buddhism/Sikhism/Bahá'í/Shinto/traditional/…), `religion_tradition_families` (nested under a
    religion — Catholic/Orthodox/Protestant; Sunni/Shia; Orthodox/Conservative/Reform; Theravada/
    Mahayana/Vajrayana; …), `religion_sub_traditions` (optional, generic — rite/school/madhhab/
    sampradaya). All `code` + translatable `name` (D-Code/D-i18n), parent FK where nested.
  - **Organization nodes reuse `tenant_units`** with a **catalog-driven** `unit_kind` via
    `religion_org_kinds` (`code`/`name`, optional `religion_id`, `ordinal`); three **seeded religion
    graphs** — `canonical` (governance tree, **authority-bearing**), `tradition` (taxonomic,
    **directory-only**), `affiliation` (voluntary DAG, **directory-only**) (D-Graphs/D-DirectoryGraphs).
  - `religion_org_profiles` (`unit_id` PK/FK → `tenant_units`, `religion_id`, optional
    `tradition_family_id`/`sub_tradition_id`, `short_code`); `religion_org_policies` (generic,
    data-driven eligibility/exclusion — replaces any faith-specific doctrinal flag).
  - A `ReligionService` for the catalogs + org-profile/policy management (catalogs instance-scope;
    org placement reuses tenant unit/edge endpoints on the religion graphs).
- **Implements:** D-Religion (taxonomy + organization), refines L-SingleDomain. Reuses D-Graphs,
  D-DirectoryGraphs, D-Code, D-i18n. See a new [religion](modules/religion.md) module.
- **Exit:** seed **three different religions** in one deployment, each with its own org-kind names;
  build a governance hierarchy in the `canonical` graph (e.g. diocese→parish **and** council→mosque); a
  community affiliates with a cross-cutting network in the `affiliation` graph **without** inheriting
  admin; a data-driven `religion_org_policies` row blocks an ineligible org — with **no
  Christianity-specific enum anywhere in the schema**.

## M23 — Clergy grades & credentials

**Status: planned.** Binding via **D-ClergyCredential** in [roadmap-decisions.md](architecture/roadmap-decisions.md).
Clergy standing is religion-native (**not** the `rank` ladder) and faith-agnostic. Builds on M22 +
reuses [membership](modules/membership.md) positions, [authorization](modules/authorization.md), and
[order](modules/order.md).

**Goal.** Record clergy/religious-functionary **standing** (ordination / investiture / recognition) and
**offices** for any faith, while keeping authority in the one PDP path.

- **Delivers:**
  - `religion_clergy_grades` — an **ordered, per-tradition catalog** (`code`/`name`, `grade_category_id`
    → generic `religion_grade_categories`, `ordinal`, optional `tradition_family_id`): bishop/presbyter/
    deacon; imam/mufti/sheikh; rabbi/cantor; bhikkhu/lama; pujari/swami; …
  - `religion_clergy_credentials` — a **reified Link** `link__clergy_credential` (`person` → grade within
    a tradition/org unit; `granted_on`, conferring-authority provenance, `status ∈ {active,suspended,
    revoked}`, effective-dated, `source`/`confidence`); **indelible where sacramental** (status flip,
    never delete).
  - **Offices** = `religion_office_types` catalog + `membership` **positions** (unit-owned billets) +
    authority via `authorization` role assignments; religion `order` (decree) types — credential
    conferral, appointment, transfer, suspension/revocation, leave.
  - `ReligionService` sub-resources for grades/credentials/offices; reads holder-scoped, writes audited.
- **Implements:** D-ClergyCredential (credential ≠ rank ≠ permission; parallels D-Rank). Reuses
  D-Position, D-Orders, D-OrderApply, D-Audit. See [religion](modules/religion.md).
- **Exit:** confer a clergy grade on a person in a tradition (e.g. ordain a presbyter **and** separately
  recognize an imam in another tradition in the same deployment); appoint each to a worship-community
  office (position + role assignment) via an issued appointment order; suspend via status flip without
  deleting the credential; the PDP grants authority only from the assignment, **never** the grade.

## M24 — Religious affiliation & belief (`pii:special`)

**Status: planned.** Binding via **D-ReligiousAffiliation** + **D-SpecialPII** in
[roadmap-decisions.md](architecture/roadmap-decisions.md). The first feature to **store GDPR Art. 9 data**, so it
**extends the D-CryptoProvider envelope to `pii:special`** (resolving the person-field half of DS-29).
Builds on M22 + the M0 crypto seam.

**Goal.** Record a person's **lay religious affiliation/belief** as protected special-category data.

- **Delivers:**
  - **D-SpecialPII:** the `pkg/crypto` envelope + `KeyProvider` seam (D-CryptoProvider) now also protect
    **`pii:special`** person/affiliation columns (same ciphertext/wrapped-DEK/blind-index/crypto-erase
    mechanics); D-PIITiers' "`pii:special` not stored" caveat lifted for **encrypted** person fields
    (audit-payload ceiling unchanged).
  - `religion_affiliations` — a **reified Link** `link__affiliated_with` (`person` → religion/tradition/
    community unit; `affiliation_type_id` → generic `religion_affiliation_types` catalog — adherent/
    member; catechumen/baptized/confirmed; shahada; bar/bat-mitzvah; …; `status`, effective-dated,
    `source`/`confidence`); value **envelope-encrypted** + blind-indexed; **crypto-erased on purge**;
    reads project through D-PersonReadScope; writes audited.
- **Implements:** D-ReligiousAffiliation, D-SpecialPII (resolves person-field half of DS-29), D-PIITiers,
  D-CryptoProvider. See [religion](modules/religion.md).
- **Excluded / parked:** rite-of-passage / life-cycle records (baptism/bar-mitzvah/…) — a generic
  catalog-typed seam, **DS-49**; the audit-payload half of DS-29 stays parked.
- **Exit:** record a lay affiliation → value stored encrypted, returned only to authorized readers;
  purge a person → affiliation crypto-erased via DEK destruction; uniqueness enforced via blind index
  without decrypting.

## M25 — Religious discovery (sites, schedules, search)

**Status: planned.** Binding via **D-Religion** (discovery surface) in
[roadmap-decisions.md](architecture/roadmap-decisions.md). The discovery substrate over religious structure + the shared
**M19 Location** (PostGIS/H3), source-of-truth in go-oikumenea; a FaithMap-style app consumes it. Builds
on M22 + [M19 Location](#m19--location).

**Goal.** Make religious organizations **discoverable** — where they meet, when they serve, under what
names — with privacy-preserving spatial search, while the CMS/rendering stays in the consuming app.

- **Delivers:**
  - `religion_sites` — a **reified Link** `link__site_of` (worship-community unit ↔ `location_locations`
    (D-Location); `site_type_id` → generic `religion_site_types` catalog — church/cathedral/chapel/
    monastery, mosque, synagogue, temple, gurdwara, shrine, mission, office, online; `visibility ∈
    {public,unlisted,private}`; `public_precision ∈ {exact,street,neighborhood,city,hidden}`;
    `is_primary` one-per-unit). Precision projection (coarsen a coordinate to an H3 cell) lives on the
    **site link**, so one shared location may be published at different precisions by different owners.
  - `religion_service_schedules` — per site: `day_of_week`/RRULE subset, start/end time, IANA `timezone`,
    service `language` (ISO 639-3), `service_type_id` → generic `religion_service_types` catalog
    (main/youth/prayer — Friday-Jumu'ah/Shabbat/daily-mass/puja/meditation/…), `mode ∈ {in_person,
    online,hybrid}`, `meeting_url`, translatable `description`.
  - `religion_aliases` — search-only alt names (`nickname`/`abbreviation`/`historical`/`misspelling`/
    `transliteration`, per-locale); never displayed.
  - **Search** over `ReligionService`: religion/tradition filter via the `tradition`/`canonical` graph
    **closure** (reuse `tenant_unit_closure`) + proximity/viewport via **PostGIS** on
    `location_locations` (D-Location) + service-language/-time + online toggle + fuzzy name/alias.
- **Implements:** D-Religion (discovery surface), D-Location (sites + PostGIS search), D-Graphs
  (closure-driven filter), D-i18n, D-Audit. See [religion](modules/religion.md).
- **Exit:** attach a primary site with coordinates and a weekly main-service schedule to a community;
  search "communities within 5km offering a given-language service on a given day" returns it with
  **coarsened** coordinates per `public_precision`; a transliteration alias matches a query.

---

## M26 — Vehicles (+ subnational subdivisions foundation)

**Status: planned.** Binding via **D-Vehicles** + **D-GeoSubdivisions** in
[roadmap-decisions.md](architecture/roadmap-decisions.md). The **last `todo.md` item** — a vehicle registry
that binds people **and** companies to vehicles in one queryable graph, bundling a shared
`geo_subdivisions` ISO-3166-2 foundation (exactly as M19 bundled the PostGIS/h3 bootstrap with
Location). Additive over person + the M21 Company registry.

**Goal.** Hold vehicles at registry grade — a brand/model/type taxonomy, the physical vehicle (VIN),
and the ownership/plate record — so a person or company links to the vehicles they own/operate and to
the manufacturer behind a marque, with the plate **region** modelled as structured data rather than
free text.

- **Delivers:**
  - **`geo_subdivisions`** — a shared, **platform-seeded** ISO-3166-2 registry (mirrors `geo_countries`/
    D-Geo: `code` PK e.g. `'UA-32'`, `country_code` → `geo_countries`, optional `parent_id`,
    `subdivision_type`, translatable `name` via `entity_type='subdivision'`); UA subset migration-seeded,
    the full global set rides M17; `GET /subdivisions?country=` + instance-scope `subdivision.manage`.
  - **Catalogs:** `vehicle_types` (taxonomy tree, self-FK + denormalized root, no closure — the
    `rank_types` pattern), `vehicle_brands` (`country_code` → `geo_countries`), `vehicle_models`
    (`brand_id`, `generation`, manufacture window), `vehicle_registration_number_types`.
  - **Object:** `vehicle_vehicles` (RID PK; `type_id`/`model_id`; `manufacture_date`; `vin` unique
    among active, nullable, `pii:basic`; `color`; `attributes` JSONB; soft-delete).
  - **Reified Links:** `vehicle_brand_manufacturers` (`link__manufactured_by`: brand → `company_companies`,
    temporal); `vehicle_registrations` (`link__registered_to`: vehicle → **polymorphic owner**
    person **XOR** company, `country_code` → `geo_countries`, `subdivision_id` → `geo_subdivisions`,
    `registration_number` unique-active-per-country, `number_type_id`, temporal + `status`).
  - Holder-scoped reads (D-PersonReadScope) for person-owned registrations, audited writes
    (`CreateVehicle`/`RegisterVehicle`/`TransferRegistration` + catalog edits), and a `PersonPurged`
    subscriber erasing person-owned registrations; national vehicle/brand registries ride M17.
- **Implements:** D-Vehicles (extends D-Ontology) + D-GeoSubdivisions (extends D-Geo). See a new
  `vehicle` module + [platform](modules/platform.md) (the `geo_subdivisions` registry).
- **Excluded / parked:** vehicle lifecycle/intelligence feeds — insurance/MTPL, technical inspection,
  accidents, theft/wanted, odometer, telematics (**DS-52**, connector-fed, mirrors DS-45); column-izing
  stabilized vehicle specs out of `attributes` (**DS-53**, the DS-6 pattern); the full ISO-3166-2 set +
  residence/Location `admin_area_*` retrofit (**DS-51**).
- **Exit:** seed UA subdivisions and read them localized; create a vehicle with a VIN under a brand/
  model/type; register it to a person in a plate region, then transfer it to a company (a new
  registration row, the prior one closed); query who owns a vehicle and which company makes a brand;
  purging a person erases their owned registrations.

---

## Cross-cutting threads (woven through, not separate milestones)

- **Audit** (M1) and **i18n** (M2) are consumed by every later module — land them before the domain.
- **Observability** (M0) accrues per-endpoint RED metrics + the `pdp.decisions{result}` counter as
  modules arrive.
- **RLS backstop** is *defined* with M7 and *fully enabled* in M11 (staged, per upgrade-safety).
- **PII discipline** (D-PIITiers + `werror` redaction + purge) is applied as each PII-bearing table
  lands (M5, M9, audit payload ceiling in M1).

## Deferred to post-v1 (parked seams)

The [open-questions.md](open-questions.md) DS backlog stays parked unless its trigger fires. The
former common blocker — the **background worker runtime (DS-25)** — is now **promoted to M16** (the M17
connector framework needs scheduled syncs), which unblocks the other scheduler-dependent seams
(audit-retention partitioning DS-28, future-dated order effects, expiry sweeps, duty-roster DS-37). The
`pii:special` / audit-payload envelope extension stays parked as DS-29.

The **M12** milestone (above) is now **scoped** — a person/document enrichment bundle (emails, phones,
call signs, RU·BY·LATAM personal-ID schemes, per-document-type attribute schema). Its one newly parked
seam is **DS-40** (phone carrier/provider lookup, needs an external service).

The **M13** and **M14** milestones (above) are now **delivered** — they **promote** DS-41 (social &
messenger channels) and DS-42 (person↔person relationships) out of the backlog into binding decisions
(D-PersonSocialChannels, D-PersonRelationships). They add **no** new parked seams: social-graph metrics
are excluded outright, and the only deferral (free-text social `bio`/location) rides the **existing**
DS-29 envelope seam rather than a new entry.

The **M16–M21** milestones (above) are a new **planned** domain cluster derived from `todo.md`.
They **promote DS-25** (worker runtime → M16) and add new parked seams: **DS-44** (additional ingestion
connectors — SQL/JDBC, object-store), **DS-45** (company registry-intelligence feeds — financials/court/
tax/sanctions), **DS-46** (company web/contact channels), and **DS-47** (ownership-graph closure +
computed-UBO). The Glottolog language dataset rides the M17 connector, so item 1 needs no parked seam of
its own.

The **M26** milestone (above) is the **last** planned domain milestone — `todo.md` item 5
(Vehicles) — promoted into binding decisions (D-Vehicles + D-GeoSubdivisions). It adds new parked seams:
**DS-51** (full ISO-3166-2 subdivision set + residence/Location `admin_area_*` retrofit), **DS-52**
(vehicle lifecycle/intelligence feeds — insurance/inspection/accidents/telematics), and **DS-53**
(column-ize stabilized vehicle specs). Its `geo_subdivisions` foundation and brand/model registries ride
the M17 connector. With M26 designed, `todo.md` is fully consumed and removed.

The **M22–M25** milestones (above) are the **multi-faith religion vertical** (`todo.md` item 4),
which **promotes DS-48** (Religion) into binding decisions (D-Religion, D-ClergyCredential,
D-ReligiousAffiliation) and **resolves the person-field half of DS-29** (D-SpecialPII extends the
envelope to `pii:special`). They add new parked seams: **DS-49** (rite-of-passage / life-cycle records,
`pii:special`) and **DS-50** (location-scoped role assignments — a consuming app's per-site "campus
admin"; today an assignment's scope is `unit|subtree`). The audit-payload half of DS-29 stays parked.
