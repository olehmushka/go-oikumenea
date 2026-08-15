# Module: religion

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.religion_*`

## Purpose

Owns the **religion vertical** for **any faith** (D-Religion) — Christianity, Islam, Judaism, Hinduism,
Buddhism, Sikhism, Bahá'í, Shinto, traditional/indigenous, … — by adding only the religion-specific
structures on top of the existing core: religious **organizations** reuse the [tenant](tenant.md) unit
graph, **clergy offices** reuse [membership](membership.md) positions + [authorization](authorization.md)
role assignments, **decrees** reuse [order](order.md), and **sites** reuse the shared
[location](location.md) entity. What this module adds is the **faith taxonomy** (religions → traditions
→ sub-traditions), **clergy grades & credentials** (ordination/investiture/recognition), **lay
affiliation** (GDPR Art. 9 belief data), and the **discovery substrate** (sites, service schedules,
search). A FaithMap-style discovery/CMS app sits *on top* and uses go-oikumenea as its
identity/authorization/directory backend — the CMS (pages/blocks/themes) stays in that app.

**Binding design rule (D-Religion): no faith's vocabulary is hard-coded.** Every religion-specific
value — organization kind, sub-tradition, clergy grade, office type, affiliation type, site type,
service type — is a **catalog row** (D-Code/D-i18n), keyed per religion/tradition and seeded with
cross-faith examples. There is **no `CHECK` enum of faith terms and no `if faith == …` branch** in
code; this is how one schema fits every religion and honors L-SingleDomain's "no org-type discriminator
in code" (refined by D-Religion).

## Entities & aggregates

**Milestone map.** **M22 (built):** the taxonomy + organization slice below. **M23 (built):** clergy
grades/credentials (migration `0024_religion_clergy`). **M24 (built):** lay affiliation (`pii:special`,
migration `0025_religion_affiliation`). **M25 (built):** discovery (sites/schedules/search,
migration `0026_religion_discovery`) — described later in this doc. The M23–M25 per-tradition catalogs
FK a `religion_taxa` node (`tradition_taxon_id`) rather than a fixed `tradition_family` row.

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — service **16** —
**M22 Objects:** `Taxon` (the recursive taxonomy node), `TaxonRank` (the level catalog),
`Classification` (the theism catalog), `OrgKind`, `PolicyKind`, `OrgPolicy`. **M23 Objects (built):**
`ClergyGrade` (16,1,8), `GradeCategory` (16,1,7), `OfficeType` (16,1,9). **M24 Objects (built):**
`AffiliationType` (16,1,10). **M25 Objects (built):** `SiteType` (16,1,11), `ServiceType` (16,1,12),
`ServiceSchedule` (16,1,13), `Alias` (16,1,14). **Reuses** `Unit` ([tenant](tenant.md)) for the organization
nodes (an `OrgProfile` is a 1:1 extension of a Unit, keyed by the unit RID — no own RID) and `Position`
([membership](membership.md)) for clergy offices.
**M22 Links:** `link__classified_as` (`religion_org_classifications` — unit ↔ taxon, one primary).
**M23 Links (built):** `link__clergy_credential` (`religion_clergy_credentials`, 16,2,2 — person ↔ grade
within an org unit). **M24 Links (built):** `link__affiliated_with` (`religion_affiliations`, 16,2,3,
`pii:special`). **M25 Links (built):** `link__site_of` (`religion_sites`, 16,2,4 — org unit ↔ a shared
location). *(The taxonomy closure, the theism tags on a
taxon, and the per-unit theism override
are bare M:N/derived join tables with no RID, like `tenant_unit_closure` / `language_languoid_countries`.)*
**Actions:** catalog + taxonomy + org edits (M22), each audited, `action__<type>` RID (16,3,0).

- **Taxonomy** — a **single recursive `religion_taxa` tree** (`parent_id` self-FK + a maintained
  `religion_taxa_closure`), each node carrying a catalog-driven **level marker** (`TaxonRank`:
  religion → branch → tradition → sub-tradition → denomination) and an optional `wikidata_id` anchor.
  Reuses the proven `language_languoids`-forest / `education_unit_closure` pattern. Instance-admin
  managed; a rich curated seed (deep Christianity + broad world religions) ships in the migration.
- **Religion-type ("theism") classification** — a `Classification` catalog (monotheistic/polytheistic/
  …) tagged M:N onto taxa, resolving **nearest-declared-wins** down the closure; a unit may **override**
  its inherited type.
- **Organization** — a religious body (denomination/jurisdiction/community/mosque/monastery/…) is a
  **`tenant_units` row** placed in religion graphs; `OrgProfile` holds its 1:1 faith attributes,
  `religion_org_classifications` its M:N tradition tags (one primary), and `OrgPolicy` its data-driven
  eligibility rules.
- **Clergy** *(M23, built)* — `ClergyGrade`/`GradeCategory`/`OfficeType` catalogs + the reified,
  public `link__clergy_credential` (person ↔ grade within an org unit). **Affiliation** *(M24, built)* —
  the reified lay-belief `link__affiliated_with` (`pii:special`, envelope-encrypted, crypto-erased on
  purge). **Discovery** *(M25, built)* — `religion_sites` (→ [location](location.md)),
  `religion_service_schedules`, `religion_aliases` + the closure-aware PostGIS discovery search.

## Data model

Conventions (URN RID PKs (D-ResourceIdentifiers), `TIMESTAMPTZ`, `set_updated_at`, soft-delete,
`TEXT`+`CHECK` only for **fixed lifecycle statuses** — never for faith vocabulary) per
[conventions.md](../architecture/conventions.md).

### Taxonomy (recursive tree + closure)

**`religion_taxon_ranks`** (the ordered **level scaffold** — structural, not faith vocabulary) — `id`
PK (16,1,2); `code`; `name` (translatable); `ordinal INT`; `status`; `sort_order`; timestamps;
soft-delete. *Seed:* `religion`(0) → `branch`(1) → `tradition`(2) → `sub_tradition`(3) →
`denomination`(4). Extensible; a faith need not use every level (the closure carries true depth).

**`religion_taxa`** (one node in the recursive faith classification tree) —
- `id` PK (16,1,1)
- `parent_id TEXT REFERENCES religion_taxa(id) ON DELETE RESTRICT` — NULL = a **root religion**; a
  strict-tree containment self-FK (the `education_units` / `language_languoids` pattern), **not** a
  reified Link
- `rank_id TEXT NOT NULL REFERENCES religion_taxon_ranks(id)` — the level marker
- `religion_id TEXT REFERENCES religion_taxa(id)` — denormalized **root** (derived via the closure,
  like `language_languoids.family_code`)
- `code` (stable, unique among active); `name` (translatable); `description`; `wikidata_id TEXT?`
  (external anchor, e.g. `Q5043`); `icon TEXT?`; `sort_order`; provenance (`source`, `source_version`);
  timestamps; soft-delete.

**`religion_taxa_closure`** (derived transitive closure; `ancestor_id`, `descendant_id`, `depth` PK;
**no RID**; reflexive row) — rebuilt in SQL on every taxon insert/reparent (mirrors
`education_unit_closure`); bulk-built from the migration seed.

**`religion_classifications`** (the religion-type / "theism" catalog) — `id` PK (16,1,3); `code`;
`name` (translatable); `description`; `status`; `sort_order`; timestamps; soft-delete. *Seed:*
monotheistic, polytheistic, henotheistic, monistic, nontheistic, pantheistic, panentheistic,
animistic, dualistic, deistic, agnostic, atheistic.

**`religion_taxon_classifications`** (theism tags on a taxon; `taxon_id`, `classification_id` PK; **no
RID**) — a faith may carry several (Hinduism = monotheistic + polytheistic + monistic). **Resolution is
nearest-declared-wins:** a taxon's effective type is the set declared on the nearest ancestor (via the
closure) that declares any — a descendant declaring its own set **overrides** the inherited one. Seeded
at the `religion` level. A read-time projection, never stored.

> **Curated seed (D-Religion refined).** The migration ships a real-world taxonomy
> (`deploy/religion-presets/gen-presets.py`, anchored to Wikidata QIDs): **deep Christianity** (the five
> major branches → traditions → sub-traditions, plus the globally-significant historic churches as
> denomination-level taxa — Orthodox autocephalous churches, the Eastern Catholic sui-iuris churches,
> Oriental Orthodox churches, major Protestant denominations) and the **major world religions** to
> branch/tradition depth. **Boundary:** the seed stops at the major historic churches; a specific
> *governed instance* (this diocese/parish) is a `tenant_units` row linking to the nearest taxon.

### Organization (reuses `tenant_units`)

**`religion_org_kinds`** (catalog naming each organizational level) — `id` PK (16,1,4); optional
`religion_id` FK → `religion_taxa` (NULL = generic across faiths); `code`; `name` (translatable);
`ordinal INT`; timestamps; soft-delete. *Seed:* denomination/jurisdiction/diocese/deanery/parish/
congregation/mission/monastery/community/mosque-community/temple-community/council.

> **Graphs.** Organization nodes are `tenant_units` placed in **three seeded religion graphs**
> ([tenant](tenant.md) `tenant_graphs`, D-Graphs/D-DirectoryGraphs, seeded idempotently by migration
> 0023): **`canonical`** (governance / jurisdictional tree, **authority-bearing** — the PDP cascades
> `subtree` grants here), **`tradition`** (taxonomic placement, **directory-only**), **`affiliation`**
> (voluntary association DAG, **directory-only**). A unit's `tenant_units.unit_kind` is set from a
> `religion_org_kinds.code` (a descriptive label, never branched on).

> **M41 / D-UnifiedOrgGraph — the church-domain exception.** A religious body is a `tenant_unit` in a
> **`church`-domain `tenant_organization`** with a `religion_org_profiles` sidecar — religion is the
> *template* the [education](education.md) and [company](company.md) verticals were brought onto.
> A **top-level body** is created first-class via **`POST /religion-orgs`** (`createRootOrg`): a
> `church`-domain organization + its root unit + profile; descendants are added with
> `createChildOrg`. Unlike the *reference* domains university/company (`pdp_scoped=false`,
> instance-global), `church` is an **operational** domain (`pdp_scoped=true`) — but religious bodies use
> the **instance-global** `canonical`/`tradition`/`affiliation` graphs (migration-seeded, not per-org),
> not the auto-seeded per-org `command`/`operational` graphs. Authority still cascades down the
> `canonical` graph (reach-RLS applies).

**`religion_org_profiles`** (per-organization faith attributes; one row per religious-body unit) —
- `unit_id TEXT PRIMARY KEY REFERENCES tenant_units(id) ON DELETE RESTRICT` (a 1:1 Unit extension — no
  own RID)
- `org_kind_id TEXT REFERENCES religion_org_kinds(id)` — optional level label
- `short_code TEXT` — optional abbreviation (display/search aid)
- `created_at`, `updated_at`, `deleted_at`

**`religion_org_classifications`** *(Link `link__classified_as` (16,2,1); the M:N tradition tags on a
unit)* — `id` PK; `unit_id` FK; `taxon_id` FK → `religion_taxa`; `is_primary BOOLEAN` (partial-unique
**one primary per unit**); optional `source`/`confidence`; timestamps; soft-delete. A body often fits
several at once (Reformed Baptist; Eastern Catholic = Catholic + Byzantine-rite).

**`religion_unit_classifications`** (the optional per-unit **theism override**; `unit_id`,
`classification_id` PK; **no RID**) — when a unit declares any rows here they **override** its inherited
taxon classification (the unit branch of nearest-declared-wins).

**`religion_policy_kinds`** (the data-driven org-policy vocabulary) — `id` PK (16,1,5); `code`; `name`
(translatable); `description`; timestamps; soft-delete. *Seed:* `excludes_child_creation`,
`excluded_body`.

**`religion_org_policies`** (generic, data-driven eligibility/exclusion — replaces any faith-specific
doctrinal flag) — `id` PK (16,1,6); `unit_id` FK; `policy_kind_id` FK → `religion_policy_kinds`;
`reason TEXT`; `decided_by_person_id`; `decided_at`; timestamps; soft-delete. *Example:* a body marked
`excludes_child_creation` blocks creating congregations beneath it via `POST
/units/{id}/child-orgs` (the generic analog of the dropped Christianity-specific "Nicene gate").

> **Clergy (M23, migration `0024`) and Lay affiliation (M24, migration `0025`) are BUILT.** Discovery
> (below) remains **designed but deferred** to **M25**. The per-tradition catalogs FK a `religion_taxa`
> node via `tradition_taxon_id` (at `tradition`/`sub_tradition`/`religion` rank) instead of the retired
> `religion_tradition_families` table — the column is `tradition_taxon_id` throughout the built tables.

### Clergy (D-ClergyCredential) — *built (M23, migration `0024`)*

**`religion_grade_categories`** (per-tradition grouping of grades — generic, replaces a fixed
major/minor enum) — `id` PK; optional `tradition_taxon_id`; `code`; `name` (translatable);
`ordinal`; timestamps; soft-delete.

**`religion_clergy_grades`** (ordered, per-tradition catalog) — `id` PK; optional `tradition_taxon_id`
FK; `grade_category_id` FK → `religion_grade_categories`; `code` (unique among active within a
tradition); `name` (translatable); `ordinal INT` (seniority **within the tradition**); timestamps;
soft-delete. *Seed (per tradition): Christianity →* bishop/presbyter/deacon (+ subdeacon/reader);
*Islam →* imam/mufti/sheikh; *Judaism →* rabbi/cantor; *Buddhism →* bhikkhu/lama; *Hinduism →*
pujari/swami. **No cross-tradition comparator** (DS-43 stays parked); `ordinal` orders only within a
tradition.

**`religion_clergy_credentials`** *(Link `link__clergy_credential`)*
- `id` PK — RID, `link__clergy_credential` slot
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `clergy_grade_id TEXT NOT NULL REFERENCES religion_clergy_grades(id) ON DELETE RESTRICT`
- `org_unit_id TEXT NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT` — the tradition/organization
  body that conferred/recognizes the standing
- `granted_on DATE` — when conferred (`pii:none` — an organizational fact)
- `conferred_by_person_id TEXT REFERENCES person_persons(id) ON DELETE SET NULL` — optional provenance
  (the ordaining authority)
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','revoked'))` —
  **indelible where sacramental**: revocation/laicization is a status flip, never a hard delete
