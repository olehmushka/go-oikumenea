# Module: document

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.document_*`

## Purpose

Owns the **documents a person holds** — identity papers and government-issued personal codes
(passport, national ID, driver's licence, military ID; tax number, social-insurance number)
(D-Documents). A document is **attached to exactly one person** and records *what a person has*,
distinct from an [order](order.md), which records an administrative *act*. The document **kind** is
not hard-coded: it is drawn from an **instance-admin-managed catalog** (`document_types`) of stable
`code` + translatable `name`, exactly like the rank scheme and locale registry (D-Code / D-i18n), so
each deployment/country adds its own document kinds without a code change.

This module stores **metadata only** — numbers, issuers, validity dates — never document binaries
(scanned passports); that is a parked seam.

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Document` and
`Personal code` (held via the `HOLDS_DOCUMENT`/`HOLDS_CODE` links), plus the `Document type` and
`Personal-code scheme` catalogs (the scheme keeps a natural `code` PK, not an RID). **Actions:**
`AttachDocument`/`UpdateDocument`/`DeleteDocument`, personal-code attach/update/delete, catalog edits
— audited, `action__<type>` RID; the `PersonPurged` event crypto-erases codes.

- **Document type** (catalog) — a stable `code` (e.g. `passport`, `driver-license`) + translatable
  `name`, and an **optional `attr_schema`** declaring typed/validated `attributes` for documents of
  that type (D-DocumentAttrSchema). Instance-admin-managed reference data, for **papers**.
- **Document** (aggregate root) — a person-held **paper** of some type, with its number, issuer,
  issuing country, and validity window.
- **Personal-code scheme** (catalog) — a **country-namespaced** national-identifier scheme
  (e.g. `ua-rnokpp`, `us-ssn`), carrying `country_iso`, a semantic `generic_category`, and optional
  validation (D-PersonalCodes). Instance-admin-managed reference data, for **codes**.
- **Personal code** — a person-held government identifier of some scheme, its `value`
  **envelope-encrypted at rest** (`pii:sensitive`; D-CryptoProvider).

(People live in [person](person.md); a document/code only *points* at a person. Countries are the
`geo_countries` registry (D-Geo). Translatable type/scheme `name`s live in the
[localization](localization.md) store.)

## Data model

Conventions per [conventions.md](../architecture/conventions.md).

**`document_document_types`** (instance-admin catalog)
- `id` PK
- `code TEXT NOT NULL UNIQUE` — **stable, locale-agnostic** identifier (D-Code); immutable by
  convention. Seeded with a representative set (`passport`, `national-id`, `tax-id`,
  `social-insurance`, `driver-license`, `military-id`); the instance admin adds more — `pii:none`
- `name TEXT NOT NULL` — default-locale label; **translatable** via [localization](localization.md)
- `attr_schema JSONB` — **optional** per-type attribute schema (D-DocumentAttrSchema): when non-null it
  declares the fields a document's `attributes` may/must carry (`{ "fields": { "<name>": { "type":
  "string|number|boolean|date", "required": <bool>, "enum": [...]? } } }`), validated on every document
  write. The seeded `military-id` type ships one (VOS/specialty, fitness category, mobilization
  category, issuing commissariat). When null, `attributes` is free-form — `pii:none`
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired'))`
- `sort_order INT`
- `created_at`, `updated_at`, `deleted_at`

**`document_documents`** (person-held papers / codes)
- `id` PK
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT` — the holder
- `type_id TEXT NOT NULL REFERENCES document_document_types(id) ON DELETE RESTRICT`
- `number TEXT` — the document number (passport no., licence no.) — **`pii:basic`** (highly
  identifying personal data). *(Government personal codes — РНОКПП, SSN — are no longer stored here;
  they live in `document_personal_codes` below, encrypted.)*
- `issuer TEXT` — issuing authority (e.g. "ДМС України") — `pii:basic`
- `issuing_country CHAR(2) REFERENCES geo_countries(code) ON DELETE RESTRICT` — the country that
  issued this paper; nullable. Lets one person hold several same-type papers from different countries
  (e.g. a UA **and** a PL passport) — D-Geo / D-PersonalCodes — `pii:none`
- `issued_on DATE`, `expires_on DATE` — validity window (a `DATE`, not an instant) — `pii:none`
- `attributes JSONB NOT NULL DEFAULT '{}'` — long-tail per-type fields (place of issue, series,
  endorsements); column-ize a key once shared/queried. When the document's **type declares an
  `attr_schema`**, `attributes` is **validated against it on write** (D-DocumentAttrSchema). —
  `pii:special` **(ceiling)**: a grab-bag may hold up to special-category data, so tagged at the
  ceiling (D-PIITiers); special-category fields must not land here without the envelope seam
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','superseded','revoked'))` —
  `active` = current; `superseded` = replaced by a renewal/reissue (e.g. a new passport); `revoked` =
  invalidated by the issuer. Transitions are **admin-set via `PUT /documents/{id}`**:
  `active → superseded`, `active → revoked`, and back to `active` (all reversible; self-asserted
  metadata, not an authority-checked validity workflow — that is a parked seam). Status is **orthogonal
  to `deleted_at`**: status records the document's real-world standing, `deleted_at` records removal of
  the record (soft-delete via `DELETE`).
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** `UNIQUE (person_id, type_id, number) WHERE number IS NOT NULL AND deleted_at IS
  NULL` — a person does not hold the same numbered document twice.
