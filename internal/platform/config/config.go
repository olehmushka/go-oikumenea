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

	// Reserved seams, declared now so the config shape is forward-compatible but wired in later
	// milestones: bootstrap_admin (D-Bootstrap, M8), the crypto/KMS block (D-CryptoProvider, M9),
	// and account.identity_linking.enabled (identity-federation, M8). Intentionally omitted here.
}

// Postgres holds the operator-supplied connection string.
type Postgres struct {
	DSN string `yaml:"dsn"`
}

// Runtime is the hot-reloadable configuration (var/conf/runtime.yml), read through a refreshable.
type Runtime struct {
	wconfig.Runtime `yaml:",inline"`

	// DefaultPageSize is the token-pagination default (API conventions). Tunable at runtime.
	DefaultPageSize int `yaml:"default-page-size"`
}
