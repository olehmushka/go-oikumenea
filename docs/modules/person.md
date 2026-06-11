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
- `country_of_birth CHAR(2) REFERENCES geo_countries(code) ON DELETE RESTRICT` — nullable; the
  person's country of birth (D-Geo) — `pii:basic`
- `attributes JSONB NOT NULL DEFAULT '{}'` — the long-tail directory fields (service number,
  contact, etc.); column-ize a key once it is shared/queried (escape-hatch discipline) —
  `pii:special` **(ceiling)**: a grab-bag may hold up to special-category data, so it is tagged at
  the ceiling (D-PIITiers); special-category fields must not land here without the envelope seam
- *(no `rank_id` column — rank is held via `person_ranks`, one per rank system; D-Rank)*
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','deactivated','purged'))`
- `deactivated_at TIMESTAMPTZ`, `purge_after TIMESTAMPTZ` — reversibility window
- `created_at`, `updated_at`, `deleted_at`

Indexes: a trigram/`citext` index on `display_name` for directory search (added when
search is built); partial unique on any natural key the operator configures (none mandated).

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

**`person_email_types`** / **`person_phone_types`** (instance-admin catalogs — D-Code/D-i18n)
- `code TEXT PRIMARY KEY` — natural key (e.g. `personal`, `work`, `other`; phone `mobile`, `home`,
  `work`, `other`); locale-agnostic, immutable by convention. Not an RID (catalog carve-out, like
  `document_personal_code_schemes`).
- `name TEXT NOT NULL` — default-locale label; other locales in the [localization](localization.md)
  store (`entity_type='email_type'` / `'phone_type'`).
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired'))`, `sort_order INTEGER`
- `created_at`, `updated_at`, `deleted_at`. All `pii:none`.

**`person_emails`** (multi-valued contact email — D-PersonContactChannels)
- `id` PK (`new_rid('person','email')`)
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
- `id` PK (`new_rid('person','phone')`)
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `type_code TEXT NOT NULL REFERENCES person_phone_types(code) ON DELETE RESTRICT`
- `number TEXT NOT NULL` — **E.164-normalized** via `github.com/nyaruka/phonenumbers` — `pii:contact`
- `country CHAR(2) REFERENCES geo_countries(code) ON DELETE RESTRICT` — **derived** from the number;
  nullable when underivable — `pii:contact`
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE`
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** one **active** row per `(person_id, number)`:
  `UNIQUE (person_id, number) WHERE deleted_at IS NULL`. Index `(person_id) WHERE deleted_at IS NULL`.
- Carrier/provider is **not** stored (not statically derivable; parked DS-40).

**`person_call_signs`** (multi-valued informal identifier / позивний — D-PersonContactChannels)
- `id` PK (`new_rid('person','call_sign')`)
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
- `id` PK (`new_rid('person','messenger_link')`)
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
- `id` PK (`new_rid('person','social_account')`)
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
- `id` PK (`new_rid('person','social_handle')`)
- `account_id TEXT NOT NULL REFERENCES person_social_accounts(id) ON DELETE CASCADE`
- `handle TEXT NOT NULL` — `pii:contact`
- `valid_from TIMESTAMPTZ NOT NULL`, `valid_to TIMESTAMPTZ` — NULL = current
- `created_at`, `updated_at`, `deleted_at`. Index `(account_id) WHERE deleted_at IS NULL`.

All four tables follow the holder **read-scope rule** (D-PersonReadScope); writes audited; **erased on
person purge** (the `pii:contact` columns NULLed + `DeleteAll*` of the child rows). **No** time-series
social-graph metrics are stored (excluded outright; D-PersonSocialChannels).

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
  (D-PersonSocialChannels), the person↔person relationship rows on **either** endpoint
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
