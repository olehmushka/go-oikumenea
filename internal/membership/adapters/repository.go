// Package adapters implements the membership domain ports against infrastructure: the pgx/sqlc
// repository over the oikumenea.membership_* tables. It depends on the database, never the reverse
// (overview.md). Generated sqlc code lives in the membershipsql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/membership/adapters/membershipsql"
	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX —
// the pool for reads, or a caller-supplied transaction so a write and its audit row commit together
// (D-Audit).
type Repository struct {
	q *membershipsql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: membershipsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- positions

func (r *Repository) InsertPosition(ctx context.Context, p domain.Position) (domain.Position, error) {
	var sortOrder interface{}
	if p.SortOrder != nil {
		sortOrder = int32(*p.SortOrder)
	}
	row, err := r.q.InsertPosition(ctx, membershipsql.InsertPositionParams{
		UnitID:         p.UnitID,
		Code:           p.Code,
		Title:          p.Title,
		RequiredRankID: text(p.RequiredRankID),
		SortOrder:      sortOrder,
	})
	if err != nil {
		return domain.Position{}, mapWriteErr(err)
	}
	return toPosition(row), nil
}

func (r *Repository) GetPosition(ctx context.Context, id string) (domain.Position, error) {
	row, err := r.q.GetPosition(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Position{}, domain.ErrPositionNotFound
		}
		return domain.Position{}, err
	}
	return toPosition(row), nil
}

func (r *Repository) UpdatePosition(ctx context.Context, id string, patch domain.PositionPatch) (domain.Position, error) {
	row, err := r.q.UpdatePosition(ctx, membershipsql.UpdatePositionParams{
		Title:          textPtr(patch.Title),
		RequiredRankID: textPtr(patch.RequiredRankID),
		SortOrder:      int4Ptr(patch.SortOrder),
		ID:             id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Position{}, domain.ErrPositionNotFound
		}
		return domain.Position{}, mapWriteErr(err)
	}
	return toPosition(row), nil
}

func (r *Repository) AbolishPosition(ctx context.Context, id string) (domain.Position, error) {
	row, err := r.q.AbolishPosition(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Position{}, domain.ErrPositionNotFound
		}
		return domain.Position{}, err
	}
	return toPosition(row), nil
}

func (r *Repository) ListPositions(ctx context.Context, unitID string, filter domain.PositionFilter, after string, limit int) ([]domain.Position, error) {
	switch filter {
	case domain.FilterVacant:
		rows, err := r.q.ListVacantPositionsByUnit(ctx, membershipsql.ListVacantPositionsByUnitParams{UnitID: unitID, After: after, Lim: int32(limit)})
		if err != nil {
			return nil, err
		}
		return positionsFrom(rows), nil
	case domain.FilterFilled:
		rows, err := r.q.ListFilledPositionsByUnit(ctx, membershipsql.ListFilledPositionsByUnitParams{UnitID: unitID, After: after, Lim: int32(limit)})
		if err != nil {
			return nil, err
		}
		return positionsFrom(rows), nil
	default:
		rows, err := r.q.ListPositionsByUnit(ctx, membershipsql.ListPositionsByUnitParams{UnitID: unitID, After: after, Lim: int32(limit)})
		if err != nil {
			return nil, err
		}
		return positionsFrom(rows), nil
	}
}

// ---------------------------------------------------------------- memberships

func (r *Repository) InsertMembership(ctx context.Context, m domain.Membership) (domain.Membership, error) {
	row, err := r.q.InsertMembership(ctx, membershipsql.InsertMembershipParams{
		PersonID:      m.PersonID,
		UnitID:        m.UnitID,
		PositionID:    text(m.PositionID),
		OrderItemID:   text(m.OrderItemID),
		EffectiveFrom: tsArg(m.EffectiveFrom),
	})
	if err != nil {
		return domain.Membership{}, mapWriteErr(err)
	}
	return toMembership(row), nil
}

func (r *Repository) GetMembership(ctx context.Context, id string) (domain.Membership, error) {
	row, err := r.q.GetMembership(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Membership{}, domain.ErrMembershipNotFound
		}
		return domain.Membership{}, err
	}
	return toMembership(row), nil
}

