# Plan — F-009: guardrail the production HS256 (symmetric) issuer path

## Context

`docs/list_to_fix.md` **▶ NEXT** is **F-009** (Medium, security). The inbound-token validator
(`internal/identityfederation/middleware/validator.go`) supports two issuer types: `oidc`
(production, RS256 + JWKS) and `hs256` (a symmetric HMAC key from install config, intended
local-dev only — see the `IssuerHS256` const comment, "local-dev"). But **nothing prevents a
production deployment from configuring an `hs256` issuer.** An HS256 verification key *is* a
credential-equivalent: anyone holding the install secret can mint valid tokens for any subject —
directly contradicting L-AuthzOnly ("we validate, we never hold credentials"). The "local-dev
only" intent is enforced by **convention/comment, not code or config validation**.

Intended outcome: refuse `hs256` issuers **at boot** unless the deployment environment is
local/dev, failing closed. The `Install.Environment` slot (`local|dev|staging|prod`, already used
for RIDs — `internal/platform/config/config.go:17`) is the existing lever the review points at.

## Approach (review option a — reuse the existing environment slot)

Gate symmetric issuers on `install.Environment ∈ {local, dev}`; refuse at boot otherwise. No new
config field (keeps it simple; avoids a new default to document).

### 1. `internal/identityfederation/middleware/validator.go` — the guard (domain knowledge lives here)

Add an exported, fail-closed guard near the `IssuerType` consts:

```go
// GuardSymmetricIssuers refuses HS256 (symmetric) issuers outside local/dev. A symmetric key is a
// credential-equivalent the service would hold (anyone with the install secret can mint valid
// tokens), so it is allowed only where minting test tokens is the point. Fail-closed: any
// environment other than local/dev rejects, including an empty/unknown value.
func GuardSymmetricIssuers(issuers []IssuerConfig, environment string) error {
    if environment == "local" || environment == "dev" {
        return nil
    }
    for _, ic := range issuers {
        if ic.Type == IssuerHS256 {
            return fmt.Errorf("issuer %q uses symmetric HS256, permitted only in local/dev (environment=%q)", ic.Issuer, environment)
        }
    }
    return nil
}
```

(Adds `"fmt"` to the import block.) Tighten the `IssuerHS256` const comment to say "local/dev only;
refused at boot elsewhere (GuardSymmetricIssuers)."

### 2. `cmd/oikumenea/main.go` — wire it into the boot path (fail closed)

In `buildAndRegister`, replace the inline `NewValidator(validatorConfig(install))` at line 199 so
the config is built once, guarded, then bound:

```go
vcfg := validatorConfig(install)
if err := middleware.GuardSymmetricIssuers(vcfg.Issuers, install.Environment); err != nil {
    cleanup()
    return nil, werror.Wrap(err, "reject symmetric issuer outside local/dev")
}
authenticator.Bind(middleware.NewValidator(vcfg), identitySvc, personSvc, install.IDP.JIT.Enabled, authzSvc, pool)
```

This matches the existing `cleanup()` + `werror.Wrap` boot-failure pattern used throughout the
function (e.g. lines 180-187, 191-195). `werror` is already imported.

### 3. Tests — `internal/identityfederation/middleware/validator_test.go`

Add `TestGuardSymmetricIssuers`, a table test over the existing `IssuerConfig` shape:
- `hs256` issuer + `environment ∈ {prod, staging, ""}` → **error** (names the issuer);
- `hs256` issuer + `environment ∈ {local, dev}` → nil;
- `oidc` issuer + `environment = prod` → nil (asymmetric issuers unaffected).
Reuse the `testIssuer`/`testKey` constants already in the file.

### 4. Docs — `docs/modules/identity-federation.md`

- In the validation-steps / install-config prose (around lines 80-94), add: symmetric (HS256)
  issuers are **local/dev only and refused at boot** in staging/prod (the validator holds no
  credentials elsewhere — L-AuthzOnly).
- Add a line to the **Invariants & safety** list (~line 135): *"HS256 (symmetric) issuers are
  local/dev only — the service refuses to boot with one configured in staging/prod
  (GuardSymmetricIssuers); production issuers are OIDC/JWKS (asymmetric)."*
- Optionally tighten the `Issuer.Type` comment in `config.go:59` to "(local/dev only; refused at
  boot in staging/prod)".

### 5. Tracker — `docs/list_to_fix.md` (same pass)

- Move **F-009** from *To do* to *Done* with a `*Done 2026-06-11.*` note (boot guard + test + doc).
- Advance **▶ NEXT** to **F-010** (migration filename off-by-one), renumber *To do*.
- Mark `✅` + a **Status:** line at the F-009 section heading, matching the F-001…F-007 closures.

## Files touched

- `internal/identityfederation/middleware/validator.go` (guard fn + const comment)
- `cmd/oikumenea/main.go` (boot-time guard call)
- `internal/identityfederation/middleware/validator_test.go` (new test)
- `docs/modules/identity-federation.md` (constraint + invariant)
- `internal/platform/config/config.go` (comment tightening — optional)
- `docs/list_to_fix.md` (tracker + ✅)

## Verification

- `go build ./...` — clean.
- `go test ./internal/identityfederation/middleware/...` — new `TestGuardSymmetricIssuers` passes,
  existing HS256 accept/reject tests still pass.
- `go vet ./internal/identityfederation/...` — clean.
- Manual sanity: a config with an `hs256` issuer and `environment: prod` must make
  `buildAndRegister` return a wrapped error (boot fails); the same config with `environment: dev`
  boots. Confirm by reading the guard call path (no live prod IdP needed).
- Docs link-checker (CLAUDE.md snippet) from `docs/` — `links OK`.
