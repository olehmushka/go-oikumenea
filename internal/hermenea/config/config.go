// Package config defines hermenea's install configuration (M16 / D-Hermenea). It embeds the witchcraft
// base install config and adds hermenea's own Postgres DSN, the oikumenea import endpoint coordinates,
// the declarative source registry, and the worker tunables. Shared secrets are NOT here — they are
// RUNTIME env vars (HERMENEA_OIKUMENEA_TOKEN, OIKUMENEA_HERMENEA_TOKEN), read in cmd/hermenea.
package config

import (
	wconfig "github.com/palantir/witchcraft-go-server/v2/config"
)

// Install is hermenea's static operator config (var/conf/install.yml).
type Install struct {
	wconfig.Install `yaml:",inline"`

	// Postgres is hermenea's OWN database (separate from oikumenea's).
	Postgres Postgres `yaml:"postgres"`

	// Oikumenea points the loader at oikumenea's public API.
	Oikumenea Oikumenea `yaml:"oikumenea"`

	// Sources declares the import sources hermenea seeds at boot (idempotent upsert). Operators may
	// also register sources at runtime; this is the declarative baseline.
	Sources []Source `yaml:"sources"`

	// Worker tunes the runtime loops + retry policy. Zero fields fall back to documented defaults.
	Worker Worker `yaml:"worker"`
}

// Postgres holds hermenea's connection string.
type Postgres struct {
	DSN string `yaml:"dsn"`
}

// Oikumenea is the loader target.
type Oikumenea struct {
	// BaseURL is oikumenea's HTTPS base (e.g. https://oikumenea:8443).
	BaseURL string `yaml:"base-url"`
	// InsecureSkipVerify disables TLS verification (for the self-signed local-dev cert). Never in prod.
	InsecureSkipVerify bool `yaml:"insecure-skip-verify"`
}

// Source is a declaratively-seeded import source.
type Source struct {
	Code          string `yaml:"code"`
	Name          string `yaml:"name"`
	ConnectorType string `yaml:"connector-type"` // http | file
	ObjectType    string `yaml:"object-type"`    // oikumenea import target (e.g. geo-countries)
	Locator       string `yaml:"locator"`        // URL (http) or path (file)
	Cron          string `yaml:"cron"`           // optional: @every <dur> / @hourly / @daily / @weekly
	Enabled       *bool  `yaml:"enabled"`        // default true
}

// EnabledOrDefault returns Enabled, defaulting to true when unset.
func (s Source) EnabledOrDefault() bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

// Worker tunables (all milliseconds; zero => default).
type Worker struct {
	PollIntervalMs int `yaml:"poll-interval-ms"` // queue poll cadence (default 2000)
	ScheduleTickMs int `yaml:"schedule-tick-ms"` // cron evaluation cadence (default 30000)
	BackoffBaseMs  int `yaml:"backoff-base-ms"`  // first retry delay (default 5000)
	BackoffMaxMs   int `yaml:"backoff-max-ms"`   // backoff cap (default 300000)
	JobTimeoutMs   int `yaml:"job-timeout-ms"`   // per-job hard timeout (default 120000)
	StaleAfterMs   int `yaml:"stale-after-ms"`   // requeue stale 'running' jobs after (default 600000)
}
