# Module: person

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.person_*`

## Purpose

Owns the **personnel directory** — the core aggregate of the whole service. A person is an
individual record, **instance-global** (one record per individual for the deployment, never
per-unit; D-PersonGlobal). A person exists **independently** of any login account
(L-AccountOptional) and of any unit membership: a roster of people who never sign in and
belong to no unit yet is first-class. A person carries exactly **one rank** from the
system-wide scheme — a **directory attribute, not a permission** (D-Rank).

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Person` (the core
aggregate; holds one rank via the `HOLDS_RANK` link — a directory attribute, **never** an authz
input) and the per-person `Name variant`. **Links:** the temporal `Citizenship` and `Residence`
(person → country). **Actions:** `CreatePerson`/`UpdatePerson`, `DeactivatePerson`/`PurgePerson`
(crypto-erase, emits `PersonPurged`), `AssignRank`, citizenship/residence/variant upserts — audited,
`action__<type>` RID.

- **Person** (aggregate root) — names (canonical + CLDR structured parts), bio attributes
  (`birthdate`, `sex`, `country_of_birth`), structured/free attributes, status, optional `rank_id`,
  lifecycle timestamps.
- **Citizenship** — a person's nationality in a country, effective-dated; a person may hold several
  (D-Geo).
- **Residence** — a person's effective-dated residence in a country/region (D-Geo).

(Accounts live in [identity-federation](identity-federation.md) — at most one per person, and
that account carries the person's login points across IdPs (e.g. Google + Keycloak);
memberships in [membership](membership.md); rank definitions in [rank](rank.md). Person only
*points* at a rank.)

## Data model

Conventions per [conventions.md](../architecture/conventions.md).

**`person_persons`**
- `id` PK
- `code TEXT` — **optional** stable, locale-agnostic external identifier (e.g. a personnel /
  service number) for external-system reference (D-Code); `UNIQUE WHERE code IS NOT NULL AND
  deleted_at IS NULL` — `pii:basic`
- `display_name TEXT NOT NULL` — the **canonical** full name form; remains authoritative for
  search/display (the structured parts below are advisory) — `pii:basic`
- **Structured name parts — the Unicode CLDR Person Names fixed field set** (all optional, all
  `pii:basic`; D-PersonNamesCLDR). `display_name` stays authoritative; these parts are advisory and
  used for locale-aware formatting. Anything rarer (Arabic nasab chains, 4+ surnames, clan/tribal)
  is **not** a typed field — it lives in the authoritative `display_name` (and a per-locale variant
  for a Latin form). Do not over-structure (W3C "personal names around the world"):
  - `title TEXT` — honorific / title prefix (`Dr.`, `Rev.`, `Ms.`); rank covers most military ones
  - `given TEXT` — first / forename
  - `given2 TEXT` — second given name; **also where the Slavic по-батькові / отчество (and Icelandic
    patronymic) lives** — pure CLDR has no dedicated patronymic field, so formal Slavic address
    ("Тарас Григорович") is assembled by locale-aware formatting from `given` + `given2`, not a
    typed patronymic field
  - `surname TEXT` — primary surname
  - `surname_prefix TEXT` — nobiliary / genealogical particle (`van`, `von`, `de`, `bin`)
  - `surname2 TEXT` — second surname (Hispanic / Lusophone double surname)
  - `generation TEXT` — generational suffix (`Jr.`, `Sr.`, `III`)
  - `credentials TEXT` — post-nominal credentials (`PhD`, `MD`)
  - `preferred TEXT` — known-as / nickname
- `birthdate DATE` — calendar date of birth (a `DATE`, not a `TIMESTAMPTZ` — it is a day, not an
  instant); nullable. Partial/approximate (year-only) dates are an open seam — `pii:basic`
- `sex TEXT NOT NULL DEFAULT 'not_known' CHECK (sex IN ('not_known','male','female',
  'not_applicable'))` — **biological sex, ISO/IEC 5218** (stored as readable `TEXT` per the
  `TEXT`+`CHECK` convention, not the numeric `0/1/2/9`); **not** GDPR Art. 9 — gender *identity*
  (which would be `pii:special`) is out of scope (D-PersonBio) — `pii:basic`
