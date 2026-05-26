# Glossary

The domain vocabulary used across every doc. Module docs assume these terms. Grouped
thematically; alphabetical index at the end.

---

## Organization

**Unit.** A node in the organization graph — the thing this service calls a *tenant*. A
brigade / regiment / battalion / platoon, or a university / campus / faculty / department.
All organizational entities are units; there is no separate "group" concept. Owned by the
[tenant](modules/tenant.md) module.

**Unit kind.** An optional, instance-configured label on a unit describing its level
(e.g. `brigade`, `battalion`, or `university`, `department`). It is **descriptive data**,
not a behavioral discriminator — the code does not branch on it. Distinct from drafts'
`tenant_type`, which is dropped.

**Unit graph / DAG.** Units relate by parent→child edges. A unit may have **more than one
parent** (a directed acyclic graph), and there may be **more than one root** (units with no
parent). Cycles are forbidden.

**Closure.** A maintained transitive-closure table (`ancestor → descendant`) that lets the
PDP answer "is U a descendant of T?" in one indexed lookup instead of walking edges.

**Visibility.** A unit is `public` (discoverable) or `shadow` (private, hidden from
discovery). A person may belong to several units, some public, some shadow. The
[authorization](modules/authorization.md) module gates reads on this.

**Lifecycle state.** A unit's status (`active`, `suspended`, `archived`, …). Transitions
are recorded as append-only events.

---

## People

**Person.** The core aggregate — an individual in the directory. **Instance-global**: one
record per individual for the whole deployment (not per-unit). Exists independently of any
login account and of any unit membership. Owned by the [person](modules/person.md) module.

**Account.** An *optional* login attachment to a person. People without accounts (rosters,
personnel who never sign in) are first-class. Owned by
[identity-federation](modules/identity-federation.md).

**External identity.** A verified `(issuer, subject)` pair from an external IdP, linked to
an account. The basis on which an inbound token is mapped to a person.

**Membership.** A `person ↔ unit` assignment — the org-belonging join. One person may hold
many memberships across many units (public and shadow). Optionally fills a position; carries
effective dates. Owned by the [membership](modules/membership.md) module.

**Position.** A **unit-owned billet** — a post belonging to one unit (e.g. *Commander*,
*Deputy*, *Dean*, *Chaplain*) that **exists whether or not anyone fills it**. Has a stable
`code`, a translatable title, and an optional required rank. A person fills it via a membership
that references it. Distinct from rank: rank is the person's standing across the whole org;
position is what they *do* in a specific unit. Owned by [membership](modules/membership.md).

**Vacancy.** A derived state: an active position with **no** active membership filling it. Not
a stored column — the closure of "active position, unfilled".

---

## Rank

**Rank scheme.** The single, **system-wide** seniority ladder for the deployment, edited by
the instance admin (never adopted per-unit). Three ordered levels:
**rank category → rank type → rank**. Owned by the [rank](modules/rank.md) module.

**Rank category.** Top of the scheme (e.g. `army`, `navy`, `marines`). Ordered.

**Rank type.** A grouping within a category (e.g. `officers`, `warrant`, `enlisted`).
Ordered, expresses the broad seniority band.

**Rank.** A specific grade (e.g. `private`, `sergeant`, `colonel`). Ordered, expresses exact
seniority. A person holds **one** rank.

**Rank ≠ permission.** Rank is a **directory attribute** describing seniority. It grants no
authorization whatsoever. Authority comes only from role assignments.

---

## Authorization

**Atomic permission.** A code-defined, runtime-immutable permission string
(e.g. `unit.update`, `person.read`, `rank.scheme.manage`). The closed vocabulary lives in
code; adding one is a code change. Owned by [authorization](modules/authorization.md).

**Role.** A named, composed set of atomic permissions. **Base roles** are platform-defined;
**custom roles** are instance-defined. A role does not, by itself, target anything — it is
bound to a unit and scope by an assignment.

**Role assignment.** The tuple `(subject_person, role, target_unit, scope)` with provenance
and optional expiry. The unit of authorization grant.

**Scope.** A property of a role assignment, one of:
- `unit` — the role's permissions apply **only at `target_unit`**. Descendants get
  **nothing — not even read**.
- `subtree` — the role's permissions apply at `target_unit` **and all its descendants**
  (cascading across the DAG).

The `target_unit` is **independent of where the subject sits**: a low-placed person can hold
a `subtree` role on a high-level unit.

**PDP (Policy Decision Point).** The component that answers "may person P perform action A
on unit U?" It unions instance-admin permissions, `unit`-scoped grants at U, and
`subtree`-scoped grants on any ancestor of U (via the closure), then applies the
shadow-visibility gate on reads. No per-permission filtering within an assignment; no
cross-request caching (a revoke takes effect immediately).

**Instance admin.** A holder of an **instance-level** authority scope, distinct from unit
role assignments. Manages the rank scheme, role definitions, supported locales & translations,
and global config. The "top-permission role" — bootstrapped at install.

**Effective permissions.** The union of all permissions a person holds on a given unit,
computed by the PDP at decision time.

---

## Localization (i18n)

**Code.** A **stable, locale-agnostic** machine identifier on a structural/catalog entity
(unit, role, position, rank node, locale; optional on person). What external systems reference
in their own code. Operator-assigned, unique, immutable by convention. The permission string is
the degenerate case (it *is* the code). Distinct from the translatable `name`.

**Locale.** A supported language for the deployment, identified by an ISO 639-3 code (e.g.
`ukr`, `eng`). The set is **instance-admin-managed** (seeded with `ukr` + `eng`, more can be
added). Owned by the [localization](modules/localization.md) module.

**Supported language.** Synonym for an enabled locale.

**Translation.** A localized value of a translatable field (`name`/`title`/`description`) of
some entity, stored in the [localization](modules/localization.md) translation store and
managed by the instance admin. Translatable fields are returned in every response as a
`locale → text` map (no Accept-Language negotiation).

**Transliteration.** A per-person alternate **name variant** for a locale/script (e.g. "Олег" /
"Oleh"). Person-managed data on the person record — *not* the instance-admin translation store.

---

## Cross-cutting

**Audit log.** An append-only record of permission-sensitive actions, correlated by
`request_id`. Never updated or deleted (guarded by a `reject_mutation()` trigger). Owned by
[audit](modules/audit.md).

**Append-only / immutable event log.** A table whose rows are never updated or deleted;
current state is derived or kept in a separate mutable overlay. Used for audit and unit
lifecycle events.

**Reversibility.** Destructive actions are soft (a `deleted_at` / state flip with a grace
window), never immediate hard removal; a reversal is itself an audited action.

**Dormant seam.** A column or table shipped but intentionally unused (always NULL / empty),
reserved so a future capability is *additive*, not a rewrite (e.g. the password/2FA columns
reserved for a future full-IdP pivot).

**Expand / contract.** The migration discipline: a release only **adds**; removals happen in
a later, announced release after the old shape is unused. Guarantees data-safe upgrades.

**PDP context.** The resolved `(person, [account])` plus request metadata that the transport
layer derives from a validated inbound token and passes to the PDP and to audit.

---

## Alphabetical index

Account · Append-only event log · Atomic permission · Audit log · Closure · Code · Dormant seam ·
Effective permissions · Expand/contract · External identity · Instance admin · Locale ·
Membership · PDP · PDP context · Person · Position · Public/shadow · Rank · Rank category ·
Rank scheme · Rank type · Reversibility · Role · Role assignment · Scope · Supported language ·
Translation · Transliteration · Unit · Unit graph (DAG) · Unit kind · Vacancy · Visibility