- `effective_from TIMESTAMPTZ NOT NULL DEFAULT now()`, `effective_to TIMESTAMPTZ`
- `source TEXT`, `confidence TEXT` — optional attribution (mirrors the social-account model)
- `created_at`, `updated_at`, `deleted_at`
- A person may hold **several** credentials (concurrent/successive grades, multiple traditions).

> **Offices** are not a new table: a clergy office (pastor of a parish, imam of a mosque, head rabbi)
> is a [membership](membership.md) **`Position`** owned by the organization unit, typed by the
> `religion_office_types` catalog below, **filled** by a membership, with **authority** granted by an
> [authorization](authorization.md) role assignment on that unit. Conferral/appointment/transfer/
> suspension may cite an [order](order.md) (decree) of a religion `order_type`.

**`religion_office_types`** (catalog) — `id` PK; optional `tradition_taxon_id`; `code`; `name`
(translatable); timestamps; soft-delete. *Seed:* pastor, rector, chaplain, imam-of-mosque, head-rabbi,
abbot, head-priest, …

### Lay affiliation (D-ReligiousAffiliation, D-SpecialPII — `pii:special`) — *built (M24, migration `0025`)*

**`religion_affiliation_types`** (catalog, per tradition) — `id` PK (16,1,10); optional `tradition_taxon_id`;
`code`; `name` (translatable); timestamps; soft-delete. *Seed:* generic adherent/member; *Christianity →*
catechumen/baptized/confirmed; *Islam →* shahada; *Judaism →* bar/bat-mitzvah.

