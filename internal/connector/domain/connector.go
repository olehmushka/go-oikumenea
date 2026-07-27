// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package domain holds the connector-plane module's entities, ports and invariants (M53 /
// D-ConnectorPlane) — the registry of connectors (deployable agents beside the core, of which
// hermenea is the first), the sources they sync, and the sync runs they report. The domain owns its
// Repository interface and imports no framework.
//
// The plane's governing property is VISIBILITY, NOT ORCHESTRATION: the core records what a connector
// reports and shows it to operators; it never schedules, triggers or retries a run. That is why there
// is no Trigger/Retry anywhere in this package — scheduling lives inside the connector, and
// D-ConnectorPlane explicitly rejects core-side orchestration (its alternative (a)).
//
// Altitude note: "connector" here is a whole agent. hermenea's internal `Fetcher` seam (a fetch
// strategy — http/file/wof-sqlite) carried this name through M52 and was renamed in M53 so the two
// never collide.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Connector status values.
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// Sync-run states. A run is opened `running` and closed `succeeded` or `failed`.
const (
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
)

// ---- sentinel errors (mapped to Conjure Connector:* in transport) ----
var (
	ErrConnectorNotFound = errors.New("connector: connector not found")
	ErrSourceNotFound    = errors.New("connector: source not found for this connector")
	ErrConflict          = errors.New("connector: code is registered to a different principal")
	ErrInvalid           = errors.New("connector: invalid registration or report")
	// ErrNoPrincipal is returned when a self-service surface is reached without a machine subject.
	// The PEP gates these on service-held codes, so this is a defence-in-depth guard rather than the
	// primary check — it keeps the invariant "a registry row is always bound to a real principal"
	// true even if a future gate changes.
	ErrNoPrincipal = errors.New("connector: self-registration requires a service principal")
)

// Connector is a registered agent. PrincipalID is the M51 service principal it authenticates as; it
// is bound by the core from the CALLER, never accepted from a request, so a connector cannot register
// or report as another.
type Connector struct {
	ID          string
	Code        string
	Name        string
	Description string // "" = none
	PrincipalID string // "" = none (an operator-created row not yet claimed by an agent)
	Status      string
	LastSeenAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Source is one dataset a connector syncs, as reported by that connector. A read model: the connector
// stays authoritative for execution.
type Source struct {
	ID          string
	ConnectorID string
	Code        string
	Name        string
	ObjectType  string // "" = not a push-mode source
	Schedule    string // "" = trigger-only; verbatim, never parsed by the core
	Enabled     bool
}

// SyncRun is one reported execution. The counts are the connector's account of its work.
type SyncRun struct {
	ID            string
	SourceID      string
	ExternalRunID string // "" = none
	State         string
	Created       int64
	Updated       int64
	Skipped       int64
	Error         string // "" = none
	StartedAt     time.Time
	FinishedAt    *time.Time
}

// SourceDeclaration is one source in a registration payload.
type SourceDeclaration struct {
	Code       string
	Name       string
	ObjectType string
	Schedule   string
	Enabled    bool
}

// RegistrationInput is a connector's idempotent self-description. PrincipalID is filled by the
// application layer from the request context, not from the wire.
type RegistrationInput struct {
	PrincipalID string
	Code        string
	Name        string
	Description string
	Sources     []SourceDeclaration
}

// Validate enforces the shape a registration must have. Codes follow D-Code (non-empty, no
// whitespace) because they are operator-facing handles that appear in audit rows and URLs.
func (r RegistrationInput) Validate() error {
	if strings.TrimSpace(r.PrincipalID) == "" {
		return ErrNoPrincipal
	}
	if !validCode(r.Code) {
		return wrapInvalid("connector code is required, must be <=128 chars and contain no whitespace")
	}
	if strings.TrimSpace(r.Name) == "" {
		return wrapInvalid("connector name is required")
	}
	seen := make(map[string]struct{}, len(r.Sources))
	for _, s := range r.Sources {
		if !validCode(s.Code) {
			return wrapInvalid("source code is required, must be <=128 chars and contain no whitespace")
		}
		if strings.TrimSpace(s.Name) == "" {
			return wrapInvalid("source name is required: " + s.Code)
		}
		if _, dup := seen[s.Code]; dup {
			return wrapInvalid("duplicate source code in registration: " + s.Code)
		}
		seen[s.Code] = struct{}{}
	}
	return nil
}

// ReportInput is one sync-run report. SourceCode is resolved within the CALLING connector, so a
// connector needs no prior read to report.
type ReportInput struct {
	PrincipalID   string
	SourceCode    string
	ExternalRunID string
	State         string
	Created       int64
	Updated       int64
	Skipped       int64
	Error         string
	StartedAt     *time.Time
	FinishedAt    *time.Time
}

// Validate enforces the run-state invariant the schema also checks: a finished run says when it
// finished, a running one does not. Validating here as well keeps the error a readable 400 rather
// than a constraint violation surfacing as a 500.
func (r ReportInput) Validate() error {
	if strings.TrimSpace(r.PrincipalID) == "" {
		return ErrNoPrincipal
	}
	if !validCode(r.SourceCode) {
		return wrapInvalid("sourceCode is required")
	}
	switch r.State {
	case RunRunning:
		if r.FinishedAt != nil {
			return wrapInvalid("a running run must not carry finishedAt")
		}
	case RunSucceeded, RunFailed:
		if r.FinishedAt == nil {
			return wrapInvalid("a finished run must carry finishedAt")
		}
	default:
		return wrapInvalid("state must be one of running|succeeded|failed: " + r.State)
	}
	if r.Created < 0 || r.Updated < 0 || r.Skipped < 0 {
		return wrapInvalid("counts must not be negative")
	}
	return nil
}

// Terminal reports whether a run has finished (either way).
func (r SyncRun) Terminal() bool { return r.State == RunSucceeded || r.State == RunFailed }

func wrapInvalid(msg string) error { return errors.Join(ErrInvalid, errors.New(msg)) }

// validCode is the shared code-shape guard (D-Code), mirroring the authorization domain's rule.
func validCode(code string) bool {
	if code == "" || len(code) > 128 {
		return false
	}
	return !strings.ContainsFunc(code, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// Repository is the connector-plane persistence port (implemented by adapters over raw pgx). Bound to
// a single command surface — the pool for reads, or a caller's transaction for an audited write
// (D-Audit). The application layer owns transaction boundaries; the repository never opens its own.
type Repository interface {
	// registry
	UpsertConnectorByPrincipal(ctx context.Context, in RegistrationInput) (Connector, error)
	GetConnector(ctx context.Context, id string) (Connector, error)
	GetConnectorByPrincipal(ctx context.Context, principalID string) (Connector, error)
	ListConnectors(ctx context.Context, after string, lim int) ([]Connector, error)
	TouchLastSeen(ctx context.Context, connectorID string) error

	// sources — ReplaceSources retires declared-away sources so boot registration converges.
	ReplaceSources(ctx context.Context, connectorID string, decls []SourceDeclaration) ([]Source, error)
	ListSources(ctx context.Context, connectorID string) ([]Source, error)
	GetSourceByCode(ctx context.Context, connectorID, code string) (Source, error)

	// runs — UpsertRun is idempotent on (source, externalRunId) so connector retries converge.
	UpsertRun(ctx context.Context, sourceID string, in ReportInput) (SyncRun, error)
	ListRuns(ctx context.Context, sourceID, after string, lim int) ([]SyncRun, error)
}
