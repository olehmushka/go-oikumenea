// Package domain holds the rank module's pure logic: the three-level scheme (category -> type ->
// rank), its containment invariants, and the Repository port it needs from the outside world
// (overview.md layering). No I/O, no framework imports — only the standard library. Rank owns the
// single, system-wide seniority scheme (L-OneRankScheme / D-Rank); a rank is a DIRECTORY attribute
// and never an authorization input — the PDP never reads it.
package domain

import (
	"context"
	"errors"
	"strings"
)

// Sentinel errors mapped to Conjure SerializableErrors by the transport layer. The DB constraints
// (partial-unique codes, RESTRICT FKs) enforce the same shapes as a backstop.
var (
	ErrCategoryNotFound = errors.New("rank category not found")
	ErrTypeNotFound     = errors.New("rank type not found")
	ErrRankNotFound     = errors.New("rank not found")
	ErrCodeConflict     = errors.New("rank scheme code already exists")
	ErrInvalid          = errors.New("invalid rank scheme node")
	ErrInUse            = errors.New("rank scheme node is in use")
)

// Level names a tier of the scheme (used by the polymorphic delete endpoint).
type Level string

const (
	LevelCategory Level = "category"
	LevelType     Level = "type"
	LevelRank     Level = "rank"
)

// ValidLevel reports whether l is one of the three known scheme levels.
func ValidLevel(l Level) bool {
	return l == LevelCategory || l == LevelType || l == LevelRank
}

// Category is the top level of the scheme, ordered. Name is the default-locale text (the i18n store
// holds the other locales; the transport assembles the response map).
type Category struct {
	ID        string
	Code      string
	Name      string
	SortOrder int
}

// Type is a band within a category, ordered.
type Type struct {
	ID         string
	Code       string
	Name       string
	SortOrder  int
	CategoryID string
}

// Rank is a specific grade within a type, ordered for exact seniority. Abbreviation is "" when unset.
type Rank struct {
	ID           string
	Code         string
	Name         string
	Abbreviation string
	SortOrder    int
	TypeID       string
}

// CategoryPatch / TypePatch / RankPatch are partial updates: a nil field leaves the stored value
// unchanged. `code` is immutable by convention and not patchable.
type (
	CategoryPatch struct {
		Name      *string
		SortOrder *int
	}
	TypePatch struct {
		Name      *string
		SortOrder *int
	}
	RankPatch struct {
		Name         *string
		Abbreviation *string
		SortOrder    *int
	}
)

// Validate enforces the node invariants before insert: a non-empty, whitespace-free code and a
// non-empty name.
func (c Category) Validate() error { return validateNode(c.Code, c.Name) }

// Validate also requires the owning category id.
func (t Type) Validate() error {
	if strings.TrimSpace(t.CategoryID) == "" {
		return wrapInvalid("categoryId is required")
	}
	return validateNode(t.Code, t.Name)
}

// Validate also requires the owning type id.
func (r Rank) Validate() error {
	if strings.TrimSpace(r.TypeID) == "" {
		return wrapInvalid("typeId is required")
	}
	return validateNode(r.Code, r.Name)
}

func validateNode(code, name string) error {
	if !validCode(code) {
		return wrapInvalid("code must be non-empty and contain no whitespace")
	}
	if strings.TrimSpace(name) == "" {
		return wrapInvalid("name is required")
	}
	return nil
}

func wrapInvalid(msg string) error { return errors.Join(ErrInvalid, errors.New(msg)) }

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

// Repository is the persistence port the rank application service depends on; the pgx/sqlc adapter
// implements it. Each method runs on whatever DBTX the adapter was constructed with, so a write and
// its audit row share one transaction (D-Audit). A nil sortOrder on insert appends the node last
// (max active sibling order + 1).
type Repository interface {
	// categories
	InsertCategory(ctx context.Context, code, name string, sortOrder *int) (Category, error)
	GetCategory(ctx context.Context, id string) (Category, error)
	UpdateCategory(ctx context.Context, id string, patch CategoryPatch) (Category, error)
	SoftDeleteCategory(ctx context.Context, id string) error
	ListCategories(ctx context.Context) ([]Category, error)
	CountActiveTypes(ctx context.Context, categoryID string) (int, error)

	// types
	InsertType(ctx context.Context, categoryID, code, name string, sortOrder *int) (Type, error)
	GetType(ctx context.Context, id string) (Type, error)
	UpdateType(ctx context.Context, id string, patch TypePatch) (Type, error)
	SoftDeleteType(ctx context.Context, id string) error
	ListTypes(ctx context.Context) ([]Type, error)
	CountActiveRanks(ctx context.Context, typeID string) (int, error)

	// ranks
	InsertRank(ctx context.Context, typeID, code, name string, abbreviation *string, sortOrder *int) (Rank, error)
	GetRank(ctx context.Context, id string) (Rank, error)
	UpdateRank(ctx context.Context, id string, patch RankPatch) (Rank, error)
	SoftDeleteRank(ctx context.Context, id string) error
	ListRanks(ctx context.Context) ([]Rank, error)
}
