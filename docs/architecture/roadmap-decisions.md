# Roadmap decisions (planned tier — M16–M54)

The landed architectural decisions for the **planned milestones M16–M54** — verticals that are
**decided and designed but not yet built** (no `internal/` or `migrations/` artifacts exist for them
yet). They carry the same ADR rigor as [`decisions.md`](decisions.md) and are **authoritative for
those verticals' design**; they become **binding-against-code** as each milestone enters
implementation (at which point a decision may be promoted into `decisions.md` or stay referenced
here).

They live in a separate file (split out of `decisions.md` per the F-008 review finding) so the
binding **[`decisions.md`](decisions.md)** reflects the **built / in-progress surface (M0–M15)** —
"what the code is actually held to" — without ~500 lines of unbuilt verticals it must keep coherent.
The [`milestones.md`](../milestones.md) stage board sequences these; the
[`ontology-mapping.md`](../ontology-mapping.md) registry classifies their (planned) Object/Link/Action
kinds.

Decisions here, in milestone order: **D-Worker** (M16, *superseded*) · **D-Hermenea** (M16, supersedes
D-Worker, folds D-DataIngestion) · **D-GeoPlaces** (M16, WOF gazetteer + first connector, supersedes
D-GeoSubdivisions, pulls PostGIS forward) · **D-DataIngestion** (M17→folded into M16) · **D-Languages**
(M18) · **D-Location** (M19) · **D-Education** (M20) · **D-Companies** (M21) · **D-Religion** /
**D-ClergyCredential** / **D-ReligiousAffiliation** / **D-SpecialPII** (M22–M25) · **D-GeoSubdivisions**
(*superseded by D-GeoPlaces*) / **D-Vehicles** (M26) · **D-ClientSDK** (M27, unified Go + TypeScript
SDKs from the Conjure contract; *in implementation*) · **D-UnitCodeLifecycle** (M28) ·
**D-TenantOrganizations** (M40, Domains + Organizations: a multi-domain tenant over the unit graph) ·
**D-UnifiedOrgGraph** (M41, verticals reuse the tenant org-graph + per-vertical sidecars; `pdp_scoped`
reference/operational split) · **D-Pinax** (M45, the `pinax` reference plane: an `origin` marker +
bundled YAML seed presets `go:embed`-ed into oikumenea and boot-autoseeded create-if-absent, one
import pipeline shared with the hermenea connectors).

The **north-star topology cluster** (M51–M54, from the [north star](north-star.md) agreed
2026-07-18): **D-HeadlessTopology** (M52, oikumenea internal-only behind unprivileged
user-token-passthrough facades) · **D-ServiceIdentities** (M51, machine clients via IdP
client-credentials → service principals; its RLS reach arm split out to **M55**) · **D-ConnectorPlane**
(M53, a connector registry + the push / pull-wiring / on-demand-lookup contract — its RLS service arm
split out to **M55**) · **D-DataPacks** (M54, plugins as versioned data packs + per-module enable
flags).

The **person-intelligence / OSINT-enrichment cluster** (M29–M37, derived from
[`draft_superbrain_schema.md`](../draft_superbrain_schema.md)): **D-OverlayFoundation** (M29, the
provisional-entity + attribution + `legal_basis` substrate every overlay rides) · **D-ExternalOrgs**
(M30, a registry of external organizations) · **D-PhysicalIdentity** (M31) · **D-PersonAddresses**
(M32) · **D-InstitutionalTies** (M33) · **D-Watchlists** (M34, live-lookup via hermenea) ·
**D-PersonOverlays** (M35, financial/behavioral/psychological) · **D-HealthVulnerability** (M36,
`pii:special`) · **D-LoginSecurityLog** (M37). The two **deferred stubs** (M38 criminal/legal records,
M39 compensation/payroll) carry **no** decision yet — they are designed in their own later session.

> Cross-references into the **core** decisions (D-CryptoProvider, D-WebUI, D-Geo, D-Rank, D-Ontology,
> D-PersonReadScope, …) point at [`decisions.md`](decisions.md); references among the planned-tier
> decisions resolve within this file.

---

## Decision index

Load this table, fetch the block you need. The built/in-progress (M0–M15) decisions have their own
index in [`decisions.md`](decisions.md).

