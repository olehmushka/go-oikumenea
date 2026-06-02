// Package domain holds the audit module's pure logic: the AuditEntry it records, its invariants,
// and the Repository port it needs from the outside world (overview.md layering). No I/O, no
// framework imports — only the standard library. The audit log is the append-only Action ledger
// (D-Audit / docs/modules/audit.md): each entry records one Action and is keyed by that Action's
// RID.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ActorType is one of the two audited actor kinds (D-Audit). There is no super_admin kind — an
// instance admin is a person, marked instance-scoped by the action's permission.
type ActorType string

const (
	ActorPerson ActorType = "person"
	ActorSystem ActorType = "system"
)

// Outcome records whether the audited action succeeded, was denied (a denied write attempt is
// recorded too, D-Audit), or errored.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeDenied  Outcome = "denied"
	OutcomeError   Outcome = "error"
)

// ErrNotFound is returned by Repository.Get when no entry has the given Action RID.
var ErrNotFound = errors.New("audit entry not found")

// ErrInvalidEntry is the sentinel wrapped by Entry.Validate failures (mapped to INVALID_ARGUMENT
// by the transport layer). The DB CHECK constraints enforce the same shape as a backstop.
var ErrInvalidEntry = errors.New("invalid audit entry")

// Entry is one immutable audit record: who did what to which target, when, in which request, with
// before/after context. ID is the Action RID this entry records (action__<type>); CreatedAt is
// assigned by the database on insert.
type Entry struct {
	ID            string
	CreatedAt     time.Time
	ActorType     ActorType
	ActorPersonID string // required iff ActorType == ActorPerson
	Subsystem     string // required iff ActorType == ActorSystem
	Action        string
	TargetType    string
	TargetID      string
	UnitID        string
	RequestID     string
	Before        json.RawMessage // pii:special ceiling — no special-category data until DS-29
	After         json.RawMessage
	Outcome       Outcome
}

// Validate enforces the entry invariants before it is recorded (D-Audit): a well-formed Action
// RID, the mutually-exclusive actor shape, the required fields, and a known outcome.
func (e Entry) Validate() error {
	if !isActionRID(e.ID) {
		return wrap("id must be an action__<type> RID")
	}
	if e.Action == "" || e.TargetType == "" || e.RequestID == "" {
		return wrap("action, targetType, and requestId are required")
	}
	switch e.Outcome {
	case OutcomeSuccess, OutcomeDenied, OutcomeError:
	default:
		return wrap("outcome must be success, denied, or error")
	}
	switch e.ActorType {
	case ActorPerson:
		if e.ActorPersonID == "" || e.Subsystem != "" {
			return wrap("person actor requires actorPersonId and no subsystem")
		}
	case ActorSystem:
		if e.Subsystem == "" || e.ActorPersonID != "" {
			return wrap("system actor requires subsystem and no actorPersonId")
		}
	default:
		return wrap("actorType must be person or system")
	}
	return nil
}

func wrap(msg string) error { return errors.Join(ErrInvalidEntry, errors.New(msg)) }

// isActionRID mirrors the audit_log_action_rid_shape CHECK: urn:oikumenea:<svc>:<env>:action__<t>:<uuid>.
func isActionRID(id string) bool {
	parts := strings.Split(id, ":")
	return len(parts) == 6 && parts[0] == "urn" && parts[1] == "oikumenea" &&
		strings.HasPrefix(parts[4], "action__")
}

// Cursor is the keyset position for newest-first pagination over (created_at, id).
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// Filter selects and paginates audit entries. A nil pointer field matches everything (D-Audit:
// the log is filterable by every audited entity type). Limit is the exact number of rows the
// repository returns; the application passes pageSize+1 to detect a further page.
type Filter struct {
	ActorPersonID *string
	ActorType     *ActorType
	TargetType    *string
	TargetID      *string
	UnitID        *string
	Action        *string
	Outcome       *Outcome
	Since         *time.Time
	Until         *time.Time
	Cursor        *Cursor
	Limit         int
}

// Repository is the persistence port the audit application service depends on; the pgx/sqlc
// adapter implements it. Insert runs on whatever DBTX the adapter was constructed with, so a
// caller can record in its own transaction (D-Audit).
type Repository interface {
	Insert(ctx context.Context, e Entry) error
	Get(ctx context.Context, id string) (Entry, error)
	Query(ctx context.Context, f Filter) ([]Entry, error)
}
