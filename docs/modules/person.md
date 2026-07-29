# Module: person

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.person_*`

## Purpose

Owns the **personnel directory** — the core aggregate of the whole service. A person is an
individual record, **instance-global** (one record per individual for the deployment, never
per-unit; D-PersonGlobal). A person exists **independently** of any login account
(L-AccountOptional) and of any unit membership: a roster of people who never sign in and
belong to no unit yet is first-class. A person carries at most **one rank per rank system** from the
system-wide scheme (D-Rank, extended by D-RankSystems) — a **directory attribute, not a permission**;
a single-system deployment still holds at most one rank, a multi-track one (university/church) may
carry concurrent standings.

## Module structure (D-PersonModuleSplit)

Internally the person module is **three Go modules behind one Conjure `PersonService`**, split by
**data sensitivity + change cadence** (review-2026-07 R-09): `internal/person` (**core** — identity,
CLDR names, bio, ranks, read-scope, merge/purge orchestration), [personprofile](personprofile.md)
(citizenships, residences, addresses, contact/social channels, `SPEAKS` languages, person↔person
relationships, non-encrypted institutional ties), and [personsensitive](personsensitive.md)
(physical identity/ethnicity, overlays, watchlists + sanctions, encrypted party membership — the whole
envelope-crypto surface). One `internal/person/transport` `Service` composes the three and delegates
each handler to its owner. The split is **code-only**: the `oikumenea.person_*` schema and the
`PersonService` contract are unchanged. **Data-model ownership follows the concern:** this doc owns the
**core** entities — `person_persons`, the `HOLDS_RANK` link (`person_ranks`), and `person_name_variants`
(incl. the alias `variant_kind` — aliases are still names) — plus the read-scope rule, the lifecycle,
the PII-governance purge summary, and the **single `PersonService` API surface**;
[personprofile.md](personprofile.md) owns the data model for citizenships / residences / addresses /
contact & social channels / `SPEAKS` languages / person↔person relationships / non-encrypted
institutional ties, and [personsensitive.md](personsensitive.md) owns the sensitive/encrypted overlays.
The **data layer is fully split**: each concern owns its own `adapters`/`queries` + generated sqlc
package over its own repository port (`Repository` / `ProfileRepository` / `SensitiveRepository`),
`domain.Person` is lean (the transport composes profile child-slices), and purge fans out via
`PersonPurged` — core `repo.Purge` scrubs only core tables, while `personprofile`/`personsensitive` (and
`education`/`company`/`document`/…) erase their own rows via `SubscribePersonPurge`; the R-08 lint
enforces per-`person_*`-table ownership. D-PersonModuleSplit is **fully delivered** (the entity
data-model move landed 2026-07-09).

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Person` (the core
aggregate; holds one rank **per rank system** via the reified `HOLDS_RANK` link — a directory
attribute, **never** an authz input), the per-person `Name variant`, the contact channels `Email`/`Phone`/`Call sign`, the standalone
`Social account` (D-PersonSocialChannels), and the instance-admin `Email type`/`Phone type`/`Platform`/
`Relation type` catalogs. **Links:** the reified `HOLDS_RANK` (person → rank, one per rank system;
D-Rank), the temporal `Citizenship` and `Residence` (person → country),
`HOLDS_EMAIL`/`HOLDS_PHONE`/`HOLDS_CALL_SIGN` (person → channel), `REACHABLE_ON` (phone/email →
platform) and `HOLDS_ACCOUNT` (person → social account, carrying `source`/`confidence`; both
D-PersonSocialChannels), and the **person↔person** ties `PARTNERED_WITH`/`KIN_PARENT_OF`/`GUARDIAN_OF`/
`SPONSOR_OF`/`NEXT_OF_KIN`/`ASSOCIATED_WITH` (D-PersonRelationships; the scoped `SOCIAL_TIE` is
**deferred — not built**). **Actions:**
`CreatePerson`/`UpdatePerson`, `DeactivatePerson`/`PurgePerson` (crypto-erase, emits `PersonPurged`),
`AssignRank`, citizenship/residence/variant/email/phone/call-sign/messenger-link/social-account upserts,
and partnership/kinship/guardianship/sponsorship/next-of-kin/association upserts — audited,
`action__<type>` RID.

