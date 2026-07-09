# Module: personprofile

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md) ·
> [person](person.md) (the core aggregate + authoritative data model)
> Table prefix: `oikumenea.person_*` (shared with [person](person.md) — see below)

> **Concern module (D-PersonModuleSplit, review-2026-07 R-09).** `personprofile` is one of the three
> Go modules the person god-module split into, behind the **one** Conjure `PersonService`. It owns the
> **non-sensitive contactability + relationship** concerns. It is **not** a separate schema or a
> separate API service: the tables it manages live in the `oikumenea.person_*` schema whose
> **authoritative data model + endpoint documentation stays in [person.md](person.md)**; this doc
> records the *concern boundary* and points back. (Since PR-2b this module owns its **own**
> `adapters`/`queries` slice + generated `personprofilesql` package over the `domain.ProfileRepository`
> port; see [decisions.md](../architecture/decisions.md) D-PersonModuleSplit.)

## Purpose

Owns the **who-you-can-reach-and-how** surface of a person: where they hold nationality and live, how
to contact them, their social presence, their person↔person ties, and their **non-encrypted**
institutional affiliations. These concerns change on a different cadence than core identity and carry
**no** envelope-encrypted or `pii:special` data (that is [personsensitive](personsensitive.md)) — so
grouping them keeps the contact/relationship reviewers and helpers in one place, away from both the
identity core and the crypto surface.

## Entities & aggregates

