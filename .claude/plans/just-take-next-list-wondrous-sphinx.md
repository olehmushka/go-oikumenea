# F-012 — Verify order auto-apply same-transaction subscriber contract

## Context

`docs/list_to_fix.md` is an adversarial-review tracker; its **▶ NEXT** item is **F-012**
(Low, architecture/**verify**, confidence *Speculative*). The concern: D-OrderApply's
all-or-nothing guarantee depends on order subscribers (membership/person) running **inside the
order-issue transaction**, so a subscriber failure (e.g. the one-holder position index tripping)
rolls back the issuing order's own writes — but the reviewer did not confirm the dispatch-within-txn
wiring end-to-end, nor find a test proving it. The recommended fix is to *"add (or point this review
at) an integration test: issue an appointment order whose fill hits the one-holder index, assert the
order stays draft and no membership row was written."*

**Finding from exploration: the contract is already correctly implemented and already tested.**

- The bus is synchronous and same-transaction: `pkg/events/events.go:60-68` — `Publish(ctx, tx, evt)`
  loops handlers in-process, passes the caller's `tx` through unchanged, and returns the first
  handler error immediately.
- Order issue threads one `tx` through the publish: `internal/order/application/service.go:204-244`
  (`IssueOrder` runs inside `s.inTx`; `bus.Publish(ctx, tx, …)` at `:227` and per-item at `:230-238`;
  any `Publish` error → `ErrEffectFailed` → rollback via the `inTx` defer).
- Subscribers use that same `tx`: membership `fillPositionTx`/`createMembershipTx`
  (`internal/membership/application/service.go:325-336`), person `setRankTx`
  (`internal/person/application/service.go:317-326`).
- One-holder index: `migrations/20260601000006_membership.sql:86-89`
  (`membership_memberships_one_holder_idx`).
- Existing test **`TestIssueAllOrNothingRollback`** (`internal/order/order_integration_test.go:195-234`)
  already issues a second appointment onto a filled billet, asserts `IssueOrder` returns
  `ErrEffectFailed` (`:217`), the order stays `OrderDraft` (`:226`), and the holder is still person A
  (`:231`).

So F-012 is satisfiable by **pointing the review at the existing test**. The one item in the
finding's checklist the test only proves *by inference* is *"no membership row was written"* (it
asserts the holder is unchanged, not that person B has zero memberships). To make the test literally
match the finding's wording, add one direct assertion.

## Change

**1. Strengthen `TestIssueAllOrNothingRollback`** —
`internal/order/order_integration_test.go` (after the holder check, ~`:233`).

Add a direct assertion that the rolled-back person B has **zero** memberships, reusing the exact
pattern already used in `TestRecordOnlyStandsAlone` (`:248-254`):

```go
// And no membership row was written for person B — the failed fill rolled back cleanly.
pageB, err := e.mem.ListPersonMemberships(ctx, personB, 50, "")
if err != nil {
    t.Fatalf("list B memberships: %v", err)
}
if len(pageB.Memberships) != 0 {
    t.Fatalf("rolled-back order left %d memberships for person B, want 0", len(pageB.Memberships))
}
```

No production code, schema, migration, or doc-decision change — the contract already holds; this only
makes the proof explicit.

**2. Update the tracker** — `docs/list_to_fix.md`:
- Move the **F-012** row from **To do** to **Done** with a `*Done 2026-06-11.*` note summarizing that
  the contract was verified (bus/issue/subscriber trace) and the rollback test was strengthened with
  the explicit "no membership row for B" assertion.
- Advance **▶ NEXT** to **F-013** (`attributes` JSONB `pii:special` ceiling is convention-only) and
  drop F-012 from the To-do list (renumber remaining).
- Mark the **§F-012** section heading with `✅` and add a `**Status:** ✅ VERIFIED 2026-06-11 …` line
  matching the style of the other closed findings.

## Verification

- Run the order integration suite against the test DB (build tag `integration`):
  ```bash
  OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
    go test -tags integration ./internal/order/...
  ```
  Expect `TestIssueAllOrNothingRollback` (and the rest) green. If `oikumenea_test` is stale, reset via
  `scripts/setup-test-db.sh` first.
- `go build ./...` and `go vet ./internal/order/...` clean (test-only edit, should be unaffected).
- Run the docs link-checker (CLAUDE.md snippet) after the tracker edit — expect `links OK`.