**`religion_affiliations`** *(Link `link__affiliated_with`, **`pii:special`**)*
- `id` PK (16,2,3) — RID, `link__affiliated_with` slot
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `religion_id TEXT REFERENCES religion_taxa(id) ON DELETE RESTRICT` — optional faith anchor
- `tradition_unit_id TEXT REFERENCES tenant_units(id) ON DELETE RESTRICT` — optional tradition/body
- `community_unit_id TEXT REFERENCES tenant_units(id) ON DELETE RESTRICT` — optional local community
- `affiliation_type_id TEXT NOT NULL REFERENCES religion_affiliation_types(id) ON DELETE RESTRICT`
- `value_ciphertext BYTEA`, `wrapped_dek BYTEA`, `key_ref TEXT`, `value_blind_index BYTEA` — the
  **envelope-encrypted** belief value + blind index (D-CryptoProvider extended to `pii:special` by
  D-SpecialPII); the affiliation detail is GDPR Art. 9 and stored encrypted at rest
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','lapsed','renounced'))`
- `effective_from TIMESTAMPTZ NOT NULL DEFAULT now()`, `effective_to TIMESTAMPTZ`
- `source TEXT`, `confidence TEXT` — provenance/weight
- `created_at`, `updated_at`, `deleted_at`
- **Crypto-erased on purge** (the `PersonPurged` subscriber destroys the wrapped DEK); reads project
  through [D-PersonReadScope](../architecture/decisions.md); writes audited.