- **Person** (aggregate root) — names (canonical + CLDR structured parts), bio attributes
  (`birthdate`, `date_of_death`, `sex`, `country_of_birth`), structured/free attributes, status,
  lifecycle timestamps. Ranks are held via the `person_ranks` link (one per rank system), not a column.
- **Person rank** (`HOLDS_RANK` link) — the rank a person holds in one rank system; at most one active
  per `(person, system)`. `system_id` is derived from the rank. A directory attribute (D-Rank).
- **Citizenship** — a person's nationality in a country, effective-dated; a person may hold several
  (D-Geo).
- **Residence** — a person's effective-dated residence in a country/region (D-Geo).
- **Email / Phone / Call sign** — a person's contact/identity channels, multi-valued, each a
  catalog-typed (email/phone) or free (call sign) child row; `is_primary` marks at most one active per
  channel (D-PersonContactChannels).
- **Email type / Phone type** — instance-admin catalogs (`code` + translatable `name`) for the
  email/phone `kind` (D-Code/D-i18n).
- **Messenger link / Social account** — a person's social-network & messenger presence
  (D-PersonSocialChannels): a *messenger link* annotates an existing email/phone with reachability on a
  platform; a *social account* is a standalone catalog-typed handle carrying a stable platform id, a
  rename history, verification flags, and sourced/weighted attribution.
- **Platform** — instance-admin catalog (`code` + translatable `name`, `category ∈ messenger|social`)
  of the social networks / messengers a person may appear on (D-PersonSocialChannels).
- **Person↔person relationships** — per-type reified self-links between two in-directory persons
  (D-PersonRelationships): *partnership* (marriage/engagement), *kinship* (`parent_of`), *guardianship*,
  *sponsorship* (godparent/advisor/mentor), *next-of-kin* (nomination), *association* (COI/no-contact),
  and *social link* (friend/follower). Each mirrors the `membership_memberships` temporal-link shape.
- **Relation type** — instance-admin catalog (`code` + translatable `name` + `category`) for the
  open-ended relation labels (sponsorship / association / next-of-kin kinds; D-PersonRelationships).

(Accounts live in [identity-federation](identity-federation.md) — at most one per person, and
that account carries the person's login points across IdPs (e.g. Google + Keycloak);
memberships in [membership](membership.md); rank definitions in [rank](rank.md). Person only
*points* at ranks, one per rank system.)

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
- `date_of_death DATE` — calendar date of death (a `DATE`, not a `TIMESTAMPTZ`); nullable. A **bio
  attribute, not a lifecycle state** — it does **not** transition `status` to
  `deactivated`/`purged` (a deceased person stays an active directory record; D-PersonBio M12
  amendment). Partial/approximate dates share `birthdate`'s open seam — `pii:basic`
- `sex TEXT NOT NULL DEFAULT 'not_known' CHECK (sex IN ('not_known','male','female',
  'not_applicable'))` — **biological sex, ISO/IEC 5218** (stored as readable `TEXT` per the
  `TEXT`+`CHECK` convention, not the numeric `0/1/2/9`); **not** GDPR Art. 9 — gender *identity*
  (which would be `pii:special`) is out of scope (D-PersonBio) — `pii:basic`
- `country_of_birth_id uuid REFERENCES geo_countries(id) ON DELETE RESTRICT` — nullable; the country RID (F-014); the
  person's country of birth (D-Geo) — `pii:basic`
- `attributes JSONB NOT NULL DEFAULT '{}'` — the long-tail directory fields (service number,
  contact, etc.); column-ize a key once it is shared/queried (escape-hatch discipline) —
  `pii:special` **(ceiling)**: a grab-bag may hold up to special-category data, so it is tagged at
  the ceiling (D-PIITiers); special-category fields must not land here without the envelope seam
