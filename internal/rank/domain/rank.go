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
	ErrSystemNotFound   = errors.New("rank system not found")
	ErrCategoryNotFound = errors.New("rank category not found")
	ErrTypeNotFound     = errors.New("rank type not found")
	ErrRankNotFound     = errors.New("rank not found")
	ErrGradeNotFound    = errors.New("standardized grade not found")
	ErrCodeConflict     = errors.New("rank scheme code already exists")
	ErrInvalid          = errors.New("invalid rank scheme node")
	ErrInUse            = errors.New("rank scheme node is in use")
)

// Level names a tier of the scheme (used by the polymorphic delete endpoint).
type Level string

const (
	LevelSystem   Level = "system"
	LevelCategory Level = "category"
	LevelType     Level = "type"
	LevelRank     Level = "rank"
)

// ValidLevel reports whether l is one of the known scheme levels.
func ValidLevel(l Level) bool {
	return l == LevelSystem || l == LevelCategory || l == LevelType || l == LevelRank
}

// Tier groups standardized grades for cross-system seniority (D-RankSystems). Enlisted is junior to
// warrant, which is junior to officer; within a tier, Grade.Ordinal orders junior -> senior.
type Tier string

const (
	TierEnlisted Tier = "enlisted"
	TierWarrant  Tier = "warrant"
	TierOfficer  Tier = "officer"
)

// tierRank maps a tier to its cross-tier order (higher = more senior). An unknown tier sorts lowest.
func tierRank(t Tier) int {
	switch t {
	case TierEnlisted:
		return 0
	case TierWarrant:
		return 1
	case TierOfficer:
		return 2
	default:
		return -1
	}
}

// System is the top level of the scheme (D-RankSystems): a national/organizational rank ladder. One
// scheme may hold several at once (a coalition directory). Country is the ISO-3166 national origin, ""
// for a supranational system (NATO/UN). Name is the default-locale text (the i18n store holds the
// others).
type System struct {
	ID        string
	Code      string
	Name      string
	SortOrder int
	Country   string
}

// Grade is a standardized cross-system comparability node (NATO STANAG 2116): the seeded reference
// catalog two ranks compare through. Equivalence = same Code; seniority = Tier then Ordinal.
type Grade struct {
	Code    string
	Tier    Tier
	Ordinal int
	Name    string
}

// Category is a branch within a system, ordered. SystemID is the owning system. Name is the
// default-locale text (the i18n store holds the other locales; the transport assembles the map).
type Category struct {
	ID        string
	Code      string
	Name      string
	SortOrder int
	SystemID  string
}

// Type is a band within a category, ordered, forming a tree: ParentTypeID is the owning parent type
// ("" = a root type of CategoryID). Ranks attach to leaf types only. CategoryID and SystemID are
// carried on every type (denormalized) so a nested type's CategoryID/SystemID equal its parent's.
type Type struct {
	ID           string
	Code         string
	Name         string
	SortOrder    int
	SystemID     string
	CategoryID   string
	ParentTypeID string
}

// Rank is a specific grade within a type, ordered for exact seniority. Abbreviation is "" when unset.
// GradeCode is "" when the rank carries no standardized cross-system grade. SystemID is denormalized
// from the owning type.
type Rank struct {
	ID           string
	Code         string
	Name         string
	Abbreviation string
	GradeCode    string
	SortOrder    int
	SystemID     string
	TypeID       string
}

// CategoryPatch / TypePatch / RankPatch are partial updates: a nil field leaves the stored value
// unchanged. `code` is immutable by convention and not patchable.
type (
	SystemPatch struct {
		Name      *string
		SortOrder *int
		Country   *string
	}
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
		GradeCode    *string // set if non-nil; cannot be cleared via patch (open seam)
		SortOrder    *int
	}
)

// Validate enforces the node invariants before insert: a non-empty, whitespace-free code and a
// non-empty name.
func (s System) Validate() error { return validateNode(s.Code, s.Name) }

// Validate enforces the node invariants before insert: a non-empty, whitespace-free code and a
// non-empty name.
func (c Category) Validate() error { return validateNode(c.Code, c.Name) }

