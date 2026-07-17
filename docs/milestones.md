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
| **M18** | Language & writing systems | full Glottolog 5.3 languoid forest + ISO-15924 writing systems; person/unit/locale language links; first new M16 consumer | M2, M5, M16 (M3 for the unit tie) |
| **M19** | Location | standalone `location_locations`; PostGIS `GEOGRAPHY`; app-derived MGRS; multi-format coordinate input + `source_coordinate`; structured address over `geo_countries` | M0 |
| **M20** | Education | institutions + structure tree + buildings (Location); enrollments, mentorship, groups, dorm stays; institution positions | M5, M14, M19 (M17 for registries) |
| **M21** | Companies | legal-entity registry: legal form + ownership, registration schemes (LEI), industry classes, positions, equity/UBO links, company↔company graph | M5, M19 (M17 for registries) |
| **M22** | Religion core (multi-faith) | faith-agnostic taxonomy catalogs (religions→traditions→sub-traditions); org nodes reuse tenant units in `canonical`/`tradition`/`affiliation` graphs; catalog-driven org kinds, profiles, policies | M3, M5 |
| **M23** | Clergy grades & credentials | per-tradition ordered clergy-grade catalog + reified credential link; offices as positions + role assignments; religion decree types | M22, M6, M7, M10 |
| **M24** | Religious affiliation & belief | `pii:special` envelope extended (D-SpecialPII); lay affiliation as a reified link; affiliation-type catalog | M22 (+ M0 crypto) |
| **M25** | Religious discovery | sites → Location, service schedules, aliases; closure + PostGIS search; site/service-type catalogs | M22, M19 |
| **M26** | Vehicles | vehicle brand/model/type taxonomy + the vehicle object (VIN); temporal brand↔Company manufacturer link; the ownership+plate registration link (polymorphic person\|company owner, **plate region → the WOF `geo_places` gazetteer** — D-GeoSubdivisions superseded, no new geo table) | M5, M21, M16 |
| **M28** | Unit code lifecycle | codeless units (`tenant_units.code` nullable ⇒ non-separate sub-unit) + audited editable codes (`PUT /units/{id}/code`, `tenant_unit_code_events` ledger); the RID is the external handle (`todo.md` items 2 & 3; D-UnitCodeLifecycle amends D-Code) | M3 |
| **M29** | OSINT overlay foundation | provisional persons (`status=provisional` stubs) + manual `MergePerson`; the `source`/`confidence` attribution convention; the structured `legal_basis` catalog (GDPR Art 6/9) — the substrate M30–M37 ride (D-OverlayFoundation) | M5, M1, M21 |
| **M30** | External organizations registry | a dedicated `external_organizations` module (RID service 18) for party/government/military/NGO/registrant orgs, catalog-typed, provisional/resolved, hermenea-fed (D-ExternalOrgs) | M29 |
| **M31** | Physical identity & description | aliases (fold into `person_name_variants`), `person_physical_descriptions` + distinguishing marks, declared ethnicity (`pii:special`), blood type (D-PhysicalIdentity) | M29, M24 |
| **M32** | Structured addresses | `person_addresses` → `location_locations` (home/work/mailing, effective-dated, derivable work address) over the M19 PostGIS point (D-PersonAddresses) | M29, M19 |
| **M33** | Institutional & political ties | per-type person↔org links: party (`pii:special`), government positions (PEP trigger), lobbying, foreign-military (reuse membership), external references; emergency-contact relation type (D-InstitutionalTies) | M29, M30, M21 |
| **M34** | Watchlists & regulatory exposure | live-lookup sanctions/PEP/Interpol **via hermenea** (match-metadata only, ≤24h cache) + `person_regulatory_sanctions` overlay; Interpol API connector (D-Watchlists) | M29, M33, M16 |
| **M35** | Financial/behavioral/psychological overlays | crypto-wallet attribution, declared personality, **inferred** political-leaning (`pii:special`, never merged with declared) (D-PersonOverlays) | M29, M24 |
| **M36** | Health & vulnerability | category-level `person_health_records` (hospitalization/mental-health/disability, `pii:special`, never inferred) + `person_insurance` (D-HealthVulnerability) | M29, M24 |
| **M37** | Login security log | first-party `account_login_events` (ip/context/country/vpn/tor) on the federation seam, fed from the OIDC/JWKS validation path (D-LoginSecurityLog) | M8 |
| **M38** | *(deferred)* Criminal/arrest/court records | designed in its own session — mandatory `disposition`, expungement suppression, jurisdiction rules (draft 6.1–6.3); no decision yet | M5 |
| **M39** | *(deferred)* Compensation/payroll | a separate operational-HR module (the org as payer), out of OSINT-dossier scope; no decision yet | M6 |
| **M40** | Domains + Organizations (multi-domain tenant) | a two-tier model over the unit graph: `tenant_domains` (org-kind catalog) → `tenant_organizations` (the realm a person joins) → units (`org_id`/`domain_id`/`kind_id`); per-org graphs; domain-scoped `tenant_unit_kinds`; `GET /units` requires `?org` (D-TenantOrganizations) | M3 |
| **M41** | Unified org-graph (verticals reuse tenant) | education + company adopt `tenant_organizations`/`tenant_units` (education drops its duplicate `education_unit_closure`); each vertical keeps a `<vertical>_org_profiles` sidecar; `pdp_scoped` domain flag splits operational (reach-RLS, auto-graphs) from reference (university/company: instance-global) (D-UnifiedOrgGraph) | M40, M20, M21, M22 |
| **M42** | Structural color | replace free-text color (`vehicle.color`, person `eye_color`/`hair_color`) with `platform_colors` — a per-domain (`eye`/`hair`/`vehicle`) operator-managed catalog (RID `1,1,1`, i18n name, nullable `hex`) referenced by **hard FK**; in-place creation from a `ColorPicker` (D-Color) | M26, M31 |
| **M43** | Ethnicity catalog & ethnolinguistic links | promote `person_ethnicity_types` to a **hierarchical** catalog (parent + closure) with group-level M:N to languages (Glottolog) + homeland countries; opt-in **CIA World Factbook** `ethnicity-scheme` import — fetched + parsed live at runtime by a `factbook` StreamingConnector + PagedMapper (public domain; flat catalog + homeland-country ties — Factbook has no hierarchy/language; default empty); the encrypted person↔ethnicity link is unchanged; the group↔language tie is never inferred onto a person (D-PhysicalIdentity amendment) | M31, M18 |
| **M44** | Finance (bank accounts & payment cards) | a new `finance` module (RID service 19): bank accounts (**envelope-encrypted IBAN** + blind index) held by a polymorphic person\|company holder link, and payment cards (**envelope-encrypted PAN** + BIN/last-4 display, `debit`/`credit`, **no CVV** — PCI Req 3.2) hanging off an account; a **bank is a `company`-domain `tenant_organization`** (M21/M41), not a new entity (**D-Finance**; retires the final `todo.md` idea — banks/accounts/cards) | M5, M21, M9 |
| **M45** | Pinax reference plane | name + consolidate the instance-global world-model catalogs (colors, countries, Glottolog languages + writing systems, rank systems, religion taxa, ethnicity types + lang/country links) as **`pinax`** — a plane-wide `origin` marker + **bundled YAML seed presets** `go:embed`-ed into oikumenea and boot-**autoseeded** (`pinax.autoseed`, create-if-absent / fill-if-empty / never-delete) through the same `/import` application service the hermenea connectors use; massive data (cities/regions) stays a remote connector (**D-Pinax**) | M18, M16, M42, M43 |
| **M46** | Authorization measurement harness | Phase 0 of the [2026-07 architecture review](architecture/review-2026-07.md): a synthetic national-scale seed generator (`scripts/seed-scale` — 10⁵-unit DAG, 10⁶ persons, grant mix), `pg_stat_statements` in compose, a pgx query-counter tracer (`db.WithQueryCounter`) usable from integration tests, and recorded baseline numbers for the authorization request path (review § Measurements) | M7, M11 |
| **M47** | The authorization wall | Phase 1 of the review — R‑01…R‑03 + R‑12 as one program: request-scoped authority context + an epoch-validated cross-request grant cache (**D-AuthzGrantCache**), reach computed by SQL semi-joins instead of app-side materialization, RLS policies reshaped to a live reach predicate with O(1) GUCs (**D-RLSLiveReach**, amends D-RLSDefenseInDepth), lazy per-request connection pinning, and a deadline on the synchronous hermenea watchlist call | M46 |
| **M48** | Incremental closure maintenance | Phase 2 of the review (R‑04, amends the D-Graphs maintenance note): edge attach/detach adjusts only the affected `anc*(parent) × desc*(child)` closure slice in the write transaction (attach = closure∘closure join with `LEAST`-depth merge; detach = slice delete + bounded re-derivation + reflexive prune, gated on the edge having existed) under a per-graph `FOR NO KEY UPDATE` lock (also closes the guard-then-insert cycle race); the full rebuild survives only as the D-ClosureIntegrity repair path; `VerifyClosureForGraph` becomes depth-inclusive; proven by a random attach/detach differential test vs a Go BFS oracle + scale numbers (single edit 16.6 s → 3–265 ms) | M46 |
| **M49** | Import pipeline industrialization | Phase 3 of the review (R‑05 + the Phase‑3 parts of R‑13, amends D-Hermenea): chunked import runs — `CanonicalEnvelope` gains optional `(runId, seq, isLast)`, the loader sends ~5k-record chunks (one oikumenea transaction each) ended by a trailing finalize chunk, with a finite per-request deadline replacing `WithHTTPTimeout(0)`; the 4 high-volume upsert handlers (geo-places, language-scheme, external-organizations, person-regulatory-sanctions) replace per-record loops with one parallel-array UNNEST merge per chunk (two-pass self-referential parent resolution); a checksum-guarded per-job resume cursor (`worker_jobs.resume_seq`, hermenea migration `0005`) makes a killed run resume instead of restart; `worker.concurrency` fans out N SKIP-LOCKED claim loops; oikumenea boot seeding (pinax + first-admin) runs under a `pg_advisory_lock`. Proven by a 1M-record synthetic WOF run in compose surviving kill −9 of both binaries | M48 |
| **M50** | Search & audit physicals | Phase 4 of the review (R‑06 + R‑07; new decisions D-PersonSearch, D-AuditRetention). **R‑06**: pg_trgm GIN over a generated `search_text` on `person_persons` + `person_name_variants`; directory search matches names **and** transliterations/aliases, filtered in SQL via UNION-of-id-set semi-joins (admin `SearchPersons`, scoped `VisiblePersonIDsForSubjectSearch`) — the Go post-filter (empty-page-while-`hasMore` bug) is deleted. **R‑07**: `audit_log` monthly RANGE-partitioned on `created_at`, PK `(id, created_at)`, index diet 6→4, boot-time `ensure_audit_partition` roll-forward under the boot-seed advisory lock + `DEFAULT` backstop, operator `detach_audit_partitions_before` + `audit.retention-months` config (default retain forever); D-Audit semantics (append-only, RLS) intact. Migrations `0001`/`0005` edited in place | M46 |

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
| **M18** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — both i18n gaps closed (`name` is now a `locale→text` map via `NamesByID`; `i18n_locale_languages` reconciled on import) and re-proven e2e (full 27k Glottolog 5.3 load + the new person/unit/locale language UI). See M18 Verdict (resolved). |
| **M19** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — `location_locations` (PostGIS point + app-derived MGRS + multi-format coordinate input + `source_coordinate`) + audited LocationService CRUD + radius/bbox; stock postgis image; unit + e2e integration tests (D-Location amended 2026-06-17: MGRS app-side, H3 dropped) |
| **M20** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — `education` module (RID service 14): external reference institutions + recursive unit tree (+ closure), buildings (→M19 location), groups, positions/appointments (one-holder), person enrollments + dorm stays (purge-erased) + sponsorship education context; migration `0020_education` (ISCED-seeded degree levels); audited EducationService CRUD; `/education` web page; integration test proves the full slice (closure/cycle/reparent, fill/PositionAlreadyFilled/end, purge erasure). **Reference-layer extension** (`university_ontology.md` adoption, migration `0021_education_reference` + `EducationReferenceService`): curriculum/courses (+ prerequisite cycle guard), research (centres/groups/grants/publications), governance/policy, credentials (qualifications/`diploma` doc-type/accreditation), scholarships, and 6 person↔reference links (purge-erased); operational SIS deliberately excluded — second integration test green |
| **M21** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — `company` module (RID service 15): legal-entity registry over person + M19 location. Catalogs (legal forms / registration schemes w/ per-scheme validators incl. LEI / NACE industry classes), companies (`legal_form` + orthogonal `ownership_category`), per-scheme registrations (validated), industry assignments (one primary), locations (→M19), positions/appointments (one-holder), and the ownership/affiliation graph — foundings/shareholdings (**polymorphic person\|company holder**, ownership DAG), beneficiaries (UBO), successions, branches. Migration `0022_company`; audited CompanyService CRUD + `GET /companies/{id}/ownership-graph` + `/persons/{id}/company-affiliations`; `/companies` web page + person companies panel; integration test proves the full exit slice (LEI+EDRPOU validation, fill/PositionAlreadyFilled, 60% corporate shareholder, UBO, subsidiary/branch, predecessor succession, ownership-graph query, person-purge erasure). DS-45/46/47 (intelligence feeds, web/contact, ownership closure) parked |
| **M22** | ✅ | ✅ | ✅ | ✅ | ✅ | 🚧 | backend verified + UI built — recursive `religion_taxa` tree + closure (D-Religion **refined** 2026-06-19: level-marker catalog, theism classification w/ nearest-declared-wins + unit override, M:N org classifications/one primary, Wikidata anchor). Migration `0023_religion` + curated 98-taxon seed (deep Christianity incl. major historic churches + world religions) via `deploy/religion-presets`. `internal/religion` module (raw-pgx repo, audited app svc reusing tenant for canonical-graph `createChildOrg`, transport on `religion.read`/`religion.catalog.manage`/`religionorg.manage`) wired into `main.go`; `pkg/rid` service 16. **Integration tests green** (seed+inheritance+override, reparent cycle guard, profile+one-primary, effective-type taxon+unit override, `excludes_child_creation` blocks child + canonical edge). `/religion` web taxonomy browser + create + theism-tag editor (type-checks + `next build`). **Remaining: live HTTP/UI click-through + commit** |
| **M23** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — clergy grades & credentials (D-ClergyCredential): per-tradition `religion_grade_categories`/`religion_clergy_grades`/`religion_office_types` catalogs + reified **public** `religion_clergy_credentials` link (`link__clergy_credential`, RID `16,2,2`), indelible (status flip active/suspended/revoked, never deleted). Migration `0024_religion_clergy` (per-tradition seed: bishop/imam/rabbi/bhikkhu/swami…) + RLS on `org_unit_id`; `clergy.manage` perm gated against the conferring unit over the canonical graph. `ReligionService` endpoints (`/clergy-grades`, `/grade-categories`, `/office-types`, person/unit credential lists, add, status-flip update); person-detail "Religion" card + `/religion` roster panel. Integration test green (add→list by person+unit→suspend→reject bad status) |
| **M24** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — lay affiliation (D-ReligiousAffiliation / D-SpecialPII): `religion_affiliation_types` catalog + reified `pii:special` `religion_affiliations` link (`link__affiliated_with`, RID `16,2,3`). The belief value is **envelope-encrypted at rest** (reuses `pkg/crypto` `Cipher` Seal/Open/BlindIndex — D-SpecialPII extends the sensitive-tier envelope unchanged) and **crypto-erased** via `ErasePersonAffiliations` (shared open seam with document `ErasePersonRecords` — the `PersonPurged` subscriber is still deferred). Migration `0025_religion_affiliation`; `affiliation.manage` perm gates read+write (person data). `ReligionService` endpoints (`/affiliation-types`, person affiliation list/add/update/delete); person-detail "Religion" card (Art. 9 notice). Integration test green: **ciphertext at rest contains no plaintext + blind index present, decrypt round-trips, crypto-erase drops the envelope (row survives, value empties)** |
| **M25** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — religious discovery (D-Religion discovery surface): `religion_site_types`/`religion_service_types` catalogs, the reified `religion_sites` link (`link__site_of`, `16,2,4` — org unit ↔ shared `location_locations`, one-primary-per-unit), `religion_service_schedules` (weekly day / RRULE, IANA tz, language, mode, meeting-url guard), and search-only `religion_aliases`. Migration `0026_religion_discovery` (per-tradition seed) + RLS on the unit-scoped tables; `site.manage`/`schedule.manage` perms gated over the canonical graph. `ReligionService` discovery endpoints incl. `GET /discovery/sites` (closure filter via `religion_taxa_closure` + PostGIS `ST_DWithin`/`ST_Intersects`) with **app-side `public_precision` coarsening** (H3 dropped per D-Location 2026-06-17 → `domain.Coarsen` rounding). `/religion` web panels (site/service-type catalogs, discovery search, per-unit sites/aliases). Integration test green (radius+language+day search returns the site, exact/city/hidden coarsening, transliteration alias match, one-primary invariant, online-requires-meeting-url) |
| **M26** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — `vehicle` module (RID service 17): brand/model/type + plate-number catalogs, the `vehicle_vehicles` object (VIN unique-active), the temporal brand→Company `manufactured_by` link, and the ownership+plate `registered_to` link (polymorphic person\|company owner, plate region → the WOF `geo_places` gazetteer per the **D-GeoPlaces supersession** — no `geo_subdivisions` built; country FKs are `country_id`). Migration `0027_vehicle`; audited VehicleService CRUD + register/transfer-as-history + `GET /persons/{id}/vehicles`; a read-only `GET /geo/v1/places` region picker added to GeoService; `/vehicles` web page + person vehicles panel; Go+TS SDK façades extended. Integration test proves the exit slice (VIN/plate conflicts, RegionInvalid, transfer closes prior + opens new, manufacturer link, person-purge erasure). |
| **M27** | ✅ | ✅ | ✅ | ➖ | ✅ | ✅ | verified — unified Go + TypeScript SDKs from the Conjure contract (D-ClientSDK). Go façade `client.New`/`NewWithTokenProvider` (`clients/go/client.go`) over the existing per-service clients; new `clients/typescript` npm package generated by **conjure-typescript** (`scripts/gen-ts-client.sh` + `tools/ir2openapi -dump-ir` + `rewrite-ir-packages.mjs` 3-seg IR rewrite) with a `createOikumeneaClient` façade. Both expose `hermenea`/`import` — oikumenea proxies `/hermenea/v1/*` (D-Hermenea), so one client reaches native + proxied endpoints. `web/` swapped onto the TS SDK (dropped `schema.d.ts` + `openapi-fetch`/`openapi-typescript`); the imports page reads+triggers hermenea through the typed façade. Verified: `client` module builds + façade smoke test; `clients/typescript` tsc build; `web` `next build` clean; UI Docker image (repo-root context + `outputFileTracingRoot`) builds and boots (login `307`). Migrated `➖` (tooling-only, no schema). Publishing tags (npm/pkg.go.dev) are a follow-up |
| **M28** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — unit code lifecycle (D-UnitCodeLifecycle, amends D-Code): codeless units (`tenant_units.code` nullable ⇒ non-separate sub-unit) + audited editable codes (`PUT /units/{id}/code` perm `unit.recode` in the **admin** base role, append-only `tenant_unit_code_events` RID `4,1,4` registered in `platform_rid_types` + `pkg/rid`, `409 Tenant:UnitCodeConflict`); the **RID** is the external handle, not the code. `Unit.code`/`UnitRef.code`/`CreateUnitRequest.code` now `optional`; new `setUnitCode` + `listUnitCodeEvents` endpoints. `web/` create-without-code + "Edit code" action + code-history card. The existing `20260601000003_tenant.sql` + `20260601000000_schema_bootstrap.sql` were edited in place (dev DB reset + `atlas migrate hash`), no new migration file. Integration test green: codeless create + coexisting codeless siblings, set/correct/clear each appending a ledger row, duplicate active code → `409`; full suite re-run shows no other module's seed tests broke. Consumed `todo.md` items 2 & 3 |
| **M29** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — OSINT overlay foundation (D-OverlayFoundation): `person_persons.status` gains `provisional` (a minimal-PII stub; 0005 edited in place) + audited `MergePerson` (`POST /persons/{id}/merge`, perm `person.merge`, admin tier) that re-homes the stub's person-owned edges, publishes the new `PersonMerged` event so **every** person-referencing module (membership/order/identity/document/authz/education/company/vehicle/religion) re-points its rows in the merge transaction, then tombstones the stub `purged`; `POST /provisional-persons` create. The `source`/`confidence`/`as_of` attribution convention is documented in conventions.md for verbatim M30–M37 reuse. New platform `platform_legal_basis_kinds` catalog (migration `0028`, GDPR Art 6 + Art 9, seeded) + `PlatformCatalogService` (`legal-basis.read`/`.manage`). `/people` web provisional-create + merge action + `/legal-basis` catalog view. Integration test green (provisional create, kinship + cross-module account re-homed onto the canonical person, stub tombstoned, non-provisional source rejected); full suite re-run clean after the 0005 edit |
| **M30** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — external_organizations registry (D-ExternalOrgs, RID service 18): `external_org_kinds` catalog (party/government_body/military/ngo/registrant/other, migration-seeded) + `external_organizations` (RID PK, translatable `name`, optional `code`/`country`→geo_countries/`wikidata_id`, provisional/resolved + the M29 attribution column-set). Migration `0029_external_orgs`; `internal/externalorg` (raw-pgx repo, audited `ExternalOrganizationService` CRUD + provisional→resolved `merge`), perms `externalorg.read`/`externalorg.manage` (manage = instance-plane). A hermenea import target: `external-organizations` object-type + idempotent (wikidata-id-keyed) handler in `internal/dataimport`, fed by a live **Wikidata SPARQL** connector — new `internal/hermenea/wikidataorgs` mapper over the `http` connector (User-Agent fix; `wikidata-orgs-ua` source). `/external-orgs` web page + TS SDK façade. Integration test (party+ministry distinct kinds, wikidata/code conflict, provisional→merge tombstone, idempotent import) + the real-connector→mapper→loader hermenea pipeline test (deterministic; the live Wikidata fetch is env-gated `OIKUMENEA_WIKIDATA_E2E` — proven via curl, but the sandbox IP is WDQS-rate-limited) |
| **M31** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — physical identity & description (D-PhysicalIdentity, migration `0030_person_physical_identity`). Aliases fold into `person_name_variants` via a `variant_kind` discriminator (transliteration\|aka\|former_legal\|maiden\|pseudonym\|cover — partial-unique one-transliteration-per-locale, aliases free-form by RID + `source`/`confidence`); `person_physical_descriptions` (effective-dated height/weight/eye/hair/build/`blood_type`, pii:basic) + `person_distinguishing_marks` (tattoo/scar/piercing/birthmark, pii:special ceiling); declared-only open `person_ethnicity_types` + the encrypted `person_ethnicities` link (`link__has_ethnicity`, RID `6,2,9`) — the declared code is **envelope-encrypted** (reuses `pkg/crypto` Seal/Open/BlindIndex, NO plaintext FK) + NOT-NULL `legal_basis` → `platform_legal_basis_kinds`; biometrics excluded. Person now holds the envelope cipher (`NewService` + `main.go` build-cipher moved ahead of `person.Register`). New person RIDs `6,1,11..13` + `6,2,9`; `PersonService` endpoints (name-aliases / physical-descriptions / distinguishing-marks / ethnicity-types / ethnicities) + Go/TS SDK regen (`--verify` clean); `/persons/[id]` "Physical identity" card + alias section. Integration test green: AKA + former-legal aliases (transliteration coexists), description+blood-type+mark, **ethnicity ciphertext at rest holds no plaintext + blind index present + decrypt round-trips**, purge crypto-erases the ethnicity (row tombstone) and hard-deletes description/mark; full person + cross-module (order/membership/authz/document/religion) suites re-run green |
| **M32** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — structured, effective-dated person addresses over M19 Location (D-PersonAddresses). Migration `0031_person_addresses` (`person_addresses` reified link `link__lives_at` RID `6,2,10` → `location_locations`, `role∈home\|work\|mailing\|other`, effective-dated, one-active-primary partial-unique, `privacy_seeking`, `source`/`confidence` attribution, `pii:contact` → hard-deleted on purge; `person_residences` retained for legal residence). New `domain.LocationLookup` seam (mirrors `ColorLookup`) satisfied by geo `LocationExists` and late-bound `personSvc.SetLocationLookup(geoSvc)` in `main.go`; audited `PersonService` list/upsert/delete (`/persons/{id}/addresses`, read on `person.read`, write on `person.update`, unknown location → `Person:PersonInvalid`); Go+TS SDK regen (`--verify` clean); `/persons/[id]` **Addresses** card (location typeahead via the shared `SearchSelect`, role/primary/privacy). Integration test green (add home, demote-on-second-primary, unknown-location rejected, privacy round-trip, purge hard-deletes); full person + geo suites re-run green |
| **M33** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — institutional & political ties (D-InstitutionalTies), migration `0032_person_institutional_ties`. Per-type reified person↔org affiliation edges (mirroring M14): **`person_party_memberships`** (link `6,2,11`, `pii:special` — the party identity is **envelope-encrypted + blind-indexed** like the M31 ethnicity, NOT-NULL `legal_basis` Art. 9, crypto-erased on purge), **`person_government_positions`** (link `6,2,12`, `pii:basic`, `pep_trigger` auto-true on create + persists post-office → the M34 PEP seam `IsPoliticallyExposed`; optional polymorphic `org_id` + `country_id`), **`person_lobbying_relationships`** (link `6,2,13`, `pii:basic`, `issues[]`/`filing_id`/`source_url`), and **`person_external_references`** (object `6,1,14`, `pii:basic`, idempotent by `(person,url)`; a hermenea import target). Emergency contacts reuse M14 — the migration only seeds an `emergency` `person_relation_type` (no new entity); foreign-military service reuses membership+rank (no table). Inferred political leaning stays a **separate** M35 overlay (never merged). `PersonService` list/upsert/delete on all four (`person.read`/`person.update`, audited); merge re-points the four person-owned tables; purge crypto-erases party + hard-deletes the plaintext ties. Go+TS SDK regen + web "Institutional & political ties" card (party w/ Art-9 legal-basis picker, gov positions w/ PEP flag, lobbying, external refs). Integration test green: party ciphertext-at-rest holds no plaintext + blind index present + decrypts; `legalBasis` required; PEP derivation flips on gov-position add/delete; lobbying `issues[]` round-trip; external-ref idempotent by URL; `emergency` type seeded; purge crypto-erases party (row tombstone) + hard-deletes gov/lobbying/external. Full person + cross-module integration sweep green; web `tsc` + `next build` clean |
| **M34** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — watchlists & regulatory exposure (D-Watchlists). Migration `0033_person_watchlists`: `person_watchlist_matches` (object `6,1,15`, one active row/person — match METADATA only, `pii:sensitive`, hard-erased on purge) + `person_regulatory_sanctions` (object `6,1,16`, `pii:sensitive`, idempotent by `(person, externalId)`). The **first synchronous oikumenea→hermenea** surface: `CheckWatchlists` runs OUT to the companion via the late-bound `WatchlistLookup` seam (`SetWatchlistLookup` in main.go, mirrors `SetLocationLookup`), combines the returned metadata with the locally-derived M33 PEP flag (`IsPoliticallyExposed`), and upserts the single per-person match. hermenea owns egress + a ≤24h cache (`hermenea.watchlist_cache`, migration `0004`); the **real INTERPOL Red Notices connector** ships (`internal/hermenea/watchlist`, env-gated live + deterministic fixture tests) with OFAC/EU/UN sanctions as a documented `SanctionsStub`. `person_regulatory_sanctions` is also a hermenea **import target** (`POST /import/person-regulatory-sanctions`, idempotent handler + `regulatorysanctions` passthrough mapper). `PersonService` endpoints (`watchlist-check`/`watchlist-match`/`regulatory-sanctions` CRUD); merge re-homes the durable sanctions + drops the transient match; purge hard-deletes both. Go SDK regen; `/persons/[id]` **Watchlists & regulatory exposure** card (run-check button + PEP badge + sanctions panel). Integration tests green (person: metadata-only persist, re-check refresh, PEP snapshot, merge/purge; dataimport: create/skip/update/unresolved-skip; hermenea: interpol mapper + cache-first/expiry). Sanctions providers + an operator-registered reg-sanctions bulk source remain open seams |
| **M35** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — financial/behavioural/psychological overlays (D-PersonOverlays). Migration `0035_person_overlays`: `person_crypto_wallets` (object `6,1,17`, pii:sensitive — address public on-chain but attribution sensitive; dedup active `(person,chain,address)`; M34 sanctioned-wallet synergy), `person_personality` (object `6,1,18`, pii:sensitive; `method` CHECK enforces **declared-survey / HR-assessment only** — no text-inference; one active per framework), and `person_political_leaning` (object `6,1,19`, **INFERRED**, pii:special — the spectrum ∈ [-1,1] is **envelope-encrypted** + blind-indexed like the M33 party, NOT-NULL `legal_basis` Art. 9, one active row/person refreshed in place, **crypto-erased on purge**; a **SEPARATE** table from the declared M33 party membership, never merged). `PersonService` list/upsert/delete on all three (`person.read`/`person.update`, audited); merge re-homes wallets+personality (leaning's partial-unique person_id can't collide → dropped on the merge Purge, never merged with declared); purge hard-deletes wallet+personality (pii:sensitive) and crypto-erases the leaning. Go+TS SDK regen; `/persons/[id]` **Financial, behavioural & psychological overlays** card (wallets + personality + inferred-leaning editor w/ Art-9 legal-basis picker). Integration test green (wallet round-trip+dedup, personality rejects inferred method, **leaning ciphertext at rest holds no plaintext spectrum + blind index present + decrypts**, single-active replace, legalBasis required, purge hard-deletes sensitive + crypto-erases leaning); full person suite re-run green; web `tsc` + `next build` clean; **live HTTP smoke** (wallet, personality 400-on-`text_inference`, encrypted leaning round-trip + no-plaintext-at-rest, missing-legalBasis 400). Compensation/payroll deferred → M39 |
| **M36** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — health & vulnerability records (D-HealthVulnerability). Migration `0038_person_health`: `person_health_records` (object `6,1,20`, `pii:special` — the category-level `detail` is **envelope-encrypted** + blind-indexed like the M33 party / M35 leaning, plaintext `kind∈{hospitalization,mental_health,disability}` drives a one-active-per-`(person,kind)` index, NOT-NULL `legal_basis` Art. 9, **need-to-know read gate** `person.health.read`, category-level only / never a diagnosis / never inferred, crypto-erased on purge) + `person_insurance` (object `6,1,21`, `pii:sensitive` plaintext, gated on `person.read`, hard-erased on purge). New `person.health.read` code folded into the existing `sensitive-reader` base role (no new role, no PDP change). `PersonService` list/upsert/delete on both (`personsensitive` module: `application/health.go`, sqlc + adapter, `ErasePerson` extended; transport `health.go`); action catalog + `pkg/rid` + `ontology-mapping.md` §3.1 updated. Go+TS SDK regen; `/persons/[id]` **Health & vulnerability** card (legal-basis picker + insurance). Integration test green (encrypted-at-rest holds no plaintext + blind index + decrypt round-trip, legalBasis required, one-active-per-kind replace, purge crypto-erases health / hard-deletes insurance); full person + cross-module suite re-run green; web `tsc` clean; **live HTTP smoke** (health 200 w/ legalBasis, 400 without; insurance 200; raw ciphertext holds no plaintext). Pre-existing shared gap noted: unknown `legalBasis` code → 500 (also true of M33/M35 — `ErrUnknownLegalBasis` unmapped in the person transport) |
| **M37** | ✅ | ✅ | ⬜ | ⬜ | ➖ | ⬜ | designed — first-party login security log (D-LoginSecurityLog); UI `➖` (security-log read view optional) |
| **M38** | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | deferred — criminal/arrest/court records; own design session (no decision yet) |
| **M39** | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | deferred — compensation/payroll operational-HR module (no decision yet) |
| **M41** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — Unified org-graph (D-UnifiedOrgGraph). **Phase 0 done + verified**: `tenant_domains.pdp_scoped` + denormalized `tenant_units.pdp_scoped` (derived in SQL); `tenant_units_reach` RLS exempts reference units (`NOT pdp_scoped`); `CreateOrganization` lazy-seeds per-org graphs only for operational domains + new `EnsureGraph`; boot-seed sets university/company = reference. **Phase 1 (education) done + verified**: `education_institutions`/`education_units`/`education_unit_closure`/`education_unit_kinds` dropped (0020/0021 edited in place); an institution = `university`-domain tenant org + `education_org_profiles` sidecar; units = tenant units in a per-org `structure` graph (closure = `tenant_unit_closure`); buildings/groups/positions/enrollments re-pointed to tenant org/unit RIDs (column names kept); `EducationService` delegates structure to the tenant service (verify/rebuild-closure + unit-kind-upsert endpoints dropped); svc-14 `institution`/`education_unit`/`unit_kind`/`education_unit_parent_of` RID types removed. **Phase 2 (company) done + verified**: `company_companies` dropped (0022 edited in place; svc-15 `company` object RID removed); a company = `company`-domain tenant org + `company_org_profiles` sidecar (no per-org graph — companies have no unit tree); registrations/positions/locations + the ownership-graph links (foundings/shareholdings/beneficiaries/successions/branches) re-pointed to tenant org RIDs (column names kept); the cross-module `vehicle_brand_manufacturers.company_id` FK → `tenant_organizations`; `CompanyService` delegates org create/rename to the tenant service. **Phase 3 (religion) done + verified**: first-class `createRootOrg` (`POST /religion-orgs`, instance perm `religion.catalog.manage`) builds a `church`-domain tenant org + root religious-body unit + profile (+ optional primary classification) — replaces hand-seeded org/unit fixtures; the church-domain exception documented (church `pdp_scoped=true` but uses the instance-global canonical/tradition/affiliation graphs, not per-org command/operational); new `TestCreateRootOrg` (root in church domain + child lands in canonical graph). company+vehicle+tenant+education+religion+authz+membership+order+person+RLS suites green on fresh `oikumenea_test`; full unit + integration sweep green. **Phase 4 (web + SDK + atlas) done + verified**: TS SDK regenerated (`scripts/gen-ts-client.sh`; education `closureReport` removed, religion `createRootOrg` added, M40/M41 tenant types surfaced) — `clients/typescript` builds + `--verify` drift gate passes; web fixed for the M40 tenant `Unit` field rename `unitKind`(free-text)→`kindId`(RID) in `units/[unitId]`, `UnitForms`, `units/new`, `lib/ontology/registry` — `tsc --noEmit` clean + `next build` succeeds; `atlas migrate validate` passes (the atlas release channel is now uniformly v1.2.x — local + official-latest + CI `setup-atlas@v0` produce identical `atlas.sum`, so the re-hashed sum is commit-ready; no separate "stable" atlas exists/needed). **All phases (0–4) verified. Deferred follow-up (additive, not a gate):** `POST /import/education-institutions` + `company-registry` import handlers (no hermenea connectors yet) |
| **M42** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — structural color (D-Color, migration `0030_person_physical_identity` — squashed with M31/M43). `platform_colors` is platform's first RID Object (`1,1,1`): per-domain palette (`eye`/`hair`/`vehicle`, TEXT+CHECK), stable `code` + i18n `name` (localization store keyed by RID, entity `color`), nullable `hex` swatch, seeded eye/hair/vehicle baselines. The three free-text columns became hard FKs — `vehicle_vehicles.color_id`, `person_physical_descriptions.eye_color_id`/`hair_color_id` (`uuid REFERENCES … ON DELETE RESTRICT`, backfilled by `lower(text)` match then dropped). A single-column FK can't enforce the palette, so person/vehicle application services validate the color's `domain` via a cross-module `ColorLookup` (the platform color service), returning `ErrColorMismatch`. Catalog read/upsert on `PlatformCatalogService` (`color.read` reader-tier / `color.manage` instance-plane, audited `color.upsert`); `api.platformCatalog.listColors`/`upsertColor` in Go+TS SDKs. Web `ColorPicker` (per-domain typeahead + swatch + **create-in-place**) wired into the vehicle form + the person physical-identity card. Integration tests green: person eye/hair color round-trip + wrong-palette (`vehicle` color as eye) rejected; vehicle color round-trip + eye-palette color rejected; person + vehicle suites re-run green. `tsc` + `next build` clean; live smoke (create color → list → set on person/vehicle) |
| **M43** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — ethnicity taxonomy & ethnolinguistic links (D-PhysicalIdentity amendment, migration `0030_person_physical_identity` — squashed with M31/M42). `person_ethnicity_types` promoted from a flat, seeded-empty catalog to a **hierarchical** one (no new RID types — the catalog keeps `6,1,13`, `parent_id` is a plain self-FK): `parent_id` + `wikidata_id` + import provenance, `person_ethnicity_type_closure` (recursive-CTE rebuild, like `language_languoid_closure`), and group-level M:N `person_ethnicity_type_languages` (→ `language_languoids`) + `person_ethnicity_type_countries` (→ `geo_countries`) — bare associations, RID-less. **The encrypted person↔ethnicity link (`person_ethnicities`) is unchanged**; the group↔language tie is reference metadata **never inferred onto a person**. Opt-in import from the **CIA World Factbook** (public domain), fetched + parsed **LIVE at runtime — no committed preset, no Python**: new `ethnicity-scheme` dataimport handler (closure-rebuilding, idempotent — a clone of `language-scheme`) + `EthnicityStore`/`EthnicityRepo`; a `factbook` **StreamingConnector** (`internal/hermenea/connector`) that enumerates the ~260 country files in the `factbook/factbook.json` GitHub mirror via one git-tree API call + stages them, and the `factbookethnicities` **PagedMapper** that parses each country's "Ethnic groups" free-text in Go, derives ISO from the Internet ccTLD (`.uk`→GB exception), and dedups group→countries. Source `factbook-ethnicities` in `hermenea-install.yml` (`connector-type: factbook`, locator `factbook/factbook.json@master`) **`enabled: false`** (default catalog stays EMPTY). Factbook is **flat + homeland-country-linked** (no hierarchy/language ties — the parent/closure/language-M:N machinery stays in the schema, unpopulated by this source). Live test (env-gated `OIKUMENEA_FACTBOOK_E2E`) proves the runtime path: **634 ethnic groups fetched+parsed from GitHub** (Ukrainian→UA, Russian→RU+UA, White→GB). Wikidata (richer, hierarchical) was evaluated but its endpoints 403 datacenter/CI IPs (Wikimedia bot policy T400119); Factbook over GitHub is reachable at runtime. Person read surface: `listEthnicityTypes(topLevel|parent|query)` + computed `hasChildren` + `getEthnicityType` (assembles group languages/countries), i18n `name` map; web `EthnicityPicker` (lazy tree + search, yields the catalog `code`) replaces the flat `<select>` in the person ethnicity form. Integration tests green: dataimport (parent resolved, closure depth, language/country ties w/ unresolved-key drop, idempotent re-run + version bump) + person read (roots/children filters, `hasChildren`, `getEthnicityType` languages/countries); the existing encrypted ethnicity add/list/purge tests unchanged. Go+TS SDK regen; web `tsc` + `next build` green |
| **M40** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — Domains + Organizations multi-domain tenant (D-TenantOrganizations). **Decided/Designed**: D-TenantOrganizations + amendments (D-Graph/D-Graphs/D-Code/L-SingleDomain/L-UnitIsTenant) + CLAUDE.md; ontology-mapping rows + `pkg/rid` (`4,1,5..8`); tenant.md/glossary; `api/tenant.conjure.yml` (`Domain`/`UnitKind`/`Organization` + catalog/org endpoints + `org`-required `listUnits` + nullable `orgId` on `Graph`). **Migrated**: `0003_tenant.sql` edited in place (4 new tables + `org_id`/`domain_id`/`kind_id` on units + nullable `org_id`/per-org+global indexes on graphs) + `platform_rid_types` seed; **full 29-migration set applies cleanly** (religion `0023` global-graph seed reconciled to `org_id IS NULL`); re-hash atlas.sum with stable atlas before commit. **Backend**: Conjure+sqlc regen; `internal/tenant` reworked (domain/org/unit-kind services, `CreateOrganization` seeds per-org `command`+`operational` graphs in-tx, boot-seeds domain+unit-kind catalogs, dropped global graph seed; edges/closure resolve the graph within the unit's org with a global fallback); authz `domain/unit-kind/organization` perms + org-aware `ResolveGraph`; religion `CreateChildOrg` inherits the parent's org. **Verified (backend)**: `go build ./...` + 27 unit-test packages green; **all 7 affected integration suites pass on a live DB** (tenant, authz PDP per-org graph, membership, order, person read-scope, religion, RLS). **UI ✅**: TS SDK already regenerated (M41 Phase 4 surfaced the M40 tenant types); the **Organizations** console (create / inline-edit / suspend-archive-restore lifecycle — `web/src/components/OrganizationManager.tsx`, `/organizations`) and domain-first unit creation (`units/new`, `UnitCreateMenu`, `NewUnitForm`) were built alongside M41; the remaining **domain + unit-kind catalog console** now ships as `/domains` (`web/src/app/(dashboard)/domains/page.tsx` + `DomainManager` — create/edit/retire a domain, each row expands to a lazy-loaded `UnitKindManager` for the domain-scoped unit kinds incl. optional attr-schema JSON), wired into `Nav`/`messages.ts` (`nav.domains`). Web `tsc --noEmit` clean + `next build` succeeds (`/domains` route compiled). Live catalog→picker smoke (create domain → add unit-kind → appears in `units/new` kind picker) is the remaining manual check |
| **M45** | ✅ | ✅ | ✅ | ✅ | ➖ | ✅ | verified — the **`pinax`** reference plane (D-Pinax + [pinax plane note](architecture/pinax.md)): a plane-wide `origin` marker (`seeded`/`operator`), **bundled YAML seed presets** `go:embed`-ed into oikumenea + boot-**autoseeded** (`pinax.autoseed`, default on) through the same application import service the HTTP `/import` wraps — **create-if-absent / fill-if-empty / never-delete**, version-gated via `pinax_seed_state`, `--reconcile` (`oikumenea seed`) for explicit refresh. All 7 handlers built (`internal/pinax` seeder + native-importer escape hatch for ranks; `internal/dataimport` handlers for religions/colors + country fill-if-empty enrichment inc. **low-res border polygons** → `geom`/`centroid`/`bbox`; languages/writing-systems/ethnicities reuse the existing import handlers with `CreateOnly`); in-place migration edits (`origin` + `color_id` + `pinax_seed_state`) landed. **Full data bundle** produced by the reproducible generator [`deploy/pinax-presets/gen`](../deploy/pinax-presets/): languages **27 177** Glottolog 5.3 (topo-sorted), 998 CLDR script links, 250 countries (174 with Natural-Earth borders), 100 religion taxa + theism, 634 Factbook ethnic groups, 206 ranks, color palettes. Verified e2e: fresh 27k-languoid load (closure+`family_code`), country polygons materialize `geom`/centroid/bbox, version-gated re-boot no-op, operator row survives `--reconcile`, `autoseed:false` gate; unit + integration tests green. Massive `geo_places` stays a remote hermenea connector. UI `➖` (headless). **i18n translation overlay (D-i18n):** two new locales **`spa`** (LATAM) + **`por`** (Brazil) seeded in migration `0002`; a generic `translations` object-type/handler resolves each entity's natural key→i18n `entity_id` (code for country/ethnicity_type, RID for languoid/writing_system/religion_taxon/rank_*/color) and writes `i18n_translations` create-if-absent; the `translations` preset (1243 rows: CLDR uk/en/es-419/pt for **264 countries, ~600 languages, ~210 scripts** + curated **colors/religions/ranks**) is generated by `deploy/pinax-presets/gen`. Country name went `string`→`map<string,string>` (geo.conjure + geo transport `NamesByID` + regen Go/TS SDK + web `CountrySelect`); colors were already map-typed. Verified: `NamesByID` returns 4-locale maps e2e (country Україна/Ucrania/Ucrânia, languoid українська/ucraniano, color Синій/Azul, rank Coronel). Ethnicities have no multilingual source (English-only). |
| **M44** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | verified — `finance` module (bank accounts + payment cards, D-Finance), **RID service 19**. A `company`-domain `tenant_organization` is the bank; `finance_accounts` (`19,1,1`, **envelope-encrypted IBAN** + blind index unique among active) held via the polymorphic `finance_account_holders` link (`19,2,1`, person\|company, `primary`/`joint`/`authorized_signer`, temporal); `finance_cards` (`19,1,2`, **envelope-encrypted PAN** + clear BIN/last-4, `debit`/`credit`, **no CVV**, containment FK CASCADE) under an account; `finance_account_types`/`finance_card_networks` catalogs (`19,1,3`/`19,1,4`, seeded). Migration `0034_finance` (bumped `ExpectedSchemaRevision`); new `pkg/personalcode` `iban` (ISO-13616 mod-97) + `pan` (Luhn + `SplitPAN`) validators reusing `pkg/crypto` Seal/Open/BlindIndex — **zero new crypto machinery**. `internal/finance` (raw-pgx repo, audited app svc, transport on `finance.read`/`finance.manage`/`finance.catalog.manage`), wired into `main.go` (reuses the shared cipher + `personalcode.New()`) + `SubscribePersonEvents` re-homing. `FinanceService` Conjure contract → Go+TS SDK façades regenerated (both build). `/finance` web workspace (accounts + catalogs + account drawer w/ holders + cards) + person "Bank accounts" panel; `tsc` + `next build` clean. Integration test green (**IBAN ciphertext at rest holds no plaintext + blind index unique + decrypt round-trips**, joint 2nd holder, card BIN/last-4 clear + duplicate-PAN conflict, bad IBAN/PAN rejected, **person purge crypto-erases the solely-held account + its cards while a company-held account survives**, catalog reads); `rid.AssertMatches` clean incl. svc 19; full unit + representative integration regression green. Parks **DS-54** (BIN+last-4-only) + **DS-55** (balance/transaction ledger) |
| **M46** | ✅ | ✅ | ✅ | ➖ | ➖ | ✅ | verified — authorization measurement harness ([review-2026-07](architecture/review-2026-07.md) Phase 0). `scripts/seed-scale` generator (deterministic 10⁵-unit multi-parent DAG + 1.2M-row closure + 10⁶ persons/memberships + grant mix with three probe subjects, `pgx.CopyFrom` + client-side UUIDv8 RID minting, 36 s wall); `pg_stat_statements` preloaded in both compose files + best-effort CREATE EXTENSION in `setup-test-db.sh`; `db.WithQueryCounter` pgx tracer (installed on every `db.NewPool`, no-op unless attached) + `AssertQueryCount` helper; `TestScaleMeasure` (`internal/authorization/scale_measure_integration_test.go`, gated on `OIKUMENEA_SCALE_DSN`) measured the baseline recorded in review § Measurements: root-subtree subject = 2.67 s reach expansion, **7.4 MB GUC payload**, 4 grants-joins + 112 statements per guarded request, 1.46 s visible-persons page. No schema change (`Migrated ➖`), headless (`UI ➖`) |
| **M47** | ✅ | ✅ | ✅ | ✅ | ➖ | ✅ | verified — the authorization wall ([review-2026-07](architecture/review-2026-07.md) Phase 1: R‑01+R‑02+R‑03+R‑12 as one program; 3 new decisions **D-AuthzRequestContext**, **D-AuthzGrantCache**, **D-RLSLiveReach** amends D-RLSDefenseInDepth). **R‑01**: authority fetched once/request (`ContextWithAuthority` snapshot; `middleware.RLSResolver`→`AuthorityResolver`) + epoch-validated 2 s per-process grant cache (`authz_epoch` single-row counter added to migration `0007` in place, bumped in every authority-mutating tx incl. person-merge repoint; local reset post-commit ⇒ same-process revokes immediate, cross-replica ≤2 s — both integration-tested; tracer-asserted **1 grants-join cold / 0 warm** per guarded request vs 4 before). **R‑02**: reach never leaves Postgres — membership semi-joins `VisiblePersonIDsForSubject` (adaptive sparse/dense plan shapes behind `CountReadableUnitsCapped`, threshold 1000) + `SubjectCanReadPerson`, shadow gate via `ReadableUnitsForSubjectAmong`; RLS policies (migrations `0011`/`0023`/`0024`/`0026` in place) call live `oikumenea.authz_unit_in_reach(unit, wr)`; GUCs slim to `app.person_id`+`app.is_instance_admin` (**7.4 MB → 40 B**), 1 round trip; backstop now **exact under revocation** (asserted on a pinned conn); `EffectiveReach`/`RLSStateFor` deleted, `domain.ReachSet` kept as the property-tested oracle of the randomized Go⇄SQL⇄policy **reach differential test**. **R‑03**: lazy pinning (`db.WithLazyConn`/`RequestQuerier` across the 8 RLS-consuming modules) — non-RLS requests hold no pool conn (in-flight > pool size proven); acquire+GUC = 1 round trip (was 4+4). **R‑12**: `WithHTTPTimeout(10s)` on the watchlist client + stall test (fails at deadline, 0 pooled conns held across the egress). Measured on the M46 world: guarded request **2.65 s/112 stmts → ~0 ms**, visible page **1.46 s → 39 ms** warm, RLS unit page 3.2 ms (review § Measurements, before+after). Verified: full unit+integration suite green on fresh DB (49 pkgs), live dev-server boot (readiness 200, 27k-languoid autoseed under new policies, authz_epoch=4 from base-role seeding, authenticated `GET /person/v1/persons` 200 in 6 ms), isolated `docker compose -p m47e2e` fresh-stack e2e (oikumenea+hermenea readiness 200, 15 live-reach policies, non-superuser role). UI ➖ |
| **M48** | ✅ | ✅ | ✅ | ➖ | ➖ | ✅ | verified — incremental closure maintenance ([review-2026-07](architecture/review-2026-07.md) Phase 2 / R‑04; amends the **D-Graphs** maintenance note, strengthens D-ClosureIntegrity verify). `AddEdge`/`RemoveEdge` no longer DELETE+rebuild the whole graph closure: attach = reflexive seed + `anc*(parent) × desc*(child)` closure∘closure join with `LEAST`-depth merge; detach = slice delete → bounded re-derivation (edge-walk inside the ancestor set + one trusted closure jump, provably shortest-depth) → reflexive prune, **gated on `DeleteEdge` rows>0** (phantom detach must not shrink); per-graph `tenant_graphs FOR NO KEY UPDATE` lock serializes closure maintenance (attach/detach/rebuild) and closes the pre-existing guard-then-insert cycle race; `RecomputeClosure` survives only as the D-ClosureIntegrity repair path (`POST /closure/rebuild`); `VerifyClosureForGraph` diff is now **depth-inclusive**. New queries in `tenant.sql` (sqlc regen confined to `tenantsql`), repo `ExtendClosureForEdge`/`ShrinkClosureForEdge`/`LockGraphForClosure`. **No schema change** (`Migrated ➖`, migration untouched), headless (`UI ➖`). Verified: `TestClosureIncrementalPropertyDifferential` (random attach/detach incl. rejected cycles/dups/phantoms, stored rows+depths ≡ independent Go BFS oracle after EVERY op + depth-inclusive verify zero-drift; 40-seed soak green) + 4 targeted regressions (alternative path survives at greater depth, `LEAST` shortcut + exact reversal, last-detach clears reflexive rows, phantom detach untouched); tenant/membership/religion/authz/platform integration suites + full unit sweep green; scale numbers on the M46 100k-unit world (`TestScaleClosureMeasure`): single edge edit **16.6 s (full rebuild before) → 3–265 ms**, rows touched = slice size (5 for a mid-tree leaf, 1,914 for moving a 658-unit subtree), recorded in review § Measurements |
| **M49** | ✅ | ✅ | ✅ | ✅ | ➖ | ✅ | verified — import pipeline industrialization ([review-2026-07](architecture/review-2026-07.md) Phase 3: R‑05 + the Phase‑3 parts of R‑13; **amends D-Hermenea** in [roadmap-decisions.md](architecture/roadmap-decisions.md)). **Chunked runs**: `CanonicalEnvelope` + `(runId, seq, isLast)` (optional — absent = pre‑M49 single-shot, pinax/CLI untouched), loader slices ~5k-record chunks (install-tunable `oikumenea.chunk-size` / `http-timeout-ms`, default 120 s — `WithHTTPTimeout(0)` retired), trailing empty finalize chunk runs the batch finalizers (languoid closure etc. gate on `chunk.IsLast && (touched || chunked)`); oikumenea stays stateless per chunk. **Set-based apply**: the 4 high-volume handlers merge each chunk with one `unnest(...) ON CONFLICT … WHERE source_version IS DISTINCT FROM EXCLUDED` statement + `RETURNING (xmax=0)` exact counts; self-referential parents (geo place / languoid) resolve in a second pass over the touched rows so a parent may arrive in the same chunk; in-chunk duplicates dedupe last-wins; the 7 low-volume handlers keep loops. **Resume**: `worker_jobs.resume_seq`+`resume_checksum` (additive hermenea migration `0005`) persisted per acked chunk; a retried attempt skips acked chunks iff the re-staged checksum matches. **R‑13 parts**: `worker.concurrency` N claim loops (SKIP LOCKED already multi-claimer-safe; WorkerID now hostname-based + per-goroutine suffix); pinax autoseed + first-admin bootstrap under session `pg_advisory_lock` (`db.LockBootSeed`, first advisory lock in the repo — loser waits then no-ops); `docker-compose.scale.yml` un-publishes host ports for `--scale`. Verified: geo merge differential test vs in-Go oracle (random chunked runs incl. child-before-parent in-chunk, dups, edition bumps — table state + Summary ≡ oracle); loader chunk/seq/finalize/resume unit tests; cursor round-trip + 4-way worker-concurrency barrier integration tests; advisory-lock mutual-exclusion test; 27k Glottolog full-load e2e green on the merge path (27127 created + 50 updated, closure 212,955); **1M-record synthetic WOF compose run** (`scripts/gen-wof-synthetic` + `wof-geo-synth` source): 204 chunks, wall **4 m 31 s including kill −9 of hermenea (seq 56) and of oikumenea (seq 126)** — both resumed from the cursor, audit shows no chunk applied twice; merge tx **320 ms mean / 616 ms max** (vs one whole-dataset transaction before); RSS at full throughput oikumenea **41 MiB** / hermenea **87 MiB**; idempotent re-run **1,000,000 skipped in 72 s**; `--scale app=2` fresh-DB boot: one pinax pass (8 presets applied once, 27,177 languoids) + replica 2 bootstrap “skipped: admin exists”, zero constraint noise. Numbers in review § Measurements |
| **M50** | ✅ | ✅ | ✅ | ✅ | ➖ | ✅ | verified — search & audit physicals ([review-2026-07](architecture/review-2026-07.md) Phase 4: R‑06 + R‑07; 2 new decisions **D-PersonSearch**, **D-AuditRetention** amends the physical layout of D-Audit). **R‑06 (D-PersonSearch)**: `pg_trgm` + `STORED` generated `search_text` + GIN trigram index on `person_persons` **and** `person_name_variants` (migration `0005` in place); directory search matches names **and** transliterations/aliases, filtered **in SQL** — admin `SearchPersons` and scoped `VisiblePersonIDsForSubjectSearch` are UNION-of-id-set semi-joins (each branch stays a GIN bitmap scan; an `A OR EXISTS(subquery)` predicate is not index-able), keyset on person RID; the Go `matchesPersonQuery` post-filter is deleted (fixes the scoped empty-page-while-`hasMore` bug). **R‑07 (D-AuditRetention)**: `audit_log` monthly RANGE-partitioned on `created_at` (migration `0001` in place), PK `(id, created_at)`, index diet **6→4** (dropped `actor_type`/`request_id` singles), boot-time `ensure_audit_partition` roll-forward under `db.LockBootSeed` + `DEFAULT` backstop, operator `detach_audit_partitions_before` helper + `audit.retention-months` config (default 0 = retain forever); `reject_mutation` + RLS cascade to partitions unchanged (D-Audit semantics intact). No new migration file (edited `0001`/`0005` in place → `ExpectedSchemaRevision` unchanged; re-hash `atlas.sum` with stable atlas before commit). No UI change (web already sends `?query=`). Verified: `internal/person/search_integration_test.go` (admin variant/alias find; scoped drain — no empty page while `hasMore`, no out-of-reach leak) + person/membership/audit/RLS integration suites green on rebuilt `oikumenea_test`; DB-level partition routing (Jul→month partition, gap→DEFAULT), UPDATE/DELETE rejected on partitions, detach rehearsal; plans (200k rows): admin search **0.35 ms** custom / 4.3 ms forced-generic GIN bitmap vs 183 ms seq scan. Numbers in review § Measurements |

Notes on the planned tier (M16–M37): all have a landed `D-<Name>` decision (in
[roadmap-decisions.md](architecture/roadmap-decisions.md)), so all are at least
**decided**. **M29–M37** are the **person-intelligence / OSINT-enrichment cluster** (derived from
[draft_superbrain_schema.md](draft_superbrain_schema.md), foundation-first on M29); each has a
`D-<Name>` block + a module-doc home (the new [external-organizations](modules/external-organizations.md)
doc for M30; appended sections on [person](modules/person.md) for M31–M36 and
[identity-federation](modules/identity-federation.md) for M37). **M29–M36 are verified**; **M37**
(first-party login security log) remains at the **designed** gate. **M38–M39** are **deferred stubs** — no decision/module doc yet, so they sit *before* the decided
gate (their stage-board rows are all `⬜`). **M16 is verified** as the **hermenea** companion service (D-Hermenea
supersedes D-Worker and **absorbs M17/D-DataIngestion**); its module doc
([hermenea.md](modules/hermenea.md)) exists, so its *Designed* gate is `✅`, and its UI gate is `➖`
(a headless companion service — no console surface). *Designed* `✅` means a dedicated module doc
exists — present for **M16** ([hermenea.md](modules/hermenea.md)), **M18**
([language.md](modules/language.md)), **M19**
([location.md](modules/location.md)), **M20** ([education.md](modules/education.md)),
**M21** ([company.md](modules/company.md)), **M22–M25** ([religion.md](modules/religion.md)),
**M26** ([vehicle.md](modules/vehicle.md)) and **M27** ([clients.md](modules/clients.md)).
**M27 is verified** as the unified Go + TypeScript SDKs (D-ClientSDK), with its UI gate `✅` (the web
console consumes the TS SDK) and its *Migrated* gate `➖` (tooling-only — no schema/migration). M15's backend is additive over the M4 rank migration
(`20260601000004_rank.sql`), not a separate file. M12 is now **verified**: its exit criteria are met
across the board — grounded in migrations `0012_person_contacts` + `0016_person_date_of_death`, the
`person`/`document` integration suites (`TestContactChannels`, `TestContactTypeCatalogs`,
`TestPurgeGate`, `TestDocumentAttrSchema`, `TestPersonalCodeValidationAndUniqueness`), the
`PersonForms.tsx` console managers, and a live HTTP demo against the running server (email
provider-derivation, phone E.164 + country-derivation, call-sign uniqueness `409`, `date_of_death`
round-trip).

**M44** (D-Finance) is a standalone **decided + designed** milestone off the final `todo.md` idea
(banks → bank accounts → cards): authoritative first-party financial directory data — **not** an OSINT overlay
(so it carries no M29 `source`/`confidence` attribution) and **not** payroll (M39). A bank reuses the
M21/M41 company org registry; IBAN/PAN reuse the M9 `document_personal_codes` envelope-encryption seam.
CVV2/CVC2 is excluded outright (PCI-DSS Req 3.2) and no balance/transaction ledger is in scope.

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

**Status: verified.** The core model — import, closure, `family_code`, country ties, cross-module
links — was verified and sound; the two i18n binding-convention gaps an earlier review (2026-06-15)
raised are now **closed and re-proven end-to-end** (see **Verdict (resolved)** at the end of this
section). The web console UI (language browser + person/unit/locale language editors) is built.

**Verified core (unchanged).** Verified end-to-end by loading the **real bundled presets**
through the actual hermenea mappers + oikumenea import handlers + SQL into a live PostGIS DB: **27,177
languoids** (4,853 families / 8,618 languages / 13,706 dialects), 212,955 closure rows, `family_code`
derived for every node (English → Indo-European, 3,236 descendants via the closure), 11,909 country
ties, 864 CLDR script links (134 gracefully skipped — unseeded scripts / unmatched languages), **zero
orphaned parents** (parent-first load + FK resolution), an **idempotent re-run** (all 27,177 skipped),
the `person_languages` `level='language'` composite-FK enforced (language insert OK, family rejected),
and the read queries (`ListLanguoids` filters / `GetLanguoid` / `ListWritingSystems`). The HTTP/worker
path is the **generic `POST /import/{objectType}` + `file` connector already proven by M16**; the two
new object-types are thin registrations on it. Binding via **D-Languages** in
[roadmap-decisions.md](architecture/roadmap-decisions.md). The **first NEW consumer of the M16 hermenea
pipeline** — proves the framework end-to-end via the bundled Glottolog snapshot (`file` connector;
swap to `http` for a newer CLDF release). Additive over person/localization; the unit tie adds a tenant
dependency. See the new [language](modules/language.md) module.

**Goal.** Model the world's languages faithfully (the full **Glottolog 5.3** genealogy), their writing
systems (ISO 15924), and a person's language proficiency — at analytics grade, so language becomes a
queryable, linkable dimension.

- **Delivers** (migration `migrations/20260601000018_language.sql`, RID service **13**):
  - **`language_languoids`** — the recursive **Glottolog forest**, one table: RID `id` PK with
    `code` (glottocode) a **UNIQUE** lookup key (F-014 / D-ResourceIdentifiers — like `geo_countries`,
    not a bare-`code` PK); `level ∈ {family, language, dialect}`; translatable `name`; self-FK
    `parent_id` (father — strict tree, RESTRICT); denormalized `family_code` (root family, derived in
    SQL via the closure on import); nullable **UNIQUE** `iso639_3`; `macroarea`; representative
    `latitude`/`longitude` (plain numeric — M18 precedes the PostGIS Location); AES
    `status ∈ {not_endangered…extinct}`; `glottolog_version` + `(source, source_version, imported_at)`
    provenance.
  - **`language_languoid_closure`** — maintained transitive closure (mirrors the tenant closure),
    rebuilt in SQL at the end of each `language-scheme` import, so "all languages under Indo-European"
    is one lookup.
  - **`language_languoid_countries`** — plain M:N → `geo_countries` (from CLDF `Country_IDs`).
  - **`writing_system_script_types`** catalog (migration-seeded `logographic`/`syllabary`/`alphabet`/
    `abjad`/`abugida`/`featural`); **`writing_systems`** (RID PK, ISO-15924 `code` UNIQUE, translatable
    `name`, `script_type`; **migration-seeded** with the living-language scripts);
    **`language_writing_systems`** reified M:N (`is_primary`, RID link) — **import-loaded from CLDR**
    (the `language-scripts` object-type), since neither Glottolog nor ISO-15924 carries the mapping.
  - **Language links:** `person_languages` (child of `person_persons`: `language_id` constrained to
    `level='language'` via a composite FK, `cefr_level ∈ {A1…C2}` nullable, `is_native`; `pii:basic`,
    purge-erased); `tenant_unit_languages` (unit official/working language); `i18n_locale_languages`
    (locale → canonical language, one per locale).
  - **Read API:** read-only `LanguageService` (`GET /language/v1/languages`,
    `GET /language/v1/languages/{id}`, `GET /language/v1/writing-systems`; `language.read`).
  - **Population:** the bundled preset is the **full pinned Glottolog 5.3 CLDF snapshot**
    (`deploy/language-presets/glottolog-5.3.json`, **27,177** languoids — opt-in asset, never a
    migration, CC-BY-4.0 attribution carried), plus `cldr-scripts.json` (CLDR language→script links),
    both reproducible via `deploy/language-presets/gen-presets.py`. Loaded by the hermenea `glottolog`
    (parent-first, in-memory) + `cldrscripts` mappers via `POST /import/{language-scheme,language-scripts}`.
- **Implements:** D-Languages, D-DataIngestion (first new consumer), **D-i18n** (the languoid/script
  `name` is a `locale→text` map assembled via `LocalizationService.NamesByID`, entity types
  `languoid`/`writing_system`). See the [language](modules/language.md) module + ties on
  [person](modules/person.md) / [tenant](modules/tenant.md) / [localization](modules/localization.md).
- **UI (delivered):** ontology-registry entries (`languoid` / `writing_system` → explorer + ⌘K +
  object view) + a server-side-search language picker + the person *Languages spoken* editor (SPEAKS,
  CEFR + native), the unit official/working-language editor, and the read-only locale→language display
  on the localization page (reconciled by import).
- **Endpoints (links):** read-only `LanguageService` + the new sub-resources `GET|PUT|DELETE
  /person/v1/persons/{id}/languages` (SPEAKS), `GET|PUT|DELETE /tenant/v1/units/{id}/languages`
  (official/working), and read-only `GET /localization/v1/locale-languages`. `person_languages` is
  erased on person purge.
- **Exit:** import the Glottolog snapshot; query all languages under a family via the closure; a person
  speaks two languages with native + CEFR; a unit declares a working language; purge erases
  `person_languages`.

**Verdict (i18n consistency review, 2026-06-15 — resolved).** The review asked whether the module is
internally consistent and free of duplicated tables, and found two binding-convention gaps. Both are
now closed and re-verified:

- **No table duplication, no model redundancy.** `language_languoid_countries` reuses the shared
  `geo_countries` registry (no parallel country table); the locale (`i18n_locales`, ISO-639-3 UI
  language) and the languoid (`language_languoids`, Glottolog genealogical node) are **deliberately
  distinct concepts**, bridged by `i18n_locale_languages` — *not* duplicates.
- **Gap 1 — names now use the i18n store (resolved).** The `LanguageService` (and the new person/unit
  language responses) return `name` as a `locale→text` map assembled via `LocalizationService.NamesByID`
  (default-locale `name` column + `i18n_translations` overlay, entity types `languoid`/`writing_system`)
  — the shape every other entity uses (cf. rank/tenant). Re-verified: an `i18n_translations` override
  for `entity_type='languoid'` surfaces in the map (`internal/dataimport/language_integration_test.go`,
  `TestLanguageNameLocaleMapAndOverride`).
- **Gap 2 — `i18n_locale_languages` is reconciled (resolved).** `ReconcileLocaleLanguages` runs at the
  end of each `language-scheme` import, matching `i18n_locales.code` to `language_languoids.iso639_3`
  (self-healing, idempotent; `ukr`→Ukrainian, `eng`→English). Re-verified in the same suite +
  `TestLanguageSchemeImportClosureAndReconcile`, and the read surface is exposed at
  `GET /localization/v1/locale-languages`.
- **Re-verification.** Fast integration suite green; the full **27,177-languoid Glottolog 5.3** preset
  re-loads end-to-end through the real hermenea mappers + import handlers (212,955 closure rows,
  `family_code` for every node, 864 CLDR script links / 134 gracefully skipped, locale links present,
  idempotent re-run) via `OIKUMENEA_LANG_E2E=1 go test -tags integration` (`language_preset_e2e_test.go`).
  The `verified` gate is restored.

## M19 — Location

**Status: verified.** Binding via **D-Location** in [roadmap-decisions.md](architecture/roadmap-decisions.md)
(see its **2026-06-17 amendment** — app-derived MGRS, no H3, multi-format coordinate input). A new shared
**standalone** entity that M20 (education buildings/dorms) and M21 (company addresses) reference by FK.
Re-adopts geography/PostGIS — explicitly noted as *dropped from `drafts/`* in decisions.md, now reversed
here with rationale. Built on the existing **`location` RID service (12)** beside the
geo_countries/geo_places registry: migration `migrations/20260601000019_location.sql`
(`location_locations` + `location_location_types`, the `(12,1,3)`/`(12,1,4)`/`(12,3,0)` RID rows), the
**app-side coordinate converters + MGRS** (`internal/geo/domain/coordinate.go`, via `github.com/wroge/wgs84`),
the audited **LocationService** CRUD + radius/bbox in `internal/geo` (`api/location.conjure.yml`), the
readiness-gate PostGIS check, and the `/locations` web console page (browser + create-from-coordinate in
any format + radius search; `location`/`location_type` ontology-registry entries).

**Verification (delivered).** The stock **`postgis/postgis:16-3.4`** image suffices (no custom image, no
h3-pg, no cgo); MGRS is derived in pure Go and radius/bbox use PostGIS `ST_DWithin` on the GiST index.
Unit tests (`internal/geo/domain/coordinate_test.go`) prove the MGRS derivation matches the known Kyiv
fixture `36UUA2418291607` and round-trips MGRS/UTM/СК-42 ↔ WGS84. Integration tests
(`internal/geo/location_integration_test.go`, against a real PostGIS DB) prove: create derives the MGRS on
write and preserves the original input in `source_coordinate`; create from MGRS/UTM resolves to the same
canonical point; an update recomputes the MGRS; an out-of-range coordinate is rejected
(`CoordinateOutOfRange`) and an unparseable one rejected (`CoordinateInvalid`); `ListLocationsNear`
includes/excludes by `ST_DWithin`; soft-delete removes the row from reads; each write emits exactly one
`system`-actor audited Action. The migration (edited in place; the location tables had no production data)
applies cleanly + idempotently.

**Goal.** A reusable, analytics-grade place entity: a precise coordinate (enterable in several formats)
with a derived MGRS index plus a structured postal address over the existing country registry, so
anything with a location (buildings, campuses, dormitories, company addresses) points at one shared,
queryable record.

- **Delivers:**
  - **`location_locations`** — RID PK; **`geom GEOGRAPHY(POINT, 4326)` required** (PostGIS); **app-derived
    `mgrs`** (pure Go, from the resolved WGS84 coordinate); **`source_coordinate` JSONB** (the original
    input verbatim); structured address: `country_id` (NOT NULL → `geo_countries`),
    `admin_area_1`/`admin_area_2`, `locality`, `street`, `house_number`, `postal_code`, `raw_address`;
    soft-delete; spatial GIST index.
  - A pluggable **coordinate-input registry** (WGS84 lat/lon, MGRS, UTM, СК-42 numeric + grid) that
    converts to canonical WGS84 and derives the MGRS, a `LocationService` (CRUD + radius/`ST_DWithin`
    query), and the readiness-gate PostGIS check (the only operator-DB spatial prerequisite).
- **Implements:** D-Location (reverses the `drafts/` geography drop; MGRS app-side, no H3). See a new
  `location` module + [platform](modules/platform.md) (extension bootstrap).
- **Exit:** create a location from a coordinate in any supported format; the MGRS is derived on write and
  the original input preserved; a payload with no coordinate is rejected (coordinate required); a radius
  query returns nearby locations.

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

**Status: verified.** Built as the [company](modules/company.md) module (RID service 15), binding via
**D-Companies** in [roadmap-decisions.md](architecture/roadmap-decisions.md). A
legal-entity registry over person + the M19 Location foundation — **independent of education** (per
decision). Scoped to **structural** registry data (identity + affiliation + ownership graph) for
analysis and linking; volatile intelligence (financials/court/tax/sanctions) is parked (DS-45/46/47).

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

- **Delivers** (taxonomy reshaped per the **D-Religion refinement**, 2026-06-19):
  - **Recursive taxonomy:** a single `religion_taxa` tree (`parent_id` self-FK + `religion_taxa_closure`,
    the `language_languoids`/`education_units` pattern) — **no** fixed 3-table shape. Each node carries a
    catalog-driven level marker (`religion_taxon_ranks`: religion→branch→tradition→sub_tradition→
    denomination), an optional `wikidata_id`, and a denormalized root `religion_id`. A **rich curated
    seed** (deep Christianity incl. the major historic churches + broad world religions, 98 taxa) ships
    in the migration via `deploy/religion-presets`.
  - **Religion-type ("theism") classification:** a `religion_classifications` catalog tagged M:N onto
    taxa, resolving **nearest-declared-wins** down the closure (a taxon **or** unit may override).
  - **Organization nodes reuse `tenant_units`** with a **catalog-driven** `unit_kind` via
    `religion_org_kinds`; three **seeded religion graphs** — `canonical` (governance tree,
    **authority-bearing**), `tradition` (taxonomic, **directory-only**), `affiliation` (voluntary DAG,
    **directory-only**) (D-Graphs/D-DirectoryGraphs).
  - `religion_org_profiles` (1:1 Unit extension); `religion_org_classifications` (**M:N tradition tags,
    one primary**); `religion_org_policies` + `religion_policy_kinds` (generic, data-driven
    eligibility/exclusion — replaces any faith-specific doctrinal flag).
  - A `ReligionService` for the taxonomy + catalogs + org profile/classification/policy management
    (taxonomy/catalogs instance-scope; per-unit org ops on `religionorg.manage` over the canonical
    graph; `POST /units/{id}/child-orgs` enforces `excludes_child_creation`).
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

**Status: verified.** Binding via **D-Religion** (discovery surface) in
[roadmap-decisions.md](architecture/roadmap-decisions.md). The discovery substrate over religious structure + the shared
**M19 Location** (PostGIS), source-of-truth in go-oikumenea; a FaithMap-style app consumes it. Built
on M22 + [M19 Location](#m19--location) (migration `0026_religion_discovery`, `internal/religion`
discovery layer, `ReligionService` discovery endpoints, `/religion` web panels, integration test).

> **Coarsening is app-side (H3 dropped).** The design below sketched `public_precision` as a coarsening
> to an **H3 cell**, but **D-Location was amended 2026-06-17 to drop H3 entirely** (stock PostGIS image;
> `ST_DWithin`/GiST already serves radius search). M25 therefore coarsens **in Go by coordinate
> rounding** (`domain.Coarsen`: `exact`→full, `street`→4 dp, `neighborhood`→3 dp, `city`→2 dp,
> `hidden`→omitted), never via a DB cell.

**Goal.** Make religious organizations **discoverable** — where they meet, when they serve, under what
names — with privacy-preserving spatial search, while the CMS/rendering stays in the consuming app.

- **Delivers:**
  - `religion_sites` — a **reified Link** `link__site_of` (worship-community unit ↔ `location_locations`
    (D-Location); `site_type_id` → generic `religion_site_types` catalog — church/cathedral/chapel/
    monastery, mosque, synagogue, temple, gurdwara, shrine, mission, office, online; `visibility ∈
    {public,unlisted,private}`; `public_precision ∈ {exact,street,neighborhood,city,hidden}`;
    `is_primary` one-per-unit). The precision projection (app-side coarsening — see the note above) lives
    on the **site link**, so one shared location may be published at different precisions by different owners.
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

## M26 — Vehicles

**Status: verified.** Binding via **D-Vehicles** in
[roadmap-decisions.md](architecture/roadmap-decisions.md). The **last `todo.md` item** — a vehicle
registry that binds people **and** companies to vehicles in one queryable graph. Additive over person +
the M21 Company registry. See the [vehicle](modules/vehicle.md) module doc.

**Doc reconciliation (the plate-region foundation).** The original M26 plan bundled a shared
`geo_subdivisions` ISO-3166-2 registry as the plate-region foundation (**D-GeoSubdivisions**). That
decision was **superseded by D-GeoPlaces** in M16: the WOF **`geo_places`** gazetteer is already built
and live (35k UA places), and D-GeoPlaces is explicit — *"D-Vehicles' plate-region FK `subdivision_id`
→ `geo_places` (placetype=region)."* So M26 **builds no geography table**; the plate region is an FK
into existing `geo_places`, and all country FKs are `country_id uuid → geo_countries(id)` (the geo
re-key amendment), not an ISO `code`. The "subnational subdivisions foundation" sub-deliverable is
dropped accordingly.

**Goal.** Hold vehicles at registry grade — a brand/model/type taxonomy, the physical vehicle (VIN),
and the ownership/plate record — so a person or company links to the vehicles they own/operate and to
the manufacturer behind a marque, with the plate **region** modelled as a `geo_places` FK rather than
free text.

- **Delivers:**
  - **Catalogs:** `vehicle_types` (taxonomy tree, self-FK + denormalized root, no closure — the
    `rank_types` pattern), `vehicle_brands` (`country_id` → `geo_countries`), `vehicle_models`
    (`brand_id`, `generation`, manufacture window), `vehicle_registration_number_types`.
  - **Object:** `vehicle_vehicles` (RID PK `17,1,1`; `type_id`/`model_id`; `manufacture_date`; `vin`
    unique among active, nullable, `pii:basic`; `color`; `attributes` JSONB; soft-delete).
  - **Reified Links:** `vehicle_brand_manufacturers` (`link__manufactured_by` `17,2,1`: brand →
    `company_companies`, temporal); `vehicle_registrations` (`link__registered_to` `17,2,2`: vehicle →
    **polymorphic owner** person **XOR** company, `country_id` → `geo_countries`, `subdivision_id` →
    **`geo_places`** (placetype=region), `registration_number` unique-active-per-country,
    `number_type_id`, temporal + `status` — registration is the ownership history).
  - A read-only **`GET /geo/v1/places`** region picker added to GeoService (over `geo_places`).
    Holder-scoped reads for person-owned registrations, audited writes (`CreateVehicle`/`RegisterVehicle`
    transfer-as-history + catalog edits), and the `ErasePersonRegistrations` purge path (the `PersonPurged`
    subscriber is a deferred shared seam); national vehicle/brand registries ride M16 (hermenea).
- **Implements:** D-Vehicles (extends D-Ontology). See the [vehicle](modules/vehicle.md) module. The
  plate region rides D-GeoPlaces (M16); **D-GeoSubdivisions stays superseded — not built**.
- **Excluded / parked:** vehicle lifecycle/intelligence feeds — insurance/MTPL, technical inspection,
  accidents, theft/wanted, odometer, telematics (**DS-52**, connector-fed, mirrors DS-45); column-izing
  stabilized vehicle specs out of `attributes` (**DS-53**, the DS-6 pattern); the full WOF locality
  backfill + residence/Location `admin_area_*` retrofit (**DS-51**).
- **Exit (verified).** Create a vehicle with a VIN under a brand/model/type (duplicate VIN → `409`);
  register it to a person in a plate region (a non-region place → `Vehicle:RegionInvalid`; a duplicate
  active plate per country → `409`); transfer it to a company (a new registration row, the prior one
  closed); query who owns a vehicle and which company makes a brand; purging a person erases their owned
  registrations. Proven by `internal/vehicle/vehicle_integration_test.go` (`TestVehicleVertical`).

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
(Vehicles) — promoted into binding decisions (**D-Vehicles**; **D-GeoSubdivisions** was superseded by
D-GeoPlaces in M16, so no subdivision registry is built — the plate region rides the WOF `geo_places`
gazetteer). It adds new parked seams: **DS-51** (full WOF locality backfill + residence/Location
`admin_area_*` retrofit), **DS-52** (vehicle lifecycle/intelligence feeds —
insurance/inspection/accidents/telematics), and **DS-53** (column-ize stabilized vehicle specs). Its
brand/model registries ride the M16 hermenea connectors. M26 is the last planned *domain* milestone; the
remaining `todo.md` items 2 & 3
(units-without-codes, editing-unit-codes) are the **M28** unit-code-lifecycle milestone (below).

The **M22–M25** milestones (above) are the **multi-faith religion vertical** (`todo.md` item 4),
which **promotes DS-48** (Religion) into binding decisions (D-Religion, D-ClergyCredential,
D-ReligiousAffiliation) and **resolves the person-field half of DS-29** (D-SpecialPII extends the
envelope to `pii:special`). They add new parked seams: **DS-49** (rite-of-passage / life-cycle records,
`pii:special`) and **DS-50** (location-scoped role assignments — a consuming app's per-site "campus
admin"; today an assignment's scope is `unit|subtree`). The audit-payload half of DS-29 stays parked.

---

## M28 — Unit code lifecycle

**Goal.** Make a unit's `code` a proper **optional, mutable, human-readable** business ID — so the
org graph can hold **codeless sub-units** and operators can **correct** a code — while the **RID**
stays the stable handle external systems reference. Consumes `todo.md` items 2 (units without codes)
and 3 (editing codes of units). Decision: **D-UnitCodeLifecycle**
([roadmap-decisions.md](architecture/roadmap-decisions.md)), which **amends D-Code** for the unit
scope only.

**Why now.** Real org graphs contain *non-separate sub-units* (a line battalion / platoon — Ukr.
*підрозділ* vs *окрема частина*) that have no independent external designation, yet
`tenant_units.code` was `NOT NULL`; and a code (a human-readable ID) sometimes needs correcting
(typo, reorganization). The enabling fact: **internally every reference to a unit is by its RID**
(positions, edges, closure, authorization assignments, audit all FK `tenant_units.id`) — so omitting
or changing a code breaks nothing internal, and external callers are expected to key on the RID. No
code aliases are needed; an audit trail suffices.

- **Delivers:**
  - **Schema** (edit the **existing** `migrations/20260601000003_tenant.sql` in place — dev DB
    reset + `atlas migrate hash`; no new migration file): `tenant_units.code` → nullable; unique
    index predicate → `WHERE deleted_at IS NULL AND code IS NOT NULL`; new append-only
    `tenant_unit_code_events` (RID slot `4,1,4`, `reject_mutation()` guard; `old_code`/`new_code`
    nullable to record NULL↔value transitions).
  - **API** (`api/tenant.conjure.yml`): `CreateUnitRequest.code` → `optional<string>`;
    `setUnitCode` → `PUT /units/{id}/code` (`{ code?, reason? }`); `Tenant:UnitCodeConflict` (409).
    `UpdateUnitRequest` is unchanged (code stays out of the generic patch).
  - **Backend** (`internal/tenant/`): `Unit.Code` → pointer; `Validate()` relaxed (code optional,
    valid shape if present); `SetUnitCode` application method (uniqueness check among active coded
    units + `tenant_unit_code_events` append in one txn) emitting a `UnitCodeChanged` domain event
    consumed by [audit](modules/audit.md); new `unit.recode` permission, the `RecodeUnit` Action.
  - **UI** (`web/`): create a unit with an empty code (renders as a non-separate sub-unit); an "Edit
    code" action on the unit detail page (optional reason, surfaces the 409); optional code-change
    history. Consumes the TS SDK (M27 / D-ClientSDK).
- **Depends on:** M3 (tenant). Additive to the built tenant surface.
- **Out of scope:** graph/role/rank/locale codes (keep `code TEXT NOT NULL UNIQUE`, immutable by
  convention); code aliases / old-code redirect (rejected — external refs use the RID); a redundant
  `is_separate` flag (separateness is derived from code presence).
- **Verified when:** integration tests prove a codeless create + codeless siblings coexisting, a
  recode set/correct/clear each appending a `tenant_unit_code_events` row, a duplicate active code →
  `409`, and `code` absent from the generic `PUT /units/{id}`; a live demo shows a unit's
  positions/edges/closure untouched by a rename (they key on the RID).

---

# Person-intelligence / OSINT-enrichment cluster (M29–M39)

**Status: planned (designed).** A nine-milestone cluster (M29–M37) + two deferred stubs (M38–M39)
derived from [draft_superbrain_schema.md](draft_superbrain_schema.md) — the filtered OSINT
people-intelligence schema whose per-field verdicts (`DEVELOP`/`OVERLAY`/`LIVE-LOOKUP`/`DEFERRED`/
`EXCLUDED`) are the binding source for *what* each milestone holds. The cluster decisions live in
[roadmap-decisions.md](architecture/roadmap-decisions.md) (D-OverlayFoundation … D-LoginSecurityLog).
**Foundation-first:** M29 builds the shared substrate (provisional nodes, the `source`/`confidence`
attribution convention, the structured `legal_basis` catalog) so M30–M37 are each a thin vertical
slice. Three rules hold throughout: **declared ≠ inferred** (never merged); **every overlay carries
`source`+`confidence`**; **special-category data is gated** (5-tier `pii:*` + envelope [D-SpecialPII] +
`legal_basis` + audit). Risk climbs across the sequence — M31 is mostly first-party `DEVELOP` data, M36
is the strictest `pii:special` health tier.

**Excluded outright** (recorded only in [draft_superbrain_schema.md](draft_superbrain_schema.md), per
its verdicts — not promoted): **biometrics** (1.5) and **GPS location history** (2.4); revisit only
under a hard lawful requirement + legal review (and, for biometrics, token/reference-only).

## M29 — OSINT overlay foundation

**Goal.** The cross-cutting substrate every enrichment milestone needs, built once. Binding via
**D-OverlayFoundation**.

- **Delivers:**
  - **Provisional persons + manual resolution** — `person_persons.status` gains `provisional` (a
    minimal-PII stub so every relationship/overlay edge points at a node); a `MergePerson` audited
    action promotes/merges a provisional into a canonical person with a `confidence`, re-homing edges
    in one txn (`PersonMerged` event). **No automatic candidate matching** (parked).
  - **Attribution convention** — the reusable `source ∈ {self_declared, operator_verified, imported}`
    / `confidence ∈ {confirmed, probable, possible}` / `as_of` column-set, documented in
    [conventions.md](architecture/conventions.md), reused verbatim by M30–M37; inferred values kept in
    a separate column-space, never merged into declared.
  - **`legal_basis` catalog** — a migration-seeded `platform_legal_basis_kinds` (GDPR Art. 6 lawful
    bases + Art. 9 conditions) referenced by FK from every gated/special-category overlay (NOT NULL on
    `pii:special`), plus an optional free-text justification.
- **Implements:** D-OverlayFoundation (extends D-Ontology, D-PersonSocialChannels, D-PIITiers). See
  [person](modules/person.md) + [platform](modules/platform.md).
- **Exit:** create a provisional person and point a relationship at it; merge it into a canonical
  person (edges re-homed, audited); a gated overlay write without a `legal_basis` is rejected; an
  imported attribution row records `source=imported`+`confidence`.

## M30 — External organizations registry

**Status: verified.** Built end-to-end (migration `0029_external_orgs`, `internal/externalorg`, the
`external-organizations` hermenea import target + the live Wikidata SPARQL connector
`internal/hermenea/wikidataorgs`, the `/external-orgs` web page + TS SDK façade) and proven by the
`externalorg` integration suite + the real-connector hermenea pipeline test. The live Wikidata fetch is
opt-in (`OIKUMENEA_WIKIDATA_E2E=1`); the query returns real data but the WDQS endpoint rate-limits cloud
IPs, so the default suite uses a deterministic captured-payload e2e instead.

**Goal.** A node-space for external organizations a person is tied to (party, government body, foreign
military, NGO, lobbying registrant). Binding via **D-ExternalOrgs**.

- **Delivers:** a new `external-organizations` module (**RID service 18**): `external_org_kinds`
  catalog (party | government_body | military | ngo | registrant | other) + `external_organizations`
  (RID PK, translatable `name`, `kind`, optional `country`→`geo_countries`, optional `wikidata_id`,
  provisional/resolved status + attribution); `ExternalOrganizationService` (CRUD + read); a hermenea
  import target (Wikidata / public registries).
- **Implements:** D-ExternalOrgs (extends D-Ontology). See
  [external-organizations](modules/external-organizations.md).
- **Exit:** register a political party and a government ministry as distinct kinds; a provisional org
  stub resolves to a canonical one; neither pollutes the tenant PDP graph nor the M21 company registry.

## M31 — Physical identity & description

**Goal.** First-party physical-identity attributes + aliases. Binding via **D-PhysicalIdentity**.

- **Delivers:** `person_name_variants.variant_kind`
  (`transliteration|aka|former_legal|maiden|pseudonym|cover`) — aliases fold in, **no new table**;
  `person_physical_descriptions` (height/weight/eye/hair/build/**blood_type**, effective-dated,
  `pii:basic`) + `person_distinguishing_marks` (tattoo/scar/piercing/birthmark, `pii:special` ceiling);
  declared `person_ethnicity_types` catalog + the encrypted `person_ethnicities` link (`pii:special` +
  envelope + `legal_basis`, declared-only). Biometrics **excluded**.
- **Implements:** D-PhysicalIdentity (extends D-PersonNamesCLDR, D-SpecialPII). See
  [person](modules/person.md).
- **Exit:** record an AKA + a former legal name as name variants; a physical description with blood
  type + a distinguishing mark; a declared ethnicity stored encrypted with a `legal_basis`; purge
  erases/crypto-erases them.

## M32 — Structured addresses

**Goal.** Precise, effective-dated address history over the M19 Location point. Binding via
**D-PersonAddresses**.

- **Delivers:** `person_addresses` — a reified link `person` → `location_locations` (M19)
  (`role ∈ {home,work,mailing,other}`, `valid_from`/`valid_to`, `is_primary`, `privacy_seeking`,
  `source`+`confidence`; `pii:contact`, purge-erased); a work address may be derived from the person's
  unit location. `person_residences` (country-grade) retained for legal-residence semantics.
- **Implements:** D-PersonAddresses (extends D-Location). See [person](modules/person.md) +
  [location](modules/location.md).
- **Exit:** attach a home + a distinct mailing address (the latter flags `privacy_seeking`); a radius
  query over `location_locations` finds the person's home; purge erases the addresses.

## M33 — Institutional & political ties

**Goal.** Per-type person↔organization affiliation edges. Binding via **D-InstitutionalTies**.

- **Delivers:** `person_party_memberships` (`pii:special`, envelope + `legal_basis`);
  `person_government_positions` (`pep_trigger` auto-true, persists post-office, `pii:basic` — **feeds
  M34**); `person_lobbying_relationships`; **foreign/historical military service** (reuse
  [membership](modules/membership.md) against M30 external_organizations + rank, `discharge_type`/
  `clearance_level` `pii:sensitive`); `person_external_references` (wikipedia/news/registry, a hermenea
  target); an `emergency` `person_relation_type` (no new entity). Every edge carries
  `source`+`confidence`.
- **Implements:** D-InstitutionalTies (extends D-Ontology, D-OverlayFoundation, D-ExternalOrgs). See
  [person](modules/person.md).
- **Exit:** record a party membership (encrypted, lawful-basis-gated) + a government position that sets
  the PEP trigger + a foreign-military service against an external-org stub + a Wikipedia reference;
  purge erases all of them.

## M34 — Watchlists & regulatory exposure

**Goal.** Live-lookup sanctions/PEP/Interpol screening + a regulatory-sanctions overlay — never store
the lists statically. Binding via **D-Watchlists**.

- **Delivers:** the **live-lookup** path **through hermenea** (hermenea owns egress to OFAC/EU/UN/
  INTERPOL APIs + a ≤24h cache; only `person_watchlist_matches` metadata — `on_list`/`lists[]`/
  `program`/`match_score`/`last_checked`/`next_check_due` — is persisted); PEP derived from M33
  government positions; the **Interpol API** connector (interpol.api.bund.dev, the original `todo.md`
  item 1); `person_regulatory_sanctions` overlay (regulator/action/amount/status, `pii:sensitive`, a
  hermenea import target); a `CheckWatchlists` audited action.
- **Implements:** D-Watchlists (extends D-Hermenea). See [person](modules/person.md) +
  [hermenea](modules/hermenea.md).
- **Excluded / deferred:** criminal/arrest/court records (6.1–6.3) → **M38** (own session).
- **Exit:** a live watchlist check stores only match metadata (no list rows) and is audited; a PEP flag
  derives from a held government position; re-checking refreshes `last_checked`; a regulatory-sanction
  overlay row imports via hermenea.

**Verified** (migration `0033_person_watchlists`, schema revision `0033_person_watchlists`). Both tables
land as person Objects: `person_watchlist_matches` (`6,1,15`, one active row per person via a partial-unique
`person_id` — `CheckWatchlists` refreshes it in place; `pii:sensitive`; hard-erased on purge) and
`person_regulatory_sanctions` (`6,1,16`, `pii:sensitive`, idempotent by `(person, external_id)`).

The load-bearing new element is the **first synchronous `oikumenea → hermenea` call**: until M34, hermenea
only pushed *into* oikumenea (`POST /import`) plus a fire-and-forget `POST /sync`. `CheckWatchlists`
(`POST /persons/{id}/watchlist-check`) runs a live screening OUT to the companion through a late-bound
`domain.WatchlistLookup` seam (`SetWatchlistLookup` in `main.go`, mirroring `SetLocationLookup` — the PDP
core makes no egress call itself; a `watchlistclient` adapter wraps the generated `HermeneaServiceClient`
with the `OIKUMENEA_HERMENEA` trust direction). It combines the returned match metadata with the
**locally-derived** PEP flag (M33 `IsPoliticallyExposed` over `person_government_positions`) and upserts the
single per-person match. hermenea owns the egress + a ≤24h cache (`hermenea.watchlist_cache`, migration
`0004`) behind a new `checkWatchlist` `HermeneaService` endpoint; the **real INTERPOL Red Notices
connector** ships (`internal/hermenea/watchlist`, env-gated live + deterministic fixture) with OFAC/EU/UN
sanctions as a documented pluggable `SanctionsStub` (no match unless configured).
`person_regulatory_sanctions` is additionally a hermenea **import target**
(`POST /import/person-regulatory-sanctions`, an idempotent person-scoped handler + a `regulatorysanctions`
passthrough mapper over an operator-registered source). Merge re-homes the durable sanctions and drops the
stub's transient match (re-checking regenerates it); purge hard-deletes both. Open seams: real sanctions
providers (OFAC/EU/UN) behind the stub, and an operator-registered bulk reg-sanctions source (no committed
preset — person-scoped rows don't fit a code-keyed catalog preset).

## M35 — Financial, behavioral & psychological overlays

**Goal.** Three gated, attribution-tagged overlays. Binding via **D-PersonOverlays**.

- **Delivers:** `person_crypto_wallets` (address/chain/attribution_method/balance_approx,
  `pii:sensitive` — synergy with M34); `person_personality` (MBTI/Big-Five/DISC/Enneagram, **declared
  survey or formal HR assessment only**, `pii:sensitive`, no text-inference); **inferred**
  `person_political_leaning` (spectrum −1..1, `inference_sources`+`confidence`, `pii:special` +
  envelope + `legal_basis`, **never merged** with the declared M33 party membership).
- **Implements:** D-PersonOverlays (extends D-OverlayFoundation, D-SpecialPII). See
  [person](modules/person.md).
- **Excluded:** compensation/payroll → **M39** (separate operational-HR module).
- **Exit:** attribute a crypto wallet with method+confidence; record a declared personality type; store
  an inferred political-leaning that stays separate from declared affiliation; purge/crypto-erases all.

## M36 — Health & vulnerability (`pii:special`)

**Goal.** Category-level health/vulnerability under the strictest gate. Binding via
**D-HealthVulnerability**.

- **Delivers:** `person_health_records` (`kind ∈ {hospitalization, mental_health, disability}`,
  **category-level only, no diagnosis**, `is_public_record`, `source`+`confidence`; `pii:special` +
  envelope + app-layer need-to-know + full audit + `legal_basis`, **never inferred**); `person_insurance`
  (`type ∈ {health,life,disability,ltc}`, provider, `employer_sponsored`, `pii:sensitive`).
- **Implements:** D-HealthVulnerability (extends D-SpecialPII). See [person](modules/person.md).
- **Exit:** record a category-level hospitalization (encrypted, need-to-know-gated, fully audited,
  lawful-basis attached); an insurance record; a non-authorized read is denied; purge crypto-erases.

## M37 — Login security log

**Goal.** First-party login/IP security telemetry on the federation seam. Binding via
**D-LoginSecurityLog**.

- **Delivers:** `account_login_events` (`account_id`, `ip`, `occurred_at`,
  `context ∈ {login,activity,registration}`, `resolved_country`/`resolved_isp`, `is_vpn`/`is_tor`,
  `user_agent`; `pii:contact`, retention-bounded, purge-erased), emitted by the existing OIDC/JWKS
  validation middleware on the `/whoami`/token-validation path — **not** OSINT enrichment, **not**
  stored credentials (L-AuthzOnly holds).
- **Implements:** D-LoginSecurityLog (extends L-AuthzOnly). See
  [identity-federation](modules/identity-federation.md).
- **Exit:** a validated inbound token records a login event with resolved country + VPN/Tor flags; the
  retention sweep prunes old events; person purge erases the subject's events.

## M38 — Criminal / arrest / court records *(deferred — own design session)*

**Status: deferred.** In scope and important, but **not yet decided/designed** — it needs a dedicated
session (draft macro-category 6.1–6.3). Hard requirements to carry in: mandatory `disposition`
(arrest ≠ guilt); expungement/sealing **suppression**; jurisdiction-specific storage/display rules
(Ban-the-Box, FCRA); `pii:sensitive` + gated + audited. **Distinct** from internal
`discipline-incentive` orders ([order](modules/order.md)). No `D-<Name>` decision exists yet.

## M39 — Compensation / payroll *(deferred — separate operational-HR module)*

**Status: deferred.** A separate operational-HR concern (the org as **payer**), intentionally out of
the OSINT-dossier scope. Designed when that module is opened. No `D-<Name>` decision exists yet.

## M44 — Finance (bank accounts & payment cards)

**Goal.** A new `finance` module holding **bank accounts (IBAN) and payment cards** as authoritative,
encrypted-at-rest directory data, with a person (or company) as the account holder and a bank modeled
as a company org. Binding via **D-Finance**. Retires the final `todo.md` idea (banks/accounts/cards).

- **Delivers:**
  - **Objects (RID service 19):** `finance_accounts` (`19,1,1`) — `institution_id` → a `company`-domain
    `tenant_organizations` row (the bank, M21/M41), **envelope-encrypted IBAN** (`iban_ciphertext`/
    `iban_wrapped_dek`/`key_ref`/`iban_blind_index`, `pii:sensitive`, blind index unique among active),
    `currency` (ISO 4217), `account_type_id`, `status`; `finance_cards` (`19,1,2`) — `account_id` →
    `finance_accounts` (structural containment FK, CASCADE), **envelope-encrypted PAN** + display
    `bin`/`last_four`, `network_id`, `card_type ∈ {debit,credit}`, optional `expiry_month`/`expiry_year`,
    optional `cardholder_person_id`.
  - **Catalogs:** `finance_account_types` (`19,1,3`; current/savings/deposit/loan…) + `finance_card_networks`
    (`19,1,4`; visa/mastercard/amex…), instance-admin-extensible (D-Code/D-i18n).
  - **Link:** `finance_account_holders` (`link__held_by`, `19,2,1`) — the ownership edge; **polymorphic**
    `holder_kind ∈ {person,company}` + `holder_id` (text, no FK), `role ∈ {primary,joint,authorized_signer}`,
    effective-dated. This is the "person → bank account" relation; joint/corporate accounts fall out of it.
  - **Reuse:** `pkg/crypto` `Cipher.Seal/Open/BlindIndex` (D-CryptoProvider) + `pkg/personalcode`
    validator precedence (compiled IBAN ISO-13616 mod-97 + PAN Luhn/BIN-network). A `finance`
    `PersonPurged` subscriber crypto-erases person-solely-held accounts + their cards (mirrors
    [document](modules/document.md)).
- **Implements:** D-Finance (extends D-Ontology, D-PersonalCodes, D-UnifiedOrgGraph). See
  [finance](modules/finance.md).
- **Excluded / parked:** **CVV2/CVC2/CID** — never stored (**PCI-DSS Req 3.2**, prohibited even
  encrypted). Storing the full PAN brings **PCI-DSS cardholder-data scope**; a BIN+last-4-only,
  out-of-scope mode is parked as **DS-54**. Account **balances / transactions** are out of scope
  (this is a directory of accounts, not a payments ledger) — parked as **DS-55**. No M29
  `source`/`confidence` attribution (authoritative first-party, not an overlay).
- **Exit (verified when):** an integration test proves a person holds an account (IBAN **ciphertext at
  rest holds no plaintext** + blind-index present/unique + decrypt round-trips), a **joint** second
  holder is added, a card under the account (PAN encrypted, BIN/last-4 clear, duplicate PAN → conflict),
  and a **person purge crypto-erases** the solely-held account + its cards while a company-held account
  survives; catalog reads return `locale → text` name maps.

## M45 — Pinax reference plane

**Goal.** Name and consolidate the **reference plane** — the instance-global, read-mostly world-model
catalogs — as **`pinax`**, and give it a single seeding contract: an `origin` marker + bundled YAML
presets self-seeded at boot, sharing one import pipeline with the hermenea connectors. Binding via
**D-Pinax**; see the [pinax plane note](architecture/pinax.md).

- **Delivers:**
  - **The `pinax` plane (a naming convention, no new RID service):** `platform_colors`,
    `geo_countries`, `language_languoids` + `writing_systems`, `rank_*` systems, `religion_taxa`, and
    `person_ethnicity_types` (+ closure / `_languages` / `_countries`) grouped as the world model —
    distinct from the operational core and from the small structural type/kind catalogs (which stay
    migration-seeded).
  - **`origin` marker, plane-wide:** `origin TEXT NOT NULL DEFAULT 'operator' CHECK (origin IN
    ('seeded','operator'))` on every seeded reference table; the seeder writes `'seeded'`, ordinary API
    inserts default `'operator'`, `--reconcile` touches `'seeded'` only.
  - **Bundled YAML presets (7):** languages (Glottolog ~27k, CLDR names, `language↔country`) · writing
    systems + CLDR-derived `language↔writing_system` wiring · countries (WOF enrichment, **no live
    calls**) · religions (`religion_taxa`) · ethnicities (Factbook, deduped, + country/language refs) ·
    ranks (UA + US per branch) · colors (`platform_colors` palettes). Each preset has a manifest
    (`source` / `source_version` / `license` / `depends_on` / translation provenance).
  - **`go:embed` + boot autoseed in oikumenea:** presets embedded in the binary, self-seeded on boot
    via the same application import service the HTTP `POST /import/{objectType}` wraps (in-process);
    config **`pinax.autoseed`** (default `true`), version-gated by a new `pinax_seed_state` table; an
    explicit `oikumenea seed` (`--reconcile`) for `autoseed:false` / manual refresh.
  - **Seed algorithm — create-if-absent, fill-if-empty, never delete:** matched on `code`
    (`INSERT … ON CONFLICT (code) DO NOTHING`); migration-seeded skeletons (locales, countries) are
    enriched fill-if-empty (coords / translations / `color_id`, never overwriting non-empty values); a
    vanished-upstream code persists (no auto-delete/deprecate).
  - **Colors:** extend `platform_colors.domain` with `rank`/`religion`/`ethnicity`/`country`, add
    `color_id` FKs to the seeded catalogs, seed palettes; `platform_colors` joins the plane (gains
    `origin`).
- **Implements:** D-Pinax (extends D-Ontology, D-i18n, D-Hermenea, D-DataIngestion; amends D-Languages,
  D-Geo, D-Rank, D-Religion, D-PhysicalIdentity/M43, D-Color). See the
  [pinax plane note](architecture/pinax.md).
- **Approach / consequence:** **in-place edits** to existing migrations (`0004_rank`, `0018_language`,
  `0023_religion`, the `geo_countries` migration, `0030` ethnicity, `platform_colors`) add `origin` +
  `color_id` + `pinax_seed_state` — **no new migration file** (re-hash `atlas.sum` + `DROP SCHEMA`
  reset). **No new RID service, no new module.** Reverses M15's "ranks folded into migration `0004`
  seed" and M43's "live Factbook fetch, no committed preset" in favor of bundled YAML.
- **Excluded / deferred:** currency (ISO 4217; no currency table yet) and religion↔country relations —
  considered, out of scope for M45. A physical `pinax` service/DB and a separate `pinax.*` schema were
  both rejected (see D-Pinax *Why not*).
- **Exit (verified when):** a **fresh oikumenea boots with `hermenea` down** and self-seeds the 7
  presets (create-if-absent; languages have CLDR names + wired writing systems; countries enriched
  fill-if-empty; ethnicity catalog populated from the bundled Factbook preset with country refs); a
  **second boot is a no-op** (version-gated, `origin='seeded'` untouched); an **operator-added** rank/
  religion/ethnicity row (`origin='operator'`) **survives** a re-run; an operator-corrected name
  survives (its higher-provenance `locale→text` entry is not clobbered); `pinax.autoseed:false` skips
  seeding and `oikumenea seed` performs it; the massive `geo_places` backfill still runs via the remote
  hermenea connector.
