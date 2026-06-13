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

**Dependency note.** Many entries were blocked on the same missing capability — a **background
job/worker runtime** (**DS-25**), now **promoted to milestone M16** (D-Worker), because the M17 data-
ingestion framework needs scheduled syncs. Building it unblocks the scheduler-dependent seams (partition
maintenance DS-28, expiry sweeps, future-dated order effects).

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

**DS-38 · Partial / approximate birthdate & death date (+ gender identity).**
Narrowed by [D-PersonBio](architecture/decisions.md): `birthdate` and `date_of_death` (M12) ship as full
`DATE`s and `sex` is ISO/IEC 5218.
- *Default* — full-precision `birthdate DATE` / `date_of_death DATE`; gender *identity* is **not stored**.
- *Trigger* — (a) records that know only year (or year-month) for a birth **or death** → an additive
  partial-date representation; (b) gender identity, which is `pii:special` (GDPR Art. 9): its **crypto
  blocker is now lifted** by [D-SpecialPII](architecture/roadmap-decisions.md) (the envelope extends to
  `pii:special` person fields, M24), so *storing* it is an additive encrypted column — a separate parked
  product choice, no longer gated on the envelope. `parked`

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
- *Note (religion).* Ecclesiastical clergy are now modeled **outside** the rank module
  ([D-ClergyCredential](architecture/roadmap-decisions.md), M23) as a **per-tradition** ordered
  `religion_clergy_grades` catalog; there is intentionally **no cross-tradition comparator** (grades
  order only within a tradition), so this seam stays parked and clergy never sets `grade_code`.

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

> **DS-25** (background job / worker runtime) was **promoted** to milestone **M16** — now binding as
> [D-Worker](architecture/roadmap-decisions.md), owned by [platform](modules/platform.md). The
> scheduler-dependent residuals it enables (future-dated order effects, expiry sweeps) ride the M16
> runtime when their own triggers fire. Per the ID-stability rule the number is **retired, not reused**.

**DS-44 · Additional ingestion connectors (SQL/JDBC, object-store).**
Introduced by [D-DataIngestion](architecture/roadmap-decisions.md): the M17 framework ships an **HTTP(S)**
(and degenerate `file`) connector; the `Connector` interface is pluggable.
- *Default* — HTTP + file connectors only.
- *Trigger* — a real SQL/JDBC source (a national registry DB) or an S3/MinIO object-store source is
  needed → implement the connector behind the existing `Connector` seam (driver deps included). `parked`

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