// Validate also requires the type to be rooted somewhere: under a parent category (a root type) or
// under a parent type (a nested type). The application derives the effective category from the parent
// when nesting.
func (t Type) Validate() error {
	if strings.TrimSpace(t.CategoryID) == "" && strings.TrimSpace(t.ParentTypeID) == "" {
		return wrapInvalid("a type needs a parent category or a parent type")
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

// IsSenior compares two ranks ACROSS systems via their standardized grades (D-RankSystems). It reports
// whether a is strictly senior to b, and whether the comparison is known at all. When either grade is
// the zero value (a rank with no grade_code) the pair is incomparable across systems and known is
// false — the caller must never treat a false `senior` as "junior or equal" without checking known.
// Equivalent grades (same tier+ordinal) return (false, true). Intra-system seniority is the structural
// sort order and is NOT computed here.
func IsSenior(a, b Grade) (senior bool, known bool) {
	if a == (Grade{}) || b == (Grade{}) {
		return false, false
	}
	ra, rb := tierRank(a.Tier), tierRank(b.Tier)
	if ra < 0 || rb < 0 {
		return false, false
	}
	if ra != rb {
		return ra > rb, true
	}
	return a.Ordinal > b.Ordinal, true
}

// Equivalent reports whether two grades denote the same standardized grade (the cross-system
// equivalence relation, D-RankSystems): a non-zero grade equal by code. Two ungraded ranks are NOT
// equivalent (an absent grade carries no cross-system meaning).
func Equivalent(a, b Grade) bool {
	return a != (Grade{}) && a.Code == b.Code
}

// Repository is the persistence port the rank application service depends on; the pgx/sqlc adapter
// implements it. Each method runs on whatever DBTX the adapter was constructed with, so a write and
// its audit row share one transaction (D-Audit). A nil sortOrder on insert appends the node last
// (max active sibling order + 1).
type Repository interface {
	// systems
	InsertSystem(ctx context.Context, code, name string, sortOrder *int, country *string) (System, error)
	GetSystem(ctx context.Context, id string) (System, error)
	GetSystemByCode(ctx context.Context, code string) (System, error)
	UpdateSystem(ctx context.Context, id string, patch SystemPatch) (System, error)
	SoftDeleteSystem(ctx context.Context, id string) error
	ListSystems(ctx context.Context) ([]System, error)
	CountActiveCategories(ctx context.Context, systemID string) (int, error)

	// grades — the seeded standardized-grade reference catalog (read-only).
	ListGrades(ctx context.Context) ([]Grade, error)
	GetGradeByCode(ctx context.Context, code string) (Grade, error)

	// categories
	InsertCategory(ctx context.Context, systemID, code, name string, sortOrder *int) (Category, error)
	GetCategory(ctx context.Context, id string) (Category, error)
	GetCategoryByCode(ctx context.Context, systemID, code string) (Category, error)
	UpdateCategory(ctx context.Context, id string, patch CategoryPatch) (Category, error)
	SoftDeleteCategory(ctx context.Context, id string) error
	ListCategories(ctx context.Context) ([]Category, error)
	CountActiveTypes(ctx context.Context, categoryID string) (int, error)

	// types — parentTypeID nil = a root type of categoryID; otherwise the type nests under it.
	// system_id is derived from the category by the adapter (denormalized).
	InsertType(ctx context.Context, categoryID string, parentTypeID *string, code, name string, sortOrder *int) (Type, error)
	GetType(ctx context.Context, id string) (Type, error)
	GetTypeByCode(ctx context.Context, categoryID string, parentTypeID *string, code string) (Type, error)
	UpdateType(ctx context.Context, id string, patch TypePatch) (Type, error)
	SoftDeleteType(ctx context.Context, id string) error
	ListTypes(ctx context.Context) ([]Type, error)
	CountActiveRanks(ctx context.Context, typeID string) (int, error)
	CountActiveChildTypes(ctx context.Context, typeID string) (int, error)

	// ranks — system_id is derived from the type by the adapter (denormalized); gradeCode nil = none.
	InsertRank(ctx context.Context, typeID, code, name string, abbreviation, gradeCode *string, sortOrder *int) (Rank, error)
	GetRank(ctx context.Context, id string) (Rank, error)
	GetRankByCode(ctx context.Context, typeID, code string) (Rank, error)
	UpdateRank(ctx context.Context, id string, patch RankPatch) (Rank, error)
	SoftDeleteRank(ctx context.Context, id string) error
	ListRanks(ctx context.Context) ([]Rank, error)
}
