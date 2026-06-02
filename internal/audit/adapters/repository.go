// Package adapters implements the audit domain ports against infrastructure: the pgx/sqlc
// repository over oikumenea.audit_log. It depends on the database, never the reverse
// (overview.md). Generated sqlc code lives in the auditsql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/audit/adapters/auditsql"
	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository. It is bound to a single
// db.DBTX — the pool for reads, or a caller-supplied transaction so an audited write commits iff
// the change commits (D-Audit).
type Repository struct {
	q *auditsql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: auditsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

// Insert records one entry. The Action RID (e.ID) is supplied by the caller; created_at defaults
// at the database.
func (r *Repository) Insert(ctx context.Context, e domain.Entry) error {
	return r.q.InsertAuditEntry(ctx, auditsql.InsertAuditEntryParams{
		ID:            e.ID,
		ActorType:     string(e.ActorType),
		ActorPersonID: text(e.ActorPersonID),
		Subsystem:     text(e.Subsystem),
		Action:        e.Action,
		TargetType:    e.TargetType,
		TargetID:      text(e.TargetID),
		UnitID:        text(e.UnitID),
		RequestID:     e.RequestID,
		Before:        e.Before,
		After:         e.After,
		Outcome:       string(e.Outcome),
	})
}

// Get reads one entry by its Action RID, translating pgx's no-rows into the domain sentinel.
func (r *Repository) Get(ctx context.Context, id string) (domain.Entry, error) {
	row, err := r.q.GetAuditEntry(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Entry{}, domain.ErrNotFound
		}
		return domain.Entry{}, err
	}
	return toEntry(row), nil
}

// Query runs the filtered, keyset-paginated read.
func (r *Repository) Query(ctx context.Context, f domain.Filter) ([]domain.Entry, error) {
	params := auditsql.QueryAuditLogParams{
		ActorPersonID: textPtr(f.ActorPersonID),
		ActorType:     textStringer(f.ActorType),
		TargetType:    textPtr(f.TargetType),
		TargetID:      textPtr(f.TargetID),
		UnitID:        textPtr(f.UnitID),
		Action:        textPtr(f.Action),
		Outcome:       textStringer(f.Outcome),
		Since:         timestamptzPtr(f.Since),
		Until:         timestamptzPtr(f.Until),
		PageLimit:     int32(f.Limit),
	}
	if f.Cursor != nil {
		params.CursorID = text(f.Cursor.ID)
		params.CursorCreatedAt = timestamptz(f.Cursor.CreatedAt)
	}

	rows, err := r.q.QueryAuditLog(ctx, params)
	if err != nil {
		return nil, err
	}
	entries := make([]domain.Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, toEntry(row))
	}
	return entries, nil
}

func toEntry(row auditsql.OikumeneaAuditLog) domain.Entry {
	return domain.Entry{
		ID:            row.ID,
		CreatedAt:     row.CreatedAt.Time,
		ActorType:     domain.ActorType(row.ActorType),
		ActorPersonID: row.ActorPersonID.String,
		Subsystem:     row.Subsystem.String,
		Action:        row.Action,
		TargetType:    row.TargetType,
		TargetID:      row.TargetID.String,
		UnitID:        row.UnitID.String,
		RequestID:     row.RequestID,
		Before:        row.Before,
		After:         row.After,
		Outcome:       domain.Outcome(row.Outcome),
	}
}

// text builds a pgtype.Text from a string ("" → SQL NULL).
func text(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// textPtr builds a pgtype.Text from an optional string filter (nil → SQL NULL → matches all).
func textPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// textStringer builds a pgtype.Text from an optional string-kinded filter (ActorType, Outcome).
func textStringer[T ~string](v *T) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*v), Valid: true}
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func timestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
