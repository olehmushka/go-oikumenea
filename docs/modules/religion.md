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

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) —
**Objects (catalogs):** `Religion`, `TraditionFamily`, `SubTradition`, `OrgKind`, `OrgProfile`,
`OrgPolicy`, `ClergyGrade`, `GradeCategory`, `OfficeType`, `AffiliationType`, `SiteType`, `ServiceType`,
`ServiceSchedule`, `Alias`. **Reuses** `Unit` ([tenant](tenant.md)) for the organization nodes and
`Position` ([membership](membership.md)) for clergy offices.
**Links:** `link__clergy_credential` (clergy standing), `link__affiliated_with` (lay affiliation,
`pii:special`), `link__site_of` (organization ↔ location).
**Actions:** `ConferCredential`, `SuspendCredential`, `AppointClergy`, `RecordAffiliation`,
`AttachSite`, plus catalog edits — each audited, `action__<type>` RID.

- **Taxonomy** (`Religion` → `TraditionFamily` → `SubTradition`) — the faith classification tree, all
  instance-admin catalogs.
- **Organization** — a religious body (denomination/jurisdiction/community/mosque/monastery/…) is a
  **`tenant_units` row** placed in religion graphs; `OrgProfile` holds its faith attributes and
  `OrgPolicy` its data-driven eligibility rules.
- **Clergy** — `ClergyGrade` (a per-tradition ordered catalog) + the reified credential link; offices
  are `Position`s with authority from role assignments.
- **Affiliation** — the reified lay-belief link (`pii:special`).
- **Discovery** — `religion_sites` (link to [location](location.md)), `ServiceSchedule`, `Alias`.

## Data model

Conventions (URN RID PKs (D-ResourceIdentifiers), `TIMESTAMPTZ`, `set_updated_at`, soft-delete,
`TEXT`+`CHECK` only for **fixed lifecycle statuses** — never for faith vocabulary) per
[conventions.md](../architecture/conventions.md).

### Taxonomy catalogs

**`religion_religions`** (top-level catalog) — `id` PK; `code` (stable, unique among active);
`name` (translatable); `description` (translatable); `icon TEXT?`; `sort_order INT`; timestamps;
soft-delete. *Seed (examples, operator-editable):* Christianity, Islam, Judaism, Hinduism, Buddhism,
Sikhism, Jainism, Bahá'í, Shinto, Taoism, traditional/indigenous, other.

**`religion_tradition_families`** (nested under a religion) — `id` PK; `religion_id` FK →
`religion_religions`; `code` (unique among active **within a religion**); `name` (translatable);
`description` (translatable); `icon TEXT?`; `sort_order`; timestamps; soft-delete. *Seed (per religion):
Christianity →* Catholic/Orthodox/Protestant (Baptist, Methodist, Pentecostal, …); *Islam →*
Sunni/Shia/Ibadi; *Judaism →* Orthodox/Conservative/Reform; *Buddhism →* Theravada/Mahayana/Vajrayana;
*Hinduism →* Vaishnava/Shaiva/Shakta; …

**`religion_sub_traditions`** (optional generic sub-classification — rite / school / madhhab /
sampradaya) — `id` PK; `tradition_family_id` FK; `code`; `name` (translatable); `sort_order`;
timestamps; soft-delete. *Seed: Christianity →* Latin/Byzantine; *Islam →* Hanafi/Maliki/Shafi'i/
Hanbali/Ja'fari; …

### Organization (reuses `tenant_units`)

**`religion_org_kinds`** (catalog naming each organizational level, per religion) — `id` PK; optional
`religion_id` FK (NULL = generic across faiths); `code`; `name` (translatable); `ordinal INT` (relative
depth/rank of the level); timestamps; soft-delete. *Seed: Christianity →* denomination/jurisdiction/
congregation/campus; *Islam →* school/community/mosque-community; *Judaism →* movement/community;
*Buddhism →* school/monastery/sangha.