Ontology kinds are in the binding [registry](../ontology-mapping.md) (RID tokens unchanged by the split);
the **field-level data model for these entities lives in [§ Data model](#data-model) below — this module
is its owner** (moved here from [person.md](person.md), D-PersonModuleSplit). This module owns the
application logic **and data model** for:

- **Citizenship** / **Residence** — effective-dated country ties (`person_citizenships`,
  `person_residences`; D-Geo).
- **Address** — precise, effective-dated address over an M19 [location](location.md)
  (`person_addresses`, the reified `link__lives_at`; D-PersonAddresses, M32). `pii:contact`.
- **Contact channels** — `Email` / `Phone` / `Call sign` (`person_emails` / `person_phones` /
  `person_call_signs`; D-PersonContactChannels) and their `Email type` / `Phone type` catalogs.
- **Social & messenger presence** — `Messenger link` (reachability over an existing email/phone) and
  the standalone `Social account` (+ handle history), over the `Platform` catalog
  (D-PersonSocialChannels).
- **Languages spoken** — the `SPEAKS` link (`person_languages`) to a `level='language'` languoid owned
  by the [language](language.md) module (D-Languages, M18).
- **Person↔person relationships** — the per-type reified self-links (partnership, kinship,
  guardianship, sponsorship, next-of-kin, association) + the `Relation type` catalog
  (D-PersonRelationships).
- **Non-encrypted institutional ties** — `person_government_positions` (feeds the M34 PEP seam),
  `person_lobbying_relationships`, `person_external_references` (D-InstitutionalTies, M33). The
  **encrypted party membership** from the same decision lives in [personsensitive](personsensitive.md).

## Data model

Conventions per [conventions.md](../architecture/conventions.md). **This module is the authoritative
owner of the data model for every `person_*` table below** (D-PersonModuleSplit — the entity sections
were moved here from [person.md](person.md), which keeps only the core `person_persons` / `person_ranks`
/ `person_name_variants`). Since PR-2b these tables are served by `personprofile`'s **own**
`internal/personprofile/adapters` repository over its own `queries/person.sql` + generated
`personprofilesql` package (the `domain.ProfileRepository` port); it verifies the parent person exists
via a reviewed `PersonExists` read on `person_persons`. The `Person` object, the `HOLDS_RANK` link, and
the name variants remain owned by [person.md § Data model](person.md#data-model).

**`person_citizenships`** (effective-dated; a person may hold several — D-Geo)
- `id` PK
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `country_id uuid NOT NULL REFERENCES geo_countries(id) ON DELETE RESTRICT` — the country RID (F-014); `pii:basic`
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
- `country_id uuid NOT NULL REFERENCES geo_countries(id) ON DELETE RESTRICT` — the country RID (F-014); `pii:contact`
- `region TEXT` — optional sub-national region / locality — `pii:contact`
- `valid_from DATE NOT NULL`, `valid_to DATE` — effective window (`valid_to` NULL = current) —
  `pii:contact`
- `created_at`, `updated_at`, `deleted_at`
- Index `(person_id) WHERE deleted_at IS NULL`.

All `id`/`person_id`/lifecycle columns on both tables are `pii:none`; `country`/dates on citizenship
are `pii:basic`, residence columns are `pii:contact` (locator data) — D-PIITiers.

**`person_addresses`** (precise, effective-dated address history over M19 Location — D-PersonAddresses,
M32; the reified link `link__lives_at`, RID `6,2,10`). Distinct from `person_residences` (country-grade
legal residence, kept): an address is the geocoded overlay that dedups against shared
`location_locations` rows and enables spatial queries.
- `id` PK (RID `6,2,10`)
- `person_id uuid NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `location_id uuid NOT NULL REFERENCES location_locations(id) ON DELETE RESTRICT` — the M19 place; `pii:contact`
- `role TEXT NOT NULL CHECK (role IN ('home','work','mailing','other'))`
- `valid_from DATE NOT NULL` (defaults to today), `valid_to DATE` (NULL = current) — `pii:contact`
- `is_primary BOOLEAN` — at most one **active** primary per person
  (`UNIQUE (person_id) WHERE is_primary AND deleted_at IS NULL`; the app demotes the prior primary in-tx)
- `privacy_seeking BOOLEAN` — a mailing address that deliberately differs from home (itself a signal)
- `source`/`confidence` — the attribution column-set (D-OverlayFoundation)
- `created_at`, `updated_at`, `deleted_at`; index `(person_id) WHERE deleted_at IS NULL`
- The location's existence is verified before write via the cross-module `LocationLookup` seam (the
  geo/location service). `pii:contact` → **hard-deleted** on person purge.

**`person_email_types`** / **`person_phone_types`** (instance-admin catalogs — D-Code/D-i18n)
- `code TEXT PRIMARY KEY` — natural key (e.g. `personal`, `work`, `other`; phone `mobile`, `home`,
  `work`, `other`); locale-agnostic, immutable by convention. Not an RID (catalog carve-out, like
  `document_personal_code_schemes`).
- `name TEXT NOT NULL` — default-locale label; other locales in the [localization](localization.md)
  store (`entity_type='email_type'` / `'phone_type'`).
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired'))`, `sort_order INTEGER`
- `created_at`, `updated_at`, `deleted_at`. All `pii:none`.

**`person_emails`** (multi-valued contact email — D-PersonContactChannels)
- `id` PK (`new_id(6,1,5)`)
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `type_code TEXT NOT NULL REFERENCES person_email_types(code) ON DELETE RESTRICT`
- `address CITEXT NOT NULL` — the email address, stored lowercased — `pii:contact`
- `provider TEXT` — derived on write from the address domain (`gmail.com → google`); nullable when no
  mapping — `pii:contact`
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE` — at most one active primary
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** one **active** row per `(person_id, address)`:
  `UNIQUE (person_id, address) WHERE deleted_at IS NULL`. Index `(person_id) WHERE deleted_at IS NULL`.
- **Distinct from the login email** (`account_accounts.email`) — no FK; independent concerns.

**`person_phones`** (multi-valued contact phone — D-PersonContactChannels)
- `id` PK (`new_id(6,1,6)`)
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `type_code TEXT NOT NULL REFERENCES person_phone_types(code) ON DELETE RESTRICT`
- `number TEXT NOT NULL` — **E.164-normalized** via `github.com/nyaruka/phonenumbers` — `pii:contact`
- `country_id uuid REFERENCES geo_countries(id) ON DELETE RESTRICT` — the country RID (F-014), **derived** from the number (the ISO code is resolved to its RID in SQL);
  nullable when underivable — `pii:contact`
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE`
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** one **active** row per `(person_id, number)`:
  `UNIQUE (person_id, number) WHERE deleted_at IS NULL`. Index `(person_id) WHERE deleted_at IS NULL`.
- Carrier/provider is **not** stored (not statically derivable; parked DS-40).

**`person_call_signs`** (multi-valued informal identifier / позивний — D-PersonContactChannels)
- `id` PK (`new_id(6,1,7)`)
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `call_sign TEXT NOT NULL` — the call sign label — `pii:basic`
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE`
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** one **active** call sign per `(person_id, call_sign)`:
  `UNIQUE (person_id, call_sign) WHERE deleted_at IS NULL` (the leading `person_id` also serves the
  list lookup).

On all three channel tables `id`/`person_id`/`type_code`/`is_primary`/lifecycle are `pii:none`; email
`address`/`provider` and phone `number`/`country` are `pii:contact`; `call_sign` is `pii:basic`
(D-PIITiers). All three are **erased on person purge**.

### Social & messenger channels (D-PersonSocialChannels)

**`person_platforms`** (instance-admin catalog of social networks / messengers — D-Code/D-i18n)
- `code TEXT PRIMARY KEY` — natural key (e.g. `telegram`, `signal`, `instagram`, `linkedin`); not an RID
  (catalog carve-out, like `person_email_types`).
- `name TEXT NOT NULL` — default-locale label; other locales in [localization](localization.md)
  (`entity_type='platform'`).
- `category TEXT NOT NULL CHECK (category IN ('messenger','social'))`
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired'))`, `sort_order INTEGER`
- `created_at`, `updated_at`, `deleted_at`. All `pii:none`. Seeded `telegram`/`whatsapp`/`signal`/
  `viber` (messenger) + `instagram`/`linkedin`/`x`/`facebook` (social).

**`person_messenger_links`** (reachability over an existing email/phone — Link `link__reachable_on`)
- `id` PK (`new_id(6,1,8)`)
- `phone_id TEXT REFERENCES person_phones(id) ON DELETE CASCADE` — nullable
- `email_id TEXT REFERENCES person_emails(id) ON DELETE CASCADE` — nullable
- **XOR CHECK:** `CHECK ((phone_id IS NOT NULL) <> (email_id IS NOT NULL))` — exactly one channel.
- `platform_code TEXT NOT NULL REFERENCES person_platforms(code) ON DELETE RESTRICT` — write-time
  restricted to a `category='messenger'` platform.
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE`, `verified_at TIMESTAMPTZ` — both `pii:none`
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** one active link per `(phone_id, platform_code)` / `(email_id, platform_code)`
  (partial-unique, `WHERE deleted_at IS NULL`). All columns `pii:contact` by association with the
  channel, lifecycle `pii:none`.

**`person_social_accounts`** (standalone handle — Object `PersonSocialAccount`, Link `link__holds_account`)
- `id` PK (`new_id(6,1,9)`)
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `platform_code TEXT NOT NULL REFERENCES person_platforms(code) ON DELETE RESTRICT`
- `platform_user_id TEXT` — the platform's **immutable internal id**, the durable key; nullable when
  unknown — `pii:contact`
- `handle TEXT NOT NULL` — current @handle (mutable; history in `person_social_account_handles`) —
  `pii:contact`
- `display_name TEXT`, `profile_url TEXT` (derived), `language TEXT` — `pii:contact`
- `platform_verified BOOLEAN NOT NULL DEFAULT FALSE` — platform "blue-check"; `pii:none`
- `verified_by_operator_at TIMESTAMPTZ` — operator confirmation, distinct from platform verification;
  `pii:none`
- **Attribution (on the `HOLDS_ACCOUNT` link):** `source TEXT NOT NULL CHECK (source IN
  ('self_declared','operator_verified','imported'))`, `confidence TEXT NOT NULL DEFAULT 'possible'
  CHECK (confidence IN ('confirmed','probable','possible'))` — both `pii:none`
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE`
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** one active row per `(person_id, platform_code, platform_user_id)` when the id is
  known, else per `(person_id, platform_code, lower(handle))` (two partial-unique indexes,
  `WHERE deleted_at IS NULL`). Index `(person_id) WHERE deleted_at IS NULL`.
- **DS-29-gated (NOT in this schema):** free-text `bio` + `self_declared_location` are `pii:sensitive`
  and wait on the envelope seam (DS-29).

**`person_social_account_handles`** (handle-rename history — temporal)
- `id` PK (`new_id(6,1,10)`)
- `account_id TEXT NOT NULL REFERENCES person_social_accounts(id) ON DELETE CASCADE`
- `handle TEXT NOT NULL` — `pii:contact`
- `valid_from TIMESTAMPTZ NOT NULL`, `valid_to TIMESTAMPTZ` — NULL = current
- `created_at`, `updated_at`, `deleted_at`. Index `(account_id) WHERE deleted_at IS NULL`.

All four tables follow the holder **read-scope rule** (D-PersonReadScope); writes audited; **erased on
person purge** (the `pii:contact` columns NULLed + `DeleteAll*` of the child rows). **No** time-series
social-graph metrics are stored (excluded outright; D-PersonSocialChannels).

### Languages spoken (D-Languages, M18)

**`person_languages`** — the `SPEAKS` link from a person to a `level='language'` languoid owned by the
[language](language.md) module: `languoid_id` FK, `cefr_level`, `is_native`; `pii:basic`, purge-erased.
The languoid catalog itself lives in [language.md](language.md); this module owns only the person↔languoid link.

### Person↔person relationships (D-PersonRelationships)

All are **reified self-links** (`Person → Person`, both endpoints `person_persons`), mirroring
`membership_memberships`: RID PK, soft-delete, `created_at`/`updated_at`, and an effective interval +
`status` where a lifecycle applies. All are instance-global, holder-scoped on read (D-PersonReadScope),
audited on write, and **erased when either endpoint person purges**.

**`person_relation_types`** (instance-admin catalog for open-ended relation labels — D-Code/D-i18n)
- `code TEXT PRIMARY KEY`; `name TEXT NOT NULL` (localization `entity_type='relation_type'`);
  `category TEXT NOT NULL CHECK (category IN ('sponsorship','association','next_of_kin'))`;
  `status`/`sort_order`; lifecycle. All `pii:none`.

**`person_partnerships`** (marriage + engagement — Link `link__partnered_with`)
- `id` PK; `person_id_a TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`,
  `person_id_b TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `CHECK (person_id_a < person_id_b)` — canonical ordering, no self-pair
- `status TEXT NOT NULL CHECK (status IN ('engaged','married','divorced','widowed','annulled','dissolved'))`
- `effective_from DATE`, `effective_to DATE` — NULL `effective_to` = ongoing — `pii:basic`
- lifecycle. **At most one active `engaged`-or-`married` row per person** (enforced in domain +
  partial-unique helper). `person_id_*`/`status` `pii:none`; the relationship's existence is `pii:basic`.

**`person_kinships`** (directional blood/legal parentage — Link `link__kin_parent_of`)
- `id` PK; `parent_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`,
  `child_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `CHECK (parent_id <> child_id)`; `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN
  ('active','disestablished'))`; lifecycle. Siblings are **derived, not stored**. Unique active
  `(parent_id, child_id)`. `pii:basic` (the kin fact).

**`person_guardianships`** (legal guardian → ward — Link `link__guardian_of`)
- `id` PK; `guardian_id`, `ward_id` (FK CASCADE, `CHECK (guardian_id <> ward_id)`);
  `relation_code TEXT REFERENCES person_relation_types(code)` (nullable); `effective_from`/
  `effective_to`; `status`; lifecycle. `pii:basic`.

**`person_sponsorships`** (godparent / advisor / mentor — Link `link__sponsor_of`)
- `id` PK; `sponsor_id`, `sponsored_id` (FK CASCADE, no self-edge);
  `relation_code TEXT NOT NULL REFERENCES person_relation_types(code)` (`category='sponsorship'`);
  `effective_from`/`effective_to`; lifecycle. `pii:basic`.

**`person_next_of_kin`** (in-directory nomination — Link `link__next_of_kin`)
- `id` PK; `subject_id`, `contact_id` (**both** FK to `person_persons` CASCADE, no self-edge);
  `relation_code TEXT REFERENCES person_relation_types(code)` (`category='next_of_kin'`);
  `priority INTEGER NOT NULL DEFAULT 1`; lifecycle. A **nomination**, not a blood fact; external
  free-text contacts are out of scope. `pii:basic`.

**`person_associations`** (COI / no-contact / associate — Link `link__associated_with`)
- `id` PK; symmetric `person_id_a < person_id_b` (canonical pair, CASCADE);
  `relation_code TEXT REFERENCES person_relation_types(code)` (`category='association'`);
  `kind TEXT NOT NULL CHECK (kind IN ('associate','coi','no_contact'))`; lifecycle. `pii:basic`.

**`person_social_links`** (friend/follower — Link `link__social_tie`) — **deferred, not built.** Cut from
the M14 delivery: no consumer, no authoritative source, a hollow "proof of friendship" gate, and
redundant with `person_associations` for the actionable COI/no-contact case. Returns only with a real
account-level model. See [decisions.md](../architecture/decisions.md) D-PersonRelationships.

### Non-encrypted institutional ties (D-InstitutionalTies, M33)

The person↔org affiliations from M33 whose payload is **not** envelope-encrypted (the encrypted party
membership is [personsensitive](personsensitive.md)). All carry the `source`/`confidence` attribution,
are re-pointed on merge, and hard-deleted on purge.

- **`person_government_positions`** (link `6,2,12`) — office held; `pep_trigger` auto-true and persists
  post-office, feeding the M34 PEP seam `IsPoliticallyExposed` (read by [personsensitive](personsensitive.md)
  through the `PEPStatusReader` seam — never a direct table read); optional polymorphic `org_id` +
  `country_id`. `pii:basic`.
- **`person_lobbying_relationships`** (link `6,2,13`) — `issues[]` / `filing_id` / `source_url`. `pii:basic`.
- **`person_external_references`** (object `6,1,14`) — idempotent by `(person, url)`; a hermenea import
  target. `pii:basic`.

Foreign-military service reuses [membership](membership.md) against an M30
[external-organization](external-organizations.md) + rank — no table.

## Conjure endpoint sketch

No separate service. The profile operations are the citizenship / residence / address / email / phone /
call-sign / messenger-link / social-account / language / relationship / catalog rows of the single
`PersonService` table in [person.md § Conjure API surface](person.md#conjure-api-surface); the one
`person/transport.Service` delegates each of those handlers to this module.

## Dependencies

- **Calls:** [location](location.md) (the `LocationLookup` seam — verify an address's `location_id`
  exists before write; **owned by this module**), [language](language.md) (validate the `SPEAKS`
  languoid), the `geo_countries` registry ([platform](platform.md); citizenship / residence / derived
  phone country), [localization](localization.md) (email/phone-type, platform, relation-type `name`
  locale maps), and [person](person.md) core for the person's existence. Uses
  `github.com/nyaruka/phonenumbers` for E.164 normalization.
- **Called by:** the composition root, which registers it (`personprofile.Register(pool, audit)`),
  wires its `LocationLookup` seam, and subscribes it to `PersonPurged` (`SubscribePersonPurge` → erase
  its rows in the purge tx). Merge re-point of person-owned rows stays core-driven (`RepointPersonOwned`).

## Authorization touchpoints

Every operation is gated by the same `person.read` / `person.update` permissions and the **read-scope
rule** as core (D-PersonReadScope) — person↔person links are readable when the subject can read
**either** endpoint. This module **never** decides access; it calls the PDP via the shared transport.

## Patterns

- **Cross-module mutation via events** (purge erase via `PersonPurged`; merge re-point core-driven) —
  never a direct cross-module write ([patterns](../architecture/patterns.md)).
- **Late-bound seam + `MustBeBound`** for `LocationLookup` (R-11): unbound → boot failure, not a
  request-time nil deref.
- **Derived-in-SQL** contactability facts (email `provider`, phone `country`) — never a Go post-compute.

## Invariants & safety

- All profile tables follow the person **read-scope rule** and are **audited** on write.
- Effective-dated / one-primary uniqueness per person holds exactly as in [person.md § Invariants](person.md#invariants--safety)
  (one active citizenship per country, one active email/phone/call-sign per value, one active primary
  per channel, at most one active partnership, directional kinship with derived siblings, in-directory
  next-of-kin only).
- `pii:contact` columns are **erased on person purge** (NULLed / child rows deleted); this module carries
  **no** envelope-encrypted or `pii:special` data — that is [personsensitive](personsensitive.md).

## Open seams / future

- Entity-level open seams (partial addresses, phone-carrier lookup on DS-40, social `bio`/location on
  DS-29, external free-text next-of-kin) are tracked in
  [person.md § Open seams](person.md#open-seams--future).
