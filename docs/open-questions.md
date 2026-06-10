# Open questions — the deferred-seam backlog

The live backlog for the next planning session. Every entry is a **design seam that is
deliberately deferred**: the system ships a stated **default** today, and the seam stays parked
until its **trigger** fires. The recurring question per entry is only *"promote it to a milestone
now, or leave it parked?"*.

This file is **not binding** — [`architecture/decisions.md`](architecture/decisions.md) is. Each
entry points at the module doc that owns the seam and, where relevant, the decision that shaped it.

## How to read an entry

```
DS-n · Title.
  Default — what the service does today (the current, shipped position).
  Trigger — the concrete need that would justify promoting the seam to real work.
  status  — `parked` (all current entries are parked; a promoted entry leaves this file).
```

**ID scheme.** `DS-n`, **stable once assigned** — other docs cite these IDs by number, so an ID is
never reused or renumbered. **Gaps are expected:** an entry that has been resolved (or ruled out of
scope) is **removed from this file**, and its outcome lives in
[`decisions.md`](architecture/decisions.md) + the owning `modules/*.md`. So the numbering is
sparse on purpose; do not "fill in" a missing number.

**Dependency note.** Many entries are blocked on the same missing capability — a **background
job/worker runtime** (**DS-25**). Anything needing a scheduler (background purges, partition
maintenance, expiry sweeps, expiry notifications) waits behind it.

---

## tenant — [`modules/tenant.md`](modules/tenant.md)

**DS-4 · Growth of the unit-lifecycle state set.**
- *Default* — `active | suspended | archived`.
- *Trigger* — a new lifecycle state is needed → additive `TEXT`+`CHECK` expand/contract
  migration. `parked`

---

## person — [`modules/person.md`](modules/person.md)

**DS-6 · Promote `attributes` keys to columns.**
- *Default* — long-tail directory fields live in the `attributes` JSONB grab-bag.
- *Trigger* — a field stabilizes (shared / queried) → column-ize it as a typed column with its
  own PII tier. `parked`

**DS-7 · Self-service subject-rights export (GDPR Art. 15 / 20).**
The erasure half (Art. 17) is **already covered** by the operator-driven purge; only data *export*
is deferred.
- *Default* — no self-service export; subject-rights erasure is the operator purge path.
- *Trigger* — a self-service access/portability export is wanted → additive read endpoint. `parked`

**DS-38 · Partial / approximate birthdate (+ gender identity).**
Narrowed by [D-PersonBio](architecture/decisions.md): `birthdate` ships as a full `DATE` and `sex`
is ISO/IEC 5218.
- *Default* — full-precision `birthdate DATE`; gender *identity* is **not stored**.
- *Trigger* — (a) records that know only year (or year-month) → an additive partial-date
  representation; (b) gender identity, which is `pii:special` (GDPR Art. 9) and blocked until the
  envelope seam is **extended to `pii:special`** (DS-29; the `pii:sensitive` envelope already ships
  via D-CryptoProvider). `parked`