- `country_of_birth CHAR(2) REFERENCES geo_countries(code) ON DELETE RESTRICT` — nullable; the
  person's country of birth (D-Geo) — `pii:basic`
- `attributes JSONB NOT NULL DEFAULT '{}'` — the long-tail directory fields (service number,
  contact, etc.); column-ize a key once it is shared/queried (escape-hatch discipline) —
  `pii:special` **(ceiling)**: a grab-bag may hold up to special-category data, so it is tagged at
  the ceiling (D-PIITiers); special-category fields must not land here without the envelope seam
- `rank_id TEXT REFERENCES rank_ranks(id) ON DELETE RESTRICT` — one rank, nullable (a person
  may be unranked); restrict so a rank in use cannot be deleted out from under a person
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','deactivated','purged'))`
- `deactivated_at TIMESTAMPTZ`, `purge_after TIMESTAMPTZ` — reversibility window
- `created_at`, `updated_at`, `deleted_at`

Indexes: `rank_id`; a trigram/`citext` index on `display_name` for directory search (added when
search is built); partial unique on any natural key the operator configures (none mandated).

**`person_name_variants`** (transliteration / alternate name forms)
- `id` PK
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `locale TEXT NOT NULL REFERENCES i18n_locales(code) ON UPDATE RESTRICT` — the locale/script
  this form is for (e.g. `ukr` native, `eng` Latin transliteration)
- `display_name TEXT NOT NULL` — `pii:basic`. A variant is a **full** transliterated name form, so
  it carries the **same CLDR structured name parts** as `person_persons` (all optional, `pii:basic`;
  D-PersonNamesCLDR): `title`, `given`, `given2`, `surname`, `surname_prefix`, `surname2`,
  `generation`, `credentials`, `preferred`. (Bio fields `birthdate`/`sex`, `country_of_birth`,
  citizenships and residences are **not** per-variant — they live on `person_persons` / their own
  tables.)
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE`
- `created_at`, `updated_at`
- `UNIQUE (person_id, locale)`

All other columns across both tables (`id`, `rank_id`, `status`, `locale`, `is_primary`,
lifecycle timestamps) are `pii:none` (D-PIITiers); the name parts, `birthdate`, `sex`, and
`country_of_birth` are `pii:basic` as tiered above.

**`person_citizenships`** (effective-dated; a person may hold several — D-Geo)
- `id` PK
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `country CHAR(2) NOT NULL REFERENCES geo_countries(code) ON DELETE RESTRICT` — `pii:basic`
- `basis TEXT NOT NULL DEFAULT 'other' CHECK (basis IN ('birth','descent','naturalization','other'))`
  — how the citizenship was acquired
- `acquired_on DATE`, `lost_on DATE` — effective window (nullable) — `pii:basic`
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE` — the person's primary nationality (at most one active)
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** one **active** citizenship per `(person_id, country)`:
  `UNIQUE (person_id, country) WHERE lost_on IS NULL AND deleted_at IS NULL`.
- Index `(person_id) WHERE deleted_at IS NULL`.

**`person_residences`** (effective-dated residence history — D-Geo)
- `id` PK
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `country CHAR(2) NOT NULL REFERENCES geo_countries(code) ON DELETE RESTRICT` — `pii:contact`
- `region TEXT` — optional sub-national region / locality — `pii:contact`
- `valid_from DATE NOT NULL`, `valid_to DATE` — effective window (`valid_to` NULL = current) —
  `pii:contact`
- `created_at`, `updated_at`, `deleted_at`
- Index `(person_id) WHERE deleted_at IS NULL`.

All `id`/`person_id`/lifecycle columns on both tables are `pii:none`; `country`/dates on citizenship
are `pii:basic`, residence columns are `pii:contact` (locator data) — D-PIITiers.

Person names are **per-record data managed by the person's admins** — *not* the instance-admin
[localization](localization.md) translation store (D-i18n). A person has one canonical
`display_name` plus zero or more locale-tagged variants (the transliterations the user asked
for, e.g. "Тарас" / "Taras"). Responses return the canonical name plus the variants; clients pick
by locale.

### PII governance

`person_persons` is the system's primary PII store. Discipline carried from drafts'
DATA-GOVERNANCE:
- Treat `display_name`, CLDR name parts, citizenships, residences, and `attributes` as personal
  data. Log person identifiers with `werror.UnsafeParam` (redacted), never raw in service logs.