**DS-29 · PII envelope encryption — extend to audit payloads (person-field half resolved).**
The **envelope-encryption mechanism ships** ([D-CryptoProvider](architecture/decisions.md)): the
pluggable `KeyProvider` seam + `pkg/crypto` (ciphertext in DB, KEK in an external KMS, blind index,
crypto-erase) protect **`pii:sensitive`** national-identifier values, and — via
[D-SpecialPII](architecture/roadmap-decisions.md) (M24) — now also **`pii:special` person/affiliation fields**
(religious affiliation lands encrypted). The **person-field half is therefore resolved**; this seam
**narrows to audit payloads** — `audit.before`/`after` JSONB at the `pii:special` ceiling.
- *Default* — `pii:sensitive` and `pii:special` **person/affiliation** fields are envelope-encrypted;
  `pii:special` data must **not** enter audit `before`/`after` payloads (the "no special-category PII in
  audit without the envelope" rule), under the "no secrets, minimal PII" discipline.
- *Trigger* — special-category PII must land in **audit payloads** → extend the D-CryptoProvider envelope
  to the `audit.before`/`after` JSONB. (The gender-identity blocker in **DS-38(b)** is lifted by
  D-SpecialPII; *storing* gender identity stays a separate parked choice.) `parked`

**DS-30 · SIEM / streaming export sink.**
- *Default* — `audit2log` + the DB table.
- *Trigger* — external SIEM integration wanted → an additive sink behind `audit2log`. `parked`

---

## company — planned ([milestones.md](milestones.md) M21 · [D-Companies](architecture/roadmap-decisions.md))

**DS-45 · Company registry-intelligence feeds (financials, court cases, tax debt, sanctions/PEP).**
[D-Companies](architecture/roadmap-decisions.md) scopes M21 to **structural** registry data; volatile
intelligence is excluded.
- *Default* — no financials/litigation/tax/sanctions data; structural identity + ownership graph only.
- *Trigger* — an operator has a real feed → model the relevant entity and ingest it via a
  D-DataIngestion connector (these are feed-dependent and change constantly). `parked`

**DS-46 · Company web / contact channels.**
- *Default* — a company's "domain" is its **industry classification**; no web/email/phone stored.
- *Trigger* — web presence / contact reachability wanted → contact-channel child tables mirroring the
  person contact model (D-PersonContactChannels). `parked`

**DS-47 · Ownership-graph closure & computed UBO.**
[D-Companies](architecture/roadmap-decisions.md) stores direct ownership edges + a **declared** beneficiary
record; it does not traverse the chain.
- *Default* — direct `company_shareholdings` edges + declared `company_beneficiaries`; no computed
  ultimate-owner traversal.
- *Trigger* — "who ultimately controls X" over deep chains wanted → a maintained ownership closure
  (mirroring the tenant/language closure) + a computed-UBO derivation. `parked`

---

## religion — [`modules/religion.md`](modules/religion.md)

> **DS-48** (Religion domain) was **promoted** to the **M22–M25** cluster — now binding as
> [D-Religion](architecture/roadmap-decisions.md), [D-ClergyCredential](architecture/roadmap-decisions.md), and
> [D-ReligiousAffiliation](architecture/roadmap-decisions.md), owned by [religion](modules/religion.md). The
> drafts' Christianity-specific concepts return **generalized to all faiths**, catalog-driven; the
> `pii:special` belief concern is resolved by [D-SpecialPII](architecture/roadmap-decisions.md) (envelope
> extended to the special tier). Per the ID-stability rule the number is **retired, not reused**.

**DS-49 · Rite-of-passage / life-cycle records.**
Introduced by [D-ReligiousAffiliation](architecture/roadmap-decisions.md): affiliation is modeled, but
individual life-cycle observances are not.
- *Default* — no baptism / bar-bat-mitzvah / marriage-rite / funeral records; only the affiliation tie.
- *Trigger* — a deployment must record rites of passage → a **generic, catalog-typed** observance entity
  (per-tradition rite catalog), `pii:special` under the D-SpecialPII envelope. `parked`

**DS-50 · Location-scoped role assignments.**
A consuming discovery app (FaithMap-style) wants per-site "campus admin" rights; today an assignment's
scope is `unit|subtree` over a unit, not a single site/location.
- *Default* — authority is unit-scoped (`unit|subtree`); a site inherits its org unit's scope.
- *Trigger* — a deployment needs a role bounded to one site/location rather than the whole unit → add a
  `scope_location_id` (or a site-scoped assignment) to the authorization model. `parked`

---

## vehicle — planned ([milestones.md](milestones.md) M26 · [D-Vehicles](architecture/roadmap-decisions.md))

**DS-51 · Full ISO-3166-2 subdivision set + residence/Location retrofit.**
Introduced by [D-GeoSubdivisions](architecture/roadmap-decisions.md): `geo_subdivisions` ships with the
**target-country subset** migration-seeded (UA first); `person_residences.region` and
`location_locations.admin_area_1`/`admin_area_2` stay **free-text**.
- *Default* — UA (and other target) subdivisions only; residence/Location regions are free text.
- *Trigger* — global coverage or structured Location addresses wanted → ingest the **full ISO-3166-2
  set** via a D-DataIngestion connector (M17) and retrofit `person_residences.region` /
  `location_locations.admin_area_*` to a `geo_subdivisions` FK (additive expand/contract). `parked`

**DS-52 · Vehicle lifecycle / intelligence feeds.**
[D-Vehicles](architecture/roadmap-decisions.md) scopes M26 to **structural** registry data (identity, taxonomy,
ownership/plate); volatile lifecycle data is excluded (mirrors company DS-45).
- *Default* — no insurance/MTPL, technical-inspection, accident, theft/wanted, odometer, or telematics
  data; structural identity + ownership/plate only.
- *Trigger* — an operator has a real feed → model the relevant entity and ingest it via a
  D-DataIngestion connector (these are feed-dependent and change constantly). `parked`

**DS-53 · Column-ize stabilized vehicle specs.**
The DS-6 pattern (promote `attributes` keys to columns), scoped to `vehicle_vehicles`.
- *Default* — long-tail specs (engine/fuel/transmission/dimensions…) live in the `attributes` JSONB.
- *Trigger* — a spec stabilizes (shared / queried) → column-ize it as a typed column with its own PII
  tier (additive expand). `parked`