- *(no `rank_id` column — rank is held via `person_ranks`, one per rank system; D-Rank)*
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','deactivated','purged','provisional'))`
  — `provisional` is a **minimal-PII stub** (D-OverlayFoundation, M29): an unresolved external / edge-target
  person so a relationship or overlay edge points at a real node. It is resolved by **`MergePerson`**
  (`POST /persons/{id}/merge`, perm `person.merge`, admin-tier), which in one transaction re-homes the
  stub's person-owned edges, publishes the **`PersonMerged`** event so every other module re-points its
  person-referencing rows ([events](../architecture/patterns.md) — the cross-module-mutation-via-events
  rule), then tombstones the stub as `purged`. Created via `POST /provisional-persons`. No automatic
  candidate matching — resolution is manual (fuzzy dedup is a parked seam).
- `deactivated_at TIMESTAMPTZ`, `purge_after TIMESTAMPTZ` — reversibility window
- `created_at`, `updated_at`, `deleted_at`

Indexes: a **pg_trgm GIN index** over a `STORED` generated `search_text` column
(`display_name`+`code`+`given`+`surname`, lowercased), partial `WHERE deleted_at IS NULL`, backing
the directory typeahead (D-PersonSearch / review-2026-07 R-06); partial unique on any natural key the
operator configures (none mandated). `person_name_variants` carries the same generated `search_text`
+ GIN index, so search also matches transliterations and aliases (see below).

**Directory search (D-PersonSearch).** `GET /persons?query=` matches a case-insensitive substring
against the person haystack **and** any name variant, filtered **in SQL** (never a Go post-filter):
the admin path is a dedicated `SearchPersons`, the scoped path folds the trigram predicate into the
read-scope semi-join (`VisiblePersonIDsForSubjectSearch`) so pages stay O(page) and never come back
empty while `hasMore`. Match branches are a UNION of id-sets to keep each an index bitmap scan;
sub-3-char queries fall back to a scan (pg_trgm needs a full trigram).

**`person_ranks`** (the reified `HOLDS_RANK` link — one rank per rank system; D-Rank / D-RankSystems)
- `id` PK (RID entity-type token `link__holds_rank`)
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `system_id TEXT NOT NULL REFERENCES rank_systems(id) ON DELETE RESTRICT` — the rank system,
  **derived from the rank** (`rank → type → category → system`) and denormalized for uniqueness
- `rank_id TEXT NOT NULL REFERENCES rank_ranks(id) ON DELETE RESTRICT` — restrict so a rank in use
  cannot be deleted out from under a person
- `created_at`, `updated_at`, `deleted_at`
- `UNIQUE (person_id, system_id)` among active — **at most one rank per (person, system)**; all
  columns `pii:none`

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

All other columns across both tables (`id`, `status`, `locale`, `is_primary`,
lifecycle timestamps) are `pii:none` (D-PIITiers); the name parts, `birthdate`, `date_of_death`,
`sex`, and `country_of_birth` are `pii:basic` as tiered above.