**DS-40 · Phone carrier / provider lookup.**
Introduced by [D-PersonContactChannels](architecture/decisions.md): `person_phones` derives the
**country** from the E.164 number, but **not** the carrier/provider.
- *Default* — no carrier/provider on a phone; only the derived `country` is stored (the email
  `provider` is derivable from the domain, a phone's is not).
- *Trigger* — carrier/provider is wanted → it is **not statically derivable** (number portability),
  so this needs an external HLR / number-lookup service (an external-dependency seam, akin to a KMS
  backend). `parked`

> **DS-41** (social-network / messenger references) and **DS-42** (person↔person relationships) were
> **promoted** to milestones **M13** and **M14** — now binding as
> [D-PersonSocialChannels](architecture/decisions.md) and
> [D-PersonRelationships](architecture/decisions.md), owned by [person](modules/person.md). Per the
> ID-stability rule the numbers are **retired, not reused** (the 41/42 gap is expected).

---

## membership — [`modules/membership.md`](modules/membership.md)

**DS-9 · Multi-incumbent positions.**
- *Default* — one active filling per billet (unique index).
- *Trigger* — a billet must hold several incumbents → relax the one-holder unique index. `parked`

**DS-10 · Standard-title catalog.**
- *Default* — each position carries its own translatable `title`.
- *Trigger* — reusable, instance-defined titles wanted → additive title-catalog layer. `parked`

**DS-11 · Central establishment control.**
- *Default* — creating a unit's billets is **unit-scoped**.
- *Trigger* — an org wants central/instance-controlled establishment (military TO&E/MTOE,
  university position lines are typically centrally owned) → a config switch. **Strong candidate**
  for the target domains, but additive, so the default holds until needed. `parked`

**DS-12 · Richer temporal membership queries.**
- *Default* — the effective-dating columns exist; only basic queries use them.
- *Trigger* — point-in-time / history ("who was in unit U on date D?") queries wanted →
  additive. `parked`

---

## document — [`modules/document.md`](modules/document.md)

**DS-39 · Document binary / attachment storage.**
- *Default* — **metadata only** (number, issuer, validity); no scanned-document blobs.
- *Trigger* — operators must store scanned passports/photos → an object-store/blob seam (and the
  PII-at-rest controls it implies). `parked`

---

## order — [`modules/order.md`](modules/order.md)

**DS-35 · First-class leave / absence entity.**
- *Default* — leave / business-trip are `record-only` order items.
- *Trigger* — leave balances, overlap checks, or absence reporting wanted → a dedicated
  entity. `parked`

**DS-36 · First-class discipline / incentive records.**
- *Default* — reprimand / rank-deprivation / gratitude / bonus are `record-only` order items.
- *Trigger* — disciplinary history with its own lifecycle/expiry wanted → a dedicated
  entity. `parked`

**DS-37 · Duty-roster as ephemeral assignments.**
- *Default* — the daily detail (duty officer & assistants) is a `record-only` order item.
- *Trigger* — schedulable, queryable rosters wanted → a dedicated ephemeral-assignment entity
  (likely needs the worker runtime, DS-25). `parked`

---

## rank — [`modules/rank.md`](modules/rank.md)

**DS-13 · `isSenior(a, b)` seniority helper.**
Seniority is already well-defined: within a system `(system.sort_order, category.sort_order,
type.sort_order path, rank.sort_order)`; across systems via the standardized `grade_code` (D-RankSystems).
- *Default* — not exposed as an API/domain function (no caller needs it).
- *Trigger* — a caller needs seniority comparison → expose the pure domain function. `parked`

**DS-43 · Non-military cross-system rank comparator.**
The standardized-grade comparator (D-RankSystems) is **NATO STANAG 2116** (military). Academic and
ecclesiastical deployments have no published cross-system grade, so `rank_grades`/`grade_code` are
military-shaped and a non-military `rank_system` leaves `grade_code` `NULL` (no cross-system comparison).
- *Default* — no comparator for non-military domains; cross-system comparison is N/A there (and rarely
  needed, since L-SingleDomain means one domain per instance).
- *Trigger* — a real academic/ecclesiastical deployment needs cross-institution rank equivalence →
  introduce a domain-appropriate grade scale (a second seeded catalog or a generic ordinal). `parked`

---

## authorization — [`modules/authorization.md`](modules/authorization.md)

**DS-18 · Subset-scoped instance-admin role.**
- *Default* — an instance admin holds the **full** instance-scope plane (all-or-nothing).
- *Trigger* — admins must be restricted to a *subset* of instance permissions → an
  instance-admin role selecting a permission subset. (There is **no** tier above instance-admin —
  D-Bootstrap / D-Audit; this is a refinement *within* the plane, not above it.) `parked`

---

## identity-federation — [`modules/identity-federation.md`](modules/identity-federation.md)

**DS-19 · Full-IdP pivot.**
- *Default* — pure relying party (L-AuthzOnly); the `password_hash` / `mfa_enrolled_at` columns are
  dormant (CHECK-NULL), reserved for a future `account_sessions` table.
- *Trigger* — the service must itself authenticate → activate the dormant seam. `parked`

**DS-20 · Multiple simultaneous issuers (multi-IdP).**
Distinct from the **resolved** *per-account* linking switch `account.identity_linking.enabled`
(default `true`), which gates whether one account may link several login points. DS-20 is about
which *issuers the deployment as a whole* accepts.
- *Default* — supported by config + the `(issuer, subject)` model; not exercised with >1 issuer.
- *Trigger* — a deployment configures more than one issuer. `parked`

---

## localization — [`modules/localization.md`](modules/localization.md)

**DS-22 · Per-entity typed translation tables.**
- *Default* — one shared, polymorphic `i18n_translations` store (no FK).
- *Trigger* — the polymorphic model proves limiting → mechanical migration to per-entity typed
  tables (FK integrity). `parked`

**DS-23 · Bulk translation import / export (TMS).**
- *Default* — per-entity upsert only.
- *Trigger* — translation-tooling (TMS) integration wanted → additive on
  `LocalizationService`. `parked`

**DS-24 · Accept-Language negotiation.**
- *Default* — **intentionally none**; every response returns all locales as a `locale → text` map
  (D-i18n).
- *Trigger* — negotiated single-locale responses wanted → an additive read option over the same
  store. `parked`

---

## platform — [`modules/platform.md`](modules/platform.md)

**DS-25 · Background job / worker runtime.** *(the common blocker — see the dependency note above.)*
- *Default* — synchronous core only.
- *Trigger* — any scheduled work is needed: background purges,
  partition maintenance (DS-28), expiry sweeps, expiry notifications, and **future-dated
  scheduled order effects** (apply an item at its `effective_from` rather than on issue — the
  residual of [D-OrderApply](architecture/decisions.md), which applies effects synchronously on
  issue). `parked`

**DS-26 · Event bus → real broker.**
- *Default* — in-process `pkg/events` with an outbox seam.
- *Trigger* — a module is extracted → swap the in-process bus for a broker without domain
  changes. `parked`

**DS-27 · OpenTelemetry export.**
- *Default* — witchcraft tracing.
- *Trigger* — an OTel pipeline is wanted → drop-in behind the tracing seam. `parked`

---

## audit — [`modules/audit.md`](modules/audit.md)

**DS-28 · Retention via partitioning.**
- *Default* — a single `audit_log` table.
- *Trigger* — volume grows → range-partition by `created_at` + a cold-archive policy (needs
  DS-25). `parked`

**DS-29 · PII envelope encryption — extend to `pii:special` + audit payloads.**
The **envelope-encryption mechanism now ships** ([D-CryptoProvider](architecture/decisions.md)): the
pluggable `KeyProvider` seam + `pkg/crypto` (ciphertext in DB, KEK in an external KMS, blind index,
crypto-erase) protect **`pii:sensitive`** national-identifier values today. What remains parked is
**extending that same mechanism** to the `pii:special` ceiling — `audit.before`/`after` and
special-category person fields — so it still gates **gender identity under DS-38**.
- *Default* — `pii:sensitive` (personal codes) is envelope-encrypted; `pii:special` is **not stored**
  (the "no special-category PII without this envelope" rule), with the "no secrets, minimal PII"
  discipline in `before`/`after`.
- *Trigger* — special-category PII must land in audit payloads or person fields → extend the
  D-CryptoProvider envelope to `pii:special` columns/JSONB. `parked`

**DS-30 · SIEM / streaming export sink.**
- *Default* — `audit2log` + the DB table.
- *Trigger* — external SIEM integration wanted → an additive sink behind `audit2log`. `parked`
