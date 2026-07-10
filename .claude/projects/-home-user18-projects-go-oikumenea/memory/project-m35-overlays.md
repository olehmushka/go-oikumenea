---
name: project-m35-overlays
description: M35 financial/behavioural/psychological overlays (D-PersonOverlays) VERIFIED uncommitted on main 2026-07-05
metadata:
  type: project
---

**M35 (D-PersonOverlays) VERIFIED, uncommitted on `main` 2026-07-05.** Migration `0035_person_overlays`
(bumped `ExpectedSchemaRevision`→`0035_person_overlays`), 3 new person objects:

- `person_crypto_wallets` (RID `6,1,17`, pii:sensitive) — plaintext address (public on-chain, attribution
  sensitive) + `chain`/`attribution_method`/`balance_usd_approx` (**double precision**, not numeric →
  clean `float8Arg`/`float8Ptr` helpers, avoided pgtype.Numeric); dedup active `(person,chain,address)`;
  hard-deleted on purge.
- `person_personality` (RID `6,1,18`, pii:sensitive) — `method` CHECK ∈ {self_declared_survey,hr_assessment}
  enforces **declared/assessment-only, no text-inference**; one active per framework; hard-deleted on purge.
- `person_political_leaning` (RID `6,1,19`, pii:special) — **INFERRED** spectrum ∈ [-1,1]
  **envelope-encrypted** (reuses `s.seal`/`openLeaning`, spectrum sealed as `strconv.FormatFloat`), NOT-NULL
  `legal_basis`, ONE active row/person (partial-unique person_id, upsert = UPDATE-then-INSERT-on-notfound),
  crypto-erased on purge. **SEPARATE table from the declared M33 party membership, NEVER merged** — its
  partial-unique person_id is NOT in `repointOwnedStmts` (dropped on merge's Purge, like the M34 watchlist
  match); wallets+personality ARE re-homed on merge.

Built by mirroring M33 [[project-m33-watchlists]]-adjacent institutional ties (encrypted party = template for
leaning; plaintext ties = template for wallet/personality). Full stack: domain `overlays.go`, application
`overlays.go`, transport `overlays.go`, repo wrappers + purge/merge edits, sqlc queries, `person.conjure.yml`
types+endpoints (`/crypto-wallets`, `/personalities`, `/political-leaning`), Go+TS SDK regen, web
`OverlaysManager` card in PersonForms.tsx + person page.

Gotchas: (1) conjure `double`→TS `number | "NaN"` so web must `Number(x)`-coerce spectrum/balance before
`.toFixed`/comparisons. (2) legal-basis code is `legitimate_interest` (singular), Art.9 filter is
`k.article === "art9"`. (3) sqlc regen touches EVERY module's `models.go` (full-schema) — revert all NON-owner
`*sql/models.go` (kept only `personsql/models.go`); the revert also dropped M44 finance tables that were
likewise reverted last commit — this is the standing repo convention. (4) atlas hash clean (new file appends
1 line); applied to BOTH `postgres` (dev) + `oikumenea_test`.

Verified: integration `TestPersonOverlays` (ciphertext-at-rest no plaintext + blind index + decrypt,
single-active replace, dedup, purge hard-delete+crypto-erase) + full person suite green; web tsc+next build
clean; **live HTTP smoke** (wallet upsert/list, personality 400-on-`text_inference`, encrypted leaning
round-trip + `psql` no-plaintext-at-rest check `t|t`, missing-legalBasis 400). Compensation/payroll → M39.
Next unbuilt: M36 (health & vulnerability, D-HealthVulnerability) then M37 (login security log).
