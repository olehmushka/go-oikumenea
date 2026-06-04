// Package config defines the platform install + runtime configuration types (ECV + refreshable;
// docs/modules/platform.md). They embed the witchcraft base configs and add the
// operator-supplied fields go-oikumenea needs.
package config

import (
	wconfig "github.com/palantir/witchcraft-go-server/v2/config"
)

// Install is the static, operator-supplied configuration (var/conf/install.yml). Secrets are
// ECV-encrypted in real deployments; the local-dev file is plaintext.
type Install struct {
	wconfig.Install `yaml:",inline"`

	// Environment is the deployment environment segment baked into every RID via app.environment
	// (D-ResourceIdentifiers): one of local|dev|staging|prod. Constant per database (L-SingleDomain).
	Environment string `yaml:"environment"`

	// Postgres is the operator-owned database connection (L-OperatorDB).
	Postgres Postgres `yaml:"postgres"`

	// IDP configures inbound-token validation (identity-federation.md): the accepted issuer(s) +
	// JWKS/audience + just-in-time provisioning (D-JIT). Authentication is delegated (L-AuthzOnly).
	IDP IDP `yaml:"idp"`

	// Account holds identity-federation account knobs — notably identity_linking.enabled, which gates
	// linking ADDITIONAL login points to one account (default true).
	Account Account `yaml:"account"`

	// BootstrapAdmin seeds the first instance admin at first boot, binding an IdP identity (D-Bootstrap).
	// nil/absent => no bootstrap is attempted (the operator uses the recover-admin CLI instead).
	BootstrapAdmin *BootstrapAdmin `yaml:"bootstrap-admin"`

	// Crypto configures envelope encryption for pii:sensitive data (D-CryptoProvider, M9): the KMS
	// backend and the blind-index key the document module uses for personal-code values.
	Crypto Crypto `yaml:"crypto"`
}

// Postgres holds the operator-supplied connection string.
type Postgres struct {
	DSN string `yaml:"dsn"`
}

// IDP is the external identity-provider configuration consumed by the validation middleware.
type IDP struct {
	// Issuers are the accepted IdP issuers. Production issuers use OIDC discovery + JWKS (RS256);
	// a local-dev issuer may use a symmetric HS256 key (type: hs256) so tests/dev can mint tokens.
	Issuers []Issuer `yaml:"issuers"`
	// ClockSkewSeconds is the leeway applied to exp/nbf validation (default 60s when zero).
	ClockSkewSeconds int `yaml:"clock-skew-seconds"`
	// JIT configures just-in-time link-on-match provisioning (D-JIT).
	JIT JIT `yaml:"jit"`
}

// Issuer is one accepted IdP issuer.
type Issuer struct {
	Issuer   string `yaml:"issuer"`   // the `iss` value; also the OIDC discovery base URL
	Audience string `yaml:"audience"` // expected `aud`; empty skips the check
	Type     string `yaml:"type"`     // "oidc" (default) | "hs256" (local-dev symmetric)
	HMACKey  string `yaml:"hmac-key"` // verification key for type hs256 (secret; ECV-encrypted)
}

// JIT configures just-in-time provisioning: default reject-unknown; when enabled, link-on-match only
// against an EXISTING person via a token-claim -> person.code mapping (D-JIT). It never creates a person.
type JIT struct {
	Enabled bool   `yaml:"enabled"`
	Claim   string `yaml:"claim"` // token claim whose value maps to a person.code (default "person_code")
}

// Account holds identity-federation account knobs.
type Account struct {
	// IdentityLinkingEnabled gates linking ADDITIONAL login points to one account. Pointer so the
	// documented default (true) applies when the operator omits it.
	IdentityLinkingEnabled *bool `yaml:"identity-linking-enabled"`
}

// BootstrapAdmin is the operator-supplied first-admin seed (D-Bootstrap): an IdP identity bound to a
// person, granted the instance-admin plane on first boot.
type BootstrapAdmin struct {
	Issuer      string `yaml:"issuer"`       // the IdP `iss`
	Subject     string `yaml:"subject"`      // the IdP `sub`
	Email       string `yaml:"email"`        // optional asserted email
	DisplayName string `yaml:"display-name"` // the seeded person's display name
	PersonCode  string `yaml:"person-code"`  // optional stable code; when set, link-to-existing-by-code
}

// IdentityLinkingEnabled returns whether linking additional identities is permitted, defaulting to
// true when the operator did not set it (identity-federation.md).
func (i Install) IdentityLinkingEnabled() bool {
	if i.Account.IdentityLinkingEnabled == nil {
		return true
	}
	return *i.Account.IdentityLinkingEnabled
}

// Crypto configures envelope encryption for pii:sensitive data (D-CryptoProvider): the pluggable KMS
// backend plus the keyed-HMAC blind-index key. Secrets (KEK material, blind-index key) are
// ECV-encrypted in real deployments; the local-dev file is plaintext. Keys are base64-encoded.
type Crypto struct {
	// Provider selects the KeyProvider backend: local-dev (today) | aws-kms | gcp-kms | vault-transit |
	// azure-kv. Defaults to local-dev when empty.
	Provider string `yaml:"provider"`
	// BlindIndexKey is the base64-encoded HMAC key for personal-code blind indexing (required for the
	// document module). Equality lookup / cross-person uniqueness over ciphertext depends on it.
	BlindIndexKey string `yaml:"blind-index-key"`
	// DEKCacheTTLSeconds bounds how long an unwrapped DEK is cached off the KMS read path (default 300s
	// when zero; a negative value disables caching).
	DEKCacheTTLSeconds int `yaml:"dek-cache-ttl-seconds"`
	// LocalDev holds the local-dev backend's symmetric KEK (used when Provider is local-dev/empty).
	LocalDev CryptoLocalDev `yaml:"local-dev"`
}

// CryptoLocalDev is the local-dev KeyProvider's key material.
type CryptoLocalDev struct {
	// KEK is the base64-encoded 32-byte key-encryption key that wraps per-record DEKs (secret).
	KEK string `yaml:"kek"`
}

// Runtime is the hot-reloadable configuration (var/conf/runtime.yml), read through a refreshable.
type Runtime struct {
	wconfig.Runtime `yaml:",inline"`

	// DefaultPageSize is the token-pagination default (API conventions). Tunable at runtime.
	DefaultPageSize int `yaml:"default-page-size"`

	// PersonPurgeGraceHours is the reversible deactivate->purge window for persons, in hours
	// (D-PersonReadScope). Purge is refused before deactivated_at + this window. Defaults to 720h
	// (30 days) when unset. Tunable at runtime.
	PersonPurgeGraceHours int `yaml:"person-purge-grace-hours"`
}