- Indexes: `(person_id) WHERE deleted_at IS NULL`, `(type_id)`.

All other columns (`id`, `person_id`, `type_id`, `status`, `sort_order`, lifecycle timestamps,
`issuing_country`, `issued_on`/`expires_on`) are `pii:none` (D-PIITiers).

**`document_personal_code_schemes`** (instance-admin catalog — national-identifier schemes; D-PersonalCodes)
- `code TEXT PRIMARY KEY` — the **country-namespaced scheme** id (D-Code); the **natural-key PK** (the
  D-ResourceIdentifiers carve-out, like `geo_countries` / `i18n_locales` — a seeded shared reference
  registry FK'd by code, **not** an RID-keyed runtime entity, so it can be seeded in the migration).
  Immutable by convention. Seeded with a representative set (`ua-rnokpp`, `ua-unzr`, `us-ssn`,
  `de-steuer-id`, `it-codice-fiscale`, `pl-pesel`, …); the instance admin adds more — `pii:none`
- `country_iso CHAR(2) REFERENCES geo_countries(code) ON DELETE RESTRICT` — the scheme's issuing
  country (NOT NULL for national schemes) — `pii:none`
- `generic_category TEXT NOT NULL CHECK (generic_category IN ('tax-id','national-id',
  'social-insurance','health-insurance','residence-permit','other'))` — the **semantic grouping**
  that cross-scheme queries join on ("list everyone's tax IDs") — `pii:none`
- `name TEXT NOT NULL` — default-locale label; **translatable** via [localization](localization.md)
- `validation_regex TEXT` — optional data-side **fallback** format check (a compiled
  `pkg/personalcode` validator takes precedence; see *Validation*) — `pii:none`
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired'))`
- `sort_order INT`
- `created_at`, `updated_at`, `deleted_at`

**`document_personal_codes`** (person-held national identifiers; value encrypted — D-CryptoProvider)
- `id` PK (RID — `new_rid('document','personal_code')`)
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT` — the holder
- `scheme_code TEXT NOT NULL REFERENCES document_personal_code_schemes(code) ON DELETE RESTRICT` — which
  scheme (FK to the scheme's natural `code` PK); the code's **country derives from `scheme.country_iso`**
  (no per-row country)
- `value_ciphertext BYTEA` — the identifier, **envelope-encrypted** — **`pii:sensitive`**. **Nullable**
  so person purge can crypto-erase it (NULL once erased); always set on active rows.
- `wrapped_dek BYTEA` — the per-record data key, wrapped by the KMS-held KEK — `secret`. **Nullable**
  for the same reason (crypto-erase destroys it).
- `key_ref TEXT NOT NULL` — the KEK id + version used (D-CryptoProvider) — `pii:none`
- `value_blind_index BYTEA NOT NULL` — keyed HMAC of the normalized value, for equality lookup /
  uniqueness without decryption — `pii:none` (opaque; not reversible to the value)
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','superseded','revoked'))`
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** `UNIQUE (scheme_code, value_blind_index) WHERE deleted_at IS NULL` — the same code in
  the same scheme is not held by two people (cross-person), enforced over the blind index since the
  value is ciphertext.
- Index `(person_id) WHERE deleted_at IS NULL`, `(scheme_code)`.

### Validation (D-PersonalCodes)

A personal code is validated on create/update against its scheme, **code-authoritative**: a compiled
`pkg/personalcode` validator keyed on the scheme (e.g. UA-RNOKPP checksum, IT codice fiscale, US-SSN
format) is the authority; the scheme's optional `validation_regex` is a **fallback** for schemes with
no compiled validator; a scheme with neither **accepts with a logged warning**. Precedence:
**code validator > catalog regex > accept-and-warn**. Validation runs on the plaintext **before**
encryption; only the ciphertext + blind index are persisted.

### PII governance

`document_documents` is a secondary PII store (`number`/`issuer` are `pii:basic`; `attributes` is the
`pii:special` ceiling). `document_personal_codes.value` is **`pii:sensitive`** — stored only as
ciphertext (envelope encryption, KEK in the external KMS; D-CryptoProvider) with a blind index for
lookup; the plaintext is never persisted and is `werror.UnsafeParam`-redacted in logs.

Both participate in the [person](person.md) **purge**: the module subscribes to the **`PersonPurged`**
event and, for that person, erases document `number`/`issuer`/`attributes` (NULLed) **and
crypto-erases each personal code** by destroying its `wrapped_dek` (and nulling `value_ciphertext`),
rendering the value unrecoverable; the row `id` is kept as a tombstone so audit history stays
resolvable — the same discipline as the person tombstone. This erasure write is **audited as
`actor_type='system'`, `subsystem='event-subscriber'`** (D-Audit), correlated to the originating
`person.purge` by `request_id` (see [audit](audit.md)). Log document/code identifiers with
`werror.UnsafeParam` (redacted), never raw.

## Conjure API surface

`DocumentService`:

| Op | Intent | Perm |
|---|---|---|
| `POST /persons/{personId}/documents` | Attach a document (paper) to a person | `document.create` |
| `GET /persons/{personId}/documents` | List a person's documents (token-paginated) | `document.read` + shadow gate |
| `GET /documents/{id}` | Read one document | `document.read` + shadow gate |
| `PUT /documents/{id}` | Update number/issuer/issuing-country/validity/attributes/**status** | `document.update` |
| `DELETE /documents/{id}` | Soft-delete (reversible) a document | `document.delete` |
| `GET /document-types` | List the paper-type catalog | `document.type.read` |
| `POST /document-types` | Add a document type | `document.type.manage` |
| `PUT /document-types/{id}` | Edit a document type (code immutable by convention) | `document.type.manage` |
| `POST /persons/{personId}/personal-codes` | Attach a personal code (value validated + encrypted) | `personal-code.create` |
| `GET /persons/{personId}/personal-codes` | List a person's personal codes (value decrypted on read) | `personal-code.read` + shadow gate |
| `PUT /personal-codes/{id}` | Update a personal code (re-validated + re-encrypted) | `personal-code.update` |
| `DELETE /personal-codes/{id}` | Soft-delete a personal code | `personal-code.delete` |
| `GET /personal-code-schemes` | List the scheme catalog (filter by `country`/`category`) | `personal-code-scheme.read` |
| `POST /personal-code-schemes` | Add a scheme | `personal-code-scheme.manage` |
| `PUT /personal-code-schemes/{id}` | Edit a scheme (code immutable by convention) | `personal-code-scheme.manage` |

Type/scheme `name` is returned as a `locale → text` map (i18n convention). Documents **and personal
codes** are scoped **through the holder** per [person](person.md)'s canonical read-scope rule
(D-PersonReadScope) and pass the **shadow-visibility gate**: a reader must be able to read the holder
person (instance plane, or their readable unit-set intersecting the holder's active memberships).
Records of a **membership-less** holder are therefore readable **only on the instance plane**.
Reading a personal code **decrypts** its value (KMS unwrap of the wrapped DEK; D-CryptoProvider) —
itself a sensitive action, gated by `personal-code.read`. Submitting a value that fails its scheme's
validator → `Document:PersonalCodeInvalid`; a duplicate `(scheme, value)` → `Document:PersonalCodeDuplicate`.

## Dependencies

- **Calls:** [person](person.md) (person exists), [localization](localization.md) (assemble the
  type/scheme `name` locale-map; purge translations on retire), the **`geo_countries`** registry
  ([platform](platform.md); validate `issuing_country` / scheme `country_iso`), **`pkg/crypto`**
  ([platform](platform.md); envelope wrap/unwrap + blind index for personal-code values) and
  **`pkg/personalcode`** ([platform](platform.md); validators). [platform](platform.md) for infra.
  Emits `DocumentCreated`, `DocumentUpdated`, `DocumentDeleted`, `PersonalCodeCreated`,
  `PersonalCodeUpdated`, `PersonalCodeDeleted` events.
- **Consumes:** [person](person.md)'s **`PersonPurged`** event (erases the person's documents +
  crypto-erases personal codes).
- **Called by:** read surfaces listing a person's documents / personal codes; [audit](audit.md).

## Authorization touchpoints

Defines/gates: `document.create`, `document.read`, `document.update`, `document.delete` and the
parallel **personal-code** permissions `personal-code.create`, `personal-code.read`,
`personal-code.update`, `personal-code.delete` (all document plane, scoped **through the holder** per
[person](person.md)'s read-scope rule (D-PersonReadScope) + shadow gate) and the instance-plane
catalog permissions `document.type.manage`/`document.type.read` and
`personal-code-scheme.manage`/`personal-code-scheme.read`. The module never decides access — it calls
the PDP. A document or personal code carries **no** authority (it is directory data, like
rank/position).

## Patterns

- **Stable code vs translatable name** ([patterns.md](../architecture/patterns.md)) for
  `document_types` and `document_personal_code_schemes` (D-Code + D-i18n).
- **Instance-scope catalog vs unit-scope data**: the type and scheme catalogs are
  instance-admin-managed (like the rank scheme / locales); the documents / personal codes themselves
  are person-attached and read-scoped through the holder.
- **Envelope encryption for `pii:sensitive`** (D-CryptoProvider): personal-code values are ciphertext
  in the DB with the KEK in an external KMS; lookup/uniqueness uses a keyed blind index.
- **Audit-on-write**: every create/update/delete and type-catalog edit records in-transaction
  ([audit](audit.md), D-Audit).
- **Reversibility**: documents soft-delete via `deleted_at` (reversible), never hard-delete; the
  `status` transitions (`superseded`/`revoked`) are likewise reversible flips, independent of
  `deleted_at` (see the data model).

## Invariants & safety

- A document belongs to **exactly one person** and **one type**; a personal code to **one person** and
  **one scheme**; a type/scheme in use cannot be hard-deleted (`ON DELETE RESTRICT`) — it is *retired*.
- A person does not hold the same `(type, number)` twice among active documents; a `(scheme, value)`
  is unique **cross-person** (over the blind index) among active personal codes.
- A **personal code's country derives from its scheme** (`scheme.country_iso`); a **paper's**
  `issuing_country` is per-document (so a person may hold same-type papers from several countries —
  e.g. dual passports; D-Geo).
- A personal code's `value` is **`pii:sensitive`**, stored only as ciphertext (envelope encryption,
  KEK in the external KMS; D-CryptoProvider), validated before encryption (D-PersonalCodes).
- Document `number`/`issuer` are `pii:basic` and **erased on person purge**; personal codes are
  **crypto-erased** on purge (wrapped DEK destroyed); the row `id` survives as a tombstone.
- A document or personal code grants **no** authorization (directory data only).
- **`status` is admin-set, self-asserted, and reversible** (`active ↔ superseded`, `active ↔ revoked`
  via `PUT`), independent of `deleted_at`; today it is metadata, not an authority-checked validity
  workflow (parked seam).
- The type **and personal-code scheme** catalogs are **instance-admin-managed**; codes are immutable
  by convention.
- When a document's type declares an **`attr_schema`**, its `attributes` is **validated against the
  schema on every write** (unknown keys rejected, required keys enforced, declared types/enums checked;
  D-DocumentAttrSchema) — a violation is `ErrDocumentInvalid`. A null `attr_schema` leaves `attributes`
  free-form. Validated values still ride in the JSONB at the `pii:special` ceiling — a genuinely
  special-category field waits on the envelope seam (DS-29).
- **RLS backstop.** `document_documents` **and `document_personal_codes`** have **no unit column**
  (scoped via the holder), so they are **exempt from the direct `app.readable_units` predicate**
  (D-RLSDefenseInDepth): the app-layer PDP (D-PersonReadScope) is the authoritative read scope; a
  person→unit reach-join RLS policy is a noted hardening seam, not shipped.

## Open seams / future

- **Document binary/attachment storage** (scanned passports, photos) — default is metadata only; a
  blob/object-store seam is parked (open-questions DS-39).
- **Document verification / validity workflow** (mark a document verified by an authority) — additive
  later; today documents are self-asserted metadata.
- **Expiry-driven notifications** (passport about to expire) need the worker runtime (DS-25).
- **Envelope crypto for `pii:special`** (Art. 9 fields, audit payloads) reuses the D-CryptoProvider
  `KeyProvider` seam but is parked under DS-29; today only `pii:sensitive` personal-code values are
  encrypted.
- **Per-scheme `encrypt_at_rest` toggle** (operator-chosen encryption coverage per scheme) is a
  possible additive refinement; today all personal-code values are encrypted.
