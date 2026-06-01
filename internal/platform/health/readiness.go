// Package health holds the platform readiness-gating reporters (witchcraft-go-health;
// docs/modules/platform.md). Diagnostic-only reporters (e.g. closure-drift, D-ClosureDriftHealth)
// arrive with their owning modules and must NOT gate readiness.
package health

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// ReadinessSource gates /status/readiness on two conditions (upgrade-safety.md):
//   - the operator database is reachable, and
//   - its applied schema revision matches what this binary expects (refuse on newer/unknown).
//
// It implements witchcraft-go-health status.Source: Status() returns 200 when ready, 503 otherwise.
type ReadinessSource struct {
	pool *pgxpool.Pool
}

// NewReadinessSource builds the readiness gate over the given pool.
func NewReadinessSource(pool *pgxpool.Pool) ReadinessSource {
	return ReadinessSource{pool: pool}
}

type readinessMetadata struct {
	Ready          bool   `json:"ready"`
	Reason         string `json:"reason,omitempty"`
	SchemaRevision string `json:"schemaRevision,omitempty"`
	ExpectedSchema string `json:"expectedSchemaRevision"`
}

// Status reports readiness. Signature satisfies witchcraft-go-health status.Source.
func (r ReadinessSource) Status() (int, interface{}) {
	ctx := context.Background()
	meta := readinessMetadata{ExpectedSchema: db.ExpectedSchemaRevision}

	if err := r.pool.Ping(ctx); err != nil {
		meta.Reason = "database unreachable"
		return http.StatusServiceUnavailable, meta
	}

	revision, err := db.ReadSchemaRevision(ctx, r.pool)
	if err != nil {
		meta.Reason = "schema_version unreadable"
		return http.StatusServiceUnavailable, meta
	}
	meta.SchemaRevision = revision

	if revision != db.ExpectedSchemaRevision {
		// DB is newer/unknown or older than this binary understands — refuse readiness rather
		// than risk writing against an unfamiliar schema.
		meta.Reason = "schema revision mismatch"
		return http.StatusServiceUnavailable, meta
	}

	meta.Ready = true
	return http.StatusOK, meta
}
