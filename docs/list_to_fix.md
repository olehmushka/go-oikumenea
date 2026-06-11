# list_to_fix.md — adversarial review of go-oikumenea

**Date:** 2026-06-11
**Reviewer:** Fable 5, wearing both the senior-business-analyst hat (does the domain model hold for army **and** church **and** university?) and the senior-software-architect hat (are the technical decisions sound and implemented as documented?).

## Remediation progress (update as items land)

Pick the **next `☐` item** in this list and fix it; mark it `✅` here and at its section heading when done.

- ✅ **F-001** — person directory read-scope projection enforced (D-PersonReadScope). *Done 2026-06-11.*
- ☐ **F-002** — shadow-visibility gate has no caller *(next)*
- ✅ **F-004** — document/personal-code holder-scope reads enforced (rode F-001). *Done 2026-06-11.*
- ☐ **F-003** — stage board M5/M7 `verified` grounding (now honest after F-001/F-004; confirm/close)
- ☐ **F-005** — `date_of_death` decided + Migrated-✅ but absent
- ☐ **F-006** — "exactly one rank per person" under-models university/church
- ☐ **F-007** — no acting/dual-hatted/secondment temporal model
- ☐ **F-008** — M16–M26 designed-but-unbuilt ahead of unenforced core (cost-benefit)
- ☐ **F-009** — production HS256 issuer path lacks a guardrail
- ☐ **F-010** — migration filename index off-by-one from embedded revision
- ☐ **F-011** — glossary/tenant overstate closure reflexive coverage
- ☐ **F-012** — order auto-apply same-transaction subscriber contract unverified
- ☐ **F-013** — `attributes` JSONB `pii:special` ceiling is convention-only
- ☐ **F-014** — composed-URN RID apparatus heavy vs realized benefit (cost-benefit)

**F-001 + F-004 — what landed (2026-06-11):** app-layer read-scope projection only (the RLS
reach-join was deliberately **deferred** — it would break the JIT auth path / bootstrap / order-effect
writes that read `person_persons` off the bare pool pre-pinning; the binding "noted seam, not shipped"
stands). New `Enforcer.EffectiveReach`; membership cross-module query interface
(`ActiveUnitIDsForPerson`, `PersonIDsWithActiveMembershipInUnits`); person `ReadablePerson` /
`ListVisiblePersons` (+ `ListPersonsByIDs`); person & document transports gate reads and hide
non-readable holders as not-found; composition root binds the seams. Covered by a `ReadablePerson`
unit test + a DB-backed read-scope integration test. No migration, no schema bump, no decision change.

## Scope

A read-only, adversarial review of the whole repository: the `docs/` source-of-truth (README, glossary, overview, conventions, **decisions.md** [binding], **ontology-mapping.md** [binding], patterns, upgrade-safety, all `modules/*.md`, milestones, open-questions, development-process, web-ui), the `api/*.conjure.yml` contracts, the 15 Atlas migrations in `migrations/`, and the Go implementation under `internal/` and `pkg/` (the PDP, the closure maintenance, the RLS backstop, identity-federation, crypto, audit). The single deliverable is this file; no other file was modified.

## Methodology