func (r *Repository) EndMembership(ctx context.Context, id string, effectiveTo time.Time, orderItemID *string) (domain.Membership, error) {
	row, err := r.q.EndMembership(ctx, membershipsql.EndMembershipParams{
		EffectiveTo: pgtype.Timestamptz{Time: effectiveTo, Valid: true},
		OrderItemID: textPtr(orderItemID),
		ID:          id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Membership{}, domain.ErrMembershipNotFound
		}
		return domain.Membership{}, err
	}
	return toMembership(row), nil
}

func (r *Repository) ActiveFillingByPosition(ctx context.Context, positionID string) (domain.Membership, error) {
	row, err := r.q.GetActiveFillingByPosition(ctx, text(positionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Membership{}, domain.ErrMembershipNotFound
		}
		return domain.Membership{}, err
	}
	return toMembership(row), nil
}

func (r *Repository) ListMembersByUnit(ctx context.Context, unitID, after string, limit int) ([]domain.Membership, error) {
	rows, err := r.q.ListMembersByUnit(ctx, membershipsql.ListMembersByUnitParams{UnitID: unitID, After: after, Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	return membershipsFrom(rows), nil
}

func (r *Repository) ListMembershipsByPerson(ctx context.Context, personID, after string, limit int) ([]domain.Membership, error) {
	rows, err := r.q.ListMembershipsByPerson(ctx, membershipsql.ListMembershipsByPersonParams{PersonID: personID, After: after, Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	return membershipsFrom(rows), nil
}

// ---------------------------------------------------------------- mapping helpers

func toPosition(r membershipsql.OikumeneaMembershipPosition) domain.Position {
	return domain.Position{
		ID:             r.ID,
		UnitID:         r.UnitID,
		Code:           r.Code,
		Title:          r.Title,
		RequiredRankID: r.RequiredRankID.String,
		Status:         domain.PositionStatus(r.Status),
		SortOrder:      int4Val(r.SortOrder),
		CreatedAt:      r.CreatedAt.Time,
		UpdatedAt:      r.UpdatedAt.Time,
	}
}

func positionsFrom(rows []membershipsql.OikumeneaMembershipPosition) []domain.Position {
	out := make([]domain.Position, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPosition(row))
	}
	return out
}

func toMembership(r membershipsql.OikumeneaMembershipMembership) domain.Membership {
	return domain.Membership{
		ID:            r.ID,
		PersonID:      r.PersonID,
		UnitID:        r.UnitID,
		PositionID:    r.PositionID.String,
		OrderItemID:   r.OrderItemID.String,
		Status:        domain.MembershipStatus(r.Status),
		EffectiveFrom: r.EffectiveFrom.Time,
		EffectiveTo:   tsPtr(r.EffectiveTo),
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
}

func membershipsFrom(rows []membershipsql.OikumeneaMembershipMembership) []domain.Membership {
	out := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMembership(row))
	}
	return out
}

// mapWriteErr translates Postgres constraint violations into the module's domain sentinels. The
// partial-unique indexes distinguish the one-holder violation (a position already filled) from the
// plain-belonging duplicate and the per-unit code clash; FK violations name the offending reference
// (unit / person / position / rank) so the transport can return a precise error.
func mapWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	name := pgErr.ConstraintName
	switch pgErr.Code {
	case "23505": // unique_violation
		switch {
		case strings.Contains(name, "one_holder"):
			return domain.ErrPositionAlreadyFilled
		case strings.Contains(name, "belonging"):
			return domain.ErrMembershipConflict
		case strings.Contains(name, "code"):
			return domain.ErrPositionCodeConflict
		}
	case "23503": // foreign_key_violation
		switch {
		case strings.Contains(name, "position"):
			return domain.ErrUnknownPosition
		case strings.Contains(name, "person"):
			return domain.ErrUnknownPerson
		case strings.Contains(name, "unit"):
			return domain.ErrUnknownUnit
		case strings.Contains(name, "rank"):
			return domain.ErrUnknownRank
		}
	}
	return err
}

func text(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// textPtr maps a patch pointer: nil leaves the column unchanged (NULL narg → COALESCE keeps it); a
// non-nil pointer sets the column.
func textPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func int4Ptr(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

func int4Val(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int32)
	return &n
}

// tsArg maps an optional effective-from instant: the zero time means "omitted" so the query's
// COALESCE defaults it to now().
func tsArg(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	out := t.Time
	return &out
}
