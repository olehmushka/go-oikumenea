# Module: personsensitive

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md) ·
> [person](person.md) (the core aggregate + authoritative data model)
> Table prefix: `oikumenea.person_*` (shared with [person](person.md) — see below)

> **Concern module (D-PersonModuleSplit, review-2026-07 R-09).** `personsensitive` is one of the three
> Go modules the person god-module split into, behind the **one** Conjure `PersonService`. It owns
> **everything envelope-encrypted or `pii:special`** on a person — so the whole crypto surface (and its
> reviewers) live in one module (the R-09 headline). It is **not** a separate schema or API service: its
> tables live in the `oikumenea.person_*` schema whose **authoritative data model + endpoint
> documentation stays in [person.md](person.md)**; this doc records the *application-layer concern
> boundary* and points back. (Since PR-2b this module owns its **own** `adapters`/`queries` slice +
> generated `personsensitivesql` package over the `domain.SensitiveRepository` port; see
> [decisions.md](../architecture/decisions.md) D-PersonModuleSplit.)

## Purpose

Owns the **special-category and encrypted overlay** surface of a person: physical identity, the
envelope-encrypted ethnicity and party membership, the M35 inferred/self-declared overlays, and the
regulatory watchlist/sanctions picture. This is the highest-risk person code — GDPR Art. 9 data, the
`crypto.Cipher`, blind indexes, `legal_basis` gating, and crypto-erasure. Concentrating it in one
module means **one place holds the cipher and one set of reviewers guards the Art. 9 datum**, instead of
that surface being smeared across the whole person module (its state before R-09).

## Entities & aggregates

