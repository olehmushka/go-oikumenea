// Package transport implements the rank module's generated Conjure RankService interface: it
// translates the wire contract to/from the application service, assembles localized `name` maps via
// the localization service (cross-module query — overview.md), and maps domain errors to Conjure
// SerializableErrors (D-Conjure). Generated code in internal/conjure is never hand-edited.
//
// Authorization is intentionally not yet enforced here: the endpoints declare `auth: header` so the
// bearer token is parsed, but the `rank.scheme.read` / `rank.scheme.manage` checks land once
// authorization (M7) + identity-federation (M8) do. The handlers receive the token and ignore it.
package transport

import (
	"context"
	"errors"

	rankapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/rank"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/rank/application"
	"github.com/olegamysk/go-oikumenea/internal/rank/domain"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// i18n entity types the localized `name` maps are stored under (mirror the audit target types).
const (
	entityCategory = "rank_category"
	entityType     = "rank_type"
	entityRank     = "rank"
)

// Service adapts *application.Service to the generated rankapi.RankService interface. It holds the
// localization service to assemble the `locale -> text` display-name maps responses return.
type Service struct {
	app *application.Service
	loc *locapp.Service
}

// NewService builds the transport adapter over the rank application service and the localization
// service (for name-map assembly).
func NewService(app *application.Service, loc *locapp.Service) Service {
	return Service{app: app, loc: loc}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ rankapi.RankService = Service{}

// ---------------------------------------------------------------- scheme read

func (s Service) GetRankScheme(ctx context.Context, _ bearertoken.Token) (rankapi.RankScheme, error) {
	scheme, err := s.app.GetScheme(ctx)
	if err != nil {
		return rankapi.RankScheme{}, s.mapError(ctx, err, errCtx{})
	}

	// Batch the localized name maps for each level, then weave the tree.
	catNames, err := s.namesFor(ctx, entityCategory, categoryDefaults(scheme.Categories))
	if err != nil {
		return rankapi.RankScheme{}, s.mapError(ctx, err, errCtx{})
	}
	typeNames, err := s.namesFor(ctx, entityType, typeDefaults(scheme.Types))
	if err != nil {
		return rankapi.RankScheme{}, s.mapError(ctx, err, errCtx{})
	}
	rankNames, err := s.namesFor(ctx, entityRank, rankDefaults(scheme.Ranks))
	if err != nil {
		return rankapi.RankScheme{}, s.mapError(ctx, err, errCtx{})
	}

	ranksByType := make(map[string][]rankapi.Rank)
	for _, r := range scheme.Ranks {
		ranksByType[r.TypeID] = append(ranksByType[r.TypeID], toAPIRank(r, rankNames[r.ID]))
	}
	typesByCategory := make(map[string][]rankapi.RankType)
	for _, t := range scheme.Types {
		typesByCategory[t.CategoryID] = append(typesByCategory[t.CategoryID], rankapi.RankType{
			Id: t.ID, Code: t.Code, Name: typeNames[t.ID], SortOrder: t.SortOrder,
			CategoryId: t.CategoryID, Ranks: ranksByType[t.ID],
		})
	}
	categories := make([]rankapi.RankCategory, 0, len(scheme.Categories))
	for _, c := range scheme.Categories {
		categories = append(categories, rankapi.RankCategory{
			Id: c.ID, Code: c.Code, Name: catNames[c.ID], SortOrder: c.SortOrder,
			Types: typesByCategory[c.ID],
		})
	}
	return rankapi.RankScheme{Categories: categories}, nil
}

// ---------------------------------------------------------------- categories

func (s Service) AddCategory(ctx context.Context, _ bearertoken.Token, req rankapi.AddCategoryRequest) (rankapi.RankCategory, error) {
	created, err := s.app.AddCategory(ctx, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return rankapi.RankCategory{}, s.mapError(ctx, err, errCtx{level: string(domain.LevelCategory), code: req.Code})
	}
	return s.categoryToAPI(ctx, created)
}

func (s Service) UpdateCategory(ctx context.Context, _ bearertoken.Token, categoryID string, req rankapi.UpdateCategoryRequest) (rankapi.RankCategory, error) {
	updated, err := s.app.UpdateCategory(ctx, categoryID, domain.CategoryPatch{Name: req.Name, SortOrder: req.SortOrder})
	if err != nil {
		return rankapi.RankCategory{}, s.mapError(ctx, err, errCtx{categoryID: categoryID})
	}
	return s.categoryToAPI(ctx, updated)
}

// ---------------------------------------------------------------- types

func (s Service) AddType(ctx context.Context, _ bearertoken.Token, req rankapi.AddTypeRequest) (rankapi.RankType, error) {
	created, err := s.app.AddType(ctx, req.CategoryId, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return rankapi.RankType{}, s.mapError(ctx, err, errCtx{
			level: string(domain.LevelType), code: req.Code, categoryID: req.CategoryId,
		})
	}
	return s.typeToAPI(ctx, created)
}

func (s Service) UpdateType(ctx context.Context, _ bearertoken.Token, typeID string, req rankapi.UpdateTypeRequest) (rankapi.RankType, error) {
	updated, err := s.app.UpdateType(ctx, typeID, domain.TypePatch{Name: req.Name, SortOrder: req.SortOrder})
	if err != nil {
		return rankapi.RankType{}, s.mapError(ctx, err, errCtx{typeID: typeID})
	}
	return s.typeToAPI(ctx, updated)
}

// ---------------------------------------------------------------- ranks

func (s Service) AddRank(ctx context.Context, _ bearertoken.Token, req rankapi.AddRankRequest) (rankapi.Rank, error) {
	created, err := s.app.AddRank(ctx, req.TypeId, req.Code, req.Name, req.Abbreviation, req.SortOrder)
	if err != nil {
		return rankapi.Rank{}, s.mapError(ctx, err, errCtx{
			level: string(domain.LevelRank), code: req.Code, typeID: req.TypeId,
		})
	}
	return s.rankToAPI(ctx, created)
}

func (s Service) UpdateRank(ctx context.Context, _ bearertoken.Token, rankID string, req rankapi.UpdateRankRequest) (rankapi.Rank, error) {
	updated, err := s.app.UpdateRank(ctx, rankID, domain.RankPatch{
		Name: req.Name, Abbreviation: req.Abbreviation, SortOrder: req.SortOrder,
	})
	if err != nil {
		return rankapi.Rank{}, s.mapError(ctx, err, errCtx{rankID: rankID})
	}
	return s.rankToAPI(ctx, updated)
}

// ---------------------------------------------------------------- delete

func (s Service) DeleteNode(ctx context.Context, _ bearertoken.Token, level string, nodeID string) error {
	lvl := domain.Level(level)
	if !domain.ValidLevel(lvl) {
		return rankapi.NewRankInvalid("unknown level " + level + "; want one of category|type|rank")
	}
	if err := s.app.DeleteNode(ctx, lvl, nodeID); err != nil {
		return s.mapError(ctx, err, errCtx{
			level: level, categoryID: nodeID, typeID: nodeID, rankID: nodeID,
		})
	}
	return nil
}

// ---------------------------------------------------------------- response assembly

func (s Service) categoryToAPI(ctx context.Context, c domain.Category) (rankapi.RankCategory, error) {
	names, err := s.namesFor(ctx, entityCategory, map[string]string{c.ID: c.Name})
	if err != nil {
		return rankapi.RankCategory{}, s.mapError(ctx, err, errCtx{})
	}
	return rankapi.RankCategory{Id: c.ID, Code: c.Code, Name: names[c.ID], SortOrder: c.SortOrder}, nil
}

func (s Service) typeToAPI(ctx context.Context, t domain.Type) (rankapi.RankType, error) {
	names, err := s.namesFor(ctx, entityType, map[string]string{t.ID: t.Name})
	if err != nil {
		return rankapi.RankType{}, s.mapError(ctx, err, errCtx{})
	}
	return rankapi.RankType{
		Id: t.ID, Code: t.Code, Name: names[t.ID], SortOrder: t.SortOrder, CategoryId: t.CategoryID,
	}, nil
}

func (s Service) rankToAPI(ctx context.Context, r domain.Rank) (rankapi.Rank, error) {
	names, err := s.namesFor(ctx, entityRank, map[string]string{r.ID: r.Name})
	if err != nil {
		return rankapi.Rank{}, s.mapError(ctx, err, errCtx{})
	}
	return toAPIRank(r, names[r.ID]), nil
}

// namesFor assembles the locale->text name maps for a set of entities of one kind (D-i18n: all
// enabled locales, no negotiation), seeded from each entity's default-locale `name` column.
func (s Service) namesFor(ctx context.Context, entityType string, defaults map[string]string) (map[string]map[string]string, error) {
	return s.loc.NamesByID(ctx, entityType, defaults)
}

func toAPIRank(r domain.Rank, name map[string]string) rankapi.Rank {
	return rankapi.Rank{
		Id: r.ID, Code: r.Code, Name: name, Abbreviation: strPtrOrNil(r.Abbreviation),
		SortOrder: r.SortOrder, TypeId: r.TypeID,
	}
}

func categoryDefaults(cs []domain.Category) map[string]string {
	m := make(map[string]string, len(cs))
	for _, c := range cs {
		m[c.ID] = c.Name
	}
	return m
}

func typeDefaults(ts []domain.Type) map[string]string {
	m := make(map[string]string, len(ts))
	for _, t := range ts {
		m[t.ID] = t.Name
	}
	return m
}

func rankDefaults(rs []domain.Rank) map[string]string {
	m := make(map[string]string, len(rs))
	for _, r := range rs {
		m[r.ID] = r.Name
	}
	return m
}

// ---------------------------------------------------------------- error mapping

// errCtx carries the identifiers an endpoint can name in a Conjure error (only the relevant fields
// are set per call).
type errCtx struct {
	level      string
	code       string
	categoryID string
	typeID     string
	rankID     string
}

// mapError translates domain/application errors into the Conjure SerializableError contract.
func (s Service) mapError(ctx context.Context, err error, c errCtx) error {
	switch {
	case errors.Is(err, domain.ErrCategoryNotFound):
		return rankapi.NewRankCategoryNotFound(c.categoryID)
	case errors.Is(err, domain.ErrTypeNotFound):
		return rankapi.NewRankTypeNotFound(c.typeID)
	case errors.Is(err, domain.ErrRankNotFound):
		return rankapi.NewRankNotFound(c.rankID)
	case errors.Is(err, domain.ErrCodeConflict):
		return rankapi.NewRankCodeConflict(c.level, c.code)
	case errors.Is(err, domain.ErrInUse):
		return rankapi.NewRankInUse(err.Error())
	case errors.Is(err, domain.ErrInvalid):
		return rankapi.NewRankInvalid(err.Error())
	default:
		return werror.WrapWithContextParams(ctx, err, "rank request failed")
	}
}

// ---------------------------------------------------------------- value helpers

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
