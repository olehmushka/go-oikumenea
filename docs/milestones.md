# Implementation milestones

> Reads: [README](README.md) · [architecture/overview.md](architecture/overview.md) ·
> [architecture/decisions.md](architecture/decisions.md) · the [modules/](modules/) docs.

The design is complete at the architecture level ([README](README.md)); this file sequences it into
**buildable milestones**. It is a roadmap, **not** a binding decision — `decisions.md` still governs
*what* is built; this governs *in what order*. Each milestone is a vertical slice that **boots,
migrates, and demos** on its own, so the service is runnable at every step.

## Ground rules (hold for every milestone)

- **Platform first, then dependency order.** A module lands only after the modules it imports
  (queries are direct interface calls; mutations are events — [overview.md](architecture/overview.md)).
- **Contract-first.** Each module starts from its `*.conjure.yml`; server interfaces/clients are
  generated, never hand-edited (D-Conjure / D-Stack).
- **Audit-on-write from M1.** Every permission-sensitive write records in-transaction (D-Audit); no
  domain milestone ships an unaudited mutation.
- **Migrations are versioned & expand-only.** One repo-root `migrations/`, Atlas versioned, the
  `atlas migrate lint` destructive-change gate, boot-time schema-version check
  ([upgrade-safety.md](architecture/upgrade-safety.md), L-UpgradeSafe).
- **Ontology-shaped.** Every table is an Object / reified Link / Action with an RID PK
  (D-Ontology / D-ResourceIdentifiers); writes are audited Actions.
- **Generated OpenAPI** site updates as each module's Conjure lands.

## At a glance

| # | Milestone | Delivers | Depends on |
|---|---|---|---|
| **M0** | Platform & walking skeleton | witchcraft bootstrap, config, observability, DB pool, schema bootstrap (RID/`uuid_v7`/triggers/`geo_countries`), `pkg/` kernel, events bus, crypto seam | — |
| **M1** | Audit | `audit_log`, in-txn `Record()`, `AuditService` reads | M0 |
| **M2** | Localization | `i18n_locales` + `i18n_translations`, `LocalizationService`, locale→text assembly | M0 |
| **M3** | Tenant | units, graphs, edges, per-graph closure, visibility, lifecycle | M0–M2 |
| **M4** | Rank | the one rank scheme (category→type→rank) | M0–M2 |
| **M5** | Person | directory, CLDR names + variants, citizenships, residences | M0–M2, M4 |
| **M6** | Membership | positions (billets) + memberships (belonging/filling) | M3, M4, M5 |
| **M7** | Authorization + PDP | permission catalog, roles, assignments, the PDP, shadow gate, RLS backstop | M3, M5 |
| **M8** | Identity-federation + bootstrap | accounts, external identities, OIDC/JWKS middleware, first-admin bootstrap, recover-admin CLI | M5, M7 |
| **M9** | Document | papers + envelope-encrypted personal codes | M5 (+ M0 crypto) |
| **M10** | Order | наказ + items + event-driven effects on issue | M3–M6 (+ M0 events) |
| **M11** | Hardening & upgrade-safety | staged RLS enablement, lint gate, CI upgrade tests, decision-explain/time-bound polish, packaging | M7–M10 |

M1/M2 and M3/M4 are independent and may be built in parallel. Everything after M2 assumes audit + i18n exist.

---

## M0 — Platform & walking skeleton

**Goal.** A server that boots, connects to the operator DB, passes health/readiness, and round-trips a
trivial Conjure endpoint — the chassis every module bolts onto.

- **Delivers:** `cmd/oikumenea/main.go` composition root on `witchcraft.Server`; ECV install/runtime
  config (`pkg/refreshable`); observability (`svc1log`/`req2log`, `pkg/metrics`, tracing, health
  reporters incl. the **schema-version readiness gate**); the pgx pool + sqlc plumbing + per-txn RLS
  GUC seam; the gödel/`godel-conjure-plugin` build.
- **Schema bootstrap migration:** the `oikumenea` schema + `citext`; `uuid_v7()`, **`new_rid()`**,
  `set_updated_at()`, `reject_mutation()`; `schema_version`; the seeded **`geo_countries`** ISO-3166
  registry (D-Geo).
- **`pkg/` kernel:** `id`, `errors` (werror↔Conjure mapping), `pagination`, **`events`** (in-process
  bus + outbox seam), `locale`, `config`, **`crypto`** (`KeyProvider` + `pkg/crypto`, `local-dev`
  backend; D-CryptoProvider), `personalcode` registry (D-PersonalCodes).