**Profile entities moved to [personprofile.md](personprofile.md).** The full data model for citizenships,
residences, addresses, contact channels (email/phone/call-sign + type catalogs), social & messenger
presence (platforms, accounts, handle history, messenger links), languages spoken (the `SPEAKS` link),
the seven person↔person relationships (+ the relation-type catalog), and the **non-encrypted** M33
institutional ties (government positions, lobbying, external references) now lives in
[personprofile.md § Data model](personprofile.md#data-model) — its authoritative owner (D-PersonModuleSplit).
The **sensitive/encrypted** overlays (physical identity, ethnicity, party membership, watchlists &
sanctions, and the M35 wallets/personality/political-leaning overlays) are owned by
[personsensitive.md § Data model](personsensitive.md#data-model). These tables share the one
`oikumenea.person_*` schema and the one `PersonService` (see § Conjure API surface below).

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
  name parts, `birthdate`, `date_of_death`, `sex`, `country_of_birth` (D-PersonNamesCLDR /
  D-PersonBio), the rows
  of `person_citizenships` and `person_residences` (D-Geo), the rows of `person_emails`,
  `person_phones`, and `person_call_signs` (D-PersonContactChannels), the rows of
  `person_messenger_links`, `person_social_accounts` (+ `person_social_account_handles`)
  (D-PersonSocialChannels), the rows of `person_languages` (the SPEAKS link, `pii:basic`;
  D-Languages M18), the person↔person relationship rows on **either** endpoint
  (D-PersonRelationships), plus the JSONB `attributes`. The `person_ranks` rows (`pii:none`, the
  HOLDS_RANK link) are also removed on purge as part of the child-row cleanup.
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
| `PUT /persons/{id}` | Update names (canonical + CLDR parts), `birthdate`, `date_of_death`, `sex`, `country_of_birth`, attributes | `person.update` |
| `GET /persons` | Search/list the directory (token-paginated); **filtered by the declared facets** — `sex`, `status`, `unitId` (+ the `graph` narrowing arg), `rankId`, `birthdateFrom`/`birthdateTo`, `countryOfBirth`, `hasAccount` (M56, [D-ObjectFacets](../architecture/decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope)) | `person.read` |
| `GET /stats/persons` | Facet distributions over the **same** filter args + an optional `facets` CSV: `totalCount` + `buckets[{key,label,count}]` per facet, counted **inside** the D-PersonReadScope predicate (M57; [facets catalog](../architecture/facets.md)). `totalCount` equals what exhaustively paging `GET /persons` with the same filters returns — a differential test asserts it. The path is `/stats/persons`, not `/persons/stats`, because a literal segment beside `{personId}` makes the router refuse to register the route (D-ObjectFacets as-built) | `person.read` |

> **Facet semantics that are not obvious from the arg names** (M56 ticket 2):
> `unitId` is **subtree-expanding** — it matches an active membership in that unit or in any closure
> descendant, over every **authority-bearing** graph by default. `graph` narrows that expansion to one
> graph code; it is meaningless alone and is rejected with `Person:PersonInvalid`. Setting either
> `birthdate` bound **excludes** persons with an unknown birthdate (SQL three-valued logic) — the
> filter counterpart of M57's mandatory `(unknown)` bucket. `hasAccount=false` is a real filter value,
> not an absent one, so it selects the account-less half of the directory (L-AccountOptional).
>
> The same `PersonFilter` drives the instance-admin list AND the read-scope list, and every predicate
> is folded into the SQL of both — a Go-side re-filter after the page is cut would return a page
> shorter than `pageSize` while still handing back a `nextPageToken` (review-2026-07 R-06). The
> facet block therefore appears in five queries; a no-DB narg-parity test proves it is identical in
> all of them, and a DB differential asserts `scoped(f) == admin(f) ∩ reach`.
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
| `GET /persons/{id}/addresses` | List a person's addresses (M32; primary first) | `person.read` |
| `PUT /persons/{id}/addresses` | Upsert an address over an M19 location | `person.update` |
| `DELETE /persons/{id}/addresses/{id}` | Remove an address row | `person.update` |
| `GET /persons/{id}/emails` | List a person's contact emails | `person.read` |
| `PUT /persons/{id}/emails` | Upsert a contact email (type, address, primary) | `person.update` |
| `DELETE /persons/{id}/emails/{id}` | Remove a contact email | `person.update` |
| `GET /persons/{id}/phones` | List a person's contact phones | `person.read` |
| `PUT /persons/{id}/phones` | Upsert a contact phone (type, number, primary) | `person.update` |
| `DELETE /persons/{id}/phones/{id}` | Remove a contact phone | `person.update` |
| `GET /persons/{id}/call-signs` | List a person's call signs | `person.read` |
| `PUT /persons/{id}/call-signs` | Upsert a call sign (value, primary) | `person.update` |
| `DELETE /persons/{id}/call-signs/{id}` | Remove a call sign | `person.update` |
| `GET /persons/{id}/messenger-links` | List messenger reachability on the person's phones/emails | `person.read` |
| `PUT /persons/{id}/messenger-links` | Upsert a messenger link (channel, platform, primary) | `person.update` |
| `DELETE /persons/{id}/messenger-links/{id}` | Remove a messenger link | `person.update` |
| `GET /persons/{id}/social-accounts` | List the person's social accounts (+ handle history) | `person.read` |
| `PUT /persons/{id}/social-accounts` | Upsert a social account (platform, id/handle, verification, source/confidence) | `person.update` |
| `DELETE /persons/{id}/social-accounts/{id}` | Remove a social account | `person.update` |
| `GET /persons/{id}/languages` | List languages the person speaks (name as `locale→text` map) | `person.read` |
| `PUT /persons/{id}/languages` | Upsert a SPEAKS link (languoid, CEFR, native; D-Languages M18) | `person.update` |
| `DELETE /persons/{id}/languages/{languageId}` | Remove a spoken language | `person.update` |
| `GET /persons/{id}/partnerships` | List partnerships (marriage/engagement) | `person.read` |
| `PUT /persons/{id}/partnerships` | Upsert a partnership (partner, status, interval) | `person.update` |
| `GET /persons/{id}/kinships` | List parent/child kinships | `person.read` |
| `PUT /persons/{id}/kinships` | Upsert a `parent_of` kinship | `person.update` |
| `GET /persons/{id}/guardianships` | List guardianships | `person.read` |
| `PUT /persons/{id}/guardianships` | Upsert a guardian↔ward link | `person.update` |
| `GET /persons/{id}/sponsorships` | List sponsorships (godparent/advisor/mentor) | `person.read` |
| `PUT /persons/{id}/sponsorships` | Upsert a sponsorship | `person.update` |
| `GET /persons/{id}/next-of-kin` | List nominated next-of-kin (priority-ordered) | `person.read` |
| `PUT /persons/{id}/next-of-kin` | Upsert a next-of-kin nomination | `person.update` |
| `GET /persons/{id}/associations` | List associations (associate/COI/no-contact) | `person.read` |
| `PUT /persons/{id}/associations` | Upsert an association | `person.update` |
| `DELETE /persons/{id}/relationships/{id}` | Remove any person↔person link by id | `person.update` |
| `GET /person/email-types` | List the email-type catalog (locale→text names) | `person.read` |
| `GET /person/phone-types` | List the phone-type catalog (locale→text names) | `person.read` |
| `GET /person/platforms` | List the platform catalog (locale→text names) | `person.read` |
| `GET /person/relation-types` | List the relation-type catalog (locale→text names) | `person.read` |

Citizenship and residence reads follow the same **read-scope rule** as the person (D-PersonReadScope);
their writes are audited (D-Audit) and erased on **purge**. `country_of_birth`, citizenship `country`,
and residence `country` are validated against the `geo_countries` registry (D-Geo).

Emails, phones, and call signs likewise follow the person **read-scope rule** (D-PersonReadScope); their
writes are audited and **erased on purge** (D-PersonContactChannels). Phone `number` is E.164-normalized
and its `country` derived on write; email `provider` is derived from the address domain. The
email/phone `kind` is validated against the `person_email_types`/`person_phone_types` catalogs, whose
`name`s return as locale→text maps via [localization](localization.md).

Messenger links, social accounts, and all seven person↔person relationships follow the **same read-scope
rule** (D-PersonReadScope) — person↔person links are readable when the subject can read **either**
endpoint — and the **same audit + purge** discipline. A social account returns its handle-rename history,
its `platform_verified`/`verified_by_operator_at` flags, and its `source`/`confidence` attribution; the
`platform`/`relation-type` catalog `name`s return as locale→text maps. **No** social-graph
metrics are exposed, and friend/follower social ties (`person_social_links`) are **deferred** (above).

Read endpoints that list people *by unit* are served by [membership](membership.md) and pass
the shadow gate; `PersonService` directory reads are gated on `person.read` per the **read-scope
rule** below (D-PersonReadScope) — `GET /persons/{id}` checks the single-person membership
intersection, and `GET /persons` returns the **union** of people reachable that way (membership-less
people only on the instance plane).

A rank change may also be applied as the effect of a [order](order.md) `rank-change` order (the наказ
that is its legal basis, D-Orders): on order **issue**, person **subscribes** to the order's
`RankChangeOrdered` event and upserts the person's rank in the rank's system **in the issue
transaction** (D-OrderApply), emitting `PersonRankChanged`. The `person_ranks` link row carries **no**
`order_item_id` FK, so that provenance is recorded in the **audit payload** — unlike
[membership](membership.md), which links provenance structurally. "Which order raised this rank?" is
therefore answered via [audit](audit.md), not a person field.

## Dependencies

- **Calls:** [rank](rank.md) (validate `rank_id` exists), [localization](localization.md)
  (validate name-variant `locale` codes; assemble email/phone-type, **platform** and **relation-type**
  `name` locale maps), the
  **`geo_countries`** registry ([platform](platform.md); validate `country_of_birth` / citizenship /
  residence / derived-phone country codes). Uses `github.com/nyaruka/phonenumbers` to normalize phone
  numbers to E.164 and derive their country. [platform](platform.md) for infra. Emits `PersonCreated`,
  `PersonDeactivated`, `PersonRankChanged`, `PersonPurged` events. **Subscribes** to [order](order.md)'s `RankChangeOrdered` event and applies
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

**Facet rule (M56/M57, [D-ObjectFacets](../architecture/decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope)).**
`GET /stats/persons` computes every count **inside** the read-scope predicate above — the same SQL
semi-join the list uses, never a post-paged tally. **No facet exists over an envelope-encrypted
`pii:special` value** (ethnicity, party membership, political leaning, health, legal records): there
is no plaintext to group, and grouping them is exactly the join **D-DataScope**'s aggregation rule
forbids. The plaintext discriminators beside them (`person_health_records.kind`,
`person_legal_records.kind`/`disposition`) may be faceted only under their own read codes
(`person.health.read`, `person.legal-record.read`) and are **omitted** from the response for a caller
without them — never a zeroed bucket, never a 403.

## Invariants & safety

- A person needs **no** account and **no** membership to exist. A **membership-less** person is
  reachable only on the **instance plane** for reads (no unit-scoped grant reaches them;
  D-PersonReadScope).
- A person holds **at most one** rank **per rank system** (`UNIQUE (person_id, system_id)` among
  active `person_ranks`); deleting a rank still in use is blocked
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
- A person may hold **several emails / phones / call signs** (D-PersonContactChannels), each unique
  among active rows per person: `(person, address)`, `(person, number)`, `(person, call_sign)`
  respectively. `is_primary` marks at most one active per channel. The **contact email is distinct from
  the login email** (no FK to `account_accounts`). Email/phone `kind` is FK-validated against the
  type catalogs; phone `number` is E.164 and its `country` FK-validated against `geo_countries`.
- A person may hold **several social accounts** (D-PersonSocialChannels); the durable key is the
  immutable `platform_user_id` (the `handle` is mutable, with rename history). A **messenger link**
  attaches to exactly one channel (XOR phone/email) and only a `messenger`-category platform. A social
  account carries **provenance + confidence** on its `HOLDS_ACCOUNT` link (a sourced, weighted claim,
  not a bare fact) and distinguishes **platform** verification from **operator** verification. **No
  social-graph metrics** are stored; free-text `bio`/location wait on DS-29.
- **Person↔person relationships** (D-PersonRelationships) are per-type reified self-links with **both
  endpoints in-directory**: at most **one active partnership** (`engaged`/`married`) per person; kinship
  is **directional** `parent_of` with siblings derived, never stored; next-of-kin is an **in-directory
  nomination** (no external free-text contacts). A friend/follower `person_social_links` tie was scoped
  but **deferred — not built** (see [decisions.md](../architecture/decisions.md) D-PersonRelationships).
  Authority **never** derives from any relationship (D-Rank stance) — they are directory data.
  Purging **either** endpoint erases the link.

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
- **Phone carrier / provider lookup** is parked (DS-40) — not statically derivable (number
  portability), so it needs an external HLR/lookup service. Only the derived `country` is stored.
- **Social `bio` / self-declared location** on a social account are `pii:sensitive` and **wait on the
  DS-29 envelope seam** — not stored until it ships (D-PersonSocialChannels). Time-series social-graph
  metrics (follower/activity counts) are **excluded outright**, not parked.
- **External (non-directory) next-of-kin** remain out of scope — both ends of every person↔person link
  must be directory persons (D-PersonRelationships); revisit if real deployments need free-text contacts.
- **Languages spoken** (`person_languages`: a `SPEAKS` link to a `level='language'` languoid, with
  `cefr_level` + `is_native`; `pii:basic`, purge-erased) landed with **M18 / D-Languages** — the
  languoid catalog is owned by the [language](language.md) module; the editor UI is deferred.
- **External (non-directory) next-of-kin** (above) is **resolved by M29** — provisional person stubs
  (not free-text) become the node every relationship/overlay edge points at (D-OverlayFoundation).

## OSINT-enrichment cluster (M29, M31–M36)

> The [draft_superbrain_schema.md](../draft_superbrain_schema.md) per-field verdicts are the binding
> source; the cluster decisions live in [roadmap-decisions.md](../architecture/roadmap-decisions.md).
> Three rules hold: **declared ≠ inferred** (never merged), **every overlay carries
> `source`+`confidence`+`as_of`**, **special-category data is gated** (envelope [D-SpecialPII] +
> structured `legal_basis` + audit). **M29 foundation is core (this module); M31–M36 are owned by the
> concern docs** — full data model in [personsensitive.md § Data model](personsensitive.md#data-model)
> (sensitive/encrypted) and [personprofile.md § Data model](personprofile.md#data-model) (M32 addresses,
> M33 non-encrypted ties).

- **M29 · Foundation (D-OverlayFoundation) — core.** `person_persons.status` gains **`provisional`**
  (minimal-PII stubs so every relationship/overlay edge points at a node) + a manual **`MergePerson`**
  action (re-homes edges, `confidence`, `PersonMerged` event); the reusable `source`/`confidence`/`as_of`
  **attribution convention**; the structured **`legal_basis`** catalog ([platform](platform.md)
  `platform_legal_basis_kinds`, GDPR Art. 6/9), NOT NULL on every `pii:special` store. No automatic
  candidate matching (parked).
- **M31 · Physical identity (D-PhysicalIdentity) — DELIVERED (migration `0030`).** Physical descriptions,
  distinguishing marks, and the encrypted declared ethnicity (+ hierarchical `person_ethnicity_types`
  catalog fed by the Factbook import). The alias `person_name_variants.variant_kind` stays **core** (aliases
  are still names; see [§ Data model](#data-model) above). Full model:
  [personsensitive.md](personsensitive.md#data-model).
- **M32 · Addresses (D-PersonAddresses) — DELIVERED.** `person_addresses` → M19 location; owned by
  [personprofile.md](personprofile.md#data-model). `person_residences` retained for legal residence.
- **M33 · Institutional ties (D-InstitutionalTies) — DELIVERED (migration `0032`).** Encrypted party
  membership → [personsensitive.md](personsensitive.md#data-model); government positions / lobbying /
  external references → [personprofile.md](personprofile.md#data-model). Inferred political leaning is a
  **separate** M35 overlay, never merged with the declared party membership.
- **M34 · Watchlists (D-Watchlists) — DELIVERED.** Synchronous `CheckWatchlists` → [hermenea](hermenea.md);
  transient `person_watchlist_matches` + durable `person_regulatory_sanctions`. Owned by
  [personsensitive.md](personsensitive.md#data-model).
- **M35 · Overlays (D-PersonOverlays) — DELIVERED (migration `0035`).** Crypto wallets, personality, and
  the inferred (encrypted) political leaning. Owned by [personsensitive.md](personsensitive.md#data-model).
- **M36 · Health & vulnerability (D-HealthVulnerability) — designed.** `person_health_records`
  (category-level only, never inferred, `pii:special` + envelope + need-to-know) + `person_insurance`;
  will land in [personsensitive.md](personsensitive.md).
- **M38 · Criminal / arrest / court records (D-LegalRecords) — DELIVERED (migration `0016`).**
  `person_legal_records` (object `6,1,22`; category-level only, never inferred, `pii:special` GDPR
  Art. 10, envelope-encrypted offence detail; **mandatory `disposition`**; **sealed/expunged records
  suppressed** behind `person.legal-record.read-suppressed`; jurisdiction → `geo_countries`). Endpoints
  `GET/PUT /persons/{id}/legal-records` + `DELETE …/{recordId}` on the single `PersonService`, read-gated
  by `person.legal-record.read`. Owned by [personsensitive.md](personsensitive.md#data-model).

All cluster tables extend the person **purge** erasure list (`pii:contact`/`pii:basic` NULLed;
`pii:sensitive`/`pii:special` crypto-erased), per [D-PIITiers](../architecture/decisions.md). Reads on
person-PII overlays project through D-PersonReadScope; writes are audited.