> **Graphs.** Organization nodes are `tenant_units` placed in **three seeded religion graphs**
> ([tenant](tenant.md) `tenant_graphs`, D-Graphs/D-DirectoryGraphs): **`canonical`** (governance /
> jurisdictional tree, **authority-bearing** — the PDP cascades `subtree` grants here), **`tradition`**
> (taxonomic placement, **directory-only**), **`affiliation`** (voluntary association DAG,
> **directory-only**). A unit's `tenant_units.unit_kind` is set from a `religion_org_kinds.code` (a
> descriptive label, never branched on).

**`religion_org_profiles`** (per-organization faith attributes; one row per religious-body unit) —
- `unit_id TEXT PRIMARY KEY REFERENCES tenant_units(id) ON DELETE RESTRICT`
- `religion_id TEXT NOT NULL REFERENCES religion_religions(id) ON DELETE RESTRICT`
- `tradition_family_id TEXT REFERENCES religion_tradition_families(id) ON DELETE RESTRICT` — optional
- `sub_tradition_id TEXT REFERENCES religion_sub_traditions(id) ON DELETE RESTRICT` — optional
- `short_code TEXT` — optional abbreviation (display/search aid)
- `created_at`, `updated_at`, `deleted_at`
- `CHECK`: a chosen `tradition_family_id` must belong to `religion_id`; `sub_tradition_id` to that
  family (validated in the application).

**`religion_org_policies`** (generic, data-driven eligibility/exclusion — replaces any faith-specific
doctrinal flag) — `id` PK; `unit_id` FK; `policy_kind_id` → a small `religion_policy_kinds` catalog
(e.g. `excludes_child_creation`, `excluded_body`); `reason TEXT`; `decided_by_person_id`;
`decided_at`; timestamps. *Example:* a body marked `excludes_child_creation` blocks creating
congregations beneath it (the generic analog of the dropped Christianity-specific "Nicene gate").

### Clergy (D-ClergyCredential)

**`religion_grade_categories`** (per-tradition grouping of grades — generic, replaces a fixed
major/minor enum) — `id` PK; optional `tradition_family_id`; `code`; `name` (translatable);
`ordinal`; timestamps; soft-delete.

**`religion_clergy_grades`** (ordered, per-tradition catalog) — `id` PK; optional `tradition_family_id`
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

**`religion_office_types`** (catalog) — `id` PK; optional `tradition_family_id`; `code`; `name`
(translatable); timestamps; soft-delete. *Seed:* pastor, rector, chaplain, imam-of-mosque, head-rabbi,
abbot, head-priest, …

### Lay affiliation (D-ReligiousAffiliation, D-SpecialPII — `pii:special`)

**`religion_affiliation_types`** (catalog, per tradition) — `id` PK; optional `tradition_family_id`;
`code`; `name` (translatable); timestamps; soft-delete. *Seed:* generic adherent/member; *Christianity →*
catechumen/baptized/confirmed; *Islam →* shahada; *Judaism →* bar/bat-mitzvah.

**`religion_affiliations`** *(Link `link__affiliated_with`, **`pii:special`**)*
- `id` PK — RID, `link__affiliated_with` slot
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `religion_id TEXT REFERENCES religion_religions(id) ON DELETE RESTRICT` — optional faith anchor
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

### Discovery (D-Religion discovery surface)

