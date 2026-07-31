// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package domain holds the membership module's pure logic: the unit-owned billet (Position), the
// reified person->unit belonging/filling Link (Membership), and the Repository port it needs from
// the outside world (overview.md layering). No I/O, no framework imports — only the standard library.
//
// A position EXISTS while vacant (D-Position): a vacancy is an active position with no active
// filling. A membership is a person belonging to a unit, optionally filling a position; filling a
// billet is a membership that references it. Like rank, position is a DIRECTORY attribute and grants
// NO authority — this package never reads it to make a decision (D-Position / D-Rank). Visibility is
// not stored here; it derives from the owning unit (tenant.md), gated at read time once the PDP (M7)
// lands.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// Sentinel errors mapped to Conjure SerializableErrors by the transport layer. The DB constraints
// (partial-unique code / one-holder / belonging indexes, RESTRICT FKs) enforce the same shapes as a
// backstop.
var (
	ErrPositionNotFound     = errors.New("position not found")
	ErrPositionCodeConflict = errors.New("position code already exists in this unit")
	ErrPositionInUse        = errors.New("position has an active filling")
	ErrPositionInvalid      = errors.New("invalid position request")

	ErrMembershipNotFound    = errors.New("membership not found")
	ErrMembershipConflict    = errors.New("an active membership for this person and unit already exists")
	ErrPositionAlreadyFilled = errors.New("position already has an active filling")
	ErrMembershipInvalid     = errors.New("invalid membership request")
	ErrMembershipLifecycle   = errors.New("invalid membership lifecycle transition")

	// FK-validation sentinels: unknown references caught by the DB foreign keys and surfaced as
	// INVALID_ARGUMENT by the transport.
	ErrUnknownUnit     = errors.New("unit does not exist")
	ErrUnknownPerson   = errors.New("person does not exist")
	ErrUnknownRank     = errors.New("rank does not exist")
	ErrUnknownPosition = errors.New("position does not exist")
	// ErrPositionUnitMismatch: a filling cites a position that belongs to a different unit.
	ErrPositionUnitMismatch = errors.New("position does not belong to the unit")
)

// PositionStatus is the billet lifecycle state.
type PositionStatus string

const (
	PositionActive    PositionStatus = "active"
	PositionAbolished PositionStatus = "abolished"
)

// MembershipStatus is the belonging/filling lifecycle state.
type MembershipStatus string

const (
	MembershipActive MembershipStatus = "active"
	MembershipEnded  MembershipStatus = "ended"
)

// PositionFilter narrows a unit's position listing. Empty lists all active positions; Vacant lists
// active positions with no active filling; Filled lists active positions that have one.
type PositionFilter string

const (
	FilterAll    PositionFilter = ""
	FilterVacant PositionFilter = "vacant"
	FilterFilled PositionFilter = "filled"
)

// Position is a unit-owned billet (aggregate root). Title is the default-locale fallback; the
// transport assembles the locale->text map from the i18n store. RequiredRankID is "" when unset.
// Holder is populated only on a single-position read (the current active filling, if any).
type Position struct {
	ID             string
	UnitID         string
	Code           string
	Title          string // default-locale fallback; translatable via the i18n store
	RequiredRankID string // "" when unset
	Status         PositionStatus
	SortOrder      *int // nil when unset
	CreatedAt      time.Time
	UpdatedAt      time.Time

	Holder *Membership // populated by a single read; nil otherwise
}

// Validate enforces the create-time invariants: a valid code and a non-empty title. Unknown unit /
// rank are caught by the DB FKs and surfaced as ErrUnknownUnit / ErrUnknownRank.
func (p Position) Validate() error {
	if !validCode(p.Code) {
		return wrapInvalid(ErrPositionInvalid, "code must be non-empty, <=128 chars, and contain no whitespace")
	}
	if strings.TrimSpace(p.Title) == "" {
		return wrapInvalid(ErrPositionInvalid, "title is required")
	}
	return nil
}

// PositionPatch is a partial update (nil = unchanged). Code and unit are immutable by convention.
type PositionPatch struct {
	Title          *string
	RequiredRankID *string
	SortOrder      *int
}

// Validate enforces the patch invariants for the fields actually present.
func (p PositionPatch) Validate() error {
	if p.Title != nil && strings.TrimSpace(*p.Title) == "" {
		return wrapInvalid(ErrPositionInvalid, "title cannot be cleared")
	}
	return nil
}