- Erasure is the **purge** path (below): mutable PII columns are NULLed, the `id` is kept as a
  tombstone so audit history (which references the id) stays intact. The purge erasure list covers
  **every** `pii:basic`/`pii:contact`/`pii:special` column on all person tables — including the CLDR
  name parts, `birthdate`, `sex`, `country_of_birth` (D-PersonNamesCLDR / D-PersonBio), and the rows
  of `person_citizenships` and `person_residences` (D-Geo), plus the JSONB `attributes`.
  [document](document.md) rows for the person — including its **personal codes** (crypto-erased by
  destroying the wrapped DEK; D-CryptoProvider) — are erased by the `document` module's `PersonPurged`
  subscriber.
  [order](order.md) records, being **immutable legal records**, are **retained** (not erased) under
  the audit-style tombstone exception — the person stays resolvable-or-redacted via this tombstone
  (see order.md *PII governance*).
- **PII tiers are classified on every column** via `COMMENT ON COLUMN ... IS 'pii:<tier>'`
  (D-PIITiers; see the data model above and [conventions.md](../architecture/conventions.md)).
  This static classification is the companion to the two runtime controls: `werror.UnsafeParam`
  log redaction and the purge erasure path.

## Conjure API surface

`PersonService`:

| Op | Intent | Perm |
|---|---|---|
| `POST /persons` | Create a person (no account, no unit needed) | `person.create` |
| `GET /persons/{id}` | Read one | `person.read` |
| `PUT /persons/{id}` | Update names (canonical + CLDR parts), `birthdate`, `sex`, `country_of_birth`, attributes | `person.update` |
| `GET /persons` | Search/list the directory (token-paginated) | `person.read` |
| `PUT /persons/{id}/rank` | Set/clear the person's rank | `person.rank.assign` |
| `POST /persons/{id}/deactivate` | Begin reversible deactivation (grace window) | `person.lifecycle` |
| `POST /persons/{id}/reactivate` | Cancel deactivation within grace | `person.lifecycle` |
| `POST /persons/{id}/purge` | Hard-erase PII after grace (idempotent) | `person.purge` |
| `PUT /persons/{id}/name-variants` | Upsert locale name forms (transliteration) | `person.update` |
| `DELETE /persons/{id}/name-variants/{locale}` | Remove a name variant | `person.update` |
| `GET /persons/{id}/citizenships` | List a person's citizenships | `person.read` |
| `PUT /persons/{id}/citizenships` | Upsert a citizenship (country, basis, dates, primary) | `person.update` |
| `DELETE /persons/{id}/citizenships/{country}` | Remove a citizenship | `person.update` |
| `GET /persons/{id}/residences` | List a person's residence history | `person.read` |
| `PUT /persons/{id}/residences` | Upsert a residence row | `person.update` |
| `DELETE /persons/{id}/residences/{id}` | Remove a residence row | `person.update` |

Citizenship and residence reads follow the same **read-scope rule** as the person (D-PersonReadScope);
their writes are audited (D-Audit) and erased on **purge**. `country_of_birth`, citizenship `country`,
and residence `country` are validated against the `geo_countries` registry (D-Geo).

Read endpoints that list people *by unit* are served by [membership](membership.md) and pass
the shadow gate; `PersonService` directory reads are gated on `person.read` per the **read-scope
rule** below (D-PersonReadScope) — `GET /persons/{id}` checks the single-person membership
intersection, and `GET /persons` returns the **union** of people reachable that way (membership-less
people only on the instance plane).

A rank change may also be applied as the effect of a [order](order.md) `rank-change` order (the наказ
that is its legal basis, D-Orders): on order **issue**, person **subscribes** to the order's
`RankChangeOrdered` event and sets `rank_id` **in the issue transaction** (D-OrderApply), emitting
`PersonRankChanged`. Because rank is a person **column**, not a row, that provenance is recorded in
the **audit payload** — there is **no** `order_item_id` FK on the person (unlike
[membership](membership.md), which links provenance structurally). "Which order raised this rank?" is
therefore answered via [audit](audit.md), not a person field.

## Dependencies