**`religion_site_types`** (catalog, per tradition) — `id` PK; optional `tradition_family_id`; `code`;
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
  — the **publish-precision projection**: a public read returns the coordinate coarsened to the matching
  H3 cell (`exact` → point, `street` → res-9, `neighborhood` → res-7, `city` → res-5, `hidden` → none),
  the persecuted-community use case. The full coordinate stays in [location](location.md); coarsening is
  a read-time projection here, never a stored loss.
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE` — exactly one primary site per org unit
  (`UNIQUE (org_unit_id) WHERE is_primary AND deleted_at IS NULL`)
- `created_at`, `updated_at`, `deleted_at`

**`religion_service_types`** (catalog, per tradition) — `id` PK; optional `tradition_family_id`; `code`;
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

`ReligionService`:

| Op | Intent | Perm |
|---|---|---|
| `GET /religion/religions` · `…/tradition-families` · `…/sub-traditions` | Read the taxonomy catalogs | `religion.read` |
| `POST/PUT/DELETE /religion/{taxonomy}` | Manage taxonomy catalogs | `religion.catalog.manage` (instance) |
| `GET /religion/org-kinds` · `…/grade-categories` · `…/clergy-grades` · `…/office-types` · `…/affiliation-types` · `…/site-types` · `…/service-types` · `…/policy-kinds` | Read the per-tradition catalogs | `religion.read` |
| `POST/PUT/DELETE` on the above | Manage the per-tradition catalogs | `religion.catalog.manage` (instance) |
| `PUT /units/{unitId}/religion-profile` | Set a unit's `OrgProfile` (religion/tradition/sub-tradition) | `religionorg.manage` (on the unit) |
| `POST /units/{unitId}/religion-policies` | Add a data-driven org policy | `religionorg.manage` (on the unit) |
| `POST /persons/{personId}/clergy-credentials` | Confer a clergy credential | `clergy.manage` (on the org unit) |
| `POST /clergy-credentials/{id}/suspend` · `…/revoke` | Status flip (indelible) | `clergy.manage` |
| `POST /persons/{personId}/affiliations` | Record a lay affiliation (`pii:special`) | `affiliation.manage` (holder-scoped) |
| `POST /units/{unitId}/sites` | Attach a site (→ a `location`) | `site.manage` (on the unit) |
| `POST /sites/{siteId}/schedules` | Add a service schedule | `schedule.manage` (on the unit) |
| `POST /units/{unitId}/aliases` | Add a search alias | `religionorg.manage` (on the unit) |
| `GET /religion/search?near=&radiusM=&religion=&tradition=&serviceLanguage=&serviceDay=&online=&q=` | Discovery search (closure + PostGIS + filters) | `religion.read` + shadow gate + precision projection |

Translatable `name`/`description` return as `locale → text` maps. Clergy offices are created/filled via
the existing [membership](membership.md) position/fill endpoints; appointment decrees via
[order](order.md).

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

Defines/gates: `religion.read`, `religion.catalog.manage` (instance), `religionorg.manage`,
`clergy.manage`, `affiliation.manage`, `site.manage`, `schedule.manage` — the unit-scoped ones checked
against the relevant **organization unit** over the **`canonical`** graph (so authority cascades down a
governance subtree exactly as for any unit). Discovery reads pass the **shadow-visibility gate** and the
**`public_precision`** projection. **Neither a clergy grade nor an affiliation is ever an authz input**
(D-ClergyCredential / D-ReligiousAffiliation, parallel to D-Rank) — authority comes only from role
assignments.

## Invariants & safety

- **No hard-coded faith vocabulary.** Organization kinds, grades, office/affiliation/site/service types,
  and sub-traditions are catalog rows; the only `CHECK` enums are fixed *lifecycle statuses*
  (`active/suspended/revoked`, `public/unlisted/private`, `in_person/online/hybrid`, alias kinds) and
  `public_precision` (a privacy mechanism, not a faith term).
- **Single religion domain, many faiths.** A deployment is the religion domain (L-SingleDomain refined
  by D-Religion); many `religion_religions` coexist as data — no code branches on which.
- **Governance vs. taxonomy vs. affiliation are separate graphs.** Admin authority cascades only over
  the **`canonical`** graph; `tradition`/`affiliation` are directory-only (D-DirectoryGraphs) — a
  community affiliating with a network inherits **no** admin.
- **Clergy credential indelible where sacramental** — status flip, never delete; a person may hold many.
- **One primary site per org unit** (unique partial index); a site references a shared `location` and
  owns its own visibility/precision.
- **Affiliation is `pii:special`** — envelope-encrypted + blind-indexed (D-SpecialPII), crypto-erased on
  purge, holder-scoped reads, audited writes.
- **Org-policy exclusion** (`excludes_child_creation`) blocks creating child organizations beneath the
  marked body — a data-driven rule, not code.
- **RLS backstop.** The unit-scoped religion tables carry the defense-in-depth RLS policies keyed on
  `app.readable_units` / `app.writable_units` (D-RLSDefenseInDepth) — behind the authoritative PDP +
  shadow gate, not a replacement.

## Open seams / future

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
