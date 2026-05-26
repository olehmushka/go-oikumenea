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

- **Person** (aggregate root) — names, structured/free attributes, status, optional `rank_id`,
  lifecycle timestamps.

(Accounts live in [identity-federation](identity-federation.md); memberships in
[membership](membership.md); rank definitions in [rank](rank.md). Person only *points* at a
rank.)

## Data model

Conventions per [conventions.md](../architecture/conventions.md).

**`person_persons`**
- `id` PK
- `code TEXT` — **optional** stable, locale-agnostic external identifier (e.g. a personnel /
  service number) for external-system reference (D-Code); `UNIQUE WHERE code IS NOT NULL AND
  deleted_at IS NULL`
- `display_name TEXT NOT NULL` — the primary name form
- `given_name TEXT`, `family_name TEXT` — optional structured names
- `attributes JSONB NOT NULL DEFAULT '{}'` — the long-tail directory fields (service number,
  contact, etc.); column-ize a key once it is shared/queried (escape-hatch discipline)
- `rank_id UUID REFERENCES rank_ranks(id) ON DELETE RESTRICT` — one rank, nullable (a person
  may be unranked); restrict so a rank in use cannot be deleted out from under a person
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','deactivated','purged'))`
- `deactivated_at TIMESTAMPTZ`, `purge_after TIMESTAMPTZ` — reversibility window
- `created_at`, `updated_at`, `deleted_at`

Indexes: `rank_id`; a trigram/`citext` index on `display_name` for directory search (added when
search is built); partial unique on any natural key the operator configures (none mandated).

**`person_name_variants`** (transliteration / alternate name forms)
- `id` PK
- `person_id UUID NOT NULL REFERENCES person_persons(id) ON DELETE CASCADE`
- `locale TEXT NOT NULL REFERENCES i18n_locales(code) ON UPDATE RESTRICT` — the locale/script
  this form is for (e.g. `ukr` native, `eng` Latin transliteration)
- `display_name TEXT NOT NULL`, `given_name TEXT`, `family_name TEXT`
- `is_primary BOOLEAN NOT NULL DEFAULT FALSE`
- `created_at`, `updated_at`
- `UNIQUE (person_id, locale)`

Person names are **per-record data managed by the person's admins** — *not* the instance-admin
[localization](localization.md) translation store (D-i18n). A person has one canonical
`display_name` plus zero or more locale-tagged variants (the transliterations the user asked
for, e.g. "Олег" / "Oleh"). Responses return the canonical name plus the variants; clients pick
by locale.

### PII governance

`person_persons` is the system's primary PII store. Discipline carried from drafts'
DATA-GOVERNANCE:
- Treat `display_name`, structured names, and `attributes` as personal data. Log person
  identifiers with `werror.UnsafeParam` (redacted), never raw in service logs.
- Erasure is the **purge** path (below): mutable PII columns are NULLed, the `id` is kept as a
  tombstone so audit history (which references the id) stays intact. A future
  `COMMENT ON COLUMN` PII-tier annotation is an additive seam.

## Conjure API surface

`PersonService`:

| Op | Intent | Perm |
|---|---|---|
| `POST /persons` | Create a person (no account, no unit needed) | `person.create` |
| `GET /persons/{id}` | Read one | `person.read` |
| `PUT /persons/{id}` | Update names/attributes | `person.update` |
| `GET /persons` | Search/list the directory (token-paginated) | `person.read` |
| `PUT /persons/{id}/rank` | Set/clear the person's rank | `person.rank.assign` |
| `POST /persons/{id}/deactivate` | Begin reversible deactivation (grace window) | `person.lifecycle` |
| `POST /persons/{id}/reactivate` | Cancel deactivation within grace | `person.lifecycle` |
| `POST /persons/{id}/purge` | Hard-erase PII after grace (idempotent) | `person.purge` |
| `PUT /persons/{id}/name-variants` | Upsert locale name forms (transliteration) | `person.update` |
| `DELETE /persons/{id}/name-variants/{locale}` | Remove a name variant | `person.update` |

Read endpoints that list people *by unit* are served by [membership](membership.md) and pass
the shadow gate; `PersonService` directory reads are gated on `person.read` (instance/subtree
scope as configured).

## Dependencies

- **Calls:** [rank](rank.md) (validate `rank_id` exists), [localization](localization.md)
  (validate name-variant `locale` codes). [platform](platform.md) for infra. Emits
  `PersonCreated`, `PersonDeactivated`, `PersonPurged` events.
- **Called by:** [membership](membership.md) (a membership references a person),
  [identity-federation](identity-federation.md) (an account links to a person),
  [authorization](authorization.md) (assignment subject is a person id), [audit](audit.md).

## Authorization touchpoints

Defines/gates: `person.create`, `person.read`, `person.update`, `person.rank.assign`,
`person.lifecycle`, `person.purge`. Scope is typically instance- or subtree-scoped (an admin
over a subtree manages people, surfaced through their memberships). The module never decides
access — it calls the PDP, and **never reads rank to make a decision**.

## Invariants & safety

- A person needs **no** account and **no** membership to exist.
- A person holds **at most one** rank; deleting a rank still in use is blocked
  (`ON DELETE RESTRICT`).
- **Reversible lifecycle:** `active → deactivated` (sets `deactivated_at` + `purge_after`,
  reversible within grace) `→ purged` (PII NULLed, `id` retained as tombstone). Purge refuses
  before `purge_after`.
- Rank assignment grants no authority (D-Rank); enforced by convention + review, documented so
  no implementer couples them.
- Optional external `code` is unique among active persons when set; name variants are unique per
  `(person, locale)` and are person-managed data, not the instance translation store.

## Open seams / future

- Cross-deployment / federation of person identity is **out of scope** (single domain).
- `attributes` JSONB awaits column-ization as real directory fields stabilize.
- Self-service subject-rights export is a future additive endpoint; MVP erasure is the
  operator-driven purge.