- **Calls:** [rank](rank.md) (validate `rank_id` exists), [localization](localization.md)
  (validate name-variant `locale` codes), the **`geo_countries`** registry
  ([platform](platform.md); validate `country_of_birth` / citizenship / residence country codes).
  [platform](platform.md) for infra. Emits `PersonCreated`, `PersonDeactivated`, `PersonRankChanged`,
  `PersonPurged` events. **Subscribes** to [order](order.md)'s `RankChangeOrdered` event and applies
  the rank change in the issue transaction (D-OrderApply).
- **Called by:** [membership](membership.md) (a membership references a person),
  [identity-federation](identity-federation.md) (an account links to a person),
  [authorization](authorization.md) (assignment subject is a person id),
  [document](document.md) (a document is attached to a person; its `PersonPurged` subscriber erases
  the person's documents), [order](order.md) (an order item targets a person), [audit](audit.md).

## Authorization touchpoints

Defines/gates: `person.create`, `person.read`, `person.update`, `person.rank.assign`,
`person.lifecycle`, `person.purge`. The module never decides access — it calls the PDP, and
**never reads rank to make a decision**.

**Read-scope rule (canonical; D-PersonReadScope).** A person is instance-global with no unit FK
(D-PersonGlobal), but the PDP question is unit-keyed, so a person's read scope is **projected through
its memberships**. A subject may read person **P** iff **either** (1) the subject is on the
**instance plane** — an active instance admin, or holds `person.read` as an **instance-scope** grant
— **or** (2) the subject's **effective readable unit-set** (D-RLSDefenseInDepth: `subtree`
read-bearing assignments expanded over their graph's closure ∪ `unit`-scope `*.read` targets)
**intersects** the units **P** belongs to via **active** memberships ([membership](membership.md)),
with the **shadow-visibility gate** applied to that join. A **membership-less** person belongs to no
unit, so the intersection is empty and they are readable **only on the instance plane** (see
*Invariants*). There is **no "self" read exemption**. [document](document.md) reads inherit this rule
through the holder.

## Invariants & safety

- A person needs **no** account and **no** membership to exist. A **membership-less** person is
  reachable only on the **instance plane** for reads (no unit-scoped grant reaches them;
  D-PersonReadScope).
- A person holds **at most one** rank; deleting a rank still in use is blocked
  (`ON DELETE RESTRICT`).
- **Reversible lifecycle:** `active → deactivated` (sets `deactivated_at` + `purge_after`,
  reversible within grace) `→ purged` (PII NULLed, `id` retained as tombstone). Purge refuses
  before `purge_after`.
- Rank assignment grants no authority (D-Rank); enforced by convention + review, documented so
  no implementer couples them.
- Optional external `code` is unique among active persons when set; name variants are unique per
  `(person, locale)` and are person-managed data, not the instance translation store.
- **Names follow the CLDR fixed field set** (D-PersonNamesCLDR): `display_name` is authoritative;
  there is **no dedicated patronymic field** — the по-батькові lives in `given2`, and formal Slavic
  address is assembled by locale-aware formatting from `given` + `given2`.
- A person may hold **several citizenships** (one active row per `(person, country)`); `is_primary`
  marks at most one. `country_of_birth`, citizenship and residence countries are FK-validated against
  `geo_countries` (D-Geo).

## Open seams / future

- Cross-deployment / federation of person identity is **out of scope** (single domain).
- `attributes` JSONB awaits column-ization as real directory fields stabilize.
- Self-service subject-rights export is a future additive endpoint; MVP erasure is the
  operator-driven purge.
- **Partial / approximate birthdate** (year-only or year-month) is parked — the default is a full
  `DATE` (open-questions DS-38).
- **Gender identity** (distinct from ISO 5218 `sex`) is parked — it is `pii:special` (GDPR Art. 9)
  and must not be stored until the envelope-encryption seam ships (D-PersonBio, DS-29).
- **CLDR long tail** (Arabic nasab chains, 4+ surnames, clan/tribal names) is intentionally **not**
  typed — it rides in the authoritative `display_name` (+ a per-locale variant); promoting any of it
  to a field would be an additive change (D-PersonNamesCLDR).
- **Richer geography** (structured sub-national regions as a registry, geocoding) stays out of scope;
  `person_residences.region` is free text for now (D-Geo).