### Discovery (D-Religion discovery surface) — *built (M25, migration `0026`)*

**`religion_site_types`** (catalog, per tradition) — `id` PK; optional `tradition_taxon_id`; `code`;
`name` (translatable); timestamps; soft-delete. *Seed:* church/cathedral/chapel/monastery, mosque,
synagogue, temple, gurdwara, shrine, mission, office, online.

**`religion_sites`** *(Link `link__site_of`)*
- `id` PK — RID, `link__site_of` slot
- `org_unit_id TEXT NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT` — the worship community
- `location_id TEXT NOT NULL REFERENCES location_locations(id) ON DELETE RESTRICT` — the shared place
  ([location](location.md), D-Location)
- `site_type_id TEXT NOT NULL REFERENCES religion_site_types(id) ON DELETE RESTRICT`
- `visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','unlisted','private'))`
- `public_precision TEXT NOT NULL DEFAULT 'exact' CHECK (public_precision IN ('exact','street','neighborhood','city','hidden'))`
  — the **publish-precision projection**: a public read returns the coordinate coarsened **app-side in Go**
  by coordinate rounding (`exact` → full point, `street` → 4 dp ≈ 11 m, `neighborhood` → 3 dp ≈ 110 m,
  `city` → 2 dp ≈ 1.1 km, `hidden` → omitted), the persecuted-community use case. (The original sketch
  coarsened to an **H3 cell**, but D-Location dropped H3 entirely on 2026-06-17 — stock PostGIS image, no
  h3-pg — so coarsening is app-side rounding via `domain.Coarsen`, not a DB cell.) The full coordinate
  stays in [location](location.md); coarsening is a read-time projection here, never a stored loss.
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE` — exactly one primary site per org unit
  (`UNIQUE (org_unit_id) WHERE is_primary AND deleted_at IS NULL`)
- `created_at`, `updated_at`, `deleted_at`

**`religion_service_types`** (catalog, per tradition) — `id` PK; optional `tradition_taxon_id`; `code`;
`name` (translatable); timestamps; soft-delete. *Seed:* main service, youth, prayer (Friday/Jumu'ah,
Shabbat, daily mass, puja, meditation), special.

**`religion_service_schedules`**
- `id` PK — RID, `service_schedule` slot
- `site_id TEXT NOT NULL REFERENCES religion_sites(id) ON DELETE CASCADE`
- `day_of_week SMALLINT` **or** `rrule TEXT` — weekly default or an RRULE subset
- `start_time TIME`, `end_time TIME`, `timezone TEXT NOT NULL` — IANA zone (not a UTC offset)
- `language TEXT REFERENCES geo_… / ISO 639-3` — the service language (drives the language filter)
- `service_type_id TEXT NOT NULL REFERENCES religion_service_types(id) ON DELETE RESTRICT`
- `mode TEXT NOT NULL DEFAULT 'in_person' CHECK (mode IN ('in_person','online','hybrid'))`
- `meeting_url TEXT` — required when `mode ∈ {online,hybrid}` (checked in the application)
- `description TEXT` — translatable
- `created_at`, `updated_at`, `deleted_at`

**`religion_aliases`** (search-only alternative names; never displayed)
- `id` PK; `unit_id TEXT NOT NULL REFERENCES tenant_units(id) ON DELETE CASCADE`
- `alias_text TEXT NOT NULL`
- `alias_type TEXT NOT NULL CHECK (alias_type IN ('nickname','abbreviation','historical','misspelling','transliteration'))`
- `locale TEXT` — optional per-locale alias
- `created_at`, `updated_at`, `deleted_at`
- Indexed for fuzzy search across a unit's translations + aliases regardless of the searcher's locale.

## Conjure API surface

`ReligionService` (`/religion/v1`) — **M22–M24 surface** (`api/religion.conjure.yml`):

| Op | Intent | Perm |
|---|---|---|
| `GET /taxa?rank=&parent=&religion=&query=` · `GET /taxa/{id}` | Read/search the taxonomy (closure-aware) | `religion.read` |
| `POST /taxa` · `PUT /taxa/{id}` · `DELETE /taxa/{id}` · `POST /taxa/{id}/reparent` · `POST /taxonomy/rebuild-closure` | Manage the taxonomy tree (cycle-guarded; closure recomputed) | `religion.catalog.manage` (instance) |
| `GET /taxa/{id}/effective-classifications` · `PUT /taxa/{id}/classifications` | Read resolved / set declared theism tags | `religion.read` / `religion.catalog.manage` |
| `GET·PUT /taxon-ranks` · `/classifications` · `/org-kinds` · `/policy-kinds` | Read / manage the catalogs | `religion.read` / `religion.catalog.manage` (instance) |
| `GET·PUT /units/{unitId}/religion-profile` | Read / set a unit's `OrgProfile` | `religion.read` / `religionorg.manage` (on the unit) |
| `POST·DELETE /units/{unitId}/classifications` | Add / remove a tradition tag (one primary) | `religionorg.manage` (on the unit) |
| `PUT /units/{unitId}/type-overrides` · `GET /units/{unitId}/effective-type` | Set / read the unit theism override (resolved) | `religionorg.manage` / `religion.read` |
| `GET·POST·DELETE /units/{unitId}/religion-policies` | Manage data-driven org policies | `religionorg.manage` (on the unit) |
| `POST /religion-orgs` | Create a top-level body (church-domain org + root unit + profile — M41) | `religion.catalog.manage` (instance) |
| `POST /units/{unitId}/child-orgs` | Create a child org unit + canonical edge (blocked by `excludes_child_creation`) | `religionorg.manage` (on the unit for a person; a service principal instead needs an INSTANCE-WIDE grant — `RequireServiceOrTarget`, GH-39) |
| `GET·PUT /grade-categories` · `/clergy-grades?tradition=` · `/office-types` | Read / manage the clergy catalogs (M23) | `religion.read` / `religion.catalog.manage` (instance) |
| `GET /persons/{id}/clergy-credentials` · `GET /units/{unitId}/clergy-credentials` | List a person's / a unit's clergy credentials | `religion.read` (unit read on the unit) |
| `POST /persons/{id}/clergy-credentials` · `PUT /clergy-credentials/{id}` | Add / status-flip a credential (indelible; no delete) | `clergy.manage` (on the conferring unit) |
| `GET·PUT /affiliation-types` | Read / manage the lay-affiliation catalog (M24) | `religion.read` / `religion.catalog.manage` (instance) |
| `GET /persons/{id}/affiliations` · `POST` · `PUT /affiliations/{id}` · `DELETE /affiliations/{id}` | Read (decrypted) / add / update / remove a lay affiliation (`pii:special`) | `affiliation.manage` |
| `GET·PUT /site-types` · `/service-types` | Read / manage the discovery catalogs (M25) | `religion.read` / `religion.catalog.manage` (instance) |
| `GET·POST /units/{unitId}/sites` · `PUT /sites/{id}` · `DELETE /sites/{id}` | List / attach / edit / remove a unit's sites (over a shared location) | `religion.read` / `site.manage` (on the unit) |
| `GET·POST /sites/{id}/schedules` · `DELETE /schedules/{id}` | List / add / remove a site's service schedules | `religion.read` / `schedule.manage` (on the site's unit) |
| `GET·POST /units/{unitId}/aliases` · `DELETE /units/{unitId}/aliases/{id}` | List / add / remove search-only aliases | `religion.read` / `site.manage` (on the unit) |
| `GET /discovery/sites?lat=&lng=&radiusM=&minLat=…&religion=&language=&dayOfWeek=&onlineOnly=&query=` | Closure+PostGIS discovery search (coarsened coords) | `religion.read` |

Translatable `name` returns as a `locale → text` map (D-i18n, M18 `NamesByID`). Per-unit ops gate on
`religionorg.manage` checked over the **canonical** graph; taxonomy/catalog ops are instance-global.
Clergy-credential writes gate `clergy.manage` over the **canonical** graph against the conferring unit;
lay-affiliation reads/writes gate `affiliation.manage` (person data) — the belief value is decrypted
only for authorized readers and **crypto-erased** on person purge.

**Built (M23/M24):** clergy credentials + catalogs and lay affiliations (`pii:special`) — above.
**Built (M25):** discovery — site/service-type catalogs, sites (over a shared location), per-site service
schedules, search-only aliases, and the closure-aware PostGIS discovery search (`/discovery/sites`) with
app-side `public_precision` coarsening. Clergy *offices* (membership Positions) are still future work.

## Dependencies

- **Calls:** [tenant](tenant.md) (org units, the religion graphs + per-graph closure for search),
  [person](person.md) (clergy/affiliation endpoints), [membership](membership.md) (clergy offices as
  positions), [order](order.md) (appointment/conferral decrees), [authorization](authorization.md)
  (office authority), [location](location.md) (sites + PostGIS proximity/viewport),
  [localization](localization.md) (catalog `name`/`description` locale-maps), [platform](platform.md)
  (the `pkg/crypto` envelope extended to `pii:special` — D-SpecialPII; DB pool; config). **Subscribes**
  to `PersonPurged` to crypto-erase affiliations.
- **Called by:** a consuming discovery/CMS app (e.g. FaithMap) over the public API; [audit](audit.md).

## Authorization touchpoints

Defines/gates: `religion.read` — the instance-wide taxonomy/catalog/discovery reads, satisfied for a
service principal holding the grant or a person via the PDP (RequireServiceOrPerson) — plus
`religion.catalog.manage` (instance), `religionorg.manage`,
`clergy.manage`, `affiliation.manage`, `site.manage`, `schedule.manage` — the unit-scoped ones checked
against the relevant **organization unit** over the **`canonical`** graph (so authority cascades down a
governance subtree exactly as for any unit). Discovery reads pass the **shadow-visibility gate** and the
**`public_precision`** projection. `religion_sites`/`religion_service_schedules`/`religion_aliases`
additionally carry an RLS `visibility = 'public'` SELECT-only bypass (GH-34, migration `0025`, mirroring
`tenant_units_public_read` — see D-RLSDefenseInDepth) so a `religion.read` grant's "instance-wide"
promise holds at the **DB layer** too, not just the PEP: a service principal holding only an
instance-wide grant (`org_id IS NULL`) now actually sees `public` discovery rows, matching what
`SearchSites` already queries. `unlisted`/`private` sites are unaffected — they still require org-scoped
reach (or instance-admin), exactly as `D-ServiceIdentities`' "an instance-wide grant confers no
operational reach" rule requires. `CreateChildOrg` (GH-39) is the one **write** reachable by a machine
subject: it gates via `pep.RequireServiceOrTarget`, which keeps a person's check exactly as
target-scoped as `Require(religionorg.manage, unitID)` always was (unlike `RequireServiceOrPerson`,
whose person arm is the broader `RequireAnywhere` — folding `CreateChildOrg` into that pattern would
have let any person holding `religionorg.manage` anywhere create a child org under any unit, which is
why it needed its own PEP method rather than reusing the read-surface one) and lets a machine subject
in via its flat grant set, checked **instance-wide only** — a principal grant has no unit/subtree
scope yet, so an org confined to one jurisdiction-sync subtree is, in the permission model, actually
instance-wide; only the calling application keeps it pointed at the intended units (an explicitly
open question, not resolved by GH-39). **Neither a clergy grade nor an affiliation is ever an authz input**
(D-ClergyCredential / D-ReligiousAffiliation, parallel to D-Rank) — authority comes only from role
assignments.

## Invariants & safety

- **No hard-coded faith vocabulary.** Taxon ranks, classifications, organization kinds, policy kinds
  (and the deferred grades/office/affiliation/site/service types) are catalog rows; the taxonomy itself
  is data in `religion_taxa`. The only `CHECK` enums are fixed *lifecycle statuses*
  (`active/retired`, and the deferred `active/suspended/revoked`, `public/unlisted/private`, …).
- **Single religion domain, many faiths.** A deployment is the religion domain (L-SingleDomain refined
  by D-Religion); many root `religion`-rank taxa coexist as data — no code branches on which.
- **Taxonomy is a strict tree with a maintained closure.** `religion_taxa.parent_id` is acyclic
  (reparent is cycle-guarded via the closure, like `education_units`); `religion_taxa_closure` is a
  derived relation rebuilt in the same txn; `religion_id` (root) is denormalized from it.
- **Theism resolution is nearest-declared-wins.** A taxon/unit's effective classification is the set on
  the nearest declaring ancestor (closure walk); a unit override (`religion_unit_classifications`) beats
  the inherited taxon set. A read-time projection, never stored.
- **One primary classification per unit** (`religion_org_classifications`, partial-unique on
  `is_primary`); a unit may carry several tags.
- **Governance vs. taxonomy vs. affiliation are separate graphs.** Admin authority cascades only over
  the **`canonical`** graph; `tradition`/`affiliation` are directory-only (D-DirectoryGraphs) — a
  community affiliating with a network inherits **no** admin.
- **Org-policy exclusion** (`excludes_child_creation`) blocks `POST /units/{id}/child-orgs` beneath the
  marked body — a data-driven rule, not code.
- **Clergy credential indelible where sacramental** *(M23, built)* — revocation/laicization is a status
  flip (`active`/`suspended`/`revoked`), never a hard delete; a credential is never an authz input.
- **Affiliation is `pii:special`** *(M24, built)* — envelope-encrypted at rest with a blind index,
  decrypted only for authorized readers, crypto-erased on person purge.
- **Discovery** *(M25, built)* — exactly **one primary site per org unit** (`religion_sites`
  partial-unique on `is_primary`); a site reuses a shared [location](location.md) by FK (RESTRICT), never
  duplicating coordinates; `public_precision` coarsening is an **app-side** read projection
  (`domain.Coarsen` rounding — H3 dropped); `religion_aliases` are **search-only**, never displayed; an
  online/hybrid schedule requires a `meeting_url`.
- **Org-policy exclusion** (`excludes_child_creation`) blocks creating child organizations beneath the
  marked body — a data-driven rule, not code.
- **RLS backstop.** The unit-scoped religion tables carry the defense-in-depth RLS policies keyed on
  `app.readable_units` / `app.writable_units` (D-RLSDefenseInDepth) — behind the authoritative PDP +
  shadow gate, not a replacement.

## Open seams / future

- **Facets & dashboards (M58).** [D-ObjectFacets](../architecture/decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope) lands filters + a stats endpoint + a console dashboard
  for this module's listable types: `GET /taxa/stats` over `rankId`, `parent`, `religionId` and theism `classification`. **Lay affiliation has no facet** — the belief value is envelope-encrypted `pii:special` (D-ReligiousAffiliation / D-SpecialPII), which D-ObjectFacets rule 1 excludes outright. Plus the module's first ontology-registry entry.
  Facets and proposed charts are catalogued in [facets.md](../architecture/facets.md).

- **Rite-of-passage / life-cycle records** (baptism, bar/bat-mitzvah, marriage rites, funerals) as a
  generic catalog-typed `pii:special` observance — reserved as **DS-49**.
- **Location-scoped role assignments** (a consuming app's per-site "campus admin") — today an assignment's
  scope is `unit|subtree`; reserved as **DS-50**.
- **Cross-tradition clergy comparator** — none exists (no NATO-STANAG analog); `clergy_grades.ordinal`
  orders only within a tradition (**DS-43** parallel, parked).
- **Content / CMS** (pages, blocks, themes, slugs, content-i18n groups) stays in the consuming app — out
  of scope for this identity/directory service.
- **Sacred-text / doctrine catalogs** and **inter-faith mapping** are intentionally not modeled; additive
  if a real need appears.