- **Implements:** D-Stack, D-Conjure, D-ResourceIdentifiers, D-ResourceIdentifiers's `new_rid()`,
  L-UpgradeSafe scaffolding. See [platform](modules/platform.md).
- **Exit:** `serve` boots; migrations apply cleanly and re-apply idempotently; `/status/readiness`
  goes green only on a known schema; one demo endpoint returns a `SerializableError` correctly.

## M1 — Audit (cross-cutting)

**Goal.** The append-only Action ledger every later write commits into.

- **Delivers:** `audit_log` (append-only, `reject_mutation()` guard; PK = the Action RID — D-Ontology
  / D-Audit); the in-transaction `Record(ctx, entry)` application service; `AuditService` read
  endpoints (unit-scoped + shadow-gated once those exist). Actor-shape CHECK (`person` | `system`).
- **Implements:** D-Audit. See [audit](modules/audit.md).
- **Exit:** a write + its audit row share one transaction (commit/rollback together); reads paginate.

## M2 — Localization (cross-cutting)

**Goal.** The supported-locale registry + translation store so every label-bearing module can return
`locale → text` maps.

- **Delivers:** `i18n_locales` (seeded `ukr` + `eng`; exactly-one-default trigger) + the polymorphic
  `i18n_translations`; `LocalizationService`; `TranslationsFor(...)` batch assembly; delete/retire
  subscribers that purge orphaned translations.
- **Implements:** D-i18n (all locales in every response, no Accept-Language negotiation), D-Code
  (code vs translatable name). See [localization](modules/localization.md).
- **Exit:** an entity's `name` + its translation rows assemble into a locale-map response.

## M3 — Tenant (structural foundation)

**Goal.** The unit graph the PDP later reads.

- **Delivers:** `tenant_units` (RID PK, `code`, visibility, lifecycle `state`, `level`); `tenant_graphs`
  registry (seeded `command` default/undeletable + `operational`); `tenant_unit_edges`
  (Link `link__parent_of`, per graph); the maintained **per-graph closure** + on-demand
  `verify`/`rebuild` + `closure-drift` diagnostic reporter; `TenantService`.
- **Implements:** D-Graphs, D-DirectoryGraphs, D-ClosureIntegrity, D-ClosureDriftHealth, D-EdgePerms
  (permission *strings* defined here, enforced in M7). See [tenant](modules/tenant.md).
- **Exit:** build a multi-parent DAG; cycle attempts rejected per graph; closure answers
  ancestor/descendant in one lookup; lifecycle transitions recorded.

## M4 — Rank

**Goal.** The single system-wide seniority scheme persons point at.

- **Delivers:** `rank_categories` → `rank_types` → `rank_ranks` (ordered, code+translatable name);
  `RankService` (reads `rank.scheme.read`, writes instance-scope).
- **Implements:** L-OneRankScheme, D-Rank (directory attribute, never authz). See [rank](modules/rank.md).
- **Exit:** a seeded scheme reads as category→type→rank with seniority ordering.

## M5 — Person (the core aggregate)

**Goal.** The personnel directory — account-optional, instance-global.

- **Delivers:** `person_persons` (RID PK; **CLDR** structured names; `birthdate`, ISO-5218 `sex`,
  `country_of_birth`; optional `code`; `rank_id`; lifecycle incl. **purge**/crypto-erase tombstone);
  `person_name_variants` (transliteration); `person_citizenships` + `person_residences` (effective-dated);
  `PersonService`; the `PersonPurged` event.
- **Implements:** D-PersonGlobal, D-PersonNamesCLDR, D-PersonBio, D-Geo, D-PIITiers (per-column PII
  tiers + `werror` redaction). See [person](modules/person.md).
- **Exit:** create a person with no account/unit; multi-citizenship; purge NULLs PII but keeps the id
  tombstone; reads honor the (M7) read-scope rule once authz lands.

## M6 — Membership

**Goal.** People belonging to / filling billets in units.

- **Delivers:** `membership_positions` (unit-owned billets, vacant-first) + `membership_memberships`
  (Link `link__member_of`; effective-dated; optional `position_id`; `order_item_id` provenance seam);
  derived vacancy; `MembershipService`; subscribers for order auto-apply (wired in M10).
- **Implements:** D-Position, one-holder-per-billet. See [membership](modules/membership.md).
- **Exit:** fill/vacate a billet; plain belonging; roster reads (shadow gate active after M7).

## M7 — Authorization + PDP (the centerpiece)

**Goal.** The product differentiator: decisions over the unit graph with per-assignment scope.