// Membership is a person's belonging to a unit, optionally filling a position, effective-dated.
// PositionID/OrderItemID are "" when unset. EffectiveTo is nil while active.
type Membership struct {
	ID            string
	PersonID      string
	UnitID        string
	PositionID    string // "" for plain belonging
	OrderItemID   string // "" when not order-driven (provenance; D-Orders)
	Status        MembershipStatus
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Validate enforces a person and a unit reference. Unknown person/unit/position are caught by the DB
// FKs and surfaced as the matching ErrUnknown* sentinel.
func (m Membership) Validate() error {
	if strings.TrimSpace(m.PersonID) == "" {
		return wrapInvalid(ErrMembershipInvalid, "personId is required")
	}
	if strings.TrimSpace(m.UnitID) == "" {
		return wrapInvalid(ErrMembershipInvalid, "unitId is required")
	}
	return nil
}

// CanEnd reports whether the membership may be ended (only from active).
func (m Membership) CanEnd() bool { return m.Status == MembershipActive }

func wrapInvalid(base error, msg string) error { return errors.Join(base, errors.New(msg)) }

// validCode is the shared code shape guard: non-empty, <=128 chars, no whitespace (D-Code:
// operator-assigned, locale-agnostic, immutable by convention).
func validCode(code string) bool {
	if code == "" || len(code) > 128 {
		return false
	}
	return !strings.ContainsFunc(code, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// Repository is the persistence port the application service depends on; the pgx/sqlc adapter
// implements it. Each method runs on whatever DBTX the adapter was constructed with, so a write and
// its audit row share one transaction (D-Audit). Reads exclude soft-deleted rows.
type Repository interface {
	// positions
	InsertPosition(ctx context.Context, p Position) (Position, error)
	GetPosition(ctx context.Context, id string) (Position, error)
	UpdatePosition(ctx context.Context, id string, patch PositionPatch) (Position, error)
	AbolishPosition(ctx context.Context, id string) (Position, error)
	ListPositions(ctx context.Context, unitID string, filter PositionFilter, after string, limit int) ([]Position, error)

	// memberships
	InsertMembership(ctx context.Context, m Membership) (Membership, error)
	GetMembership(ctx context.Context, id string) (Membership, error)
	EndMembership(ctx context.Context, id string, effectiveTo time.Time, orderItemID *string) (Membership, error)
	// ActiveFillingByPosition returns the position's single active filling, or ErrMembershipNotFound
	// when the billet is vacant.
	ActiveFillingByPosition(ctx context.Context, positionID string) (Membership, error)
	// ActivePlainMembership returns a person's active plain belonging (no position) in a unit, or
	// ErrMembershipNotFound — the target an order's membership-end item ends when it names a unit but
	// no position.
	ActivePlainMembership(ctx context.Context, personID, unitID string) (Membership, error)
	ListMembersByUnit(ctx context.Context, unitID, after string, limit int) ([]Membership, error)
	ListMembershipsByPerson(ctx context.Context, personID, after string, limit int) ([]Membership, error)
	// ListMemberships and ListMembershipsForSubject are the two arms of the top-level facet-filtered
	// listing (M56 ticket 3 / D-ObjectFacets): the instance-admin view of every membership, and the
	// same filter set intersected with the subject's effective readable reach. Both return EVERY
	// status — the top-level list carries no implicit active-only default (see MembershipFilter).
	ListMemberships(ctx context.Context, f MembershipFilter, after string, limit int) ([]Membership, error)
	ListMembershipsForSubject(ctx context.Context, subjectPersonID string, f MembershipFilter, after string, limit int) ([]Membership, error)
	// MembershipStats is the dashboard aggregate over the SAME candidate set the two list arms page
	// (M57 / D-ObjectFacets): raw (facet, bucket, count) rows, assembled by pkg/stats. An empty
	// subjectPersonID is the instance-admin arm; otherwise the reach predicate is folded into the SQL,
	// because a count cannot be trimmed after the fact the way a page can.
	MembershipStats(ctx context.Context, subjectPersonID string, f MembershipFilter, sel stats.Selection) ([]stats.Group, error)
	// ReadableUnitIDsForSubject projects the subject's reach through the SQL function migration 0017
	// added. It exists for the differential test that asserts the function agrees with the inline CTE
	// in VisiblePersonIDsForSubject* — the parity claim 0017 makes.
	ReadableUnitIDsForSubject(ctx context.Context, subjectPersonID string) ([]string, error)
	// ActiveUnitIDsByPerson returns the distinct units a person currently belongs to via active
	// memberships (the person/document read-scope projection input; D-PersonReadScope).
	ActiveUnitIDsByPerson(ctx context.Context, personID string) ([]string, error)
	// ActivePersonIDsInUnits returns the distinct persons with an active membership in any of the
	// given units, keyset-paginated by person RID (the directory-list union; D-PersonReadScope).
	ActivePersonIDsInUnits(ctx context.Context, unitIDs []string, after string, limit int) ([]string, error)
	// VisiblePersonIDsForSubject returns the distinct persons with an active membership in any unit
	// of the SUBJECT's effective readable reach, computed as a SQL semi-join over the authz
	// assignments + tenant closure (D-PersonReadScope; review-2026-07 R-02.1 — the reach set never
	// leaves the database), narrowed by the caller's person filter.
	//
	// The filter carries both the optional case-insensitive query (the person's trigram-indexed
	// name/code haystack or any name variant) and the M56 facet set (D-ObjectFacets). ALL of it is
	// folded into the same SQL, so every predicate runs before the LIMIT and the page fills
	// correctly (review-2026-07 R-06). Keyset-paginated by person RID.
	//
	// The filter type is PERSON's: person owns the vocabulary over its own columns, membership owns
	// the reach predicate, and this seam is where the two meet.
	VisiblePersonIDsForSubject(ctx context.Context, subjectPersonID, after string, f persondomain.PersonFilter, limit int) ([]string, error)
	// VisiblePersonStatsForSubject is the DASHBOARD form of the same question (M57 / D-ObjectFacets):
	// the same filter and the same reach predicate, aggregated instead of paged. The counts are taken
	// inside the visibility predicate — a count over a set trimmed afterwards would describe rows the
	// caller may not read.
	VisiblePersonStatsForSubject(ctx context.Context, subjectPersonID string, f persondomain.PersonFilter, sel stats.Selection) ([]stats.Group, error)
	// SubjectCanReadPerson is the point probe of the same reach predicate: whether any of the
	// person's active-membership units falls in the subject's readable reach.
	SubjectCanReadPerson(ctx context.Context, subjectPersonID, personID string) (bool, error)
	// SubjectReadablePersonsAmong is the batch form of the same probe: the subset of candidate
	// persons in the subject's readable reach (unordered; D-VisibilityScope person scope, R-30).
	SubjectReadablePersonsAmong(ctx context.Context, subjectPersonID string, personIDs []string) ([]string, error)
}