Ontology kinds are in the binding [registry](../ontology-mapping.md) (RID tokens unchanged by the split);
the **field-level data model for these entities lives in [§ Data model](#data-model) below — this module
is its owner** (moved here from [person.md](person.md)'s OSINT cluster, D-PersonModuleSplit; the binding
per-field verdicts remain [draft_superbrain_schema.md](../draft_superbrain_schema.md)). This module owns
the application logic **and data model** for:

- **Physical identity** — `person_physical_descriptions` (+ `blood_type`; hard-FK eye/hair colors via
  the M42 `platform_colors` catalog) and `person_distinguishing_marks` (`pii:special` ceiling)
  (D-PhysicalIdentity, M31). The **alias** name variants (`variant_kind`) stay **core** — aliases are
  still names.
- **Encrypted ethnicity** — the `link__has_ethnicity` (`person_ethnicities`): the declared ethnicity
  **code itself is envelope-encrypted + blind-indexed**, NOT-NULL `legal_basis` (Art. 9); no plaintext
  catalog FK. The open `person_ethnicity_types` hierarchy is fed by the Factbook import (D-PhysicalIdentity).
- **Overlays (M35, D-PersonOverlays)** — `person_crypto_wallets` (`pii:sensitive`),
  `person_personality` (`pii:sensitive`, declared-survey/HR-assessment only, never text-inferred), and
  the **inferred** `person_political_leaning` (`pii:special`, spectrum ∈ [-1,1] envelope-encrypted, one
  active row, **never merged** with the declared M33 party membership).
- **Watchlists (M34, D-Watchlists)** — `person_watchlist_matches` (transient, refreshed in place via the
  synchronous `CheckWatchlists` → [hermenea](hermenea.md) call) and the durable
  `person_regulatory_sanctions` (`pii:sensitive`, a hermenea import target). Local PEP is derived from
  M33 government positions (owned by [personprofile](personprofile.md)) and snapshotted at check time.
- **Encrypted party membership** — `person_party_memberships` (`link__party_membership`, `pii:special`:
  the party identity is envelope-encrypted + blind-indexed, NOT-NULL `legal_basis` Art. 9)
  (D-InstitutionalTies, M33). The **non-encrypted** institutional ties from the same decision
  (government positions, lobbying, external references) live in [personprofile](personprofile.md).

## Data model

Conventions per [conventions.md](../architecture/conventions.md). **This module is the authoritative
owner of the data model for the sensitive/encrypted `person_*` tables below** (D-PersonModuleSplit — the
entity sections were moved here from [person.md](person.md)'s *OSINT-enrichment cluster*). The
exhaustive per-field verdicts remain the binding [draft_superbrain_schema.md](../draft_superbrain_schema.md);
the cluster decisions live in [roadmap-decisions.md](../architecture/roadmap-decisions.md). Since PR-2b
these tables are served by `personsensitive`'s **own** `internal/personsensitive/adapters` repository
over its own `queries/person.sql` + generated `personsensitivesql` package (the
`domain.SensitiveRepository` port). Its watchlist screening reads the PEP flag from `personprofile`
through a late-bound `PEPStatusReader` seam — never `person_government_positions` directly — and verifies
the parent via a reviewed `PersonExists` / `GetPerson` read on `person_persons`.

Three rules hold across every table: **declared ≠ inferred** (never merged), **every overlay carries
`source`+`confidence`+`as_of`**, and **special-category data is gated** — envelope-encrypted
(D-SpecialPII / D-CryptoProvider), NOT-NULL structured `legal_basis` (GDPR Art. 6/9), audited, and
**crypto-erased** on purge (the wrapped DEK is destroyed).

### Physical identity (D-PhysicalIdentity, M31; migration `0030`)

**`person_physical_descriptions`** — height / weight / build, hard-FK `eye_color_id` / `hair_color_id`
→ `platform_colors` (M42 · D-Color, domains `eye`/`hair`, validated app-side), and `blood_type`. `pii:basic`;
hard-deleted on purge. (The **alias** name variants — `person_name_variants.variant_kind` — stay **core**:
aliases are still names; see [person.md](person.md).)

**`person_distinguishing_marks`** — scars/tattoos/etc.; `pii:special` **ceiling**; erased on purge.

**`person_ethnicity_types`** — open, **hierarchical** declared-ethnicity catalog: `parent_id` +
`person_ethnicity_type_closure` + `wikidata_id` + group-level M:N to Glottolog `language_languoids`
(ethnolinguistic) and `geo_countries` (homelands). Fed by the opt-in **CIA World Factbook**
`ethnicity-scheme` hermenea import (public domain; no committed preset; default catalog empty — the
Factbook carries no hierarchy/language, so those columns stay unpopulated by that source). The
group↔language tie is **never** inferred onto a person.

**`person_ethnicities`** (link `link__has_ethnicity`, RID `6,2,9`, `pii:special`) — the declared
ethnicity **code itself is envelope-encrypted + blind-indexed** (NO plaintext catalog FK — the Art. 9
datum never sits in plaintext), NOT-NULL `legal_basis`; crypto-erased on purge. Biometrics **excluded**.

### Encrypted party membership (D-InstitutionalTies, M33; migration `0032`)

**`person_party_memberships`** (link `link__party_membership`, RID `6,2,11`, `pii:special`) — the party
identity is **envelope-encrypted + blind-indexed**, NOT-NULL `legal_basis` (Art. 9); carries
`source`/`confidence`; crypto-erased on purge. The **non-encrypted** M33 ties (government positions,
lobbying, external references) live in [personprofile](personprofile.md). Inferred political leaning
(below) is a **separate** overlay, **never** merged with this declared membership.

### Watchlists & sanctions (D-Watchlists, M34)

**`person_watchlist_matches`** (RID `6,1,15`) — **metadata only**, one **active** row per person,
refreshed in place by the synchronous `CheckWatchlists` → [hermenea](hermenea.md) call (the lists
themselves are never stored locally; ≤24h cache lives in hermenea). PEP is derived **locally** from M33
government positions via the `PEPStatusReader` seam and snapshotted at check time. Dropped (transient) on
merge; hard-deleted on purge.

**`person_regulatory_sanctions`** (RID `6,1,16`, `pii:sensitive`) — durable audited overlay **and** a
hermenea import target (idempotent by `(person, externalId)`). Re-homed on merge; hard-deleted on purge.

### Overlays (D-PersonOverlays, M35; migration `0035`)

**`person_crypto_wallets`** (object `6,1,17`, `pii:sensitive`) — chain + address (address is public
on-chain data but the attribution is sensitive); dedup one active `(person, chain, address)`; M34
sanctioned-wallet synergy. Plaintext; hard-deleted on purge.

**`person_personality`** (object `6,1,18`, `pii:sensitive`) — a `method` CHECK enforces
**declared-survey / HR-assessment only** — no text-inference. Plaintext; hard-deleted on purge.

**`person_political_leaning`** (object `6,1,19`, `pii:special`) — **inferred**; keeps the spectrum ∈
[-1,1] (double precision) **envelope-encrypted**, NOT-NULL `legal_basis` (Art. 9), **one active row per
person**, crypto-erased on purge. It is in a **separate** table that is **never merged** with the declared
M33 party membership — the partial-unique `person_id` is **dropped** on merge, not re-homed.

### Legal records (D-LegalRecords, M38; migration `0016`)

**`person_legal_records`** (object `6,1,22`, `pii:special`, GDPR **Art. 10**) — a category-level
criminal/arrest/court record. `kind ∈ {criminal_conviction, arrest, court_judgment}` and a **mandatory
`disposition`** (arrest ≠ guilt) stay plaintext but are marked `pii:special`; the category-level offence
`detail` (a coarse category — **NO full charge sheet**) is **envelope-encrypted**, NOT-NULL
`legal_basis` (Art. 10), `source`+`confidence`. **Many rows per person** (no one-active-per-kind
uniqueness). **Jurisdiction** is a hard FK to `geo_countries` (D-Geo). **Never inferred**;
crypto-erased on purge. **Suppression:** `is_suppressed` + `suppressed_reason ∈ {sealed, expunged}`
mark a record that is **retained** but withheld from the normal read gate — see Authorization below.

## Conjure endpoint sketch

No separate service. The sensitive operations are the physical-identity / ethnicity / overlay /
watchlist / sanctions / party-membership / legal-record rows of the single `PersonService` in
[person.md § Conjure API surface](person.md#conjure-api-surface); the one `person/transport.Service`
delegates each of those handlers to this module.

## Dependencies

- **Calls:** `pkg/crypto` (the `crypto.Cipher` — **owned/held by this module** — for
  seal/open/blind-index of ethnicity, party, and political-leaning), [platform](platform.md) (the
  `ColorLookup` seam — validate eye/hair colors — **owned here**; and the `legal_basis` catalog),
  [hermenea](hermenea.md) (the `WatchlistLookup` seam — **owned here** — for the synchronous
  `CheckWatchlists` egress; the PDP core never calls out), and [person](person.md) core for existence.
- **Called by:** the composition root, which registers it
  (`personsensitive.Register(pool, audit, cipher)`), wires its `ColorLookup` + `WatchlistLookup` seams,
  binds the `PEPStatusReader` seam, asserts `MustBeBound` (the watchlist seam), and subscribes it to
  `PersonPurged` (`SubscribePersonPurge` → crypto-erase / hard-delete in the purge tx). Merge re-point of
  person-owned rows stays core-driven (`RepointPersonOwned`).

## Authorization touchpoints

Every operation is gated by the same `person.read` / `person.update` permissions and the **read-scope
rule** as core (D-PersonReadScope), with **audit on every write** (D-Audit) — the audit payload carries
only non-PII identifiers, never the plaintext of an encrypted field. This module **never** decides
access; it calls the PDP via the shared transport.

The `pii:special` Art. 9/Art. 10 reads each carry **their own need-to-know code**, composed into the
additive **sensitive-reader** base role and deliberately NOT implied by unit-admin (D-DataScope, R-14):
`person.ethnicity.read`, `person.political_leaning.read`, `person.party_membership.read`,
`person.health.read`, and `person.legal-record.read` (D-LegalRecords, M38). Legal records add a
**second, stricter** gate: **sealed/expunged (suppressed)** rows are withheld from
`person.legal-record.read` and revealed only to a caller who **also** holds
`person.legal-record.read-suppressed` (in **no** base role — granted explicitly) or is an instance
admin. Transport probes it with a non-erroring `AllowedAnywhere` and passes `includeSuppressed` down
(the R-31 sensitive-reader redaction pattern).

## Patterns

- **Envelope encryption** for every `pii:special` datum (D-CryptoProvider / D-SpecialPII): seal +
  blind-index, NOT-NULL `legal_basis`, **crypto-erase on purge** (destroy the wrapped DEK).
- **Declared ≠ inferred** — the inferred political leaning is a **separate** table, **never** merged
  with the declared party membership (its partial-unique `person_id` is dropped on merge, not re-homed).
- **Late-bound seam + `MustBeBound`** for `WatchlistLookup` / `ColorLookup` (R-11).
- **First synchronous `oikumenea → hermenea` call** (`CheckWatchlists`) via the `WatchlistLookup` seam,
  wired in `main.go` like the location/color seams (D-Watchlists).

## Invariants & safety

- Special-category data (ethnicity, party, political leaning) is **never stored in plaintext** and
  carries a NOT-NULL `legal_basis` (GDPR Art. 6/9); it is **crypto-erased** on purge, the plaintext
  overlays hard-deleted.
- **Declared and inferred are never merged** (party membership vs. political leaning).
- Personality is **declared-survey / HR-assessment only** — no text-inference (a `method` CHECK
  enforces it). Biometrics are **excluded**.
- The transient watchlist match is one active row per person, refreshed in place; the lists themselves
  are never stored locally (≤24h cache lives in hermenea).
- Legal records are **category-level only** (no full charge sheet), **never inferred**, carry a
  **mandatory `disposition`** (arrest ≠ guilt) and a NOT-NULL Art. 10 `legal_basis`; **sealed/expunged
  records are suppressed** (retained but hidden behind `person.legal-record.read-suppressed`), never
  hard-deleted for suppression.

## Open seams / future

- **Jurisdiction-specific display / storage rules** for legal records (Ban-the-Box hiring windows,
  FCRA lookback limits) — D-LegalRecords lands the **data hook** (jurisdiction FK + suppression) but
  **not** the rule engine; that is a future milestone. The jurisdiction subnational subdivision (a
  finer FK than `geo_countries`) is a related seam.
- **First-class discipline / incentive records** (order module DS-36) remain distinct from these
  external judicial facts — reprimand/gratitude/bonus are still record-only order items.