| ID | Decision |
| --- | --- |
| [D-Worker](#d-worker--a-first-class-background-job-runtime-promotes-ds-25) | A first-class background-job runtime *(superseded by D-Hermenea)* |
| [D-Hermenea](#d-hermenea--an-out-of-process-ingestion--scheduler-companion-supersedes-d-worker-folds-d-dataingestion) | An out-of-process ingestion & scheduler companion |
| [D-DataIngestion](#d-dataingestion--a-generic-reference-data-ingestion--connector-framework-extends-d-ontology) | A generic reference-data ingestion & connector framework |
| [D-Languages](#d-languages--languages-language-groups--writing-systems-as-a-glottolog-faithful-registry-extends-d-ontology-d-i18n) | Languages, language groups & writing systems (Glottolog-faithful) |
| [D-Location](#d-location--a-shared-standalone-location-entity-postgis-app-derived-mgrs-multi-format-input) | A shared, standalone Location entity (PostGIS; app-derived MGRS) |
| [D-Education](#d-education--an-education-module-institutions-structure-and-person-bindings-extends-d-ontology) | An education module: institutions, structure, person bindings |
| [D-Companies](#d-companies--a-company-legal-entity-registry-with-an-ownershipaffiliation-graph-extends-d-ontology) | A company legal-entity registry with an ownership/affiliation graph |
| [D-Religion](#d-religion--a-multi-faith-religion-vertical-catalog-driven-taxonomy-organization-graphs--discovery-reverses-the-drafts-religion-drop-refines-l-singledomain) | A multi-faith religion vertical: taxonomy, org graphs & discovery |
| [D-ClergyCredential](#d-clergycredential--clergy-grades--credentials-as-a-per-tradition-ordered-catalog--reified-link-faith-agnostic-parallels-d-rank) | Clergy grades & credentials as a per-tradition ordered catalog + Link |
| [D-ReligiousAffiliation](#d-religiousaffiliation--lay-religious-affiliation-as-a-reified-piispecial-link-on-d-specialpii) | Lay religious affiliation as a reified pii:special Link |
| [D-SpecialPII](#d-specialpii--envelope-encryption-extended-to-the-piispecial-tier-resolves-the-person-field-half-of-ds-29) | Envelope encryption extended to the pii:special tier |
| [D-GeoPlaces](#d-geoplaces--whos-on-first-administrative-gazetteer-geoplaces-the-first-hermenea-connector-extends-d-geo-supersedes-d-geosubdivisions-pulls-postgis-forward) | Who's-On-First administrative gazetteer; first hermenea connector |
| [D-GeoSubdivisions](#d-geosubdivisions--seeded-iso-3166-2-subnational-division-registry-extends-d-geo--superseded-by-d-geoplaces) | Seeded ISO-3166-2 subnational-division registry *(superseded)* |
| [D-Vehicles](#d-vehicles--a-vehicle-registry-binding-people--companies-to-vehicles-extends-d-ontology) | A vehicle registry binding people & companies to vehicles |
| [D-ClientSDK](#d-clientsdk--unified-go--typescript-sdks-generated-from-the-conjure-contract-extends-d-conjure-d-webui) | Unified Go + TypeScript SDKs from the Conjure contract |
| [D-UnitCodeLifecycle](#d-unitcodelifecycle--unit-codes-are-optional-mutable-human-readable-ids-amends-d-code) | Unit codes are optional, mutable, human-readable IDs |
| [D-TenantOrganizations](#d-tenantorganizations--domains--organizations-a-multi-domain-tenant-over-the-unit-graph-extends-d-graphd-graphs-amends-d-code-refines-l-singledomain) | Domains + Organizations: a multi-domain tenant over the unit graph |
| [D-UnifiedOrgGraph](#d-unifiedorggraph--verticals-reuse-the-tenant-org-graph-per-vertical-sidecars-reverses-d-education-amends-d-companiesd-religion-extends-d-tenantorganizations) | Verticals reuse the tenant org-graph; per-vertical sidecars |
| [D-Color](#d-color--structural-color-as-a-per-domain-platform-catalog-referenced-by-hard-fk-extends-d-code-d-i18n-d-ontology-amends-d-vehicles-d-physicalidentity) | Structural color as a per-domain platform catalog (hard FK) |
| [D-OverlayFoundation](#d-overlayfoundation--provisional-entities-attribution--legalbasis-substrate-extends-d-ontology-d-personsocialchannels-d-piitiers) | Provisional entities, attribution & legal_basis substrate |
| [D-ExternalOrgs](#d-externalorgs--a-registry-of-external-organizations-party--government--military--ngo--registrant-extends-d-ontology) | A registry of external organizations |
| [D-PhysicalIdentity](#d-physicalidentity--aliases-physical-description--declared-ethnicity-extends-d-personnamescldr-d-specialpii) | Aliases, physical description & declared ethnicity |
| [D-PersonAddresses](#d-personaddresses--structured-effective-dated-person-addresses-over-location-extends-d-location) | Structured, effective-dated person addresses over Location |
| [D-InstitutionalTies](#d-institutionalties--personorganization-affiliation-edges-party--government--lobbying--foreign-military--references-extends-d-ontology-d-overlayfoundation-d-externalorgs) | Person↔organization affiliation edges |
| [D-Watchlists](#d-watchlists--live-lookup-sanctionspepinterpol-via-hermenea--a-regulatory-sanctions-overlay-extends-d-hermenea) | Live-lookup sanctions/PEP/Interpol + regulatory-sanctions overlay |
| [D-PersonOverlays](#d-personoverlays--financial-behavioral--psychological-overlays-extends-d-overlayfoundation-d-specialpii) | Financial, behavioral & psychological overlays |
| [D-HealthVulnerability](#d-healthvulnerability--category-level-health--vulnerability-records-piispecial-extends-d-specialpii) | Category-level health & vulnerability records (pii:special) |
| [D-LoginSecurityLog](#d-loginsecuritylog--a-first-party-loginip-security-log-on-the-federation-seam-extends-l-authzonly) | A first-party login/IP security log on the federation seam |
| [D-Finance](#d-finance--bank-accounts--payment-cards-banks-as-company-orgs-extends-d-ontology-d-personalcodes-d-unifiedorggraph) | Bank accounts & payment cards, banks as company orgs |
| [D-Pinax](#d-pinax--the-reference-plane-a-named-world-model-plane-with-an-origin-marker--bundled-yaml-seed-presets-self-seeded-at-boot-extends-d-ontology-d-i18n-d-hermenea-d-dataingestion-amends-d-languages-d-geo-d-rank-d-religion-d-physicalidentitym43-d-color) | The reference plane: origin marker + bundled YAML seed presets |
| [D-HeadlessTopology](#d-headlesstopology--oikumenea-is-internal-only-behind-unprivileged-user-token-passthrough-facades-extends-l-authzonly-amends-d-webui) | oikumenea internal-only behind unprivileged passthrough facades |
| [D-ServiceIdentities](#d-serviceidentities--machine-clients-authenticate-via-idp-client-credentials-and-resolve-to-service-principals-extends-d-hermenea-l-authzonly) | Machine clients via IdP client-credentials → service principals |
| [D-ConnectorPlane](#d-connectorplane--a-connector-registry--a-three-mode-contract-push-pull-wiring-on-demand-lookup-extends-d-hermenea-d-watchlists) | A connector registry + the push / pull-wiring / lookup contract |
| [D-DataPacks](#d-datapacks--plugins-are-versioned-data-packs--per-module-enable-flags-no-runtime-code-loading-extends-d-pinax-d-i18n) | Plugins are versioned data packs + per-module enable flags |

---

### D-Worker — A first-class background-job runtime (promotes DS-25)

> **Superseded by [D-Hermenea](#d-hermenea--an-out-of-process-ingestion--scheduler-companion-supersedes-d-worker-folds-d-dataingestion) (M16).** The
> *need* (scheduled syncs, a job queue, at-least-once with idempotency, retry/backoff, drain, a job
> ledger, health+audit) is unchanged and carried forward — but the *placement* is reversed: the
> runtime is **not** in-process inside oikumenea. It lives in a **separate companion binary,
> `hermenea`**, with its **own Postgres**, coupled to oikumenea **only over HTTP**. The single-binary
> premise below ("keeps the self-hosted, single-binary deployment story intact") is the one part
> intentionally dropped (rationale in D-Hermenea). The original text is retained for provenance.

**Decision.** The service gains an **in-process background-job runtime** — a cron **scheduler** + a
**job queue** built over the existing `pkg/events` outbox, with witchcraft-managed lifecycle. This
**promotes the long-parked DS-25** (the common scheduler blocker) onto the critical path, because the
M17 connector framework requires scheduled syncs. A `worker_jobs` ledger records status/attempts/
last_error; execution is **at-least-once with idempotency keys**; jobs retry with backoff and surface
failures via a health reporter + audit. It is a **single-process** scheduler — **no external broker**
(DS-26 stays parked) — and drains in-flight jobs on graceful shutdown.

**Why.** "Full connector/ETL" ingestion (D-DataIngestion) needs *scheduled* re-syncs, not just
on-demand imports. DS-25 was already the shared blocker for audit-retention partitioning (DS-28),
future-dated order effects (the residual of D-OrderApply), and expiry sweeps; building it now unblocks
all of them as a side effect. An in-process runtime keeps the self-hosted, single-binary deployment
story intact.

**Why not** (a) *an external broker/queue now* (Kafka, a job server): breaks the single-binary,
operator-owned-Postgres simplicity; deferred to DS-26 when a module is actually extracted. (b) *Keep
everything synchronous*: impossible for scheduled syncs and the parked sweeps. (c) *A cron sidecar
hitting HTTP endpoints*: loses in-transaction idempotency, the outbox, and unified audit.

**Consequence.** New `worker_jobs` table + the scheduler/queue in [platform](../modules/platform.md);
the in-process bus gains a durable, scheduled execution path. Lands as **M16** ([milestones](../milestones.md)),
a prerequisite for M17. DS-25 leaves [open-questions](../open-questions.md) (promoted). Additive /
expand-only.

---

### D-Hermenea — An out-of-process ingestion & scheduler companion (supersedes D-Worker, folds D-DataIngestion)

**Decision.** The background-job runtime and the reference-data ingestion pipeline are realized as a
**second deployable, `hermenea`** (`cmd/hermenea`) — a companion ETL + scheduler service beside
`oikumenea`, with its **own PostgreSQL** and its own Atlas migrations, coupled to oikumenea **only over
HTTP** (it never touches oikumenea's database). This **supersedes [D-Worker](#d-worker--a-first-class-background-job-runtime-promotes-ds-25)**
(reverses *in-process*) and **collapses [D-DataIngestion](#d-dataingestion--a-generic-reference-data-ingestion--connector-framework-extends-d-ontology) (M17) into M16** (the connector
framework moves into hermenea). The name pairs *hermenea* (interpretation — it interprets/imports the
world) with *oikumenea*.

- **What lives in hermenea** (its own DB, own ontology/RIDs): the `Connector` interface
  (`Fetch(ctx, source) → RawBatch`; **HTTP(S)** + the degenerate **`file`** connector), `import_sources`
  registry, `import_raw_batches` raw staging, a per-object-type **mapper registry** (raw records → the
  canonical envelope), an in-process **cron scheduler + `worker_jobs` queue** (`SELECT … FOR UPDATE SKIP
  LOCKED`, at-least-once with idempotency keys, **exponential backoff with per-job-type config**,
  dead-letter after max attempts, witchcraft-managed **graceful drain**, a **job-health reporter**), and
  the `import_runs` lineage ledger.
- **What lives in oikumenea**: a **generic `POST /import/{objectType}`** endpoint over an upsert
  registry — each importable object-type registers a **code-keyed, idempotent, non-destructive** upsert
  handler run in one transaction and emitted as **audited Actions** (preserving bulk-ingest ≠
  audited-edit); **per-row provenance** (`source`, `source_version`, `imported_at`) on imported rows; the
  `import.manage` permission.
- **Two trust directions, two runtime secrets** (env/ECV refreshable, never install config, never
  stored): `HERMENEA_OIKUMENEA_TOKEN` authorizes hermenea→oikumenea **import** calls; `OIKUMENEA_HERMENEA_TOKEN`
  authorizes oikumenea→hermenea **push triggers** (`POST /sync/{source}`). Beyond cron, an oikumenea
  admin/console action pushes a trigger to hermenea.
- **Service principal** — the import token maps to a `hermenea-importer` **service principal** holding
  exactly `import.manage` (instance scope), audited as a **`system`** actor (the audit actor-shape CHECK
  already permits `person | system`); see the **L-AuthzOnly** amendment in [decisions.md](decisions.md).

**Why.** The user requires hard service separation: ingestion is failure-prone, bursty, and pulls in
outbound network deps and parser surface that should **not** share a process or a database with the PDP.
An out-of-process companion keeps oikumenea's core synchronous and dependency-light, makes the importer
independently deployable/scalable/restartable, and forces a clean **HTTP-only** contract (oikumenea's
public import API is the only coupling — the extraction-ready boundary, realized). Per-row provenance +
idempotent re-sync + ingest≠edit (the transferable Foundry parts of D-DataIngestion) are all preserved.

**Why not** (a) *In-process worker (D-Worker as written)*: couples ingestion blast radius to the PDP
process and DB; rejected for separation. (b) *Hermenea writes oikumenea's DB directly*: breaks the
module/service boundary and the audit/PDP invariants; the HTTP import API is the only sanctioned write
path. (c) *Pull triggers (hermenea polls oikumenea)*: added latency + a polling endpoint for no benefit
over a direct push; push chosen. (d) *One shared secret both directions*: distinct per-direction secrets
limit blast radius. (e) *OIDC client-credentials for the service*: heavier IdP setup than the operator's
runtime shared secret; the env-secret path is simpler and audited as `system`.

**Consequence.** A new `cmd/hermenea` binary + `internal/hermenea/**` + `api/hermenea.conjure.yml` +
`migrations/hermenea/` (own `atlas.sum`); oikumenea gains the import endpoint + service-principal auth +
`import.manage` + provenance columns + a push-trigger client. **D-Worker** and **D-DataIngestion** are
superseded/folded; **M16 absorbs M17** ([milestones](../milestones.md)). DS-25 stays promoted; **DS-44**
(more connector types) stays parked. Additive / expand-only on oikumenea; hermenea is greenfield.

**Amended by M49 — chunked import runs, set-based apply, resumability, N workers**
([review-2026-07](review-2026-07.md) Phase 3: R-05 + the Phase-3 parts of R-13). The one-envelope /
one-transaction / row-at-a-time import path was a proven wall at 10⁶-record scale; the contract and
both sides of the pipeline are industrialized, preserving every D-Hermenea invariant (HTTP-only
coupling, idempotent non-destructive upserts, provenance, ingest ≠ edit):

- **Chunked envelopes.** `CanonicalEnvelope` gains optional `runId`/`seq`/`isLast`; hermenea's loader
  sends a large dataset as sequential ~5k-record chunks — each chunk one envelope = **one oikumenea
  transaction** — ended by a trailing (possibly empty) `isLast` finalize chunk that runs the
  object-type's batch finalizers (e.g. the languoid closure rebuild). All fields absent = the
  pre-M49 single-shot semantics, so small catalogs, the pinax seeder, and the `oikumenea seed` CLI
  are untouched. oikumenea keeps **no per-run state**: chunk replay is safe because every record
  apply is a natural-key idempotent upsert.
- **Set-based apply.** The four high-volume object-types (geo-places, language-scheme,
  external-organizations, person-regulatory-sanctions) replace their per-record loops with one
  parallel-array **UNNEST merge** per chunk (`INSERT … SELECT … FROM unnest(...) ON CONFLICT … DO
  UPDATE … WHERE source_version IS DISTINCT FROM EXCLUDED…` + `RETURNING (xmax = 0)` for exact
  created/updated/skipped counts); self-referential parents (geo place parent, languoid parent)
  resolve in a second pass over the touched rows, so a parent may arrive in the same chunk. The
  seven low-volume handlers keep their loops.
- **Resumability.** hermenea persists a per-job cursor (`worker_jobs.resume_seq` +
  `resume_checksum`, migration `migrations/hermenea/0005`) after every acknowledged chunk; a retried
  attempt re-stages the source and skips chunks `seq <= resume_seq` **iff** the staged checksum still
  matches (a changed source resets the cursor — full, still-idempotent re-run). Chunk boundaries are
  deterministic (fixed placetype walk + `ORDER BY` in the WOF mapper; per-push re-slicing without
  cross-push buffering).
- **Finite loader deadline.** With no request carrying a whole dataset, the loader's
  `WithHTTPTimeout(0)` escape hatch is retired: default 120 s per chunk request (install-tunable
  `oikumenea.http-timeout-ms`, plus `oikumenea.chunk-size`).
- **Worker concurrency (R-13 part).** `worker.concurrency` fans out N claim loops over the same
  `FOR UPDATE SKIP LOCKED` queue (default 1); the store was already multi-claimer-safe, and the
  scheduler's interval-bucketed idempotency keys make extra replicas' ticks fold into the same jobs.
- **Advisory-locked boot seeding (R-13 part, oikumenea side).** Pinax autoseed + the first-admin
  bootstrap run under a session-level `pg_advisory_lock` on a well-known key
  (`internal/platform/db.LockBootSeed`): a second replica booting the same fresh database **waits**,
  then re-runs the now-no-op idempotent checks — exactly one seed pass per fresh DB.

---

### D-DataIngestion — A generic reference-data ingestion & connector framework (extends D-Ontology)

> **Folded into [D-Hermenea](#d-hermenea--an-out-of-process-ingestion--scheduler-companion-supersedes-d-worker-folds-d-dataingestion) (M16 absorbs M17).** The pipeline
> shape below (sources/connectors → raw staging → mapper → canonical envelope → idempotent upsert →
> lineage) is **adopted unchanged**, but **relocated**: connectors, raw staging, the mapper registry,
> `import_sources`/`import_raw_batches`/`import_runs` and the scheduler live in the **hermenea**
> service's own DB — *not* `pkg/dataimport` inside oikumenea. oikumenea keeps only the generic
> `POST /import/{objectType}` upsert endpoint + per-row provenance + `import.manage`. The original text
> is retained for the pipeline-design rationale it still carries.

**Decision.** Bulk reference-data import becomes a **generic, reusable pipeline** in
[platform](../modules/platform.md) (`pkg/dataimport`), not a bespoke importer per domain. It mirrors
Palantir Foundry's ingestion stages — **Data Connection → Pipeline → Ontology mapping** — right-sized
for a self-hosted Go monolith (no Spark). Four parts:

- **Sources & connectors** — an `import_sources` registry (`type ∈ http|file` now; `jdbc-sql`/
  `object-store` parked as **DS-44**), credentials via the D-CryptoProvider seam; a pluggable
  `Connector` interface (`Fetch(ctx, source) → RawBatch`). First connector: **HTTP(S) download**;
  local bundled presets are the degenerate `file` case. Syncs run **on-demand or scheduled** on the
  D-Worker runtime (`import_syncs`, cron).
- **Raw staging** — `import_raw_batches` lands the fetched payload verbatim (checksum,
  `source_version`, `fetched_at`), re-mappable without re-fetch.
- **Transform → ontology** — each module **registers a mapper** for its importable object-types
  (`language-scheme`, `education-institutions`, `company-registry`, …) mapping raw records → a
  **canonical envelope** (`{object_type, source, source_version, license, generated_at, records[]}`) →
  a **code-keyed, idempotent, non-destructive upsert** into the domain catalog (never deletes;
  mismatches reported), in one transaction, emitted as **audited Actions** — preserving the
  bulk-ingest ≠ audited-edit boundary.
- **Lineage & run ledger** — `import_runs` (source, version, counts, checksum, status, errors) +
  `(source, source_version, imported_at)` provenance on every imported row + a sync-failure health
  reporter. One generic `POST /import/{objectType}` endpoint (instance-scope, `import.manage`).

**Why.** The same need recurred three times — rank presets (M15), Glottolog (M18), and the coming
education/company registries — so a per-domain importer is the wrong altitude. Foundry's separation of
*fetch → stage → curate → map* with first-class lineage is the proven pattern; the transferable parts
(explicit schema/mapper, provenance, idempotent re-sync, ingest≠edit) carry over without the heavy
machinery.

**Why not** (a) *Clone the M15 rank importer per domain*: duplication, no lineage, no scheduling.
(b) *Retrofit M15 onto this now*: M15 is shipped and works; left as a **legacy one-off** to avoid churn
(new code uses the framework). (c) *Full Foundry (Spark, branch/merge datasets)*: vastly over-scoped for
a self-hosted monolith; the raw-staging + mapper-registry slice captures the value. (d) *A live API
client per source baked into each module*: outbound deps scattered; the connector seam centralizes them.

**Consequence.** New `import_sources`/`import_syncs`/`import_raw_batches`/`import_runs` tables +
`pkg/dataimport` in [platform](../modules/platform.md); a per-module mapper-registration seam. New
Object/Action kinds in [ontology-mapping](../ontology-mapping.md). Lands as **M17**
([milestones](../milestones.md)), on M16. M15's `/rank-scheme/import` is **not** migrated. Parks
**DS-44** (more connectors). Additive / expand-only.

---

### D-Languages — Languages, language groups & writing systems as a Glottolog-faithful registry (extends D-Ontology, D-i18n)

**Decision.** A new **`language`** module holds the world's languages as a **faithful model of
Glottolog** (the standard genealogical reference), their **writing systems** (ISO 15924), and a
person's **language proficiency**.

- **`language_languoids`** — the recursive **Glottolog forest**, *one* table (not a group/language
  split): PK `code` (glottocode); `level ∈ {family, language, dialect}`; translatable `name`; self-FK
  `parent_id` (Glottolog "father" — a **strict tree**, structural containment FK, *not* a reified
  Link); denormalized `family_code` (root family, derived in SQL via the closure, per the
  denormalized-FK pattern); nullable **UNIQUE** `iso639_3` (the optional ISO 639-3 attribute —
  glottocode is the universal spine because families/dialects have no ISO code); `macroarea`;
  representative `latitude`/`longitude` (plain numeric — the `language` module precedes the PostGIS
  Location, D-Location); AES endangerment `status ∈ {not_endangered…extinct}` (replaces a naïve
  `living` boolean); `glottolog_version` provenance. A maintained **`language_languoid_closure`**
  (mirrors the tenant closure) answers descendant queries; **`language_languoid_countries`** ties
  languoids → `geo_countries` (CLDF `Country_IDs`, D-Geo).
- **Writing systems** — `writing_system_script_types` catalog (seeded `logographic`/`syllabary`/
  `alphabet`/`abjad`/`abugida`/`featural`); `writing_systems` (PK `code` ISO 15924, translatable
  `name`, `script_type`); `language_writing_systems` M:N (`is_primary`).
- **Language links** — `person_languages` (child of `person_persons`: `language_id` constrained to
  `level='language'`, `cefr_level ∈ {A1…C2}` nullable, `is_native`; `pii:basic`, purge-erased);
  `tenant_unit_languages` (a unit's official/working language); `i18n_locale_languages` (a locale's
  canonical language).
- **Population** — the migration **bootstraps the ~50 most-spoken languages** (`level='language'`,
  required columns only; real glottocodes so the first import updates them in place) so the catalog is
  usable on a fresh DB before any import — mirroring the `geo_countries` seed. Beyond that, by default
  the sources fetch **live from upstream master each run** via hermenea's
  `http-files` streaming connector + a Go transform: Glottolog CLDF (`languages.csv` + `values.csv`,
  ~27k languoids) and CLDR (`supplementalData.xml` + `iso-639-3.tab`) are staged to disk and mapped by
  the `CLDFMapper` / `SupplementalMapper` (the Go port of `deploy/language-presets/gen-presets.py`),
  emitting the whole forest as one page (single transaction). The **bundled preset** (the pinned
  Glottolog 5.3 CLDF snapshot, `deploy/language-presets/*.json`, opt-in asset / never a migration,
  CC-BY-4.0 attribution carried) remains as the offline/air-gap `file`-connector fallback. Tracking
  master trades reproducibility for freshness; a failed run is logged + retried + dead-lettered and
  never corrupts the catalog (imports are transactional).

**Why.** Language is a recurring analytics/linking dimension (who speaks what; a unit's working
language; locale provenance). Modeling it on **Glottolog** — the de-facto standard genealogy with
stable glottocodes — gives a complete, authoritative, re-importable dataset instead of a hand-curated
list, and the faithful recursive `languoid` model ("take it fully") preserves families/dialects and the
genealogical tree the simpler split would lose.

**Why not** (a) *ISO 639-3 as the PK*: families/dialects and unlisted languages have no ISO code; the
glottocode is the only universal spine. (b) *A separate `language_groups` + `languages` split*: diverges
from Glottolog's uniform languoid model and complicates the closure. (c) *Seed the full dataset in a
migration*: ~26k rows is a heavy, hard-to-maintain migration; D-DataIngestion's opt-in import is the
right home. (d) *A `living` boolean*: loses Glottolog's graded AES endangerment.

**Consequence.** New `language` module + tables above; person/tenant/localization gain language ties;
new Object/Link kinds in [ontology-mapping](../ontology-mapping.md). First **D-DataIngestion** consumer.
Lands as **M18** ([milestones](../milestones.md)), on M16 (the M17 pipeline, folded into M16) + M5/M2;
M3 for the unit tie. Additive / expand-only.

**Built (M18) — reconciliations.** Three details where the as-built code refines this decision (the
code is authoritative once built; recorded here so the two don't drift):
1. **RID PK, not `code` PK.** `language_languoids` (and `writing_systems`/`writing_system_script_types`)
   are **RID-keyed** (`id` PK, service 13), with `code`/`iso639_3` retained as UNIQUE lookup keys —
   consistent with **F-014/D-ResourceIdentifiers** making every structural entity RID-keyed (as
   `geo_countries`/`geo_places` already are). The glottocode is still the universal external spine.
2. **Writing systems are migration-seeded; the language↔script M:N is imported from CLDR.** Glottolog
   has no script data and ISO-15924 is only a code registry, so `writing_systems` + `script_types` are
   seeded in the migration (small/stable) and `language_writing_systems` is loaded by a second import
   object-type, **`language-scripts`**, sourced from CLDR `languageData` (`is_primary`). The Glottolog
   forest loads via the **`language-scheme`** object-type.
3. **Import shape.** The ~27k-languoid snapshot loads in one in-memory, parent-first envelope (not paged)
   so the closure + `family_code` rebuild sees the whole forest in one transaction; the bundled presets
   (`deploy/language-presets/{glottolog-5.3.json,cldr-scripts.json}`) are reproducible via
   `gen-presets.py`. UI is **deferred** (the `ui` gate).

---

### D-Location — A shared, standalone Location entity (PostGIS; app-derived MGRS; multi-format input)

**Decision.** A new **`location`** module provides one **standalone** place entity that anything with a
location references by FK. **`location_locations`** carries a **required** `geom GEOGRAPHY(POINT, 4326)`
(**PostGIS**), an **app-derived** MGRS string (pure Go, written on every coordinate change; nullable for
polar UPS points), the original input coordinate in a **`source_coordinate` JSONB** column, and a
structured postal address: `country_id` (NOT NULL → `geo_countries`, D-Geo), `admin_area_1`/
`admin_area_2`, `locality`, `street`, `house_number`, `postal_code`, `raw_address`; soft-delete; a
spatial GIST index. A `LocationService` offers CRUD + radius/bbox (`ST_DWithin`) queries. The coordinate
is accepted in **several formats** — WGS84 lat/lon, MGRS, UTM, СК-42 (Gauss-Krüger, numeric + grid) —
via a **pluggable converter registry** (`internal/geo/domain/coordinate.go`); the application converts
each to canonical WGS84, derives the MGRS, and persists the original input. **PostGIS is the only
operator-DB prerequisite** (the stock postgis image); the schema-bootstrap enables it and the readiness
gate checks for it. This **reverses the explicit `drafts/` drop of `location`/PostGIS/geography** —
re-adopted with rationale, exactly as D-WebUI re-adopted the UI.

**Amendment (2026-06-17).** The original decision derived **MGRS + H3** in the DB via the **h3-pg**
extension and a plpgsql `location_mgrs()` function/trigger, and accepted only WGS84 lat/lon input. This
was amended: **H3 is dropped entirely** (its sole intended use was efficient radius search, which PostGIS
already serves exactly via `ST_DWithin` on the GiST index — H3 is redundant given a real spatial index),
**MGRS moves to the application** (pure Go, no cgo, no DB extension), the coordinate gains **multi-format
input + `source_coordinate`**, and the operator image reverts from the custom `Dockerfile.postgres`
(postgis + h3-pg) to the **stock postgis image**. The change was applied by editing the M19 migration
`0019_location` in place (no new migration; the location tables had no production data).

**Why.** Both the education domain (buildings, campuses, dormitories — D-Education) and companies
(registered/operating addresses — D-Companies) need precise, queryable places, and the project's
analytics ambition ("better information for building relations & graphs") wants real spatial indexing.
A single shared entity dedupes addresses and enables "everything near point X" once, instead of
re-inventing address columns per owner. PostGIS `ST_DWithin` + GiST is the standard, efficient radius/kNN
stack; deriving MGRS in the app keeps the spatial dependency to PostGIS alone (no cgo, no custom image),
and supporting multiple input formats lets operators enter coordinates as they have them (e.g. MGRS or
СК-42 off a military map) without pre-converting.

**Why not** (a) *Embedded address columns per owner*: duplicates schema, blocks cross-entity spatial
queries and dedup. (b) *App-layer geometry, plain columns*: loses native spatial indexing / radius
queries (`ST_DWithin`) — so the authoritative `geom` stays PostGIS. (c) *Coordinate optional*: the
deployments here want spatial analytics, so the coordinate is the required spine and a precise point is
mandated (address-only records are out of scope — geocode first). (d) *Keep H3 / derive in DB*: H3 adds
an operator-DB extension (custom image) and a cgo binding for no benefit over the GiST `ST_DWithin` path;
deriving MGRS in plpgsql couples the operator image to h3-pg unnecessarily. (e) *Stay faithful to the
`drafts/` drop*: that drop was for a church-discovery scope; the army/university analytics scope
genuinely needs geography.

**Consequence.** New `location` module + `location_locations`; **PostGIS** becomes the operator-DB
prerequisite (bootstrap + readiness gate, [platform](../modules/platform.md)); the L-Conventions enum
note is unaffected. New Object kind `Location` in [ontology-mapping](../ontology-mapping.md). The
"Explicitly dropped from `drafts/`" list is updated to mark geography **re-adopted** (H3 stays dropped).
The planned religion `public_precision` projection (D-Religion, M22) was sketched as a coarsening to an
H3 cell at read time; with H3 no longer in the stack it must adopt an app-side coarsening (e.g. rounding
the coordinate / a geohash, or computing a single cell in Go) when M22 lands — a planned-tier seam, not a
built dependency. Lands as **M19** ([milestones](../milestones.md)); a foundation for M20 + M21. Additive
/ expand-only.

---

### D-Education — An `education` module: institutions, structure, and person bindings (extends D-Ontology)

**Decision.** A new **`education`** module models the education domain as **external reference
entities** (where people studied/taught), distinct from the deploying org's tenant units and
**independent of companies** (no shared organization foundation). Shape:

- **Reference catalogs** — `education_institution_kinds` (kindergarten…academy), `education_unit_kinds`
  (campus/faculty/department/chair…), `education_degree_levels` (seeded **ISCED 2011** 0–8).
- **Objects** — `education_institutions` (code, name, `kind`, `country`, founded/closed, lifecycle);
  `education_units` (a **dedicated recursive structure tree** per institution, typed, maintained
  closure, `link__education_unit_parent_of` — *not* reused tenant units); `education_buildings` (FK
  `location_id` → D-Location, kind incl. `dormitory`); `education_groups` (cohort under a unit).
- **Person bindings** — `person_education_enrollments` (`link__studied_at`: institution + optional
  unit/group, ISCED `degree_level`, field/specialty, effective-dated, status, qualification; mirrors
  the membership temporal Link; `pii:basic`); **mentorship reuses D-PersonRelationships** — extends
  M14 `person_sponsorships` with an optional **education context** (enrollment ref + role ∈
  professor/tutor/curator/advisor), no new link type; `person_dormitory_stays`
  (`link__resided_in_dormitory` — a **dedicated** stay entity: person ↔ dorm building, room, period;
  `pii:contact`, purge-erased).
- **Positions ("like a military")** — `education_positions` (institution/unit-owned billets,
  vacant-first) + `education_appointments` (`link__holds_education_position`, one-holder,
  effective-dated) — mirrors the membership module.

**Why.** The directory needs to place a person in their education history at analytics grade (who, when,
where, under whom, in which group, living where) for relationship graphs spanning army/church/
university. A dedicated structure tree keeps external institutions cleanly separate from the deploying
org's PDP-bearing tenant units; ISCED gives a standardized degree scale (the project's standards
instinct).

**Why not** (a) *Reuse tenant units for institution structure*: conflates external reference orgs with
the deploying organization, muddying the PDP and visibility semantics. (b) *A shared organization
foundation with companies*: deferred — the user chose fully independent modules (a university-as-legal-
entity tie can come later as a seam). (c) *A new mentorship link type*: M14 sponsorship already covers
advisor/mentor; reuse over reinvention. (d) *Model dorm living as a `person_residence`*: a dedicated
stay carries room/occupancy the generic residence lacks.

**Consequence.** New `education` module + tables above; [person](../modules/person.md)'s
`person_sponsorships` gains an optional education context; new Object/Link kinds in
[ontology-mapping](../ontology-mapping.md); institution registries ride D-DataIngestion. Lands as
**M20** ([milestones](../milestones.md)), on M5/M14/M19. Additive / expand-only.

**Extension (M20 reference layer — `university_ontology.md` adoption).** The M20 base is enriched with
the **reference-grade** slice of a full university ontology (`docs/university_ontology.md`), still as
**external reference data + person↔reference directory links** — explicitly **not** an operational SIS.

- **Adopted (additive, migration `0021_education_reference`):** *Curriculum/courses* — `education_programs`,
  `education_courses`, `education_curriculum_versions`, `education_curriculum_items` (reified
  `link__curriculum_item`), `education_course_prerequisites` (reified `link__course_prerequisite`,
  Go-side cycle guard); a person's enrollment gains optional `program_id` + `student_number`.
  *Research* — `education_research_centres`, `education_research_groups`, `education_grants`,
  `education_publications`. *Governance/credentials* — `education_governance_bodies`,
  `education_policies`, `education_qualifications`, `education_scholarships`,
  `education_accreditation_events`. *Person links (CASCADE, `pii:basic`, purge-erased):*
  `person_publication_authorships`, `person_research_memberships`, `person_grant_holdings`,
  `person_governance_memberships`, `person_education_qualifications` (the diploma award),
  `person_scholarship_awards`. A `diploma` row is added to the `document_types` catalog (the paper lives
  in the [document](../modules/document.md) module; the academic fact is the qualification award).
- **Deliberately excluded (kept reference, not operational):** academic **terms / calendars**,
  **course sections**, section-level **enrollment with grades**, **assessments**, GPA / grading; the
  source doc's **Person→Student/StaffMember subtype** split (the repo keeps a single person + directory
  attributes); **bi-temporal validity everywhere** (effective-dated links + soft-delete instead); and
  `country_code CHAR(2)` (the richer `geo_countries` FK is kept).
- **Surface.** A **second Conjure service** `EducationReferenceService` (own package, same `/education/v1`
  base-path) carries the reference CRUD + person links — isolated from the M20 base service. Reference
  names/titles are plain `string` (external reference data, **not** the i18n translation store) — a
  deliberate simplification vs the i18n'd M20 base entities. Reuses `education.read` / `education.manage`
  / `education.enrollment.manage`; no new permission strings. RID service 14 extended with object kinds
  9–20 and link kinds 5–12. No PDP/RLS (instance-global reference data).

**Why this scope.** The directory benefits from the curriculum/research/governance/credentials *facts*
about an institution and a person's relationship to them (analytics + relationship graphs), but the
operational SIS (scheduling, registration, grades) is a different product and conflicts with the
external-reference + single-person + soft-delete model. Adopt the reference half; leave the rest.

---

### D-Companies — A `company` legal-entity registry with an ownership/affiliation graph (extends D-Ontology)

**Decision.** A new **`company`** module holds organizations (private/public/state-owned/…) at
**registry grade**, scoped to **structural** data — identity, legal form, multi-jurisdiction
registration, locations, positions, and the **ownership/affiliation graph** — **independent of
education**. Volatile registry intelligence is **parked**.

- **Reference catalogs** — `company_legal_forms` (per-country: ТОВ/ПАТ/ФОП, LLC/JSC/GmbH…),
  `company_registration_schemes` (mirrors `document_personal_code_schemes`, D-PersonalCodes:
  `ua-edrpou`/`vat`/`us-ein`/`duns`/**LEI** ISO 17442 global spine, validators per scheme),
  `company_industry_classes` (NACE/ISIC/KVED economic-activity classification).
- **Objects** — `company_companies` (code, legal + short names, `legal_form`, `ownership_category ∈
  private|public|state_owned|municipal|foreign|mixed` — two orthogonal axes, `country`, founded/
  dissolved, lifecycle); `company_registrations` (per-scheme IDs + validation); `company_industry_assignments`
  (M:N, primary+secondary); `company_locations` (→ D-Location, role ∈ registered/operating/branch).
- **Positions** — `company_positions` + `company_appointments` (`link__holds_company_position` —
  CEO/director/employee billets; mirrors membership).
- **Equity / ownership links** — `company_foundings` (`link__founded`, founder a person **or** a
  company); `company_shareholdings` (`link__owns_stake`, **polymorphic holder** person|company, stake
  %, effective-dated — company-holder edges form the **ownership DAG**); `company_beneficiaries`
  (`link__beneficiary_of` — UBO: ultimate %, declared-vs-computed flag).
- **Company↔company** — parent/subsidiary (via shareholdings), `company_successions`
  (`link__succeeded_by` — M&A/reorganization lineage), founder-company (via foundings),
  `company_branches` (`link__branch_of` — non-independent sub-units).

**Why.** Holding companies as first-class entities lets people link into one queryable graph (employer,
founder, owner, ultimate beneficiary) — the "further linking" value the user asked for, YouControl-style.
Registration-as-scheme-registry (reusing the personal-code pattern, LEI as the global spine) makes it
multi-jurisdiction-ready; separate position vs equity links keep employment billets distinct from
ownership stakes; explicit declared UBO records what registries declare (often ≠ computed).

**Why not** (a) *One typed person↔company link*: conflates employment billets with equity stakes.
(b) *Include financials/court/tax/sanctions now*: volatile, feed-dependent, mostly useless without live
sources — parked as **DS-45** (rides D-DataIngestion connectors when a feed exists). (c) *Derive UBO
only*: loses the authoritative declared beneficiary; computed traversal/closure is the parked **DS-47**.
(d) *A shared org foundation with education*: the user chose independent modules.

**Consequence.** New `company` module + tables above; new Object/Link kinds in
[ontology-mapping](../ontology-mapping.md); GLEIF/national registries ride D-DataIngestion. Parks
**DS-45** (intelligence feeds), **DS-46** (web/contact), **DS-47** (ownership closure/computed-UBO).
Lands as **M21** ([milestones](../milestones.md)), on M5/M19. Additive / expand-only.

---

### D-Religion — A multi-faith `religion` vertical: catalog-driven taxonomy, organization graphs & discovery (reverses the `drafts/` religion drop, refines L-SingleDomain)

**Decision.** A new **`religion`** module models the religion domain for **any faith** — Christianity,
Islam, Judaism, Hinduism, Buddhism, Sikhism, Bahá'í, Shinto, traditional/indigenous, … — **reusing**
the existing `tenant` unit graph, `person`, `membership`, `order`, `authorization`, `localization`,
and the shared `location` entity (D-Location), and adding only the religion-specific structures. This
**reverses the explicit `drafts/` drop of religion-specific concepts** — re-adopted with rationale,
exactly as [D-Location](#d-location--a-shared-standalone-location-entity-postgis-app-derived-mgrs-multi-format-input)
re-adopted geography and [D-WebUI](decisions.md#d-webui--an-optional-standalone-nextjs-admin-ui-reverses-the-api-only-no-ui-drop)
re-adopted the UI. **Binding design rule: no faith's vocabulary is hard-coded.** Every
religion-specific value (organization kind, sub-tradition, clergy grade, office type, affiliation
type, site type, service type) is a **catalog row** (D-Code/D-i18n), keyed per religion/tradition and
seeded with cross-faith examples — **never a fixed `CHECK` enum**, never an `if faith == …` branch.

- **Taxonomy (catalogs).** `religion_religions` (top level: Christianity/Islam/Judaism/…) →
  `religion_tradition_families` (nested under a religion: Catholic/Orthodox/Protestant; Sunni/Shia;
  Orthodox/Conservative/Reform; Theravada/Mahayana/Vajrayana; …) → `religion_sub_traditions`
  (optional, generic — rite / school / madhhab / sampradaya: Latin/Byzantine; Hanafi/Ja'fari; …).
- **Organization nodes reuse `tenant_units`** with a **catalog-driven** `unit_kind` via
  `religion_org_kinds` (`code`/translatable `name`, optional `religion_id`, `ordinal`) — e.g.
  denomination/jurisdiction/congregation, school/community/mosque-community, movement/community,
  school/monastery/sangha. They are placed in **three seeded religion graphs** (D-Graphs):
  **`canonical`** (governance/jurisdictional tree, **authority-bearing** — the PDP cascades subtree
  grants here), **`tradition`** (taxonomic, **directory-only**, D-DirectoryGraphs), **`affiliation`**
  (voluntary association DAG, **directory-only**). `religion_org_profiles` (`unit_id` PK/FK,
  `religion_id`, optional `tradition_family_id`/`sub_tradition_id`, `short_code`) holds a unit's faith
  attributes; `religion_org_policies` is a **generic, data-driven** eligibility/exclusion mechanism
  (replacing any faith-specific doctrinal flag such as Christianity's "Nicene-affirming").
- **Clergy** — see [D-ClergyCredential](#d-clergycredential--clergy-grades--credentials-as-a-per-tradition-ordered-catalog--reified-link-faith-agnostic-parallels-d-rank).
- **Lay affiliation** — see [D-ReligiousAffiliation](#d-religiousaffiliation--lay-religious-affiliation-as-a-reified-piispecial-link-on-d-specialpii).
- **Discovery substrate (data, not CMS).** `religion_sites` (a reified Link: worship-community unit ↔
  `location_locations` (D-Location), `site_type_id` catalog, `visibility ∈ {public,unlisted,private}`,
  `public_precision ∈ {exact,street,neighborhood,city,hidden}`, `is_primary` one-per-unit);
  `religion_service_schedules` (per site: day/RRULE, time, IANA `timezone`, service `language` (ISO
  639-3), `service_type_id` catalog, `mode ∈ {in_person,online,hybrid}`, translatable `description`);
  `religion_aliases` (search-only alt names). **Search** is server-side: religion/tradition filter via
  the `tradition`/`canonical` graph **closure** (reuse `tenant_unit_closure`) + proximity/viewport via
  **PostGIS** on `location_locations` (D-Location) + service-language/-time + fuzzy name/alias. The
  `public_precision` projection (coarsen a published coordinate to an H3 cell) lives on the **site
  link**, not the shared location row — so one location can be published at different precisions by
  different owners (the persecuted-community use case).

**Identity-service use (e.g. a FaithMap-style app).** A discovery/CMS application sits **on top** and
uses go-oikumenea as its **identity / authorization / directory backend**: it delegates authentication
to an external IdP, go-oikumenea validates the inbound token and **decides** authorization (the PDP),
and the app's editorial roles map to **unit-scoped role assignments** (D-BaseRoles) on
denomination/community units. Pages/blocks/themes/slugs/content-i18n stay in that app — **out of
scope** here (this is an authorization+directory service, not a CMS).

**Why.** The original `drafts/` source *was* a religion platform (FaithMap); its religion concepts were
dropped only to keep the core generic. The user now wants the vertical back — and richer: any faith,
many faiths per deployment, with the discovery substrate. Reusing the unit graph (which already
supports multi-parent DAGs, multiple named graphs, public/shadow visibility, and a per-graph closure)
means denominations→dioceses→parishes (or councils→mosques, …) need **no new hierarchy machinery** —
they are units in a `canonical` graph, with a `tradition` taxonomy graph and a voluntary `affiliation`
graph beside it. Keeping all faith vocabulary in catalogs (not enums/branches) is the only way one
schema fits every religion and honors **L-SingleDomain's** "no org-type discriminator in code."

**Why not** (a) *A Christianity-shaped schema (denomination/parish/holy-orders columns)*: excludes
every other faith and bakes one tradition's vocabulary into the model — rejected outright per the
user's "all religions" requirement. (b) *A new bespoke hierarchy table for religious bodies*:
duplicates the tenant DAG + closure + visibility + PDP that already exist; religious governance **is**
a unit graph. (c) *Port the FaithMap CMS (pages/blocks/themes)*: that is a content app's job, not an
identity service's; only the **directory/authz/discovery data** belongs here. (d) *A separate
deployment per denomination*: the user wants many faiths/traditions co-resident (ecumenical
discovery), which the graphs + catalogs support without breaking the single-domain spirit.

**L-SingleDomain is refined, not broken** (exactly as D-RankSystems refined L-OneRankScheme). The
single domain per deployment is **religion**; *within* it, multiple religions/traditions coexist as
**catalog data + units in graphs**. There is still **no org-type discriminator branched on in code** —
`unit_kind` and every faith label remain descriptive catalog rows, never a code switch. The lock's
note below points here.

**Consequence.** New `religion` module + the tables above ([religion](../modules/religion.md)),
reusing `tenant`/`person`/`membership`/`order`/`authorization`/`localization` and `location`
(D-Location); new Object/Link/Action kinds in [ontology-mapping](../ontology-mapping.md); religious
belief data rides [D-SpecialPII](#d-specialpii--envelope-encryption-extended-to-the-piispecial-tier-resolves-the-person-field-half-of-ds-29).
Lands as the **M22–M25** cluster ([milestones](../milestones.md)) on M3/M5/M6/M7/M10/M19. **Resolves /
promotes DS-48** (the Religion domain); the "Explicitly dropped from `drafts/`" religion line is
updated to mark religion **re-adopted**. Additive / expand-only.

**Refined (M22 design, 2026-06-19).** The taxonomy half is reshaped so it can hold the **real-world
depth of any faith — especially Christianity, which nests far deeper than three levels** (Christianity →
Catholic/Eastern-Orthodox/Oriental-Orthodox/Church-of-the-East/Protestant → Lutheran/Reformed/Anglican/
Baptist/Methodist/Pentecostal/… → rite/movement → a named denomination). The three fixed catalog tables
(`religion_religions`/`religion_tradition_families`/`religion_sub_traditions`) are **replaced by a single
recursive `religion_taxa` tree + maintained closure**, reusing the proven `language_languoids`-forest /
`education_unit_closure` pattern. Six refinements:

1. **Recursive `religion_taxa` + `religion_taxa_closure`.** One self-referencing tree (`parent_id`
   RESTRICT, NULL = root religion) with a denormalized root `religion_id` (derived via closure, like
   `language_languoids.family_code`). Arbitrary depth; no fixed level columns.
2. **Catalog-driven level marker.** Each taxon carries a `rank_id` → an **ordered `religion_taxon_ranks`
   catalog** (seeded `religion`→`branch`→`tradition`→`sub_tradition`→`denomination`, extensible) so the
   main branches stay findable and Christianity-the-religion is distinguishable from
   Eastern-Orthodoxy-the-tradition. The rank set is structural, not faith vocabulary, so it stays
   catalog-driven (no `CHECK` enum of faith terms — the lock holds).
3. **Religion-type ("theism") classification.** A multi-tag `religion_classifications` catalog
   (monotheistic/polytheistic/henotheistic/monistic/nontheistic/pantheistic/animistic/dualistic/…),
   attached M:N to taxa (`religion_taxon_classifications`) and seeded at the `religion` level. Resolution
   is **nearest-declared-wins**: a descendant taxon **or** an org unit
   (`religion_unit_classifications`) may declare its own set, which **fully overrides** the inherited
   one (walk up the closure to the nearest declaring node). Read-time projection, not stored.
4. **Taxonomy/organization boundary = hybrid.** The seeded tree goes to sub-tradition/movement for most
   faiths **plus** the globally-significant classificatory bodies (Eastern Orthodox autocephalous
   churches, the 23 Eastern Catholic sui-iuris churches, Oriental Orthodox churches, major Protestant
   denominations) as `denomination`-level taxa. Concrete **governed instances** (a specific
   diocese/parish) remain `tenant_units` linking to the nearest taxon.
5. **Org → taxon as M:N tagged links, one primary.** A unit's faith classification is
   `religion_org_classifications` (`is_primary` partial-unique per unit), replacing the single
   `tradition_family_id`/`sub_tradition_id` columns on `religion_org_profiles` — a body often fits
   several classifications at once (Reformed Baptist; Eastern Catholic = Catholic + Byzantine-rite).
6. **Wikidata anchor + curated seed.** Each taxon carries an optional `wikidata_id`; the **rich curated
   seed ships in the migration** (deep Christianity + broad world religions), generated by a
   reproducible `deploy/religion-presets/gen-presets.py` anchored to Wikidata QIDs. A hermenea import
   connector is left as an open seam.

**Why the refinement.** The user's binding requirement is "cover all existing religions, especially
Christianity, reflect the real world as much as possible." A fixed 3-level shape cannot hold
Christianity's branches→families→denominations (uneven depth across faiths); a recursive tree + closure
can, and the repo already proves that pattern twice. The faith-agnostic lock is **unchanged** — every
faith term remains a catalog row, never an enum or `if faith == …` branch. **M22 builds only the
taxonomy + organization slice** (catalogs, `religion_taxa` + closure, org profiles/classifications/
policies, the three graphs); clergy (M23), affiliation (M24), and discovery (M25) are unchanged below
and deferred. The `religion_clergy_grades`/`religion_affiliation_types`/etc. catalogs in M23–M25 keep
their per-tradition shape but now FK a `religion_taxa` node instead of a `tradition_family` row.

---

### D-ClergyCredential — Clergy grades & credentials as a per-tradition ordered catalog + reified Link (faith-agnostic, parallels D-Rank)

**Decision.** Clergy/religious-functionary **standing** is modeled in the `religion` module — **not**
by reusing the linear `rank` scheme — because ordination/investiture differs from a military ladder
(sacramental/indelible where applicable, concurrent offices, per-tradition shapes). Two parts plus
offices:

- **`religion_clergy_grades`** — an **ordered, per-tradition catalog** (`code`/translatable `name`,
  `grade_category_id` → a generic `religion_grade_categories` catalog instead of a fixed major/minor
  enum, `ordinal`, optional `tradition_family_id`). Generic across faiths: bishop/presbyter/deacon;
  imam/mufti/sheikh; rabbi/cantor; bhikkhu/lama; pujari/swami.
- **`religion_clergy_credentials`** — a **reified Link** `link__clergy_credential` (`Person` →
  `ClergyGrade` within a tradition/organization unit; `granted_on DATE`, optional conferring-authority
  provenance, `status ∈ {active,suspended,revoked}`, `effective_from`/`effective_to`, `source`/
  `confidence`). **Indelible where sacramental**: a revocation/laicization is a **status flip**, never
  a hard delete. Covers ordination / investiture / recognition uniformly.
- **Offices** reuse `membership` **positions** (unit-owned billets — `religion_office_types` catalog:
  pastor / imam-of-mosque / head-rabbi / abbot / …) + authority via `authorization` role assignments;
  conferral/appointment/transfer/suspension are `order` (decree) types.

**A clergy credential is a directory fact, never an authz input** — exactly the **D-Rank** stance
(rank ≠ permission). The PDP never reads a clergy grade; authority over a community comes **only** from
a role assignment on that unit.

**Why.** Every faith has graded religious functionaries, but the grades are not a single comparable
ladder (no NATO-STANAG analog — DS-43), they are per-tradition and often non-linear, and ordination is
frequently *indelible*. A dedicated per-tradition ordered catalog + an effective-dated credential Link
captures this faithfully, while reusing `membership`/`authorization` for the *office* keeps "who may
act" in the one PDP path. Modeling the credential as a reified Link (identity, attributes, history) is
the binding D-Ontology stance.

**Why not** (a) *Reuse the `rank` module*: ranks are a single ordered scheme barred from authority for
a different reason; clergy grades are per-tradition and sacramental, and overloading `rank` muddies
both. (b) *Branch authority on clergy grade*: would reintroduce rank-as-permission, violating D-Rank.
(c) *A bare `person.clergy_grade` FK*: loses ordination history/provenance/status; the relationship
deserves reification. (d) *A cross-tradition comparator*: there is none (DS-43 stays parked); grades
compare only within a tradition's `ordinal`.

**Consequence.** New `religion_clergy_grades` + `religion_grade_categories` + `religion_office_types`
catalogs and the `religion_clergy_credentials` Link ([religion](../modules/religion.md)); new
`ClergyGrade`/`OfficeType` Objects + `link__clergy_credential` in
[ontology-mapping](../ontology-mapping.md). Part of the **M23** milestone. Additive / expand-only.
**Status: landed (M23).** Built in migration `0024_religion_clergy` (`religion_grade_categories` `16,1,7`
/ `religion_clergy_grades` `16,1,8` / `religion_office_types` `16,1,9` / `religion_clergy_credentials`
`16,2,2`); the per-tradition catalogs FK `religion_taxa` via `tradition_taxon_id`. `clergy.manage`
gates credential writes over the canonical graph. *Offices as Positions remain future work.*

---

### D-ReligiousAffiliation — Lay religious affiliation as a reified `pii:special` Link (on D-SpecialPII)

**Decision.** A person's **lay religious affiliation/belief** is recorded as a **reified Link**
`link__affiliated_with` (`religion_affiliations`: `Person` → a `religion`/tradition/community unit;
`affiliation_type_id` → a generic **`religion_affiliation_types`** catalog — adherent/member;
catechumen/baptized/confirmed; shahada; bar/bat-mitzvah; …; `status`, `effective_from`/`effective_to`,
`source`/`confidence`). The affiliation value is **GDPR Art. 9 `pii:special`** (D-PIITiers) and is
therefore **envelope-encrypted at rest** with a blind index for uniqueness, gated on
[D-SpecialPII](#d-specialpii--envelope-encryption-extended-to-the-piispecial-tier-resolves-the-person-field-half-of-ds-29);
it is **crypto-erased on person purge** (the `PersonPurged` subscriber extends to it), reads project
through D-PersonReadScope, writes are audited. Rite-of-passage / life-cycle records (baptism /
bar-mitzvah / …) are a reserved generic catalog-typed seam (**DS-49**).

**Why.** Affiliation is the defining lay-side religion datum a deployment may need — and the
project's own D-PIITiers already names *religious affiliation* as the motivating Art. 9 example. The
clergy **credential** (D-ClergyCredential) is an organizational/public fact; lay **belief** is private
special-category data with a stricter regime, so it gets the envelope and crypto-erase. Reifying it as
a Link (not a column) carries provenance/confidence/history the same way the social-account attribution
does.

**Why not** (a) *A plaintext `person.religion` column*: stores Art. 9 data unprotected — forbidden by
the "no special-category PII without the envelope" rule. (b) *Fold lay affiliation into the clergy
credential*: conflates a private belief with a public office. (c) *A fixed affiliation-type enum*:
excludes other faiths' milestones; the catalog is per-tradition.

**Consequence.** New `religion_affiliations` Link + `religion_affiliation_types` catalog
([religion](../modules/religion.md)); new `link__affiliated_with` + `AffiliationType` in
[ontology-mapping](../ontology-mapping.md); extends the `PersonPurged` erasure path. **Depends on**
D-SpecialPII. Lands as **M24**; parks **DS-49** (rite-of-passage records). Additive / expand-only.
**Status: landed (M24).** Built in migration `0025_religion_affiliation` (`religion_affiliation_types`
`16,1,10` / `religion_affiliations` `16,2,3`). The optional belief value is envelope-encrypted (reuses
`pkg/crypto` `Cipher`); `affiliation.manage` gates read+write; `ErasePersonAffiliations` crypto-erases.
The `PersonPurged` auto-trigger stays a shared open seam with document `ErasePersonRecords` (exercised
directly today).

---

### D-SpecialPII — Envelope encryption extended to the `pii:special` tier (resolves the person-field half of DS-29)

**Decision.** The envelope-encryption mechanism that **D-CryptoProvider** ships for `pii:sensitive`
(pluggable `KeyProvider`, ciphertext-in-DB, wrapped DEK, blind index, crypto-erase) is **extended
unchanged to the `pii:special` (GDPR Art. 9) tier** for **person/affiliation fields**. No new
mechanism: a `pii:special` value uses the same `value_ciphertext`/`wrapped_dek`/`key_ref`/
`value_blind_index` shape, the same KMS-on-unwrap path, and the same DEK-destruction crypto-erase on
purge. This **resolves the person-field half of DS-29** (the "extend the envelope to `pii:special`"
escalation). The **audit-payload half** of DS-29 (`before`/`after` JSONB at the `pii:special` ceiling)
remains parked — special-category data still must not enter audit payloads — so DS-29's audit scope is
untouched.

**Why.** The religion vertical (D-ReligiousAffiliation) is the first feature that genuinely needs to
**store** Art. 9 data, so the long-anticipated `pii:special` envelope must ship. Because
D-CryptoProvider already abstracts the backend and the blind-index/erase mechanics, extending the tier
is a scope change, not a new design — the cleanest possible way to unblock it. Confining the resolution
to **person/affiliation fields** (not audit payloads) keeps the blast radius small.

**Why not** (a) *Keep `pii:special` blocked and store affiliation in plaintext*: violates the
"no Art. 9 without the envelope" rule. (b) *Invent a separate special-tier mechanism*: needless; the
sensitive-tier envelope already does exactly what Art. 9 needs. (c) *Resolve the audit-payload half
too*: not required by this work and higher-risk — left parked under DS-29.

**Consequence.** The `pkg/crypto` envelope + `KeyProvider` seam now also protect `pii:special`
person/affiliation columns; D-PIITiers' "`pii:special` is **not stored**" caveat is lifted **for
encrypted person/affiliation fields** (audit-payload ceiling unchanged). **Narrows DS-29** to the
audit-payload extension only; as a side effect it lifts the envelope blocker on **DS-38(b)** (gender
identity), though *storing* gender identity stays a separate parked choice. See
[D-CryptoProvider](decisions.md#d-cryptoprovider--pluggable-envelope-encryption-for-sensitive-pii-reshapes-ds-29),
[platform](../modules/platform.md), [open-questions](../open-questions.md) (DS-29). Lands with **M24**.
**Status: landed (M24).** The existing `pkg/crypto` envelope (`Seal`/`Open`/`BlindIndex`) now also
protects the `pii:special` `religion_affiliations` value columns — no new mechanism, exactly as decided.

---

### D-GeoPlaces — Who's-On-First administrative gazetteer (`geo_places`), the first hermenea connector (extends D-Geo, supersedes D-GeoSubdivisions, pulls PostGIS forward)

**Decision.** Geography gains a **full administrative gazetteer** sourced from **Who's-On-First (WOF)**,
loaded by hermenea's **first real connector** (M16, D-Hermenea). A new shared platform table
**`geo_places`** holds the four WOF admin placetypes — **country / region / county / locality**
(city·town·village; WOF has no town/village split — that is a `population` property, not a placetype) —
as a single tree: an `id uuid` **RID PK** under the **`location` service (code 12)** — minted by
`new_id(12,1,2)` with a `rid_*` shape `CHECK` like every other Object (F-014; *amended from the
original `wof_id BIGINT` PK*) — with `wof_id BIGINT NOT NULL UNIQUE` retained as the stable WOF
import/concordance key; `placetype TEXT`+`CHECK`; `parent_id uuid` self-FK → `geo_places(id)`
(a structural containment edge derived from `wof:hierarchy` to the nearest imported ancestor, the
`rank_types` pattern — **not** a reified Link); denormalized `country_id uuid` → `geo_countries(id)`;
default-locale `name` (other locales via the i18n store, `entity_type='geo_place'`); `population`;
`hierarchy`/`concordances` JSONB; `status` (`active`/`retired`, the latter mirroring WOF
`mz:is_current=0`/supersession — **non-destructive**); the `(source, source_version, imported_at)`
provenance trio; and **PostGIS** geometry — `geom GEOMETRY(Geometry,4326)` (full shape) + DB-derived
`centroid`/`bbox`, served as GeoJSON via `ST_AsGeoJSON`, GIST-indexed. The existing **`geo_countries`
is enriched in place** (additive `wof_id` + geometry + `iso_a3`/`numeric_code`), and a
`placetype=country` WOF record mirrors its geometry onto the country row in the same import
transaction. Coverage is **global** (all four placetypes worldwide), rolled out **per country** (one
`wof-geo-<iso>` source each, cron-staggered, Ukraine first).

**Amendment (M16 — geo becomes RID-keyed, full RID end-to-end).** `geo_countries` left the
natural-key carve-out: it gained an `id uuid` **RID PK** (`new_id(12,1,1)`, location service 12) with
`code CHAR(2)` demoted to `NOT NULL UNIQUE` (the canonical external **lookup** key). **All 8 country
FK consumers** — `person.country_of_birth`, `person_citizenships`/`person_residences`/`person_phones`,
`document_documents.issuing_country`, `document_personal_code_schemes`, `rank_systems`, and
`geo_places` — repoint to `geo_countries(id)` (the columns become `*_id uuid`). The **country RID
flows end-to-end** through domain → Conjure → web (ISO `code` is lookup-only). Because country entry
was free-text ISO with no countries endpoint, a read-only **`GeoService` (`GET /geo/countries`,
`country.read`)** returns `{id, code, name, status}` so clients resolve a code to its RID and populate
pickers. Ingestion is unchanged on the wire — the WOF/ISO importer still streams natural keys and
resolves `wof_id`/`code → id` in SQL on upsert (an unresolvable non-zero parent/country trips the FK
loudly, preserving the parent-first RESTRICT guarantee); the phone country is derived as an ISO code
and likewise resolved in SQL; the rank **preset import** resolves its ISO country code to the RID
before insert.

**Pipeline / ingestion.** WOF ships as per-country "combined" SQLite admin DBs (`.db.bz2`), not a JSON
API, and a single country's geometry far exceeds the 16 MiB in-memory batch cap. So M16 adds: a new
**`wof-sqlite` StreamingConnector** (fetch `.db.bz2` → bzip2-decompress → stage to a temp file, never
in-memory/BYTEA) and a **paged-mapper seam** (`PagedMapper`) that walks the SQLite **parent-first**
(country→region→county→locality) emitting bounded pages, each loaded as its own canonical envelope via
`POST /import/geo-places`; `import_runs` aggregates the counts. Idempotency is keyed on `source_version`
(re-import of the same WOF edition skips; a newer one updates; never deletes). Hermenea stays
PostGIS-agnostic — it ships GeoJSON **text**; only oikumenea materializes geometry.

**Why.** A real gazetteer with shapes + parentage is the "better information for relations & graphs"
the registry verticals need; WOF is an open, ID-stable, concordance-rich global admin source. Keeping
`geo_countries` as the ISO-keyed FK anchor (enriched, not re-keyed) preserves every existing consumer
while giving countries WOF geometry. A WOF tree subsumes ISO-3166-2 subdivisions **and** reaches down to
localities, which ISO-3166-2 cannot. Pulling **PostGIS forward** from D-Location (M19) is required to
store/query the shapes now; M19 reuses the same stack.

**Why not** (a) *Keep the ISO-3166-2 `geo_subdivisions` plan (D-GeoSubdivisions)*: ISO-3166-2 has no
codes for cities/villages and no geometry — WOF is strictly richer, so D-GeoSubdivisions is
**superseded**. (b) *Full WOF replacement (re-key `geo_countries` onto WOF ids)*: breaks binding D-Geo
and 8 FKs — rejected. (c) *JSONB GeoJSON instead of PostGIS*: dead-weight, unqueryable — rejected.
(d) *Single planet SQLite in one batch*: multi-GB, no failure isolation — per-country streaming +
paging chosen instead.

**Consequence.** New `geo_places` table + enriched `geo_countries` (folded into the bootstrap migration,
PostGIS extension added); new import object-type `geo-places` + handler in `dataimport`; hermenea gains
the `wof-sqlite` connector, the `PagedMapper` seam, a file-staged raw batch (`staged_path`), and the
`geo-places` mapper; new reference Object `GeoPlace` in [ontology-mapping](../ontology-mapping.md).
**Supersedes D-GeoSubdivisions**; **D-Vehicles**' plate-region FK `subdivision_id` → `geo_places`
(placetype=region). Global locality coverage is millions of rows + GBs of geometry — a long, staggered
backfill, not a single sync. Lands in **M16** ([milestones](../milestones.md)). Additive / expand-only.

---

### D-GeoSubdivisions — Seeded ISO-3166-2 subnational-division registry (extends D-Geo) — **SUPERSEDED by D-GeoPlaces**

> **Superseded (M16).** Replaced by **D-GeoPlaces** (the WOF `geo_places` gazetteer), which covers
> region/county/locality with codes, geometry, and parentage that ISO-3166-2 cannot. The original
> design is retained below for provenance; `geo_subdivisions` is **not built**, and D-Vehicles'
> `subdivision_id` now targets `geo_places`.

**Decision.** Geography gains a **second seeded reference layer below the country**: a new shared table
**`geo_subdivisions`**, owned/seeded by [platform](../modules/platform.md) exactly like
[`geo_countries`](decisions.md#d-geo--seeded-iso-3166-country-registry-citizenship-birth-and-residence-as-first-class-person-data)
(D-Geo) — **not** a standalone domain module. Shape: `code TEXT` PK (**ISO 3166-2**, e.g. `'UA-32'`,
`'UA-46'`); `country_code CHAR(2)` → `geo_countries`; optional `parent_id TEXT` self-FK (a nested
subdivision — raion under oblast); `subdivision_type TEXT` (`TEXT`+`CHECK`: oblast/region/state/
province/raion/district/city/…); translatable `name` (default-locale fallback + the i18n store, new
`entity_type='subdivision'`); `status` (`active`/`retired`), `sort_order`, timestamps. All columns
`pii:none`. Instance-admin-extensible (`subdivision.manage`); read via `GET /subdivisions?country=`.
It is a **code-PK reference table like `geo_countries`/`Country`**, *not* an RID-PK Object.

**Seeding.** The target-country subset (UA first) is **migration-seeded** exactly as `geo_countries`
is; the **full global ISO 3166-2 set rides M17** (D-DataIngestion) as an optional connector, so the
~5k-row global table is never baked into a migration. `person_residences.region` and
`location_locations.admin_area_1`/`admin_area_2` **stay free-text for now**; retrofitting them to a
`geo_subdivisions` FK is the parked **DS-51** seam (additive, expand/contract).

**Why.** The vehicle registry (D-Vehicles) needs a *structured* plate-region (the registration region),
and free text defeats the analytics/graph purpose. Modelling subdivisions as a **seeded registry with
translatable names** (rather than free text or a compiled CHECK list) matches the country/locale/graph
registry pattern, lets the i18n store localize subdivision names, and lets an operator add edge-case
entities without a code change — the same rationale D-Geo gave for countries, one level down.

**Why not** (a) *Free-text region* (the original `person_residences.region` choice): unqueryable, no
referential integrity — rejected for the vehicle plate-region. (b) *A vehicle-module-local region
catalog*: subdivisions are a **shared** geography concept (residences, Location addresses also want
them), so it belongs in platform geo, not one domain module. (c) *Migration-seed the full ISO 3166-2
set*: ~5k rows of volatile reference data belong on the M17 ingestion path, not a migration.

**Consequence.** New shared platform table `geo_subdivisions` (seeded like `geo_countries`);
subdivision names join the [localization](../modules/localization.md) store (`entity_type=
'subdivision'`); new `GET /subdivisions` read + instance-scope `subdivision.manage`; new reference
Object `GeoSubdivision` in [ontology-mapping](../ontology-mapping.md). Module count **unchanged** (geo
is platform-owned reference data). Parks **DS-51** (full ISO 3166-2 set + residence/Location retrofit).
Lands with **M26** ([milestones](../milestones.md)) as the shared foundation under D-Vehicles, exactly
as M19 bundled the PostGIS bootstrap with Location. Additive / expand-only.

---

### D-Vehicles — A `vehicle` registry binding people & companies to vehicles (extends D-Ontology)

**Built (M26) — amendments.** Delivered as the [vehicle](../modules/vehicle.md) module (RID service
17), migration `0027_vehicle`. Two amendments landed at build time, both flowing from the M16
geo re-key: (1) the plate-region FK `subdivision_id` targets the WOF **`geo_places`** gazetteer
(placetype=region, **D-GeoPlaces supersedes D-GeoSubdivisions** — no `geo_subdivisions` table is built),
app-validated on write (`Vehicle:RegionInvalid`); (2) every country FK is `country_id uuid →
geo_countries(id)` (the geo RID re-key), not an ISO `code`. The polymorphic owner is `(owner_kind,
owner_id text)`; person-owned registrations are erased via `ErasePersonRegistrations` (the `PersonPurged`
subscriber is a deferred shared seam). The original design (below) said `country_code`/`geo_subdivisions`
— read those as `country_id`/`geo_places` per these amendments.

**Decision.** A new **`vehicle`** module holds vehicles at **registry grade**, scoped to **structural**
data — a brand/model/type taxonomy, the physical vehicle, and the ownership/plate record — so people
and companies link to vehicles in one queryable graph. Volatile vehicle intelligence is **parked**.

- **Reference catalogs** (instance-scope, `code` + translatable `name`, D-Code/D-i18n):
  - `vehicle_types` — a taxonomy **tree** (car/truck/motorcycle/bus/trailer/special…) via a `parent_id`
    self-FK + denormalized root; a **shallow tree with no maintained closure** (mirrors the `rank_types`
    tree — a structural containment FK, **not** a reified Link).
  - `vehicle_brands` — the marque (Toyota/BMW…); `country_code` → `geo_countries` (origin).
  - `vehicle_models` — `brand_id` FK (containment), `name`, `generation`, `manufacture_start`/`_end DATE`.
  - `vehicle_registration_number_types` — plate-type catalog (regular/temporary/transit/diplomatic/
    military/old…).
- **Object** — `vehicle_vehicles`: RID PK; `type_id`/`model_id` FK; `manufacture_date DATE`; `vin`
  (normalized, **unique among active**, nullable for VIN-less vehicles, `pii:basic`); `color`;
  `attributes JSONB` long-tail grab-bag (DS-6-style); soft-delete; audited writes.
- **Reified Links:**
  - `vehicle_brand_manufacturers` (`link__manufactured_by`): `brand_id` → `company_companies`
    (D-Companies), **temporal** `effective_from`/`effective_to` (a brand's manufacturer changes with
    acquisitions).
  - `vehicle_registrations` (`link__registered_to`): the **ownership + plate record** — `vehicle_id` →
    vehicle; a **polymorphic owner** `owner_person_id` **XOR** `owner_company_id` (person|company,
    mirroring D-Companies' polymorphic `OWNS_STAKE`/`FOUNDED` holder); `country_code` → `geo_countries`;
    `subdivision_id` → `geo_subdivisions` (the plate region, optional); `registration_number` (plate,
    **unique among active per country**); `number_type_id` → catalog; **temporal** `effective_from`/
    `effective_to` + `status` (re-registration = a new row, so registration **is** the ownership
    history). Person-owned rows are `pii:basic`, **holder-scoped** through the person owner
    (D-PersonReadScope) and **purge-erased** by a `PersonPurged` subscriber in the vehicle module
    (mirroring the [document](../modules/document.md) module's purge subscriber).
- **Containment FKs (not Links):** model→brand, vehicle→model/type, type→parent — structural FKs per
  the rank/language precedent, never reified.
- **Authorization:** catalogs are instance-scope (`vehicle.manage`); vehicle/registration reads are
  holder-scoped for person-owned rows; all writes are audited Actions (`CreateVehicle`,
  `RegisterVehicle`/`TransferRegistration`, catalog edits).

**Why.** Holding vehicles as first-class entities lets people and companies link into one queryable
graph (owner, fleet operator, manufacturer) — the "better information for relations & graphs" the user
asked for, AutoRia/registry-style. Modelling registration as a **temporal ownership+plate Link** (not a
separate ownership entity) captures transfers as history with the same discipline membership uses; a
polymorphic owner covers personal vehicles **and** company fleets; the brand→manufacturer link reuses
D-Companies so the legal entity behind a marque is one shared record.

**Why not** (a) *A separate ownership entity beside registration*: a vehicle's owner **is** whoever
holds its current registration — folding them avoids a redundant table. (b) *Person-only owner* (as the
raw todo said): excludes fleets; the polymorphic owner is strictly richer and matches the company
ownership graph. (c) *A maintained closure on `vehicle_types`*: the type taxonomy is shallow; a self-FK
+ denormalized root (the rank-type pattern) suffices. (d) *Include insurance/inspection/accident/
telematics now*: volatile, feed-dependent, useless without live sources — parked as **DS-52** (rides
D-DataIngestion connectors when a feed exists), mirroring DS-45 for companies.

**Consequence.** New `vehicle` module + tables above; brand/model reference data + national vehicle
registries ride **M17** (D-DataIngestion); new Object/Link kinds in
[ontology-mapping](../ontology-mapping.md); the `PersonPurged` erasure path extends to person-owned
registrations. Parks **DS-52** (vehicle intelligence feeds) and **DS-53** (column-ize stabilized vehicle
specs out of `attributes`, the DS-6 pattern for vehicles). **Depends on** D-GeoSubdivisions, D-Companies
(M21), and the person directory (M5). Lands as **M26** ([milestones](../milestones.md)).
Additive / expand-only.

---

### D-ClientSDK — Unified Go + TypeScript SDKs generated from the Conjure contract (extends D-Conjure, D-WebUI)

**Decision.** Ship **two symmetric, published client SDKs** for the go-oikumenea API — one **Go**, one
**TypeScript** — both **generated from the single source of truth**, the `api/*.conjure.yml` Conjure
contract, and each exposing a **unified façade** that binds one base URL + one bearer token to every
service. Consumers `go get` / `npm install` two independent packages; this repo consumes them from the
tree. See the [clients module doc](../modules/clients.md).

- **One contract, three generated surfaces.** The server interfaces (`internal/conjure`), the Go SDK
  (`clients/go/oikumenea/<module>`) and the TS SDK (`clients/typescript/src/generated`) are all generated
  from the same IR, so none can drift (D-Conjure). Generated code is **never hand-edited**.
- **Go SDK** (`clients/go/`, the existing nested module): per-service generated clients **plus a
  hand-written façade** `client.New(baseURL, token, opts…)` (and `NewWithTokenProvider`) in
  `clients/go/client.go` — one `Dial`, one bound token, every service as a field.
- **TS SDK** (`clients/typescript/`, new publishable npm package): per-service clients generated by
  **conjure-typescript** (a Node CLI — no JVM) from the same Conjure IR, **plus a hand-written façade**
  `createOikumeneaClient({ baseUrl, token? })` in `src/index.ts` over one conjure-client
  `DefaultHttpApiBridge`. The IR is obtained offline by reusing `tools/ir2openapi -dump-ir`; its package
  names are rewritten `oikumenea.<m>` → `oikumenea.api.<m>` (a derived-artifact transform — the wire
  contract is untouched) so they meet conjure-typescript's ≥3-segment rule. Driven by
  `scripts/gen-ts-client.sh` (with a `--verify` drift gate, the analog of `godel conjure --verify`).
- **Hermenea is a member of both façades.** oikumenea reverse-proxies the hermenea control/read API at
  `/hermenea/v1/*` (D-Hermenea), so `client.Hermenea` / `client.hermenea` reach the companion through
  oikumenea — a single client + base URL covers both oikumenea-native and hermenea-proxied endpoints.
  No DB coupling is introduced; this is purely the existing HTTP proxy surfaced in the SDKs.
- **Web consumes the TS SDK** (supersedes the in-`web` `openapi-typescript`/`openapi-fetch` layer,
  D-WebUI): `web/` imports `oikumenea-client` as a `file:` dependency. The browser client points the
  façade at the BFF (`/api/oikumenea`, no token in the browser); Server Components bind the session
  bearer via `oikumenea()` (`web/src/lib/api/server.ts`). The BFF proxy and token-injection trust model
  are unchanged.

**Why.** The Go SDK and an OpenAPI-derived TS type file already existed but diverged in kind (a real
typed SDK vs. types-only inside the app) and neither offered a unified one-call client. Generating both
from the one Conjure IR makes the published packages true peers that cannot drift from the server or
each other; the hand-written façades give the "single client for everything" ergonomics the per-service
generated clients lack. Using conjure-typescript (not the OpenAPI hop) keeps the TS SDK's shape a
faithful mirror of the Go SDK.

**Why not** (a) *OpenAPI → openapi-typescript for the published TS SDK*: it adds a lossy generator hop
(IR → OpenAPI → TS) so the TS surface wouldn't mirror the Go SDK; conjure-typescript consumes the same
IR the Go SDK does. (The OpenAPI artifact stays for docs/reference, and `openapi-typescript` is dropped
from `web`.) (b) *Keep the TS client inside `web/` only*: not distributable; external consumers need a
package. (c) *Hand-write the per-service clients*: violates D-Conjure (drift). (d) *A separate
gateway/proxy service for hermenea*: unnecessary — oikumenea already proxies `/hermenea/v1/*`
(D-Hermenea); the SDKs just expose it.

**Consequence.** New `clients/typescript/` npm package + `scripts/gen-ts-client.sh` +
`clients/typescript/scripts/rewrite-ir-packages.mjs`; a `-dump-ir` flag on `tools/ir2openapi`; the new
façade files `clients/go/client.go` and `clients/typescript/src/index.ts`. `web/` swaps its API layer onto
the SDK (drops `schema.d.ts` + `gen-api-types.sh` + `openapi-fetch`/`openapi-typescript`); its Docker
build context moves to the repo root so the `file:` SDK dep is in scope, and `outputFileTracingRoot` is
set so the standalone bundle includes it. No schema, contract, or server-behavior change — **additive /
tooling-only**. Publishing tags (npm / pkg.go.dev) are a follow-up; in-repo consumption verifies the
SDKs. Lands as **M27** ([milestones](../milestones.md)).

### D-UnitCodeLifecycle — Unit codes are optional, mutable, human-readable IDs (amends D-Code)

**Decision.** For **units** ([tenant](../modules/tenant.md)), the `code` is an **optional, mutable,
human-readable** business identifier — not the external machine handle. The **RID** (the UUIDv8
packing app/service/kind/type, F-014) is the stable handle external systems reference. Two changes
follow:

- **Codeless units.** A unit may be created with **no code** (`code` is nullable). A codeless unit is
  a **non-separate sub-unit** — a line battalion / platoon that is part of a parent unit but has no
  independent external designation (Ukr. *підрозділ* vs *окрема частина*). A unit *with* a code is a
  separate unit. Presence of a code *is* the signal — there is **no redundant `is_separate` flag**.
- **Editable codes.** A unit's code may be **set, corrected, or cleared** through a single **audited**
  operation (`PUT /units/{id}/code`, perm `unit.recode`). Each change appends a row to the append-only
  `tenant_unit_code_events` ledger (`old_code` → `new_code`, actor, reason, request_id). This is the
  `RecodeUnit` audited Action ([ontology-mapping](../ontology-mapping.md)). It is **not** part of the
  generic `PUT /units/{id}` patch.

Code **uniqueness** still holds **among active units that have a code** (the partial unique index is
`WHERE deleted_at IS NULL AND code IS NOT NULL` — many NULLs coexist). This amends **D-Code**
([decisions.md](decisions.md)) for the unit scope only; roles, ranks, graphs, locales, etc. keep their
`code TEXT NOT NULL UNIQUE`, immutable-by-convention.

**Why.** Two real needs from `todo.md` (items 2 & 3): (a) org graphs contain sub-units with no
external code, yet `code TEXT NOT NULL` forced a synthetic one; (b) operators must be able to fix a
code typo or a reorganization. Codes were never the machine contract — the RID is — so neither
optionality nor correction threatens external integrations that follow the RID guidance.

**Why not** (a) *Auto-generate a code when omitted*: pollutes the external namespace with synthetic
handles for units that legitimately have none; loses the *підрозділ*/*окрема частина* distinction.
(b) *Code aliases / keep the old code resolvable after a rename*: unnecessary once external callers
key on the RID; adds an alias table + cross-uniqueness burden for no real consumer. (c) *Fold `code`
into the plain `PUT /units/{id}` patch*: loses the dedicated audit Action and the isolated 409
conflict surface; muddies the common update path. (d) *A redundant `is_separate` boolean*: can
disagree with code presence — derive separateness from the code instead.

**Consequence.** `tenant_units.code` becomes nullable (the **existing** `20260601000003_tenant.sql`
migration is edited in place — dev DB reset + `atlas migrate hash` — per the design session; no new
file); the unique-index predicate gains `AND code IS NOT NULL`; a new append-only
`tenant_unit_code_events` table (RID slot `4,1,4`, `reject_mutation()` guard) lands in the same
migration. `CreateUnitRequest.code` becomes `optional<string>`; a new `setUnitCode` →
`PUT /units/{id}/code` endpoint + `Tenant:UnitCodeConflict` (409). `internal/tenant` domain `Unit.Code`
becomes a pointer; `SetUnitCode` application method (uniqueness check + event append, one txn) emits a
`UnitCodeChanged` domain event consumed by [audit](../modules/audit.md). Web unit editor allows an
empty code + an "Edit code" action. Consumes `todo.md` items 2 & 3. Lands as **M28**
([milestones](../milestones.md)).

---

### D-TenantOrganizations — Domains + Organizations: a multi-domain tenant over the unit graph (extends D-Graph/D-Graphs, amends D-Code, refines L-SingleDomain)

**Decision.** The [tenant](../modules/tenant.md) module gains a **two-tier model above the existing
unit DAG** so one deployment can host **every kind of hierarchical organization** — military,
government, company, university, church, public-org — side by side, instead of an implicit
single-domain forest of units. Three new pieces, all reusing the proven catalog / closure patterns;
the unit DAG, per-graph closure, public/shadow gate, lifecycle, and the PDP are **structurally
unchanged**.

- **Domain** (`tenant_domains`, RID `4,1,5`) — a **catalog Object** classifying *what kind* of
  organization a thing is (`military`/`government`/`company`/`university`/`church`/`public-org`, …).
  Stable `code` + translatable `name` (D-Code/D-i18n), `status active/retired`, `sort_order`.
  **Instance-admin-extensible, seeded at boot, never a `CHECK` enum** — exactly the D-Religion
  faith-vocabulary rule generalized to org kinds (no `if domain == …` branch anywhere).
- **Organization** (`tenant_organizations`, RID `4,1,6`) — a **first-class realm Object**: the
  concrete top-level entity a person joins (*US Army*, *Bundeswehr*, *Ukrainian Defence Forces*,
  *KhNU*). `code` (D-Code), translatable `name`, `domain_id` (NOT NULL → its kind), `visibility`
  public/shadow, `state active/suspended/archived`, soft-delete + lifecycle ledger
  (`tenant_org_lifecycle_events`, RID `4,1,8`). Many organizations may share a domain
  (US Army + Bundeswehr are both `military`, distinct orgs).
- **Unit kind** (`tenant_unit_kinds`, RID `4,1,7`) — a **domain-scoped catalog** that *replaces the
  free-text `tenant_units.unit_kind`* (military→{brigade, battalion, platoon}; university→{faculty,
  department, chair}). `domain_id` FK, `code`, translatable `name`, optional `attr_schema jsonb`
  (validates a unit's `metadata` per kind, the `document_types.attr_schema` pattern), `UNIQUE(domain_id, code)`.

**Units gain `org_id` (NOT NULL) + `domain_id` (NOT NULL) + `kind_id`.** A unit belongs to exactly
one organization. `domain_id` is **per-unit** (defaults to the org's domain on create) because
**mixed-domain trees are allowed** — a parent unit may own children of other domains (a government
ministry owning a state company; a holding owning a hospital + a university). Edges carry **no
same-domain constraint**.

**Graphs become per-organization (with a global escape hatch).** `tenant_graphs` gains a **nullable**
`org_id`: a **non-NULL** `org_id` is a per-org graph; **`org_id NULL` is an instance-global / cross-org
graph** — needed because the religion vertical (M22) seeds cross-faith taxonomy graphs (`tradition`,
`affiliation`, `canonical`) that span *all* religious bodies for ecumenical discovery, not one
organization. Two partial-unique code indexes ((`org_id, code`) WHERE `org_id IS NOT NULL`; (`code`)
WHERE `org_id IS NULL`); the single-default index keys on `COALESCE(org_id, sentinel)` (one default
per org, one among globals). Each organization gets its **own seeded `command` (default, locked
authority-bearing, undeletable) + `operational` graphs**, created in the **same transaction as the
organization** (replacing the former boot-time *global* command/operational seed). The "≥1 graph
always exists / `command` undeletable" invariant moves from instance-global to **per-org**.
Edges/closure/PDP isolate per organization — authority over US Army cannot leak into Bundeswehr —
using the **unchanged** per-graph closure machinery (the PDP is transparent to a graph's org scope);
a global graph's closure may legitimately span orgs (the ecumenical case).

**Domain & organization are directory attributes — never PDP or shadow-gate inputs** (exactly the
**D-Rank** "rank ≠ permission" and the `level`/`unit_kind` stance). Authority flows **only** through
role assignments cascading over graphs (now naturally org-scoped). There is no `domain ==`/`org ==`
branch in any authorization path.

**Person↔organization affiliation is derived, not stored.** A person's service history across
organizations and time (US Army 2010–2015, *then* Bundeswehr 2016–2020) is the union of their
**temporal unit memberships** ([membership](../modules/membership.md)) projected through each unit's
`org_id` — **no new affiliation table**. The person directory stays **instance-global** (the locked
D-PersonGlobal stance), which is precisely what lets one person belong to multiple organizations over
different periods.

**Listing is org-scoped.** `GET /units` **requires** an `org` query arg (`?domain`, `?unitKind`,
`?level` optional); browsing organizations by kind is `GET /organizations?domain=`. New catalog CRUD
mirrors the document/order/location shape: `/domains`, `/unit-kinds?domain=`, `/organizations`,
`PUT /organizations/{id}/state`; graphs are read per org (`GET /organizations/{id}/graphs`).

**Why.** The model was *already* domain-agnostic in name (a unit is "a brigade **or** a university
department") but had no first-class notion of *which concrete organization* a unit belongs to, and
no classification to scope by — org kind was smuggled into free-text `unit_kind`, and there was no way
to host US Army and Bundeswehr as distinct-but-comparable military organizations in one DB. A real
**Organization** object + a **Domain** catalog gives that, *and strengthens the Keycloak
differentiator* rather than breaking it: Keycloak realms are isolated silos; here organizations
**share one instance-global person directory** (so cross-org service over time works) and units can
still be linked across the graph — *organizations sharing one directory, not isolated realms*.
Per-org graphs reuse the per-graph closure for free; domain-scoped `unit_kinds` add real validation +
a per-kind `attr_schema` scaling hook that free text never could.

**Why not** (a) *Domain as a flat tag on units only, no Organization object*: cannot represent "US
Army vs Bundeswehr as distinct military orgs," makes org identity emergent/ambiguous in a multi-root
DAG, and gives nowhere to hang org lifecycle. (b) *Keycloak-style isolated realms (per-org person
silos)*: breaks the locked instance-global person directory and the very cross-org-over-time use case
that motivated this; the differentiator is explicitly *not* isolated realms. (c) *Homogeneous trees
(edges only same-domain)*: blocks real conglomerates (ministry→state-company); domain is genuinely
per-unit. (d) *Global graphs across all orgs*: one closure spanning every organization, authority
scoping leaning entirely on `target_unit` — per-org graphs isolate cleanly and the closure code is
unchanged. (e) *An explicit `person↔org` affiliation link*: duplicates the membership truth and drifts
from it; derive it. (f) *Absorb the company (Svc15) / religion (Svc16) verticals into tenant
`metadata`*: loses their dedicated typed objects — domains **classify**, the vertical modules keep
their domain-specific objects hung off organizations/units (consistent with D-Religion's
"reuse the tenant unit graph").

**L-SingleDomain is refined, not broken** (as D-Religion / D-RankSystems refined their locks). The
deployment may now hold **multiple domains**, but there is still **no org-type discriminator branched
on in code** — domain, organization, and `unit_kind` remain descriptive catalog rows feeding
listing/validation/UI, never a code switch and never a PDP input.

**Consequence.** `tenant_domains` / `tenant_unit_kinds` / `tenant_organizations` /
`tenant_org_lifecycle_events` tables + `org_id`/`domain_id`/`kind_id` on `tenant_units` + `org_id` on
`tenant_graphs` land by **editing `20260601000003_tenant.sql` in place** (dev DB reset +
`atlas migrate hash`; the tenant tables carry no production data — same path D-UnitCodeLifecycle and
D-Location took), with matching `platform_rid_types` rows + `pkg/rid` registry entries + new Object
rows in [ontology-mapping](../ontology-mapping.md). New Conjure `Domain`/`UnitKind`/`Organization`
types + catalog/org endpoints + `org` required on `listUnits` + `orgId` on `Graph`; `internal/tenant`
gains domain/org/unit-kind services, `CreateOrganization` seeds per-org graphs in-txn, `module.go`
seeds the domain + unit-kind catalogs at boot and drops the global graph seed; new permissions
`domain.{read,manage}`, `unit-kind.{read,manage}`, `organization.{read,create,update,lifecycle}`
([authorization](../modules/authorization.md)). **Amends D-Code** (org/domain/unit-kind codes are
`NOT NULL UNIQUE`, immutable-by-convention — the unit carve-out is unchanged) and **D-Graph/D-Graphs**
(graphs are per-org; the seed/undeletable invariant is per-org). [membership](../modules/membership.md)
is unaffected structurally (org history is a projection). Lands as **M40** ([milestones](../milestones.md)),
on the built M3/M5/M7. Additive / expand-only against external callers (RID-keyed). Distinct from
**D-ExternalOrgs** (M30), which registers *external* reference organizations a person is affiliated
with (parties, foreign militaries) — *this* decision is the deploying org's own structural tenant.

---

### D-UnifiedOrgGraph — Verticals reuse the tenant org-graph; per-vertical sidecars (reverses D-Education, amends D-Companies/D-Religion, extends D-TenantOrganizations)

**Decision.** **Every hierarchical organization in the deployment is a `tenant_organizations` + `tenant_units`
graph node** — the verticals stop reinventing structure. After M40 made tenant a multi-domain org graph,
`education` and `company` still each carried a *parallel* hierarchy (`education_units` + its own
`education_unit_closure`; `company_companies` as a separate legal-entity table), while `religion` already
reused tenant units. M41 makes all three consistent under **one pattern**:

> **tenant owns the org-graph** (organizations + units + per-graph closure + graphs); **each vertical keeps a
> `<vertical>_org_profiles` *sidecar*** — keyed by the tenant org/unit RID, **no own RID** (the
> `religion_org_profiles` template) — for its domain-specific attributes, plus its own catalogs and links
> re-pointed at tenant ids.

- **Education** (reverses D-Education's "dedicated structure tree, *not* reused tenant units"):
  `education_institutions` → `tenant_organizations` (domain=`university`); `education_units` → `tenant_units`;
  **`education_unit_closure` is deleted — reuse `tenant_unit_closure`** (the duplicate closure engine goes);
  `education_unit_kinds` → `tenant_unit_kinds`. The institution's attributes move to `education_org_profiles`
  (PK = org RID; `institution_kind_id`, country, founded/closed); `education_institution_kinds` /
  `education_degree_levels` stay as vertical catalogs; every dependent FK (buildings/groups/positions/
  enrollments + the M21 reference layer + 6 person links) re-points to the org/unit RIDs. The single-parent
  tree becomes a DAG (a strict superset).
- **Company** (amends D-Companies): `company_companies` → `tenant_organizations` (domain=`company`) +
  `company_org_profiles` (legal_form, ownership_category, country, founded/dissolved). **Corporate groups
  (parent→subsidiary) remain the ownership/affiliation graph** between company-orgs — `shareholdings`
  (fractional, multi-owner), `branch_of`, `successions` — **not** containment edges (equity ≠ containment;
  a strict tree can't hold a 60/40 split). A company's *internal* divisions may be `tenant_units` within it
  (additive). All company links re-point `company_id` → org RID (incl. the cross-module
  `vehicle_brand_manufacturers.company_id`).
- **Religion** (formalizes D-Religion): religious bodies already are tenant units + `religion_org_profiles`;
  M41 gives religion a first-class **church-domain root-org** path (not just test fixtures). The global
  `canonical`/`tradition`/`affiliation` graphs stay (the church-domain exception).

**`pdp_scoped` domain flag (extends D-TenantOrganizations).** `tenant_domains` gains
`pdp_scoped boolean NOT NULL DEFAULT true`, denormalized onto `tenant_units` (derived in SQL at insert).
**Operational** domains (military/government/public-org/**church**) are `true`: reach-RLS applies and
`CreateOrganization` auto-seeds per-org `command`+`operational` graphs. **Reference** domains
(university/company) are `false`: **instance-global** — public reads, app-permission writes, **exempt from
the reach-RLS predicate** (`tenant_units_reach` gains `NOT pdp_scoped OR …`), and **no auto graph seed**
(avoids 2 graph rows × tens of thousands of bulk-imported orgs). Verticals build structure under a reference
org via the idempotent `EnsureGraph(org, code, …)` tenant method (first graph becomes the org's default).

**Why.** This is what M40 was *for*: a domain classifies an org kind, so a university is a university-domain
org and a faculty is a unit. Education had literally reimplemented the tenant DAG + closure; collapsing it
removes a whole maintenance engine. Religion already proved the reuse pattern. One backbone means one
closure, one visibility/lifecycle/i18n/code machinery, uniform cross-domain queries, and graph-based imports
for reference data. The reference/operational split (`pdp_scoped`) preserves the one thing D-Education was
right to protect — external reference orgs must not inherit the deploying org's PDP/RLS scoping — now as a
**domain attribute** rather than a separate table.

**Why not** (a) *Keep the parallel hierarchies*: duplicated closure + inconsistent with religion. (b)
*Anchor-table bridge* (keep `education_institutions`/`company_companies` as thin tables): retains the
duplication the change removes. (c) *Company as a unit / org-to-org containment tree for subsidiaries*:
ownership is fractional and multi-parent — a containment tree can't represent it; the ownership graph
(with a future computed-UBO closure, DS-47) is the right model. (d) *Treat reference orgs like operational
ones*: bulk imports balloon graph rows and fight the reach-RLS WITH CHECK.

**Consequence.** Tenant: `pdp_scoped` on domains+units, RLS exemption, lazy graph seed + `EnsureGraph`
(built, M41 **Phase 0 — done+verified**). Education/company structural tables are **redefined in place**
(pre-release, no prod data → rebuild): institutions/companies become tenant orgs; `education_units`→tenant_units;
`education_unit_closure` dropped; `<vertical>_org_profiles` sidecars added; all FKs re-pointed; the education
closure-maintenance code is deleted (reuse tenant's). The education `institution`(14,1,1)/
`education_unit`(14,1,2)/`education_unit_parent_of`(14,2,1) and company `company`(15,1,1) RID types are
removed from `pkg/rid` + `platform_rid_types` + [ontology-mapping](../ontology-mapping.md) (those objects are
now tenant org/unit). **Phase 1 (education) + Phase 2 (company) — done+verified**: both verticals collapsed
onto tenant orgs/units + sidecars, all suites green; company has no per-org graph (no unit tree).
**Phase 3 (religion) — done+verified**: a first-class `createRootOrg` (`POST /religion-orgs`) builds a
`church`-domain org + root unit + profile (the church-domain exception: `pdp_scoped=true` but global
canonical/tradition/affiliation graphs). Reference orgs import via the tenant org/unit path + a sidecar upsert
handler (`internal/dataimport`) — **deferred** (additive; no hermenea connector yet). Lands as **M41**
([milestones](../milestones.md)), on M40 + M20 + M21 + M22–25.
**Reverses** D-Education's "not reused"; **amends** D-Companies (entity→org) and D-Graph/D-Graphs (reference
orgs need no graphs). Additive against external callers only by RID (the RIDs change service 14/15→4, but
pre-release has no external consumers).

---

### D-Color — Structural color as a per-domain platform catalog referenced by hard FK (extends D-Code, D-i18n, D-Ontology; amends D-Vehicles, D-PhysicalIdentity)

**Decision.** Replace the scattered free-text color fields (`vehicle_vehicles.color`,
`person_physical_descriptions.eye_color` / `hair_color`) with a single operator-managed reference
catalog, `platform_colors`, referenced by **hard FK**. Color is platform's first RID-bearing Object
(service 1, object type `(1,1,1)` = `color`), sitting next to `platform_legal_basis_kinds` on the
`PlatformCatalogService` (reachable via the `api.platformCatalog` façade).

- **Per-domain palettes (not one shared list).** A `domain` discriminator (`eye | hair | vehicle`,
  TEXT+CHECK enum) with `UNIQUE(domain, code)`. The vocabularies are genuinely different (eye colors
  are a near-closed biological set; vehicle colors an open RAL/manufacturer space), so each palette is
  independent and self-contained, reads are a trivial `WHERE domain = …`, and in-place creation from a
  picker is unambiguous. Adding a domain is a code change (each domain corresponds to a call-site), so a
  CHECK enum — not a `color_domains` catalog — is correct.
- **`code` + i18n `name` + nullable `hex`.** Per D-Code, a stable locale-agnostic `code`; per D-i18n,
  the `name` is returned as a `locale → text` map (assembled via the localization store, entity type
  `color`, **keyed by the color RID** because `code` is unique only per-domain). `hex` is a **nullable**
  representative swatch — biological eye/hair colors are categories, not precise hex; vehicle colors
  carry one.
- **Hard FK + app-layer palette check.** The three columns become `color_id` / `eye_color_id` /
  `hair_color_id` (`uuid REFERENCES platform_colors(id) ON DELETE RESTRICT`). A single-column FK cannot
  prove the referenced color is in the *right* palette, so the consuming application services
  (person `UpsertPhysicalDescription`, vehicle `Create`/`UpdateVehicle`) validate the color's `domain`
  via a cross-module `ColorLookup` query interface (the platform color service), returning
  `ErrColorMismatch` otherwise — mirroring the existing "validate code against catalog before use"
  pattern.
- **Permissions.** Reads ride a broadly-granted `color.read` (any reader populates a picker); writes the
  instance-plane `color.manage` (added to `instanceScope`). Writes are audited (`color.upsert`, a
  platform Action RID).
- **In-place creation.** The web `ColorPicker` (per-domain typeahead + swatch) offers a "Create '…'"
  affordance that upserts `{domain, code, name, hex?}` and selects it, so operators extend palettes
  without leaving the form.

**Why.** Free-text color drifts ("brown" vs "Brown" vs "коричневий" vs "dark brown"), cannot be
localized, and cannot render a swatch. A reference catalog gives consistency, i18n, swatches, and
referential integrity, while staying cheap (~30 rows).

**Consequence.** Lands in migration `0030_person_physical_identity` (originally shipped as a standalone
`0031_color`; the three uncommitted M31/M42/M43 migrations were later **squashed into `0030`**, which
creates `platform_colors` before the person/vehicle tables so `person_physical_descriptions` is created
with `eye_color_id`/`hair_color_id` FK columns directly — the vehicle table, from committed `0027`, still
takes an `ADD color_id` + `DROP color` ALTER). `D-Code`'s "every structural entity has a code" and
`D-i18n`'s "all translations in every response" both apply; the hard FK is the structural payoff over the
prior advisory free text. Built & verified at **M42** (see [milestones](../milestones.md)).

---

## Person-intelligence / OSINT-enrichment cluster (M29–M37)

These nine decisions promote the [`draft_superbrain_schema.md`](../draft_superbrain_schema.md)
per-field verdicts into a buildable, foundation-first sequence. They share three rules carried from
that draft: **declared ≠ inferred** (never merge an inferred value into a first-party assertion);
**every overlay carries `source`+`confidence`** (the D-PersonSocialChannels attribution shape); and
**special-category data is gated** (5-tier `pii:*` + envelope [D-SpecialPII] + an explicit
`legal_basis` + mandatory audit). M29 builds that shared substrate; M30–M37 each ride it as a thin
vertical slice.

### D-OverlayFoundation — Provisional entities, attribution & `legal_basis` substrate (extends D-Ontology, D-PersonSocialChannels, D-PIITiers)

**Decision.** Establish the cross-cutting machinery every later overlay milestone needs, so each is a
thin slice rather than re-inventing the foundation:

- **Provisional persons + manual resolution.** Relax the in-directory-only rule
  ([D-PersonGlobal](decisions.md)): `person_persons.status` gains **`provisional`** — a minimal-PII
  stub so every relationship/overlay edge points at a real node (an unresolved external person, an
  emergency contact, a wallet attribution target). Resolution is **manual**: a `MergePerson` audited
  action promotes/merges a provisional into a canonical person carrying a `confidence`, re-homing its
  edges in one transaction. **No automatic candidate matching** (fuzzy dedup is a parked seam — it is
  not built here).
- **Attribution convention.** A reusable column-set — `source ∈ {self_declared, operator_verified,
  imported}`, `confidence ∈ {confirmed, probable, possible}`, optional `as_of` — formalized from
  D-PersonSocialChannels for use on **every** overlay/attribution row. Inferred values are stored in a
  **separate** column-space from declared ones and **never merged**.
- **`legal_basis` (structured).** A migration-seeded **`platform_legal_basis_kinds`** catalog
  (GDPR Art. 6 lawful bases — consent / contract / legal_obligation / vital_interests / public_task /
  legitimate_interest — plus the Art. 9 special-category conditions), referenced by FK from every
  gated/special-category overlay row, with an optional free-text justification note. Adding a
  special-category overlay implies new audited Actions; the `legal_basis` FK is **NOT NULL** on every
  `pii:special` store.

**Why.** The draft's cross-cutting principles 1–3 are prerequisites, not features: party / government /
foreign-military / lobbying edges (M33), wallet attribution (M35), and emergency contacts all need a
node to point at; every overlay needs uniform provenance to be query-weightable; special-category
stores need an enforceable lawful basis, not prose. Building these once keeps M30–M37 small.

**Why not** (a) *Full entity-resolution tooling now* (candidate matching, blind-index dedup, merge
UI): a milestone of its own — the manual promote/merge covers the "edges must resolve" need; fuzzy
dedup is parked. (b) *Free-text `legal_basis`*: not enforceable or queryable; a regulator-facing field
must be structured. (c) *A separate `provisional_persons` table*: forks the person aggregate and
duplicates its PII discipline/purge — a `status` value reuses the existing lifecycle + erasure path.
(d) *Merge inferred into declared*: destroys the provenance distinction the whole cluster depends on.

**Consequence.** `person_persons.status` adds `provisional`; a `MergePerson` action + `PersonMerged`
event (edges re-homed, audited). A `platform_legal_basis_kinds` seeded catalog
([platform](../modules/platform.md)). The attribution column-set is documented as a convention in
[conventions.md](conventions.md) and reused verbatim by M30–M37. New person link/action RIDs allocated
on build. Lands as **M29** ([milestones](../milestones.md)).

### D-ExternalOrgs — A registry of external organizations (party / government / military / NGO / registrant) (extends D-Ontology)

**Decision.** External organizations a person is tied to — political parties, government bodies,
foreign military formations, lobbying registrants/clients, NGOs — live in a **dedicated
`external_organizations` registry** (new module, **RID service 18**), **not** in the operator's own
`tenant_units` and **not** in the M21 `company_companies` legal-entity registry. Each row is
catalog-typed (`external_org_kinds`: party | government_body | military | ngo | registrant | other),
carries the D-OverlayFoundation **provisional/resolved status + attribution**, an optional
`country` → `geo_countries`, optional `wikidata_id`, and a translatable `name`. It is a hermenea import
target (Wikidata / public registries).

**Why.** The user-chosen model: external orgs are conceptually distinct from both the deploying org's
own unit DAG (which is authority-bearing through the PDP) and from for-profit legal entities (M21).
Mixing them into `tenant_units` would pollute the PDP graph with non-authoritative nodes; forcing them
into `company_companies` mis-types a political party or a government ministry. A single faith-/sector-
agnostic registry with a `kind` catalog keeps every M33 affiliation edge pointing at one node-space.

**Why not** (a) *Reuse `tenant_units` (provisional)*: a party/ministry is not part of the operator's
command graph; closure/PDP machinery would treat them as authority-bearing. (b) *Reuse
`company_companies`*: legal-form/ownership-graph semantics don't fit parties/governments. (c) *Per-type
tables (parties, gov bodies, …)*: the kinds share identity/provenance/country shape — a catalog `kind`
is the D-Code-consistent choice, mirroring `religion_org_kinds`.

**Consequence.** A new `external-organizations` module + `api/external-organizations.conjure.yml`;
RID service **18** (`external_organization` object, `external_org_kind` catalog). M33 person↔org edges
FK this registry (or, for corporate ties, M21 `company_companies`). Lands as **M30**
([milestones](../milestones.md)).

### D-PhysicalIdentity — Aliases, physical description & declared ethnicity (extends D-PersonNamesCLDR, D-SpecialPII)

**Decision.** Add first-party physical-identity attributes (draft macro-category 1):

- **Aliases** — fold AKA / former-legal / maiden / pseudonym / cover names into the existing
  `person_name_variants` via a `variant_kind` discriminator
  (`transliteration | aka | former_legal | maiden | pseudonym | cover`); alias rows may carry
  `source`+`confidence`. **No new table.**
- **Physical description** — `person_physical_descriptions` (effective-dated: `height_cm`, `weight_kg`,
  `eye_color`, `hair_color`, `build`, **`blood_type`**; `pii:basic`) + `person_distinguishing_marks`
  (`kind ∈ tattoo|scar|piercing|birthmark`, `body_location`, `description`) — marks are **`pii:special`
  ceiling** (a tattoo can reveal Art. 9 data).
- **Ethnicity** — **self-declared only**, catalog-typed (`person_ethnicity_types`, open catalog + i18n
  name), stored as a reified `pii:special` link, **envelope-encrypted** (D-SpecialPII) + `legal_basis`
  + audit. **No inferred storage.** Biometric data (1.5) is **excluded**.

**Why.** These are authoritative, operator-asserted directory attributes the draft tags `DEVELOP`;
they fill real gaps (no general alias today; no physique/ethnicity). Reusing `person_name_variants` for
aliases avoids a redundant table; reusing the M24 special envelope for ethnicity avoids new crypto.

**Why not** (a) *A separate aliases table*: `person_name_variants` already is the per-person alt-name
store — a `variant_kind` is the minimal change. (b) *Store ethnicity as plain text / inferred*:
Art. 9 — must be declared, encrypted, and lawful-basis-gated. (c) *Biometrics*: highest-risk; excluded
pending legal review (token/reference-only if ever).

**Consequence.** `person_name_variants.variant_kind` (+ `source`/`confidence`) column; new
`person_physical_descriptions`, `person_distinguishing_marks`, `person_ethnicity_types` + the encrypted
`person_ethnicities` link; all erased/crypto-erased on purge. New person RIDs allocated on build. Lands
as **M31** ([milestones](../milestones.md)).

**Built (M31, migration `0030_person_physical_identity`).** RIDs `6,1,11` physical_description, `6,1,12`
distinguishing_mark, `6,1,13` ethnicity_type, `6,2,9` `has_ethnicity`. Refinement landed during build:
the declared ethnicity is modelled as an **envelope-encrypted value** holding the catalog *code* (blind-
indexed for equality search), **not** a plaintext FK to `person_ethnicity_types` — so the Art. 9 datum
(which ethnicity) is never stored in plaintext; the catalog is the controlled vocabulary the application
validates against before sealing. The envelope columns are nullable so purge crypto-erases them (row kept
as a tombstone), while physical descriptions/marks are hard-deleted on purge. The person application
service now holds the `pkg/crypto` cipher; `cmd/oikumenea/main.go` builds it ahead of `person.Register`.
The one-transliteration-per-locale uniqueness became partial (`WHERE variant_kind='transliteration'`) so
aliases coexist. Reads/writes gate on `person.read`/`person.update` (no new permission).

**Amendment — ethnicity taxonomy & ethnolinguistic links (M43, migration `0030_person_physical_identity`;
originally `0032_ethnicity_catalog`, later squashed into `0030` — `person_ethnicity_types` is created with
`parent_id`/`wikidata_id`/provenance inline).** The
flat, seeded-empty `person_ethnicity_types` catalog is promoted to a **hierarchical** reference catalog
modeled on the M18 language registry, **without touching the encrypted person↔ethnicity link**:

- **Hierarchy.** `parent_id` self-FK (like `language_languoids.parent_id`) + a rebuilt transitive closure
  `person_ethnicity_type_closure` (copy of `language_languoid_closure`); `wikidata_id` anchor + import
  provenance columns. Roots/children/search are served by a `listEthnicityTypes(topLevel|parent|query)`
  filter with a computed `has_children`, mirroring `listLanguages`.
- **Group-level ethnolinguistic + homeland links.** M:N `person_ethnicity_type_languages`
  (→ `language_languoids`) and `person_ethnicity_type_countries` (→ `geo_countries`) — bare associations
  (composite PK, **no RID**, like `language_languoid_countries`). These are reference metadata **about the
  group** and are **never inferred onto a person**: `person_ethnicities` (a person's encrypted, declared
  ethnicity) and `person_languages` (a person's `SPEAKS`, M18) stay independent first-party declarations.
- **Opt-in import — CIA World Factbook (public domain), fetched + parsed LIVE at runtime.** A new
  `ethnicity-scheme` hermenea object-type (idempotent, closure-rebuilding — a clone of `language-scheme`).
  The **source is the CIA World Factbook** via the `factbook/factbook.json` GitHub mirror, ingested
  **entirely at import time in Go — no committed preset, no Python**: a dedicated `factbook`
  **StreamingConnector** enumerates the ~260 country files with one git-tree API call and stages them; the
  `factbookethnicities` **PagedMapper** parses each country's "Ethnic groups" free-text, derives the
  country's ISO code from its Internet ccTLD, and dedups group→countries across all files. This yields a
  **flat** catalog of ethnic-group names, each linked to the countries where the Factbook lists it. The
  Factbook has **no ethnicity hierarchy and no language linkage**, so this source populates the flat catalog
  + country ties only; the `parent`/closure and language-M:N machinery above stays in the schema but is
  unpopulated by this source (a future hierarchical/linguistic source could fill them). The `factbook`
  source (`hermenea-install.yml`, locator `owner/repo@ref`) ships **`enabled: false`** — the **default
  catalog stays empty** (ethnicity is contentious; the deployment owner loads it on purpose). Unresolved
  country keys are silently dropped (resilient). No new RID types (the catalog keeps `6,1,13`).
  *(Wikidata was evaluated as the richer, hierarchical + language-linked source but its endpoints — WDQS,
  the Action API, EntityData — 403 datacenter/CI IPs per Wikimedia bot policy T400119, so it can't run in
  CI; the public-domain Factbook over GitHub is reachable at runtime from datacenters. The runtime path is
  preferred over a committed preset because the Factbook is reachable — hermenea is a runtime ingestion
  service by design.)*

**Why not** conflate ethnicity with language at the person level: a person's ethnicity ≠ their languages;
any group↔language tie is a *group*-level association only, never inferred onto a person (declared ≠ inferred).

### D-PersonAddresses — Structured, effective-dated person addresses over Location (extends D-Location)

**Decision.** Replace country-level `person_residences` granularity with structured address history:
`person_addresses` — a reified link `person` → `location_locations` (M19) with `role ∈
{home, work, mailing, other}`, `valid_from`/`valid_to`, `is_primary`, a `privacy_seeking` flag (a
mailing address ≠ home is itself a signal), and `source`+`confidence`; `pii:contact`, purge-erased. A
work address may be **derived** from the person's unit location. `person_residences` (country +
free-text region) is retained for citizenship/legal-residence semantics; addresses are the precise,
PostGIS-backed overlay.

**Why.** The draft (2.1–2.3) wants real, effective-dated, geocoded addresses; M19 Location already
provides the PostGIS point + MGRS + structured postal fields — addresses are a thin link over it.

**Why not** (a) *Extend `person_residences` in place*: conflates legal residence (country-grade,
citizenship-adjacent) with a precise mailing/home/work point — keep both. (b) *Free-text address*:
loses geospatial query + dedup against shared `location_locations`. (c) *GPS movement history (2.4)*:
**excluded** — movement traces are out of fit for an authoritative directory.

**Consequence.** New `person_addresses` link → `location_locations`; holder-scoped reads, audited
writes, purge erasure. New person link RID on build. Lands as **M32**
([milestones](../milestones.md)).

**Built (M32).** Migration `0031_person_addresses` — `person_addresses` is the reified link
`link__lives_at` (RID `6,2,10`, `pkg/rid` + `platform_rid_types`) `Person` → `location_locations`
(M19, `ON DELETE RESTRICT`): `role ∈ {home,work,mailing,other}`, `valid_from`/`valid_to`,
`is_primary` (one active primary per person — a partial-unique index; the app demotes the prior
primary in-tx), `privacy_seeking`, and the attribution column-set (`source` NOT NULL default
`operator_verified` + `confidence`). `pii:contact` → **hard-deleted** on purge (added to the person
`Purge` erasure list, beside `person_residences`; `person_residences` itself is unchanged). Location
existence is verified before write via a new **`domain.LocationLookup`** port on the person module
(mirrors `ColorLookup`), satisfied by a `LocationExists` method on the geo/location service and
late-bound `personSvc.SetLocationLookup(geoSvc)` in `cmd/oikumenea/main.go` (person is built before
geo, so the seam is late-bound like `SetMembershipReader`); an unknown location normalizes to
`Person:PersonInvalid`. Audited `PersonService` endpoints
(`GET|PUT /persons/{id}/addresses`, `DELETE …/{addressId}`) on `person.read`/`person.update`; Go+TS
SDKs regenerated; a `/persons/[id]` **Addresses** console card picks the location via the shared
`SearchSelect` typeahead.

### D-InstitutionalTies — Person↔organization affiliation edges (party / government / lobbying / foreign-military / references) (extends D-Ontology, D-OverlayFoundation, D-ExternalOrgs)

**Decision.** Model the draft's macro-category 7 as **per-type reified person↔org links**, the org
side being an M30 `external_organizations` row, an M21 `company_companies` row, or a `tenant_unit`:

- `person_party_memberships` (party, role, dates) — **`pii:special`** (Art. 9, political opinion),
  envelope + `legal_basis`.
- `person_government_positions` (title, body, country, level, role_type, dates, **`pep_trigger`**
  auto-true, persists post-office) — `pii:basic`; **feeds the M34 PEP check**.
- `person_lobbying_relationships` (registrant, client, legislative_body, issues[], filing_id,
  source_url) — `pii:basic`.
- **Foreign / historical military service** — reuse [membership](../modules/membership.md) against
  **external_organizations** military stubs + rank, with extra link attributes
  (`units[]`, `deployments[]`, `discharge_type`, `clearance_level` — the latter two `pii:sensitive`).
- `person_external_references` (`kind ∈ wikipedia|news|registry|…`, `url`, `external_id`,
  `categories[]`, `last_checked`, `disputed`) — mirrors `person_social_accounts`; a hermenea target.
- **Emergency contacts** — add an `emergency` entry to `person_relation_types` (M14); **no new
  entity**.

Every edge carries `source`+`confidence`; the inferred political-leaning **spectrum** is **not** here —
it is a separate M35 overlay and is never merged with declared party membership.

**Why.** These are person↔org affiliation edges where the org is usually external — exactly the
node-space D-ExternalOrgs + M21 provide. Per-type links (not one generic blob) keep query semantics and
tier discipline clean, mirroring the M14 relationship pattern.

**Why not** (a) *One generic "affiliation" table*: loses per-type tier (`special` party vs `basic`
gov) + attributes. (b) *Store declared + inferred politics together*: forbidden (declared ≠ inferred).
(c) *A new entity for foreign military units*: external_organizations `kind=military` already covers it
+ reuses membership/rank.

**Consequence.** New person link tables + the `external_references` object; PEP derivation reads
`person_government_positions`. New person RIDs on build. Lands as **M33**
([milestones](../milestones.md)).

### D-Watchlists — Live-lookup sanctions/PEP/Interpol via hermenea + a regulatory-sanctions overlay (extends D-Hermenea)

**Decision.** Watchlist screening is **never stored statically**. Two surfaces:

- **Live-lookup (sanctions / PEP / INTERPOL).** A check executes **through hermenea** (the
  external-coupling companion, D-Hermenea): oikumenea calls hermenea, **hermenea owns the outbound
  egress** to OFAC / EU / UN / INTERPOL-Red-Notice APIs and a **short-TTL cache (≤24 h)**; only
  **per-person match metadata** flows back and is persisted (`on_list`, `lists[]`, `program`,
  `match_score`, `last_checked`, `next_check_due`) — never the lists themselves. PEP status is derived
  from M33 `person_government_positions`.
- **Regulatory-sanctions overlay.** `person_regulatory_sanctions` (regulator, action_type, amount,
  status, date, source_url, `source`+`confidence`) — structured, API-ingestible (a hermenea import
  target), tied to a licensed professional role; `pii:sensitive`, gated.

The original **Interpol API** idea (interpol.api.bund.dev) — `todo.md` item 1, now folded into the
[draft_superbrain_schema.md](../draft_superbrain_schema.md) §6.5 live-lookup verdict — is the first
live-lookup connector built here.

**Why.** Sanctions/PEP/Red-Notice lists are volatile and licence-encumbered; storing them statically is
stale and legally fraught. Routing egress through hermenea keeps all external coupling in the companion
service (consistent with D-Hermenea) and the PDP core free of outbound internet calls. Match-metadata
is the only persistable, query-useful residue.

**Why not** (a) *oikumenea calls the APIs directly*: adds outbound egress to the operator-owned PDP
core; splits external-coupling responsibility. (b) *Ingest the full lists into a table*: stale +
licence risk + huge. (c) *Store full match details*: only metadata is needed for a flag; full hit data
re-fetched live.

**Consequence.** A hermenea watchlist connector + cache; oikumenea `person_watchlist_matches` (metadata
only) + `person_regulatory_sanctions` overlay; a `CheckWatchlists` audited action. New person RIDs on
build. Criminal/arrest/court records (6.1–6.3) are **deferred** (M38, own session). Lands as **M34**
([milestones](../milestones.md)).

### D-PersonOverlays — Financial, behavioral & psychological overlays (extends D-OverlayFoundation, D-SpecialPII)

**Decision.** Three gated overlays (draft macro-categories 4 + 5), each carrying attribution:

- **Crypto wallets** — `person_crypto_wallets` (`address`, `chain`, `attribution_method`,
  `first_seen`/`last_seen`, `balance_usd_approx`); `pii:sensitive`; synergy with M34 screening.
- **Personality** — `person_personality` (MBTI / Big-Five / DISC / Enneagram), **declared survey or
  formal HR assessment only**, `pii:sensitive` + `source`. **No NLP-from-text inference.**
- **Inferred political leaning** — a **separate** `pii:special` overlay
  (`spectrum ∈ [-1,1]`, `inference_sources`, `confidence`), envelope + `legal_basis` + audit, **never
  merged** with the declared M33 party membership.

Compensation/payroll is **out of scope** here — a separate operational-HR module (M39, deferred).

**Why.** These are provenance-tagged overlays the draft tags `OVERLAY`/`DEVELOP`; keeping inferred
politics gated, separate, and never-merged honours the declared-vs-inferred rule under the strictest
tier.

**Why not** (a) *Infer personality/politics from text*: forbidden — declared/assessment only for
personality; inferred politics is isolated and never blended. (b) *Fold compensation in*: operational
HR (the org as payer), not dossier scope.

**Consequence.** New person overlay tables (crypto/personality `pii:sensitive`; inferred-leaning
`pii:special` encrypted); audited writes, purge/crypto-erase. New person RIDs on build. Lands as
**M35** ([milestones](../milestones.md)).

### D-HealthVulnerability — Category-level health & vulnerability records (`pii:special`) (extends D-SpecialPII)

**Decision.** Add the draft's macro-category 8 under the **strictest** gate:

- `person_health_records` — typed `kind ∈ {hospitalization, mental_health, disability}`,
  **category-level only (no diagnosis)**, `is_public_record`, `source`+`confidence`; `pii:special` +
  envelope (D-SpecialPII) + app-layer **need-to-know** + **full audit** + `legal_basis`. **Never
  inferred.**
- `person_insurance` — `type ∈ {health, life, disability, ltc}`, provider, `employer_sponsored`,
  dates; `pii:sensitive`, gated.

**Why.** Health/vulnerability is the highest-sensitivity Art. 9 cluster; building it last lets it reuse
the proven M24 special-PII envelope + the M29 `legal_basis`/audit substrate. Category-level-only
storage minimizes risk while keeping the field queryable.

**Why not** (a) *Store diagnoses / inferred health*: forbidden — category-level, declared/public-record
only. (b) *A plain `pii:sensitive` health field*: special category demands the envelope + need-to-know
+ full audit.

**Consequence.** New `person_health_records` (encrypted) + `person_insurance`; need-to-know read gate,
full audit, crypto-erase on purge. New person RIDs on build. Lands as **M36**
([milestones](../milestones.md)).

### D-LoginSecurityLog — A first-party login/IP security log on the federation seam (extends L-AuthzOnly)

**Decision.** Record a **first-party login security log** — `account_login_events` on the
[identity-federation](../modules/identity-federation.md) accounts seam: `account_id`, `ip`,
`occurred_at`, `context ∈ {login, activity, registration}`, `resolved_country`, `resolved_isp`,
`is_vpn`, `is_tor`, `user_agent`. Fed from what the **OIDC/JWKS validation middleware already sees** on
the `/whoami` / token-validation path — **not** OSINT breach enrichment, and **not** stored credentials
(L-AuthzOnly holds: the service still issues no tokens). `pii:contact`, retention-bounded, purge-erased.

**Why.** The draft (2.5) wants IP/login history as first-party security telemetry. The service already
validates inbound IdP tokens, so it sees enough to log the security-relevant context without becoming
an auth provider. Split into its **own milestone** because, with delegated auth, its value and shape
are independent of the address work (M32).

**Why not** (a) *Bundle into M32 addresses*: different seam (federation, not person), different lifetime
(security log vs directory attribute). (b) *Treat as OSINT enrichment*: it is first-party login
telemetry, gated like any contact-tier data. (c) *Skip it (delegated auth)*: the validation path still
yields useful, lawful security signal.

**Consequence.** New `account_login_events` on the account service (RID `account` object); IdP
middleware emits an event per validated request; retention sweep + purge erasure. New account RID on
build. Lands as **M37** ([milestones](../milestones.md)).

**As built (M37, migration `0043`, 2026-07-21).** `account_login_events` is account Object `9,1,4`. Two
build decisions (with the user): **(1) bounded, not a firehose** — the middleware DE-DUPES to one row
per `(account, context, ip)` per window (`login-security.dedup-window-seconds`, default 1h): a repeat
within the window bumps `last_seen_at` + `occurrence_count`, else a new row. So the "event per validated
request" is collapsed at the source; retention is a plain `DELETE` sweep (`delete_login_events_before`,
`login-security.retention-days`, 0 = forever; best-effort boot sweep, a scheduled enforcer left as a
seam like D-AuditRetention). **(2) client IP through the facade** — see the D-HeadlessTopology amendment
below. The emit is **best-effort and off the request's critical path** (a detached goroutine; a failure
never fails the request); **service principals are excluded** (human accounts only); `registration`
marks the JIT link-on-match, `login` a validated request. IP-intelligence (`resolved_country`/`_isp`/
`is_vpn`/`is_tor`) is a **deferred `IPIntel` seam** (no-op default → NULL; a future hermenea connector).
Read on the instance-scope `account.security-log.read` (`GET /accounts/{id}/login-events`, admin-only);
purge-erased via `SubscribePersonPurge`. `pkg/crypto` not involved (no envelope encryption — the log is
`pii:contact`, not `pii:sensitive`).

### D-Finance — Bank accounts & payment cards, banks as company orgs (extends D-Ontology, D-PersonalCodes, D-UnifiedOrgGraph)

**Decision.** A new **`finance`** module (**RID service 19**, tables `finance_*`) holds **bank
accounts and payment cards** as authoritative, operator-asserted **directory** data — a person (or
company) holds bank accounts; each account is held at a **bank**, and cards hang off an account. A
**bank is not a new entity**: it is an existing **`company`-domain `tenant_organization`** (M21 /
M41 / D-UnifiedOrgGraph) that an account references as its holding institution — optionally flagged
via an existing `company_industry_classes` (NACE/KVED banking) assignment, never a bespoke bank table.

- **Reference catalogs** (instance-scope, `code` + translatable `name`, D-Code/D-i18n):
  - `finance_account_types` (`19,1,3`) — `current`/`savings`/`deposit`/`loan`/… ; instance-extensible.
  - `finance_card_networks` (`19,1,4`) — `visa`/`mastercard`/`amex`/… .
- **Objects:**
  - `finance_accounts` (`19,1,1`) — `institution_id` → `tenant_organizations` (a `company`-domain
    org); the **IBAN** as an **envelope-encrypted** value (`iban_ciphertext` / `iban_wrapped_dek` /
    `key_ref` / **`iban_blind_index`** — the `document_personal_codes` shape exactly, `pii:sensitive`,
    blind index **unique among active**); `currency` (ISO 4217); `account_type_id` → catalog; `status`;
    soft-delete.
  - `finance_cards` (`19,1,2)` — `account_id` → `finance_accounts` (**structural containment FK**,
    CASCADE, like `OrderItem`→`Order` — **not** a reified Link); the full **PAN** envelope-encrypted
    (`pan_ciphertext`/`pan_wrapped_dek`/`key_ref`/**`pan_blind_index`**, `pii:sensitive`) with the
    display-only clear `bin CHAR(6)` + `last_four CHAR(4)`; `network_id` → catalog; `card_type` TEXT+
    CHECK ∈ {`debit`,`credit`}; nullable `expiry_month`/`expiry_year`; optional nullable
    `cardholder_person_id` → `person_persons`; **NO CVV/CVC column ever** (see *Why not* d); soft-delete.
- **Reified Link:**
  - `finance_account_holders` (`link__held_by`, `19,2,1`) — the **ownership** edge: `account_id` →
    account; a **polymorphic holder** `holder_kind ∈ {person,company}` + `holder_id` (text, **no FK** —
    the RID self-describes, F-014; mirrors D-Vehicles `registered_to` / D-Companies `owns_stake`);
    `role ∈ {primary,joint,authorized_signer}`; **temporal** (`effective_from`/`effective_to`). Joint
    accounts and the raw todo's "person → bank account → card" relation both fall out of this edge.
- **Encryption & validation** reuse `pkg/crypto` (`Cipher.Seal/Open/BlindIndex`, D-CryptoProvider) and
  the `pkg/personalcode` validator precedence (compiled > regex > accept-warn, D-PersonalCodes): an
  **IBAN** validator (ISO 13616 mod-97 + strip-spaces normalization) and a **PAN** validator (Luhn +
  BIN→network detection). Both values are blind-indexed for dedup/uniqueness.
- **PII / purge:** IBAN & PAN are `pii:sensitive`; BIN/last4/blind-index are `pii:none`. On
  `PersonPurged` a `finance` subscriber erases the person's holder edges and **crypto-erases** any
  account (and its cards) the person **solely** holds (mirrors [document](../modules/document.md)
  `ErasePersonRecords`). Company-held accounts are unaffected by a person purge.
- **Authorization:** perms `finance.read` / `finance.manage` (+ instance-scope `finance.catalog.manage`);
  account/card reads are **scoped through the holder** (D-PersonReadScope) + shadow gate for person
  holders, and via the company for corporate accounts; all writes are audited Actions (D-Audit).

> **PCI-DSS scope warning.** Storing the full PAN (even envelope-encrypted) brings the deployment into
> **PCI-DSS cardholder-data scope** — the operator inherits the applicable control obligations.
> **CVV2/CVC2/CID is excluded outright** (PCI-DSS Req 3.2 prohibits storing it after authorization,
> encrypted or not). A **BIN+last-4-only** mode (never persist the full PAN, so the deployment stays
> out of scope) is parked as **DS-54** for operators who don't need the full number.

**Why.** Bank accounts and cards are a first-class part of a personnel dossier the operator asserts
(payroll destination, sanctioned-account cross-check, next-of-kin finance), and they need the same
directory discipline as documents: catalog-typed, encrypted-at-rest, holder-scoped, purge-erased.
Modeling the bank as a **company org** reuses the M21/M41 legal-entity registry (one shared record per
bank, its own ownership graph) instead of a parallel institution table. Modeling ownership as a
**polymorphic, temporal holder Link** captures joint accounts, corporate accounts, and account
transfers as history with the membership/vehicle discipline. Encrypting IBAN/PAN as
`document_personal_codes`-shape envelopes means **zero new crypto machinery**.

**Why not** (a) *Extend the [document](../modules/document.md) module*: a document/personal-code is a
flat person-held value; an account has **multiple holders + child cards** — an ownership graph, not a
code. A dedicated module keeps that graph and the card hierarchy clean while still reusing the
encryption seam. (b) *Fold into M35 (financial/behavioral overlays)*: that tier is **inferred OSINT
overlay** (crypto-wallet attribution with `source`/`confidence`, never merged with declared) — this is
**authoritative first-party** data, a different posture, so it does **not** carry the M29 attribution
columns. (c) *Fold into M39 (compensation/payroll)*: payroll is the org **as payer** (operational HR) —
an account directory is person-held finance, distinct. (d) *Store CVV (raw todo listed it "optional")*:
**forbidden** — PCI-DSS Req 3.2 prohibits storing CVV2/CVC2 after authorization even encrypted; there
is no compliant way to keep it, so it is dropped entirely. (e) *A single-owner `owner_id` column*:
excludes joint/corporate accounts; the reified polymorphic Link is strictly richer. (f) *A bespoke
`bank`/`institution` table*: banks **are** companies — reuse the M21 registry.

**Consequence.** New `finance` module + the tables above; RID **service 19** allocated (added to
`pkg/rid` + migration `0000` on build); a `finance` `PersonPurged` subscriber extends the crypto-erase
path; new Object/Link kinds in [ontology-mapping](../ontology-mapping.md); IBAN/PAN validators added to
`pkg/personalcode` on build. Parks **DS-54** (BIN+last-4-only, out-of-PCI-scope mode) and **DS-55**
(account balance/transaction ledger — explicitly **out of scope**: this is a directory of accounts, not
a payments system). **Depends on** the person directory (M5), D-Companies/D-UnifiedOrgGraph (M21/M41),
D-CryptoProvider + D-PersonalCodes (M0/M9), audit (M1). Retires the final `todo.md` idea (banks/accounts/
cards). Lands as **M44** ([milestones](../milestones.md)). Additive / expand-only.

---

### D-Pinax — The reference plane: a named world-model plane with an `origin` marker + bundled YAML seed presets self-seeded at boot (extends D-Ontology, D-i18n, D-Hermenea, D-DataIngestion; amends D-Languages, D-Geo, D-Rank, D-Religion, D-PhysicalIdentity/M43, D-Color)

**Decision.** Name and consolidate the **reference plane** — the instance-global, externally-sourced
or curated, read-mostly **world-model** catalogs — as **`pinax`** (a *naming convention*, **not** a
new RID service): `platform_colors`, `geo_countries`, `language_languoids` + `writing_systems`,
`rank_*` systems, `religion_taxa`, and `person_ethnicity_types` (+ its closure / `_languages` /
`_countries` links). `pinax` is a cross-cutting plane spanning existing services (1/6/12/13/rank/
religion), distinguished from the **operational core** (person / membership / order / unit — operator-
authored, org-scoped) and from the **small structural type/kind catalogs** (relation/phone/email
types, document schemes, unit-kinds, `*_kinds`… — which **stay migration-seeded**). The plane is
governed by one shared seeding contract:

- **Seed vs connector by cardinality — one import pipeline, two connector *kinds*.** Bounded/curated
  world content ships as **bundled YAML presets** loaded at boot; massive/growing data (cities,
  regions, `geo_places`) stays a **remote hermenea connector** (D-GeoPlaces, unchanged). Both funnel
  through the **same application-layer import service** the HTTP `POST /import/{objectType}` wraps —
  sharing per-import provenance `(source, source_version, imported_at)` and idempotency. The only
  difference is the byte source: a **bundled file** vs a **remote fetch**. "Seed" is just a
  `bundled_file` connector.
- **`go:embed` + boot autoseed, in oikumenea.** The bundled presets are embedded in the **oikumenea**
  binary and self-seeded on boot via that same application import service (in-process, not over
  HTTP) — so a **fresh oikumenea is usable standalone**; hermenea is reserved for the big/live
  connectors. Gated by config **`pinax.autoseed`** (ECV/refreshable, **default `true`**), and
  **version-gated** via a `pinax_seed_state` table so a warm DB does an O(#presets) no-op check, not a
  27k-row re-upsert, on every restart. An explicit `oikumenea seed` command (`--reconcile`) covers
  `autoseed:false` and manual refresh.
- **The seed algorithm: create-if-absent, fill-if-empty, never delete.** Matched on natural-key
  `code`: **absent → INSERT** (`origin='seeded'`); **present → do nothing** (a bulk
  `INSERT … ON CONFLICT (code) DO NOTHING`). Skeleton rows created by migration (locales, countries)
  are **enriched fill-if-empty** — the seeder fills `coordinates` / translations / `color_id` only
  where `NULL`/blank, and **never overwrites** a non-empty value. A code that **vanishes upstream**
  simply **persists** (no auto-delete / auto-deprecate ⇒ no orphaned operational FK — the generalized
  fix for the Crimea-FK class of bug). Upstream **corrections** to already-seeded rows propagate
  **only** via the explicit `--reconcile` (which touches `origin='seeded'` rows only). Invariant:
  **the boot seeder never overwrites existing data; it only fills gaps.**
- **`origin` marker, plane-wide.** Every seeded reference table gains
  `origin TEXT NOT NULL DEFAULT 'operator' CHECK (origin IN ('seeded','operator'))`; the seeder writes
  `'seeded'`, ordinary API inserts default `'operator'`. Reconcile only ever touches `origin='seeded'`.
  Its role reduces to: **label provenance**, **protect operator-created rows** from clobber, and
  **gate `--reconcile`**. Operator-*edited names* don't collide because i18n is a `locale → text` map
  with **per-entry provenance** (D-i18n): the seeded `cldr|curated` entry and an operator/official
  entry coexist; reconcile touches only the former, and the UI prefers the higher-provenance entry.
- **Translations: CLDR where authoritative, curated-and-marked elsewhere.** Country and language
  display names come from **CLDR** (`ukr`/`eng`, the same CLDR M18 already uses for scripts); religion,
  rank, and ethnicity names are hand-authored and marked **`source:curated`** so a later official
  translation supersedes cleanly. The **`language↔writing_system`** and **`language↔country`** wirings
  are **derived from CLDR** in the generator, not hand-authored.
- **Ethnicity seeds normally — the catalog is public reference data.** Only the *person↔ethnicity
  declaration* is Art. 9 (`pii:special`), and it stays **envelope-encrypted** in `person_ethnicities`,
  untouched. So `person_ethnicity_types` **reuses its existing `0030` schema** (catalog + closure +
  `_languages` + `_countries` + provenance — **no new module**); the loader is retargeted from the
  M43 opt-in **live** Factbook fetch to a **bundled Factbook YAML preset** (deduped catalog + homeland-
  country refs; language refs curated), seeded with the rest. This **amends D-PhysicalIdentity/M43**
  ("seeded empty, `enabled:false`") to "bundled preset, seeded by default".
- **Colors via D-Color.** "Seeded colors" extends `platform_colors.domain` (`eye`/`hair`/`vehicle`)
  with `rank`/`religion`/`ethnicity`/`country`, adds a `color_id` FK to the seeded catalogs where a
  display color is meaningful, and seeds those palettes; `platform_colors` itself joins the plane and
  gains `origin`.
- **Preset package + manifest.** One versioned reference-data tree; each preset carries a manifest —
  `source`, `source_version`, `license`, **`depends_on`** (inter-preset topo order:
  locales → languages / countries → religions → ethnicity), and translation provenance. Machine-
  generated presets (the 27k Glottolog forest) come from a reproducible generator; curated presets
  (religion / rank / ethnicity names) are hand-authored YAML.

**The bundle (7 presets, final).** (1) **languages** — Glottolog ~27k languoids + hierarchy, CLDR
names, `language↔country`; (2) **writing systems** — ISO-15924 scripts (already migration-seeded) +
the CLDR-derived `language↔writing_system` wiring; (3) **countries** — WOF enrichment of the
migration-seeded skeleton (**no live WOF calls**), CLDR names, coordinates, color; (4) **religions** —
`religion_taxa`; (5) **ethnicities** — Factbook catalog (deduped) + country & (curated) language refs;
(6) **ranks** — UA (per branch) + US (per branch); (7) **colors** — `platform_colors` palettes.
**Considered and deferred:** currency (ISO 4217; no currency table yet) and religion↔country relations.

**Why.** Reference data was scattered across the world-model modules with **no visible boundary** and
**two divergent load paths** (rank seeds baked into migration `0004`; languages loaded via the import
path), and live-fetching bounded catalogs at boot coupled a usable core to hermenea + third-party
availability. A named plane + one create-if-absent seed pipeline + embedded presets yields: a fresh
core **usable offline**, **uniform provenance/idempotency**, **operator-safe** re-runs (never clobber),
**reproducible upstream bumps** (regenerate a preset, boot fills the new codes), and — because the
`/import` boundary is preserved — an **extraction seam** if the plane ever needs its own service (the
same extraction-ready posture as the modular monolith). Physical separation (own DB/service) was
**rejected**: the design leans on single-Postgres FK integrity (closure tables, `person SPEAKS`,
ethnicity↔country↔language, rank-on-person), and reference data has no independent scaling/release/team
force — a runtime split would be unearned complexity that trades FK integrity for network hops.

**Why not** (a) *A separate `pinax` runtime service + DB*: loses the FK integrity the whole model
relies on; see above. (b) *Fold reference ownership into hermenea*: hermenea is the ingestion **verb**;
the plane is the **noun** it writes — fusing them rebuilds the coupling. hermenea stays a pure pipeline;
the preset-authoring toolchain may live adjacent to it as a build-time concern. (c) *Seed via SQL
migrations*: baking 27k languoids + translations into DDL is unmaintainable, regenerating from a new
upstream means regenerating SQL, and it already bit us (editing `0004`'s rank seed broke other modules'
`seedRank` tests). Migrations keep only the minimal skeleton (locales; FK-required rows). (d) *Upsert /
overwrite on reconcile by default*: clobbers operator edits and risks orphaning on upstream deletion;
create-if-absent + fill-if-empty + explicit `--reconcile` is strictly safer. (e) *A new schema
(`pinax.*`) for the plane*: considered; rejected in favor of a **naming convention** (keeps the
one-schema `oikumenea` decision + sqlc/migration conventions intact) — the `origin` marker + module
grouping give the boundary without a schema split.

**Consequence.** **In-place edits** to existing migrations (`0004_rank`, `0018_language`,
`0023_religion`, the `geo_countries` migration, `0030` ethnicity, and `platform_colors`) add the
`origin` column, new `color_id` FKs + `platform_colors` domains, and a `pinax_seed_state` table —
**no new migration file** (honoring the `atlas migrate hash` + `DROP SCHEMA` reset flow). **No new RID
service.** Reverses **M15**'s "ranks folded into migration `0004` seed" and **M43**'s "live Factbook
fetch, no committed preset" in favor of bundled YAML. Amends **D-Languages / D-Geo / D-Rank /
D-Religion** (their catalogs join the plane + gain `origin`), **D-PhysicalIdentity/M43** (ethnicity
loader + default-seeded), and **D-Color** (new domains + `color_id` on seeded catalogs). **Depends on**
D-Languages (M18), D-Hermenea/D-DataIngestion (M16), D-Color (M42), D-PhysicalIdentity/M43. Lands as
**M45** ([milestones](../milestones.md)); see the [pinax plane note](pinax.md). Additive / expand-only.

---

## North-star topology cluster (M51–M54)

The four decisions realizing the [north star](north-star.md) (agreed 2026-07-18): oikumenea as a
**headless internal core** behind unprivileged **facades**, with machine callers as **service
principals**, the ingestion companions generalized into a **connector plane**, and deployments
tailored by **data packs** rather than runtime code. Sequenced as **M51–M54** on the
[stage board](../milestones.md#stage-board), dependency order M51 → M52 → M53 → M54.

### D-ServiceIdentities — Machine clients authenticate via IdP client-credentials and resolve to service principals (extends D-Hermenea, L-AuthzOnly)

**Decision.** Machine callers — facades with standing of their own, and every connector calling the
core — authenticate through the **same external IdP** that authenticates humans, using the OAuth2
**client-credentials** grant. The existing OIDC/JWKS validation middleware
([identity-federation](../modules/identity-federation.md)) accepts these tokens on the same path and
resolves them not to a person but to a **service principal**: a registered machine subject holding
**per-principal grants** `(principal, permission_code, org_id)` — `org_id` **NULL = instance-wide**
(reference-catalog imports, the D-ConnectorPlane wiring codes), a **named organization** confining a
connector to that organization's data. Service principals are first-class, auditable subjects
(actor shape `system`, D-Audit) registered in oikumenea by an instance admin: `(issuer, subject) →
service principal`, mirroring the person-side external-identity mapping.

A principal **never** holds a role assignment and **never** has unit reach. Two properties of the
built system forbid the "give the machine a role" reading: `authz_role_assignments.subject_person_id`
is a hard `person_persons` FK, and the PDP satisfies **instance-scope** permissions — which
`import.manage` and every wiring code are — **only** for instance admins, never through a role
(`internal/authorization/domain/pdp.go`, D-InstanceAdmin). A principal granted a role could therefore
not import at all. Flat per-principal grants also keep machines out of the unit DAG, so the
D-RLSLiveReach / D-AuthzGrantCache hot path built by M47 is untouched; the **organization** (not the
unit) is the right blast-radius boundary for a connector, since a scraper feeds one organization
(D-TenantOrganizations). This **generalizes the M16
`hermenea-importer`** from a token-mapped singleton into a registry, and **amends D-Hermenea's
"Why not (e)"**: the shared runtime env-secret (`HERMENEA_OIKUMENEA_TOKEN`) was the right call for
*one* companion, but a fleet of facades + connectors makes per-pair shared secrets an unmanageable
mesh — the IdP already deployed for humans issues, rotates, and revokes machine credentials
instead. The env-secret path **remains supported** as a fallback for minimal installs (a
single-connector deployment without a client-credentials-capable IdP).

**Why.** Both the access plane (D-HeadlessTopology) and the connector plane (D-ConnectorPlane) need
machine callers with *scoped, revocable, auditable* identity. Reusing the IdP + the existing
middleware keeps L-AuthzOnly intact — oikumenea still stores no credentials and issues no tokens —
and gives operators one place to rotate/revoke everything. One validation path for humans and
machines means no second auth stack to harden.

**Why not** (a) *oikumenea-issued API keys*: reverses L-AuthzOnly's no-credentials stance — the
core would store secrets and become an issuer. (b) *mTLS service mesh*: real, but an
infrastructure-layer control; it does not name a PDP subject, and most target deployments
(docker-compose, single host) have no mesh. (c) *Per-pair shared secrets at fleet scale*: n×m
secret sprawl with no central revocation; kept only as the minimal-install fallback.

**Consequence.** identity-federation gains the **service-principal registry**
(`account_service_principals`, admin-managed `(issuer, subject)` mappings) and the middleware gains
the resolve-to-service-principal arm; authorization gains **`authz_principal_grants`**
`(principal, permission_code, org_id NULL)` and the PEP a service branch (`RequireService` /
`RequireServiceOrPerson`), while the PDP itself is untouched — a principal decision is a flat grant
match, not a DAG traversal. The request context carries a subject kind (person | service), on its own
snapshot/cache keyspace so a principal can never alias a person. The audit `system` actor becomes
attributable to a specific principal (`audit_log.actor_principal_id`; no third `actor_type` — D-Audit
holds). `RequireImport`'s hard-coded `hermenea-importer` branch is deleted in favor of a real grant.

**Boundary.** A service principal got **no RLS reach** in M51: the request set no `app.person_id` GUC,
so every person-shaped PEP path denied it by construction. Machine access to RLS-protected,
organization-owned data (scraped units, memberships, clergy, university staff) required an RLS service
arm, split out of D-ConnectorPlane as **M55**. M51 shipped the mechanism plus the instance-wide
capability, and the `org_id` grant column, so M55 retrofit nothing. Prerequisite for M52/M53. Lands as
**M51** ([milestones](../milestones.md)). Additive.

**RLS service arm as built (M55, migration `0042`, 2026-07-21).** A third GUC **`app.principal_id`**
now pins a machine subject (the authenticator installs a lazy RLS-scoped connection for it, exactly
as for a person; `db.RLSState` gains `PrincipalID`). An **org-confined** grant (`org_id NOT NULL`)
authorizes that organization's RLS-guarded rows — reads, writes, and the creation of brand-new
(edgeless) units — and **only** that org's; `org_id NULL` (instance-wide) grants confer no operational
reach (blast-radius = the org). Because the reach predicate may read only RLS-exempt tables, `0042`
adds a dedicated exempt projection `authz_unit_org(unit_id → org_id)` (trigger-maintained, FK
`DEFERRABLE INITIALLY DEFERRED`) that the child-table arm resolves through, while the `tenant_units`
policy checks the row's own `org_id` directly (the one case the projection can't serve mid-INSERT).
`RequireImport` now passes a **real orgID** (optional envelope `orgId`), so an org-scoped import is
accepted under a matching org grant and rejected under a foreign one. Every person-shaped PEP path
still denies a machine — the reach is bounded to what its org grant authorizes at the DB. Revocation
is immediate (grants read live). See D-RLSLiveReach's dated extension and
[milestones M55](../milestones.md).

### D-HeadlessTopology — oikumenea is internal-only behind unprivileged user-token-passthrough facades (extends L-AuthzOnly, amends D-WebUI)

**Decision.** In the target topology **oikumenea has no public exposure**: it listens on an
internal network, and every human-facing consumer reaches it through a per-audience **facade**
(BFF) speaking the Conjure API via the generated SDKs (D-ClientSDK). Facades may own the browser
session, shape/aggregate responses, and cache — but they are **unprivileged**: a facade always
forwards the **end-user's IdP token**; oikumenea re-validates it and runs the PDP against the real
user, exactly as today. There is **no on-behalf-of assertion** — no facade can tell the core "act
as person X" — and a facade holds no credential that widens access, so a compromised facade can
impersonate nobody (no confused deputy). The first facade is **console-bff**, in front of the
existing Next.js console; this **amends D-WebUI** (the console stops being a direct public-API
consumer) without changing the console's optional, separately-deployed nature.

**Why.** The product direction is oikumenea as the **brain of a system of services** — other
products (an HR app, a church app, the admin console) present it to their audiences. Making the
core internal-only shrinks its attack surface to authenticated internal callers; making facades
unprivileged keeps the PDP the *only* authority — per-audience UX concerns (sessions, aggregation,
rate shaping) land in facades without any facade entering the trust computing base.

**Why not** (a) *Facade-terminated auth + on-behalf-of headers*: makes every facade a trusted
deputy whose compromise is a full impersonation primitive; rejected. (b) *Keep the public API +
optional facades*: leaves two exposure paths to harden and lets consumers bypass their facade;
the single internal path is strictly simpler to reason about. (c) *An API gateway product in
front*: a deployment choice operators may still make; the architecture only requires that the
thing facing the public is not oikumenea itself.

**Consequence.** Compose topologies stop publishing oikumenea's port; docs/config gain the
internal-network stance. The Conjure contract is unchanged — facades consume it as-is. Requires
M51 only for facades that also need standing of their own (health probes, cache warmers); pure
passthrough works with the user token alone. Lands as **M52** ([milestones](../milestones.md)).

**Amendment (M52, 2026-07-19) — `console-bff` is the console's own server tier, not a new deployable.**
The Consequence above originally called for "a new `console-bff` deployable (thin: session +
passthrough + static console hosting or proxy)". On implementing M52 that deployable turned out to be
**already built**: the Next.js console's server tier (`web/src/auth.ts` — Auth.js v5, OIDC
Authorization-Code exchanged server-side into an httpOnly session; `web/src/app/api/oikumenea/[...path]`
— the bearer-attaching passthrough proxy) does all three jobs in one process, and does them under
exactly the constraints this decision requires: the browser never holds a token, the **end-user's**
bearer is forwarded unchanged, and there is no on-behalf-of seam anywhere. **The console's Next.js
server tier IS `console-bff`**; M52 introduces **no new binary**, and renames the compose service to
say so.

*Why not build the separate Go BFF anyway:* it would have to own the browser session, which means
duplicating or replacing Auth.js, and would then proxy static console traffic to the Next server — new
moving parts, a second session implementation, and **no security gain**, since the properties the
decision actually turns on (forwards the user token, holds no widening credential, asserts no
on-behalf-of) are already satisfied. The decision's substance is a *constraint on facades*, not a
mandate for a particular process boundary.

*Two paths to the core, both legitimate.* Server Components call oikumenea directly via
`web/src/lib/api/server.ts`; the browser calls it through the BFF proxy route. These are not two
exposure paths in the sense (b) above rejects — both attach the same session bearer and both originate
**inside the facade process** on the internal network. The browser has exactly one path.

*Consequently* the core needed no change at all for M52: it has no trusted-proxy list, no
`X-Forwarded-*` handling, and treats `azp` as diagnostic-only — the facade is just another
authenticated caller presenting a user's token.

**Amendment (M37, 2026-07-21) — the facade forwards the client IP (one trusted hop).** The login
security log (D-LoginSecurityLog) needs the *user's* IP, but with the core behind `console-bff` its
`RemoteAddr` is the facade's. So the facade now sets a **single authoritative `X-Forwarded-For`** (the
client IP from the deployment ingress; overwrite, not append), and the core trusts it **only** when
`login-security.trust-forwarded-for` is on — the one narrow place the core reads a forwarded header.
The trust boundary is the deployment's ingress (which must set XFF from the real socket, the standard
reverse-proxy contract); off by default, so a direct (non-facade) caller's `RemoteAddr` is used and a
client cannot spoof its own logged IP. This is the minimal, opt-in exception to "no `X-Forwarded-*`
handling" above — it changes no authorization behavior (the token still carries all authority).

### D-ConnectorPlane — A connector registry + a three-mode contract: push, pull-wiring, on-demand lookup (extends D-Hermenea, D-Watchlists)

**Decision.** hermenea generalizes into a **family of connectors** (Palantir data-connection-style:
agents beside the core, never inside it). Each connector keeps its own storage + scheduler and
couples to oikumenea **only over HTTP** (the D-Hermenea boundary, kept). oikumenea gains a
**connector registry** — `Connector` and `Source` become first-class Objects, with audited
**sync-run reporting** (a connector posts run start/finish/failure; the core displays and audits,
it does not orchestrate). The connector contract names **three interaction modes**, per connector
per source:

- **push** — bulk data in via the existing chunked, resumable `POST /import/{objectType}`
  envelopes (M49); the workhorse, unchanged.
- **pull-wiring** — connector → oikumenea *reads* on a narrow **wiring API**: resolve natural
  keys to RIDs, read reference catalogs (countries, languoids, legal-basis kinds), fetch its own
  sync cursors/registry row. Authenticated as a D-ServiceIdentities principal; each wiring read
  surface is its own permission code — what a connector may see is a grant, not a default.
- **on-demand lookup** — oikumenea → connector synchronous calls with a deadline: the M34
  watchlist check generalized into one typed connector-call seam (per-lookup-kind interface,
  late-bound in `main.go`, deadline mandatory per R-12).

**Why.** The user direction is "hermenea-like services" (plural) for one-shot and permanent data
flows — that needs a *contract*, not another bespoke integration. Push alone has already proven
insufficient twice: M34 needed a synchronous egress (hard-wired), and real mappers need reference
reads (today they re-derive or guess natural keys). Naming the three modes keeps each narrow
instead of growing one god-API; the registry makes the fleet observable from the core, where
operators already look.

**Why not** (a) *Core-side orchestration (oikumenea schedules connector runs)*: reverses
D-Hermenea's separation — the core would hold connector operational state and outbound coupling to
every connector; visibility-not-orchestration keeps the blast radius out. (b) *Connectors read the
core DB replica*: breaks the HTTP-only boundary and the PDP/visibility gates; rejected outright.
(c) *A message broker between core and connectors*: new infrastructure for what HTTP + idempotent
envelopes already deliver; reconsider only if fan-out demands it.

**Consequence.** New registry tables + Objects (`Connector`, `Source`, `SyncRun`) with RID types;
the wiring API endpoints + per-surface permission codes; the `watchlistclient` seam refactored into
the generic connector-call seam (M34 behavior unchanged); hermenea registers itself as the first
connector and its sources migrate into the registry; a read-only console fleet view. `import.manage`
stays the push gate. Requires M51. Lands as **M53** ([milestones](../milestones.md)). Additive /
expand-only on the core.

**Amendment (2026-07-21) — the RLS service arm moved out to M55.** As built, M53 is the connector
plane's **reads and reporting**: the registry, the instance-scope wiring API, the generalized lookup
seam, hermenea self-registration, and the console. It does **not** give a machine reach into
RLS-protected, organization-owned data — that "RLS service arm" (a principal branch in
`authz_unit_in_reach`, org-confined writes, `RequireImport`'s real orgID) is now **M55**, not here.
Two reasons. (1) *Scope honesty:* D-ConnectorPlane's own Delivers list and all four exit criteria
describe reads over instance-wide reference data — never RLS, writes, or org-owned data — so the arm
was never really in this decision's body; the M51 code comments that promised it "with
D-ConnectorPlane" over-reached. (2) *It is the separable hard part:* `authz_unit_in_reach`
([migrations/0005_document_order_rls.sql](../../migrations/0005_document_order_rls.sql)) may
read only RLS-exempt tables, because reading a policy-guarded table recurses into its own policy — so
a principal arm cannot join `tenant_units` to learn a unit's org even though `tenant_units.org_id`
exists. That is a design problem of its own, tracked as M55 rather than smuggled in here. Everything
M53 shipped is unaffected; M55 retrofits nothing (the `org_id` column already ships, M51).

**M55 delivered (migration `0042`, 2026-07-21).** The RLS service arm landed: a dedicated RLS-exempt
projection `authz_unit_org` + the `app.principal_id` GUC give an org-confined principal reach into its
organization's RLS-guarded rows (incl. creating brand-new units), and `RequireImport` passes a real
orgID. The machine write path over HTTP is the **import endpoint** (`RequireServiceOrPerson`); direct
module write APIs stay person-gated (a connector ingests via `/import/{objectType}`, it does not POST
`/units`). Per-object org-owned import handlers land with the connectors that need them; the DB
backstop confines whatever they write. See D-ServiceIdentities' and D-RLSLiveReach's dated as-built
notes and [milestones M55](../milestones.md).

### D-DataPacks — Plugins are versioned data packs + per-module enable flags, no runtime code loading (extends D-Pinax, D-i18n)

**Decision.** The plugin system is **data, not code**. A **data pack** is a versioned bundle of
seedable content — locale packs (new supported locales + translation overlays), pinax-style
world-model presets, catalogs, rank schemes — in the D-Pinax YAML preset format, loaded by the
boot autoseeder under its existing invariants (**create-if-absent / fill-if-empty / never-delete**,
version-gated, `origin`-marked). D-Pinax's `go:embed`-bundled presets generalize to
**operator-mounted packs** (a `pinax.packs` directory scanned at boot beside the embedded set;
same pipeline, same `pinax_seed_state` gating, pack name + version recorded). Alongside packs, a
**per-module enable flag** surface (install config, e.g. `modules.finance.enabled`) lets a
deployment switch verticals off: a disabled module's endpoints are hidden/404 and its permission
codes are not grantable, but its **schema still migrates** — schema presence is not capability,
and re-enabling is a config flip, not a migration. There is **no runtime code loading**: Go links
statically and the Conjure surface is generated at build time; code-level extension = building
from source against the module registration seam in `main.go` — a documented seam, not a product
feature.

**Why.** Every plugin the direction actually named ("more locales", reference content) is
data-shaped, and D-Pinax already built the safe loader for exactly that — generalizing its input
is cheap and honors upgrade-safety (L-UpgradeSafe). Runtime code plugins would fight the entire
toolchain (static linking, generated contracts, `atlas` migration lint) for a need nobody has
stated. Enable flags deliver the real "modular deployment" want — a lean army install without
finance/religion — without demoting verticals from the core (D-DataScope identity preserved).

**Why not** (a) *Go `plugin` / .so loading*: platform-fragile, version-locked to the exact
toolchain, incompatible with the single-binary story. (b) *Embedded interpreter (Lua/JS hooks)*:
a sandboxing + audit surface with no named use case. (c) *Skipping a disabled module's
migrations*: creates per-deployment schema divergence and breaks the boot-time schema-version
check; schema always converges, capability is config.

**Consequence.** The pinax loader gains a pack scanner (mounted dir + embedded, unified
`pinax_seed_state` versioning); locale packs become the first packaged kind (a pack may add an
`i18n_locales` row + translation overlays — extends D-i18n's admin-managed locale set); platform
config gains the `modules.*.enabled` map consulted at module registration in `main.go`;
witchcraft route registration + the search/links fan-in registries skip disabled modules. Lands
as **M54** ([milestones](../milestones.md)). Additive.

**As built (2026-07-21).** The pinax loader gained a `pinax.packs` mounted-dir scanner merged into the
same topo-sorted, version-gated set (collisions across bundle+packs fail boot; `pinax_seed_state.pack`
records provenance, migration `0041`). Locale packs ride a **new `locales` import objectType**
(create-if-absent into `i18n_locales`, never flipping an operator's `enabled`/`is_default`) plus the
existing `translations` objectType; the sample pack is `deploy/packs/locale-deu`. The **toggleable set
is the six enrichment verticals** (finance/religion/vehicle/externalorg/company/education); core +
depended-on reference modules (geo/language/color/…) are always on. Two enforcement refinements worth
recording: (1) **code non-grantability is prefix-based** — the authz application service holds the
disabled modules' code prefixes (`finance.`, `religion.`+`religionorg.`, …) and rejects a grant of any
code under them as `ErrUnknownPermission`, keeping the static domain catalog unchanged (a config flip,
not a code change, re-enables). (2) The **links fan-in omission is achieved *through* that code-gating,
not by dropping link descriptors** — the generic traversal is permission-gated per relationship
(D-LinkPermissions), so a disabled module's per-relationship read codes being ungrantable already makes
its facets unreachable; the descriptors stay registered so the R-28 boot coverage assertion holds and
re-enabling needs no descriptor surgery. Search providers ARE skipped directly (a disabled vertical
passes a nil service). Schema always migrates.