- **Delivers:** the **code-defined permission catalog** + seeded **base roles** (D-BaseRoles);
  `authz_roles` (+ `authz_role_permissions`), `authz_role_assignments` (Link `link__has_role`; scope,
  graph, `expires_at`), `authz_instance_admins`; the **PDP algorithm** (union across graphs via
  closure), the **shadow-visibility gate**, and the **RLS backstop** policies + GUC wiring;
  `AuthorizationService` (`/authorize`, `/authorize/batch` with decision-explain).
- **Implements:** D-Inherit, D-InstanceAdmin, D-Graphs (PDP union), D-TimeBoundGrants, D-EdgePerms
  (now enforced), D-NoRLS + D-RLSDefenseInDepth (staged in M11). See [authorization](modules/authorization.md).
- **Exit:** `unit` vs `subtree` scope behave per spec; cross-graph union; shadow gate filters reads;
  no self-escalation; person/membership read-scope rules now enforced.

## M8 — Identity-federation + first-admin bootstrap

**Goal.** Turn an inbound IdP token into a PDP context, and seed the first admin.

- **Delivers:** `account_accounts` + `account_external_identities` (append-only `FEDERATES`);
  the **OIDC discovery + JWKS** validation middleware → PDP context; `IdentityFederationService`
  (`/whoami`, account/identity management); **first-admin bootstrap** (idempotent, from install
  config) + the **`recover-admin` CLI** break-glass path.
- **Implements:** L-AuthzOnly, D-Bootstrap, D-JIT (link-on-match only). See
  [identity-federation](modules/identity-federation.md).
- **Exit:** a valid token resolves to `(person, account)`; an unknown identity is rejected; a fresh
  install bootstraps exactly one instance admin from config.

## M9 — Document

**Goal.** Person-held papers and encrypted national-identifier codes.

- **Delivers:** `document_document_types` + `document_documents` (metadata only); the
  `document_personal_code_schemes` catalog + `document_personal_codes` (**envelope-encrypted** value +
  blind index); `DocumentService`; the `PersonPurged` subscriber that erases papers and **crypto-erases**
  codes.
- **Implements:** D-Documents, D-PersonalCodes, D-CryptoProvider (`pii:sensitive` envelope), the
  read-through-holder scope. See [document](modules/document.md).
- **Exit:** attach a paper; store a personal code as ciphertext + blind-index uniqueness; purge
  crypto-erases.

## M10 — Order

**Goal.** Administrative orders (наказ) as the legal basis for status changes, applied via events.

- **Delivers:** `order_order_types` (category + effect) + `order_orders` + `order_order_items`;
  `OrderService` with **`/issue`** emitting per-item effect events that membership/person subscribers
  apply **in the issue transaction** (all-or-nothing), citing `order_item_id` provenance; revoke as a
  legal-status flip.
- **Implements:** D-Orders, D-OrderApply. See [order](modules/order.md).
- **Exit:** issuing an appointment order fills the billet in the same txn; a failing effect rolls back
  the whole issue; `record-only` items stand alone.

## M11 — Hardening & upgrade-safety

**Goal.** Tighten the cross-cutting guarantees and package for release.

- **Delivers:** **staged RLS enablement** (permissive→tightened per [upgrade-safety.md](architecture/upgrade-safety.md));
  the `atlas migrate lint` destructive-change CI gate + **CI upgrade tests** (old→new data-safe);
  finalize decision-explain / time-bound-grant ergonomics; Docker + docker-compose packaging; the
  generated **OpenAPI** reference site; load/perf pass on the PDP + closure.
- **Implements:** L-UpgradeSafe end-to-end, D-RLSDefenseInDepth (full enablement).
- **Exit:** an upgrade from the prior release applies non-destructively in CI; RLS backstop active
  without `BYPASSRLS`; image builds and runs from compose.

---

## Cross-cutting threads (woven through, not separate milestones)

- **Audit** (M1) and **i18n** (M2) are consumed by every later module — land them before the domain.
- **Observability** (M0) accrues per-endpoint RED metrics + the `pdp.decisions{result}` counter as
  modules arrive.
- **RLS backstop** is *defined* with M7 and *fully enabled* in M11 (staged, per upgrade-safety).
- **PII discipline** (D-PIITiers + `werror` redaction + purge) is applied as each PII-bearing table
  lands (M5, M9, audit payload ceiling in M1).

## Deferred to post-v1 (parked seams)

The [open-questions.md](open-questions.md) DS backlog stays parked unless its trigger fires. The
common blocker is the **background worker runtime (DS-25)** — anything needing a scheduler
(audit-retention partitioning DS-28, future-dated order effects, expiry sweeps, duty-roster
DS-37) waits behind it. The `pii:special` / audit-payload envelope extension stays parked as DS-29.