- Read the docs in the canonical reading order, then read the migrations and the load-bearing Go (`internal/authorization/domain/pdp.go`, `internal/tenant/{application,adapters}`, `internal/identityfederation/middleware`, every module's `transport/service.go` for the actual permission gates, `internal/platform/db/rls.go`, `pkg/authn`).
- Ran the repo's own coherence check (the markdown link-checker): **links OK** — zero broken relative links.
- `go build ./...` from the repo root: **clean**. `go vet` on authorization/tenant/person: **clean**. `migrations/atlas.sum` lists 15 files matching the 15 `*.sql` on disk.
- Cross-checked binding decisions against the migrations and the Go transports — the productive seam, since "doc says X, code does Y" is a top-priority finding class here.

The headline result is **not** "looks good." The product's two differentiating guarantees — *person/document read-scope projection through memberships* (D-PersonReadScope) and the *shadow-visibility gate* — are **documented as binding and shipped, marked verified on the stage board, but are not actually enforced in the code**. Everything else is secondary to that.

---

## Executive summary (ranked by impact)

1. **F-001 (Critical) — ✅ FIXED (2026-06-11) — The global personnel directory leaks to any reader.** `GET /persons` and `GET /persons/{id}` gate on "holds `person.read` *anywhere*"; D-PersonReadScope's membership-intersection projection is **not applied**, and `person_persons` has **no RLS policy** and is read on the **bare pool**. Any unit-reader sees every person in the deployment, including people in units they cannot reach and in shadow units.
2. **F-002 (Critical) — The shadow-visibility gate is never invoked.** `ShadowGate` / `FilterVisibleUnits` exist with **zero non-test callers**. The product's named differentiator ("public/shadow visibility decided by a PDP") is dead code at the application layer; what protection exists comes only incidentally from RLS on unit-keyed tables.
3. **F-003 (High) — The stage board is not grounded in artifacts, violating the repo's own rule.** M5 and M7 are marked **verified**, yet the code itself documents the read-scope rule as a deliberate *interim coarse gate* ("NOT yet applied"). `development-process.md` demands every `✅` be grounded in a real artifact; here the *verified* gate contradicts the implementation.
4. **F-004 (High) — ✅ FIXED (2026-06-11) — Document reads inherit the same hole.** Documents/personal-codes are "scoped through the holder," but the holder scope (F-001) is not enforced and `document_documents` is likewise RLS-exempt, so personal-document metadata (passport numbers, issuers) is directory-wide-readable.
5. **F-005 (Medium) — `date_of_death` is decided and milestone-listed but does not exist.** D-PersonBio's M12 amendment mandates `person_persons.date_of_death DATE`; milestones lists it as an M12 deliverable with the **Migrated gate `✅`** — but it is absent from every migration, the `person.md` data model, the glossary, the Conjure contract, and the Go code.
6. **F-006 (Medium, product) — "Exactly one rank per person" is a poor fit for the university and church verticals.** The model forces a single global seniority ladder onto domains that routinely carry *concurrent* standings (an academic who is also an administrator; clergy whose grade is explicitly modeled elsewhere). The decision is defensible for the army but is asserted as universal without addressing the multi-track case.
7. **F-007 (Medium, product) — No temporal validity / acting-appointment model for assignments-as-roles, only for memberships.** Acting CO, dual-hatting, and secondment are named as target-domain realities, but the *position fill* is one-holder and the *role assignment* carries only `expires_at`, not an "acting/deputizing" relationship — the BA-level gap the brief asked about.
8. **F-008 (Cost-benefit) — 12 designed-but-unbuilt milestones (M16–M26) of binding decisions ahead of a pre-v1 core whose two key guarantees don't work.** The decision log has grown ~3× the size of the working surface; the religion/company/vehicle/location/language verticals are fully *decided* while the authorization core is not actually enforced.

---

## Critical

### F-001 — The personnel directory is readable by any holder of `person.read`, ignoring D-PersonReadScope
- **Status:** ✅ FIXED 2026-06-11 (app-layer projection; RLS reach-join deferred — see Remediation progress).
- **Severity:** Critical
- **Lens:** consistency (doc vs code) + architecture + product
- **Location:**
  - `internal/person/transport/service.go:93-102` (`GetPerson` → `pep.RequireAnywhere(ctx, token, permRead)`), `:130-145` (`ListPersons` → same).
  - `internal/person/transport/service.go:9-15` (package comment admitting the gap).
  - `internal/person/application/service.go:116-118` (`GetPerson` runs `s.newRepo(s.pool)`), `:167-173` (`ListPersons` runs `s.newRepo(s.pool)` — the **bare pool**, not the request-pinned RLS connection).
  - `migrations/20260601000011_rls_backstop.sql:49-51` (`person_persons` is **explicitly exempt** from RLS — the lone match is the exemption comment, not a policy).
  - Binding rule it violates: `docs/architecture/decisions.md` D-PersonReadScope; `docs/modules/person.md:458-468` ("Read-scope rule (canonical)").
- **Evidence.** D-PersonReadScope is unambiguous: a subject may read person P **iff** they are on the instance plane **or** their effective readable unit-set intersects P's active-membership units (shadow-gated); "a membership-less person … is readable **only on the instance plane**." The code instead does `RequireAnywhere(person.read)`, which returns true for *any* subject holding `person.read` at *any* unit (e.g. a `unit-reader` on a single platoon). The package comment says so directly: *"the precise read-scope rule … is **NOT yet applied**; the interim gate is the coarse 'holds the permission anywhere'."* Because `person_persons` is RLS-exempt and the read runs on the bare pool, there is **no** backstop — neither the app projection nor RLS constrains the result.
- **Why it's wrong.** This is the core authorization promise of a multi-unit directory, and it is the exact leak D-RLSDefenseInDepth was written to make "impossible even when the app forgets the filter." A reader scoped to one battalion can enumerate the entire instance-global roster (names, CLDR parts, birthdate, sex, country of birth, `code`), including people whose only memberships are in shadow units. For an army deployment this is an intelligence leak; for any deployment it is a GDPR exposure of `pii:basic` at scale.
- **Recommended fix.** Implement the projection in `person` (application + transport): for `GET /persons/{id}`, compute the reader's `EffectiveReach` (already exposed on the authorization service), fetch P's active-membership units, intersect with the shadow gate, and deny when empty unless the reader is on the instance plane. For `GET /persons`, return the union reachable this way (a join, not a post-filter, for pagination correctness). Route person reads through the pinned RLS connection and add the noted person→unit reach-join RLS policy as the backstop D-RLSDefenseInDepth already contemplates. Blast radius: `person` and `document` modules + the M7 "verified" claim; no schema change required for the app projection, one additive migration for the RLS hardening.
- **Effort:** L
- **Confidence:** High

### F-002 — The shadow-visibility gate has no caller; the product differentiator is unenforced at the app layer
- **Severity:** Critical
- **Lens:** consistency + architecture + product
- **Location:**
  - `internal/authorization/domain/pdp.go:245-254` (`ShadowGate`) and `internal/authorization/application/service.go:219-235` (`FilterVisibleUnits`) — defined, exported, and (grep-confirmed) **called from no non-test file**.
  - `docs/architecture/patterns.md:149-160` ("Shadow-visibility gate … applied *after* the permission decision, as a second filter on result sets"); `docs/modules/authorization.md:200-206`; `docs/modules/tenant.md`, `membership.md`, `document.md`, `order.md` all list the gate as an owned/called pattern.
  - Read paths that *should* call it but don't: `internal/tenant/transport/service.go` (`ListUnits`, `GetUnit`, `Ancestors`, `Descendants`), `internal/membership/transport/service.go:177-186` (`ListMembers`).
- **Evidence.** The README's one-sentence pitch is "hierarchy + inheritance + **visibility**, decided by a PDP." The glossary, patterns, and five module docs all describe the gate as an active second pass owned by `authorization` and called by the read surfaces. In the code, the gate functions exist but nothing calls them. Unit-scoped reads are instead constrained only by `pep.Require(*.read, unitID)` plus, where the table has a `unit_id`, the RLS policy. For `tenant_units` the RLS policy keys on `readable_units` (which already excludes unreached shadow units), so the leak is partially masked there — but the *documented* mechanism (an explicit shadow second-pass, the thing the docs say is owned and called) is simply not wired, and for the RLS-exempt tables (`person_persons`, `document_documents`, `order_order_items`) there is no shadow filtering at all (compounding F-001/F-004).
- **Why it's wrong.** A core, advertised feature is present in the design and absent in the runtime. Either the gate must be wired into every read surface the docs claim, or the docs must stop claiming an app-layer gate and explicitly state that shadow visibility is delivered *solely* by the RLS reach computation (which would itself be a binding-decision change, since D-NoRLS/D-RLSDefenseInDepth insist the app PDP + gate are "authoritative" and RLS is "only a backstop").
- **Recommended fix.** Decide one of two coherent stories and make code and docs agree: (a) wire `FilterVisibleUnits` into tenant/membership/order/document/person read paths as the docs specify; or (b) ratify that shadow visibility *is* the RLS reach set, demote the gate prose to "computed once as the reach set," and delete the dead `ShadowGate`/`FilterVisibleUnits` if truly unused. Given person/document are RLS-exempt, option (a) is required for those tables regardless. Blast radius: every read transport.
- **Effort:** L
- **Confidence:** High

---

## High

### F-003 — Stage board marks M5/M7 "verified" while the binding read-scope rule is admittedly unimplemented
- **Severity:** High
- **Lens:** consistency
- **Location:** `docs/milestones.md:83` (M5 `verified`), `:85` (M7 `verified`); `docs/milestones.md:214-215` (M7 exit: "person/membership read-scope rules now enforced"); contradicted by `internal/person/transport/service.go:9-15`.
- **Evidence.** `development-process.md:34,63-67` makes it a hard rule: *"Never mark a gate from memory or intent — verify the artifact … a `delivered` prose line may hide a missing surface. Check."* The M7 exit criterion explicitly claims read-scope is enforced; the implementation comment says it is not. The board's own governance therefore fails on its flagship milestone.
- **Why it's wrong.** The stage board is declared "authoritative for stage." If the most load-bearing milestone is green while its exit criterion is unmet, the board cannot be trusted for any milestone, which undermines the entire process the repo is built around.
- **Recommended fix.** Until F-001/F-002 are fixed, downgrade M5 and M7 *Verified* to `🚧` with a note pointing at the read-scope/shadow-gate gap; or fix the code and keep them green. Do not leave the contradiction.
- **Effort:** S
- **Confidence:** High

### F-004 — Document & personal-code reads inherit the unenforced holder scope
- **Status:** ✅ FIXED 2026-06-11 (holder read-scope on document/personal-code reads; rode F-001).
- **Severity:** High
- **Lens:** consistency + architecture
- **Location:** `internal/document/transport/service.go:77,92` (`ListDocuments`/`GetDocument` → `RequireAnywhere(document.read)`); `migrations/20260601000011_rls_backstop.sql:49-51` (`document_documents` RLS-exempt); `docs/modules/document.md:251-254` ("scoped via the holder … app-layer PDP is the authoritative read scope").
- **Evidence.** `document.md` defers read scope to D-PersonReadScope "through the holder." Since the holder scope is not enforced (F-001) and `document_documents` carries no RLS policy, any `document.read` holder can read any person's document metadata (`number`/`issuer` are `pii:basic`). Personal-code *values* stay encrypted, but the existence, scheme, and blind-index of a person's national IDs become enumerable to an over-broad reader.
- **Why it's wrong.** The module doc's "the app-layer PDP is the authoritative read scope" is precisely the scope that is missing. The "noted hardening seam" (a person→unit reach-join policy) was the only backstop and it is explicitly not shipped.
- **Recommended fix.** Fold into the F-001 fix: the holder-scope projection must be implemented in `person` and consumed by `document`'s read paths; add the reach-join RLS policy for `document_documents`.
- **Effort:** M (rides F-001)
- **Confidence:** High

### F-006 — "Exactly one rank per person" under-models the university and church verticals
- **Severity:** High
- **Lens:** product (BA)
- **Location:** `docs/architecture/decisions.md` D-Rank ("A `person` holds **one rank**"); `migrations/20260601000005_person.sql` (`rank_id` single nullable FK); `docs/modules/rank.md:13-17`.
- **Evidence.** D-RankSystems was added to carry *multiple national systems in one registry*, but a person still points at exactly one `rank_ranks` row, and "a person's system is derived through rank→type→category→system." In a **university** a person is commonly an *academic* rank (Associate Professor) **and** an *administrative* one (Dean) simultaneously — the doc even seeds `rank_categories` with `academic`/`administrative` as parallel branches *within one system*, which makes "one rank" mean a person cannot hold both. In a **church**, clergy standing was deliberately moved out of `rank` into `religion_clergy_grades` (D-ClergyCredential) precisely because it is per-tradition and concurrent — implicitly conceding that "one global rank" does not fit that vertical, yet the lone `person.rank_id` remains the universal model.
- **Why it's wrong.** The brief asks whether the model fits army **and** church **and** university *simultaneously*. The single-rank decision is justified entirely from the army frame ("a person's standing across the whole organization") and never addresses the concurrent-track case the other two domains exhibit. The workaround (model the second track as a `membership` position) conflates *seniority* with *billet*, the very distinction D-Rank/D-Position fights to keep clean.
- **Recommended fix.** Either (a) explicitly scope D-Rank to "one rank **per rank system**" and let `person` hold one rank per system (a small join table), which the multi-system machinery already half-implies; or (b) document in D-Rank that concurrent standings are out of scope and that universities model the secondary track as a position, accepting the seniority/billet blur. State the choice; do not leave "one rank" asserted as universal.
- **Effort:** M (doc) / L (if schema changes)
- **Confidence:** Medium

---

## Medium

### F-005 — `date_of_death` is decided + milestone-listed (Migrated `✅`) but absent everywhere
- **Severity:** Medium
- **Lens:** consistency (decided-but-not-built; gate marked done)
- **Location:** `docs/architecture/decisions.md` D-PersonBio "*Amended (M12) — `date_of_death`*"; `docs/milestones.md:42` (M12 delivers "date of death") and `:90` (M12 *Migrated* `✅`). Absent from `migrations/20260601000005_person.sql` and `…000012_person_contacts.sql` (grep count 0), from `docs/modules/person.md` (no mention), from `docs/glossary.md` (no "death" entry), from `api/person.conjure.yml`, and from `internal/person`.
- **Evidence.** The decision states `person_persons` "**also carries** a nullable `date_of_death DATE` … Lands additively in M12 (item F)." The stage board marks M12's Migrated gate done. No column exists.
- **Why it's wrong.** A binding decision plus a green Migrated gate with no artifact is exactly the "✅ from memory" failure the process forbids. It also means `person.md` (the *designed* gate) is internally incomplete: D-PersonBio is amended but the module's own data model never lists the field.
- **Recommended fix.** Either add the column (one expand-only migration + `person.md` data-model row + glossary term + `pii:basic` comment + purge-list entry), or, if M12 is genuinely mid-flight, set M12 *Migrated* back to `🚧` and note `date_of_death` as pending. M12 is already labelled "scoped/in progress," so the honest move is the latter until the column ships.
- **Effort:** S
- **Confidence:** High

### F-007 — No acting/dual-hatted/secondment model; temporal validity lives only on memberships
- **Severity:** Medium
- **Lens:** product (BA)
- **Location:** `docs/modules/membership.md` (one-holder-per-position unique index; `effective_from/to`); `docs/architecture/decisions.md` D-TimeBoundGrants (`expires_at` only); `migrations/20260601000006_membership.sql:86-93`.
- **Evidence.** The brief explicitly calls out "acting/dual-hatted roles, secondments, leave overlapping appointments." The model supports: a time-bound *role assignment* (`expires_at`), and a one-holder *position*. It does **not** model "X is *acting* in Y's billet while Y is on leave" (the billet is single-holder, so the substantive holder must be ended or the acting one rejected by `Membership:PositionAlreadyFilled`), nor "seconded to unit B while still belonging to unit A in a different capacity" beyond plain multi-membership. Leave is a `record-only` order item (DS-35) with no overlap/conflict checking against appointments.
- **Why it's wrong.** Acting command and dual-hatting are bread-and-butter in all three target domains (the docs themselves cite "acting CO during a deployment"). The one-holder invariant makes the most common temporary-authority case awkward: you cannot represent "substantive holder on leave, acting holder in place" without vacating the substantive holder.
- **Recommended fix.** Document the intended pattern explicitly (likely: acting authority is a time-bound *role assignment* on the unit, not a position fill — which is coherent with "authority comes only from assignments"), and add a worked example to `membership.md`/`authorization.md`. If real deployments need a billet to show both substantive and acting incumbents, that is the multi-incumbent seam (DS-9) plus an `acting` flag — note it. This is primarily a documentation/decision gap, not necessarily code.
- **Effort:** M
- **Confidence:** Medium

### F-009 — Production HS256 issuer path accepts an operator-supplied symmetric key with no guardrail
- **Severity:** Medium
- **Lens:** architecture (security)
- **Location:** `internal/identityfederation/middleware/validator.go:84-110` (`Validate` routes on the token's **unverified** `iss` to a per-issuer config; `IssuerHS256` verifies with an install-config `HMACKey`).
- **Evidence.** `unverifiedIssuer` reads `iss` without signature verification to pick the issuer config (sound, and commented as such). But the issuer *type* is operator config: an issuer configured as `hs256` is verified with a shared secret in `install.yml`. There is no assertion that HS256 is local-dev-only; the doc calls it "local-dev," but nothing in code prevents a production deployment from configuring an `hs256` issuer, at which point anyone with the install secret can mint valid tokens for any subject.
- **Why it's wrong.** L-AuthzOnly's whole premise is "we validate, we never hold credentials." A symmetric verification key *is* a credential-equivalent the service now holds, and the "local-dev only" intent is enforced by convention, not code or config validation.
- **Recommended fix.** Gate `IssuerHS256` behind an explicit `app.environment in {dev,local}` check (the env slot already exists for RIDs) or a separate `allow_symmetric_issuers` flag that defaults false, and refuse `hs256` issuers at boot otherwise. Document the constraint in `identity-federation.md`.
- **Effort:** S
- **Confidence:** Medium

### F-010 — Migration filename index is off-by-one from the schema revision it writes (a latent trap)
- **Severity:** Medium
- **Lens:** consistency
- **Location:** every `migrations/2026060100000N_*.sql` writes `schema_version.revision = '000(N+1)_*'` — e.g. `…000011_rls_backstop.sql:123` sets `0012_rls`; `…000012_person_contacts.sql:210` sets `0013_person_contacts`; `…000014_person_relationships.sql:313` sets `0015_person_relationships`.
- **Evidence.** The first file `…000000_schema_bootstrap.sql` sets revision `0001_schema_bootstrap`, so the filename ordinal and the embedded revision are permanently offset by one across all 15 files.
- **Why it's wrong.** The boot-time schema-version check (`upgrade-safety.md` guarantee #4) compares the binary's expected revision string against `schema_version.revision`. Two parallel numbering schemes for the same artifact (filename `000011` ↔ revision `0012`) is a guaranteed source of a future off-by-one in the expected-revision constant, an `UPGRADING.md` entry, or an operator's mental model. It is not a runtime bug today, but it is a trap the convention sets for itself.
- **Recommended fix.** Pick one numbering. Cleanest: rename the embedded revisions to match the filename ordinal (or vice-versa) so "the 11th migration" and "revision 11" are the same number. Since the service is pre-release with no deployed DBs, this is a free rename now and expensive later.
- **Effort:** S
- **Confidence:** High

---

## Low

### F-011 — Glossary/tenant overstate the closure's reflexive coverage
- **Severity:** Low
- **Lens:** consistency
- **Location:** `docs/glossary.md` Closure ("`graph → ancestor → descendant`"); `docs/modules/tenant.md:94` ("`depth` … 0 = self-row for **each unit**, per graph"); vs `internal/tenant/adapters/tenantsql/tenant.sql.go:487-505` (`nodes` = units appearing in that graph's **edges** only).
- **Evidence.** The rebuild query seeds reflexive `(g,u,u,0)` rows only for units that appear in some edge of graph `g`. A unit with no edges in `g` has **no** closure row at all. `tenant.md:97` actually says this correctly ("every unit that participates in the graph's edges"), but the same doc's data-model bullet and the glossary imply "each unit."
- **Why it's wrong.** Minor internal inconsistency. The PDP is unaffected because `pdp.go:115` handles the self case explicitly (`reaches := g.TargetUnitID == in.UnitID`) before any closure lookup — but a future reader trusting "self-row for each unit" could write a query that silently drops edge-less units.
- **Recommended fix.** Reword the glossary and the `tenant.md:94` bullet to "for each unit **participating in the graph's edges**," matching `:97` and the SQL.
- **Effort:** S
- **Confidence:** High

### F-012 — `pkg/events` "synchronous, same-transaction" subscriber contract is asserted but not visibly enforced for cross-module order auto-apply
- **Severity:** Low
- **Lens:** architecture (verify)
- **Location:** `docs/architecture/decisions.md` D-OrderApply; `docs/modules/platform.md:156-158` (events bus "subscribers run synchronously within the originating transaction"); `pkg/events/events.go`.
- **Evidence.** D-OrderApply's all-or-nothing guarantee depends on the membership/person subscribers running **in the issue transaction** so a `Membership:PositionAlreadyFilled` rolls back the whole issue. This is the linchpin of the order-effects design. The review confirmed the bus exists and is in-process, but did not find an explicit test or invariant proving that an order subscriber's failure rolls back the issuing order's own writes (as opposed to committing the order and dropping the effect).
- **Why it's wrong.** If the bus dispatches subscribers *after* the order transaction commits (a common in-process pattern), the documented atomicity is silently violated and an order could be `issued` with its effect lost. This may well be correct in code; it is flagged as **unverified** because the guarantee is load-bearing and the dispatch-within-txn wiring was not confirmed end-to-end in this pass.
- **Recommended fix.** Add (or point this review at) an integration test: issue an appointment order whose fill hits the one-holder index, assert the order stays `draft` and no membership row was written. If the bus dispatches post-commit, restructure order issue to run subscribers inside the issue `tx`.
- **Effort:** M
- **Confidence:** Speculative

### F-013 — `attributes` JSONB tagged `pii:special` (ceiling) is freely writable, so the "no special-category PII without the envelope" rule rests on convention
- **Severity:** Low
- **Lens:** architecture (security) + product
- **Location:** `migrations/20260601000005_person.sql:43,83` (`attributes jsonb … COMMENT 'pii:special'`); `docs/architecture/conventions.md` PII section; D-PIITiers.
- **Evidence.** `person_persons.attributes` and `document_documents.attributes` are tagged at the `pii:special` ceiling with the governance note "special-category fields must not land here without the envelope seam." Nothing in the write path prevents an operator from putting religion/health into the plaintext JSONB — the rule is prose only, and the envelope (D-CryptoProvider) covers `pii:sensitive` columns, not the JSONB grab-bag.
- **Why it's wrong.** For the church vertical especially, religion is the motivating Art. 9 example, and the path of least resistance (drop it into `attributes`) is unguarded plaintext. The control is classification-only with no enforcement.
- **Recommended fix.** Document that this is an accepted, convention-only control (the honest framing), or add a lightweight write-time reject for known special-category keys in `attributes`. At minimum, note it explicitly as a residual risk in `conventions.md` rather than implying the tier is enforced.
- **Effort:** S
- **Confidence:** Medium

---

## Overengineering / cost-benefit (not bugs)

### F-008 — 12 fully-decided, unbuilt verticals (M16–M26) sit ahead of a core whose guarantees don't work
- **Severity:** Cost-benefit
- **Lens:** overengineering
- **Location:** `docs/architecture/decisions.md` (D-Worker, D-DataIngestion, D-Languages, D-Location, D-Education, D-Companies, D-Religion + D-ClergyCredential + D-ReligiousAffiliation + D-SpecialPII, D-GeoSubdivisions, D-Vehicles); `docs/milestones.md` stage board rows M16–M26 (all `decided`/`designed`, none built).
- **Evidence.** `decisions.md` is ~2,090 lines; roughly half is M16–M26 verticals that are *decided/designed* with zero `internal/` or `migrations/` artifacts, while the shipped core (F-001/F-002) doesn't enforce its two headline guarantees. The religion vertical alone spans four decisions and a 292-line module doc before a single religion table exists.
- **Why it's a cost (not a bug).** Overengineering is explicitly permitted on this project, so this is not a defect — but it is complexity not earning its keep *yet*: each unbuilt decision is carrying cost (it must be kept coherent with every core change, e.g. the read-scope fix in F-001 must be re-checked against D-ReligiousAffiliation's `pii:special` reads) while delivering nothing runnable. The opportunity cost is the core: the directory leaks today.
- **Recommended fix.** No deletion required — these are recommendations only. Sequence-wise, freeze M16–M26 decisions as "parked/designed" and do not deepen them until M5/M7's actual enforcement (F-001/F-002) is green and the stage board is honest (F-003). Consider moving the long-horizon verticals' detail out of the binding `decisions.md` into a separate `roadmap-decisions.md` so the *binding* file reflects what the code is actually held to.
- **Effort:** M (doc reorganization) / 0 (if just deferring)
- **Confidence:** Medium

### F-014 — The composed-URN RID apparatus is heavy relative to its realized benefit
- **Severity:** Cost-benefit
- **Lens:** overengineering
- **Location:** `docs/architecture/decisions.md` D-ResourceIdentifiers, D-RIDSeeding; `migrations/20260601000000_schema_bootstrap.sql:34-40` (`new_rid`); every table's `…_rid_shape CHECK`.
- **Evidence.** Every PK is a ~70-byte `urn:oikumenea:<service>:<env>:<entity_type>:<uuid>` TEXT, widening every index and FK join from 16 bytes (the decision itself acknowledges this). The self-describing payload's *realized* consumers are: the web console's `parseRid` (client-side type routing) and the audit `action__…` key. D-RIDSeeding then exists **only** to work around a problem the RID scheme created — `new_rid()` needs `app.environment`, which Atlas's migration connection lacks, so all RID-keyed reference rows must be boot-seeded instead of migration-seeded, splitting table-create from seed for tenant graphs, base roles, and the bootstrap admin.
- **Why it's a cost (not a bug).** The `<service>`/`<environment>` slots are constant per database (L-SingleDomain), so they add bytes to every row to encode information that is invariant for the whole deployment. A plain `uuid_v7()` PK plus a small `entity_type` column (or the table name) would give the audit-ledger and type-routing benefits without the index bloat or the D-RIDSeeding GUC dance. This is permitted overengineering, flagged so the cost is visible: it is paid on every join, every index page, and every reference-data seed path.
- **Recommended fix.** None required if the Palantir-reference-implementation goal justifies it (a stated project value). If pragmatism wins later: keep `uuid_v7()` PKs, drop the URN to a derived/virtual representation at the API boundary, and retire D-RIDSeeding (migrations could seed reference rows normally). Note the blast radius is total (every table), so this is a "decide once, early" call — appropriate to raise now, pre-release.
- **Effort:** XL
- **Confidence:** Medium

---

## Lenses where I found little

- **Migrations vs conventions:** strong. RID PKs with shape CHECKs, `TEXT`+`CHECK` enums (no native enums), `set_updated_at()` triggers, `reject_mutation()` on the two append-only tables (`audit_log`, `tenant_unit_lifecycle_events`), soft-delete + partial unique indexes, per-column `pii:` comments, and the D-RIDSeeding "pure DDL, no RID seeds in migrations" rule are all honored consistently across all 15 files. `geo_countries` and `rank_grades` correctly use natural-key seeds in-migration. No native-enum, no naive `timestamp`, no missing trigger found.
- **PDP cascade math:** correct. `pdp.go` implements union-across-graphs, `subtree` self-inclusion, the `is_authority_bearing` skip (with the write-path reject as the primary guard), per-grant memoization, and no rank/position read. The closure rebuild query collapses multi-path DAG depth with `min(depth)` correctly and is cycle-safe by construction (guarded on insert via `ClosureHasPath`).
- **Hexagonal purity:** real. Every module's `domain/*.go` imports only stdlib (`context`, `errors`, `strings`, `time`, `encoding/json`) — no witchcraft/pgx/conjure in any domain layer (grep-confirmed).
- **i18n / code-vs-name:** consistent across migrations, docs, and the localization polymorphic store; `locale → text` maps, no Accept-Language negotiation, person names correctly kept out of the admin store.
- **Conjure ↔ docs:** the person contract carries every relationship/social/messenger endpoint the docs claim, with `person_social_links` correctly absent (deferred). 11 `*.conjure.yml`, one per module.
- **Link coherence + build:** the repo's link-checker passes; `go build ./...` and `go vet` are clean.
