// Package transport implements the rank module's generated Conjure RankService interface: it
// translates the wire contract to/from the application service, assembles localized `name` maps via
// the localization service (cross-module query — overview.md), and maps domain errors to Conjure
// SerializableErrors (D-Conjure). Generated code in internal/conjure is never hand-edited.
//
// Authorization (M7): reading the scheme requires `rank.scheme.read` (held anywhere — the single
// system-wide scheme is instance-global, not unit-keyed); all writes require the instance-scope
// `rank.scheme.manage`, enforced via the PEP. The bearer token carries the acting subject (interim:
// token == person RID; see internal/authorization/pep).
package transport

import (
	"context"
	"errors"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	rankapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/rank"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/rank/application"
	"github.com/olegamysk/go-oikumenea/internal/rank/domain"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// i18n entity types the localized `name` maps are stored under (mirror the audit target types).
const (
	entitySystem   = "rank_system"
	entityCategory = "rank_category"
	entityType     = "rank_type"
	entityRank     = "rank"
)

// Service adapts *application.Service to the generated rankapi.RankService interface. It holds the
// localization service to assemble the `locale -> text` display-name maps responses return.
type Service struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the rank application service, the localization
// service (for name-map assembly), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, loc: loc, pep: enforcer}
}

// readPerm / managePerm are the rank scheme's read / instance-scope-write permission codes.
const (
	readPerm   = string(authzdomain.PermRankSchemeRead)
	managePerm = string(authzdomain.PermRankSchemeManage)
)

// compile-time assertion that the transport satisfies the generated server interface.
var _ rankapi.RankService = Service{}

// ---------------------------------------------------------------- scheme read

func (s Service) GetRankScheme(ctx context.Context, token bearertoken.Token) (rankapi.RankScheme, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return rankapi.RankScheme{}, err
	}
	scheme, err := s.app.GetScheme(ctx)
	if err != nil {
		return rankapi.RankScheme{}, s.mapError(ctx, err, errCtx{})
	}

	// Batch the localized name maps for each level, then weave the tree.
	sysNames, err := s.namesFor(ctx, entitySystem, systemDefaults(scheme.Systems))
	if err != nil {
		return rankapi.RankScheme{}, s.mapError(ctx, err, errCtx{})
	}
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

	// Weave the per-category type tree. scheme.Types is sorted by (category, sort_order, code), so the
	// child-id lists and the per-category roots below come out in seniority order; buildType recurses
	// so a node's children (and ranks, on leaves) are complete before it is attached to its parent.
	typeByID := make(map[string]domain.Type, len(scheme.Types))
	childIDs := make(map[string][]string)
	for _, t := range scheme.Types {
		typeByID[t.ID] = t
		if t.ParentTypeID != "" {
			childIDs[t.ParentTypeID] = append(childIDs[t.ParentTypeID], t.ID)
		}
	}
	var buildType func(t domain.Type) rankapi.RankType
	buildType = func(t domain.Type) rankapi.RankType {
		node := rankapi.RankType{
			Id: t.ID, Code: t.Code, Name: typeNames[t.ID], SortOrder: t.SortOrder,
			SystemId: t.SystemID, CategoryId: t.CategoryID, ParentTypeId: strPtrOrNil(t.ParentTypeID),
			Ranks: ranksByType[t.ID],
		}
		for _, cid := range childIDs[t.ID] {
			node.Children = append(node.Children, buildType(typeByID[cid]))
		}
		return node
	}
	typesByCategory := make(map[string][]rankapi.RankType)
	for _, t := range scheme.Types {
		if t.ParentTypeID == "" { // root types of the category
			typesByCategory[t.CategoryID] = append(typesByCategory[t.CategoryID], buildType(t))
		}
	}
	// Group categories under their system (scheme.Categories is sorted by sort_order, so each system's
	// list comes out in seniority order).
	categoriesBySystem := make(map[string][]rankapi.RankCategory)
	for _, c := range scheme.Categories {
		categoriesBySystem[c.SystemID] = append(categoriesBySystem[c.SystemID], rankapi.RankCategory{
			Id: c.ID, Code: c.Code, Name: catNames[c.ID], SortOrder: c.SortOrder,
			SystemId: c.SystemID, Types: typesByCategory[c.ID],
		})
	}
	systems := make([]rankapi.RankSystem, 0, len(scheme.Systems))
	for _, sys := range scheme.Systems {
		systems = append(systems, rankapi.RankSystem{
			Id: sys.ID, Code: sys.Code, Name: sysNames[sys.ID], SortOrder: sys.SortOrder,
			Country: strPtrOrNil(sys.Country), Categories: categoriesBySystem[sys.ID],
		})
	}
	return rankapi.RankScheme{Systems: systems}, nil
}

// ---------------------------------------------------------------- grades

func (s Service) GetRankGrades(ctx context.Context, token bearertoken.Token) ([]rankapi.RankGrade, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return nil, err
	}
	grades, err := s.app.GetGrades(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, errCtx{})
	}
	out := make([]rankapi.RankGrade, 0, len(grades))
	for _, g := range grades {
		out = append(out, rankapi.RankGrade{Code: g.Code, Tier: string(g.Tier), Ordinal: g.Ordinal, Name: g.Name})
	}
	return out, nil
}

// ---------------------------------------------------------------- systems

func (s Service) AddSystem(ctx context.Context, token bearertoken.Token, req rankapi.AddSystemRequest) (rankapi.RankSystem, error) {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return rankapi.RankSystem{}, err
	}
	created, err := s.app.AddSystem(ctx, req.Code, req.Name, req.SortOrder, req.Country)
	if err != nil {
		return rankapi.RankSystem{}, s.mapError(ctx, err, errCtx{level: string(domain.LevelSystem), code: req.Code})
	}
	return s.systemToAPI(ctx, created)
}

func (s Service) UpdateSystem(ctx context.Context, token bearertoken.Token, systemID string, req rankapi.UpdateSystemRequest) (rankapi.RankSystem, error) {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return rankapi.RankSystem{}, err
	}
	updated, err := s.app.UpdateSystem(ctx, systemID, domain.SystemPatch{Name: req.Name, SortOrder: req.SortOrder, Country: req.Country})
	if err != nil {
		return rankapi.RankSystem{}, s.mapError(ctx, err, errCtx{systemID: systemID})
	}
	return s.systemToAPI(ctx, updated)
}

// ---------------------------------------------------------------- import

func (s Service) ImportRankScheme(ctx context.Context, token bearertoken.Token, req rankapi.ImportRankSchemeRequest) (rankapi.ImportRankSchemeResponse, error) {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return rankapi.ImportRankSchemeResponse{}, err
	}
	sum, err := s.app.ImportPreset(ctx, toPreset(req))
	if err != nil {
		return rankapi.ImportRankSchemeResponse{}, s.mapError(ctx, err, errCtx{level: string(domain.LevelSystem), code: req.System.Code})
	}
	return rankapi.ImportRankSchemeResponse{Created: sum.Created, Updated: sum.Updated, Skipped: sum.Skipped}, nil
}

// ---------------------------------------------------------------- categories

func (s Service) AddCategory(ctx context.Context, token bearertoken.Token, req rankapi.AddCategoryRequest) (rankapi.RankCategory, error) {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return rankapi.RankCategory{}, err
	}
	created, err := s.app.AddCategory(ctx, req.SystemId, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return rankapi.RankCategory{}, s.mapError(ctx, err, errCtx{level: string(domain.LevelCategory), code: req.Code, systemID: req.SystemId})
	}
	return s.categoryToAPI(ctx, created)
}

func (s Service) UpdateCategory(ctx context.Context, token bearertoken.Token, categoryID string, req rankapi.UpdateCategoryRequest) (rankapi.RankCategory, error) {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return rankapi.RankCategory{}, err
	}
	updated, err := s.app.UpdateCategory(ctx, categoryID, domain.CategoryPatch{Name: req.Name, SortOrder: req.SortOrder})
	if err != nil {
		return rankapi.RankCategory{}, s.mapError(ctx, err, errCtx{categoryID: categoryID})
	}
	return s.categoryToAPI(ctx, updated)
}

// ---------------------------------------------------------------- types

func (s Service) AddType(ctx context.Context, token bearertoken.Token, req rankapi.AddTypeRequest) (rankapi.RankType, error) {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return rankapi.RankType{}, err
	}
	var categoryID, parentID string
	if req.CategoryId != nil {
		categoryID = *req.CategoryId
	}
	if req.ParentTypeId != nil {
		parentID = *req.ParentTypeId
	}
	created, err := s.app.AddType(ctx, categoryID, req.ParentTypeId, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return rankapi.RankType{}, s.mapError(ctx, err, errCtx{
			level: string(domain.LevelType), code: req.Code, categoryID: categoryID, typeID: parentID,
		})
	}
	return s.typeToAPI(ctx, created)
}

func (s Service) UpdateType(ctx context.Context, token bearertoken.Token, typeID string, req rankapi.UpdateTypeRequest) (rankapi.RankType, error) {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return rankapi.RankType{}, err
	}
	updated, err := s.app.UpdateType(ctx, typeID, domain.TypePatch{Name: req.Name, SortOrder: req.SortOrder})
	if err != nil {
		return rankapi.RankType{}, s.mapError(ctx, err, errCtx{typeID: typeID})
	}
	return s.typeToAPI(ctx, updated)
}

// ---------------------------------------------------------------- ranks

func (s Service) AddRank(ctx context.Context, token bearertoken.Token, req rankapi.AddRankRequest) (rankapi.Rank, error) {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return rankapi.Rank{}, err
	}
	created, err := s.app.AddRank(ctx, req.TypeId, req.Code, req.Name, req.Abbreviation, req.GradeCode, req.SortOrder)
	if err != nil {
		return rankapi.Rank{}, s.mapError(ctx, err, errCtx{
			level: string(domain.LevelRank), code: req.Code, typeID: req.TypeId,
		})
	}
	return s.rankToAPI(ctx, created)
}

func (s Service) UpdateRank(ctx context.Context, token bearertoken.Token, rankID string, req rankapi.UpdateRankRequest) (rankapi.Rank, error) {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return rankapi.Rank{}, err
	}
	updated, err := s.app.UpdateRank(ctx, rankID, domain.RankPatch{
		Name: req.Name, Abbreviation: req.Abbreviation, GradeCode: req.GradeCode, SortOrder: req.SortOrder,
	})
	if err != nil {
		return rankapi.Rank{}, s.mapError(ctx, err, errCtx{rankID: rankID})
	}
	return s.rankToAPI(ctx, updated)
}

// ---------------------------------------------------------------- delete

func (s Service) DeleteNode(ctx context.Context, token bearertoken.Token, level string, nodeID string) error {
	if err := s.pep.Require(ctx, token, managePerm, ""); err != nil {
		return err
	}
	lvl := domain.Level(level)
	if !domain.ValidLevel(lvl) {
		return rankapi.NewRankInvalid("unknown level " + level + "; want one of system|category|type|rank")
	}
	if err := s.app.DeleteNode(ctx, lvl, nodeID); err != nil {
		return s.mapError(ctx, err, errCtx{
			level: level, systemID: nodeID, categoryID: nodeID, typeID: nodeID, rankID: nodeID,
		})
	}
	return nil
}

// ---------------------------------------------------------------- response assembly

func (s Service) systemToAPI(ctx context.Context, sys domain.System) (rankapi.RankSystem, error) {
	names, err := s.namesFor(ctx, entitySystem, map[string]string{sys.ID: sys.Name})
	if err != nil {
		return rankapi.RankSystem{}, s.mapError(ctx, err, errCtx{})
	}
	return rankapi.RankSystem{
		Id: sys.ID, Code: sys.Code, Name: names[sys.ID], SortOrder: sys.SortOrder,
		Country: strPtrOrNil(sys.Country),
	}, nil
}

func (s Service) categoryToAPI(ctx context.Context, c domain.Category) (rankapi.RankCategory, error) {
	names, err := s.namesFor(ctx, entityCategory, map[string]string{c.ID: c.Name})
	if err != nil {
		return rankapi.RankCategory{}, s.mapError(ctx, err, errCtx{})
	}
	return rankapi.RankCategory{Id: c.ID, Code: c.Code, Name: names[c.ID], SortOrder: c.SortOrder, SystemId: c.SystemID}, nil
}

func (s Service) typeToAPI(ctx context.Context, t domain.Type) (rankapi.RankType, error) {
	names, err := s.namesFor(ctx, entityType, map[string]string{t.ID: t.Name})
	if err != nil {
		return rankapi.RankType{}, s.mapError(ctx, err, errCtx{})
	}
	return rankapi.RankType{
		Id: t.ID, Code: t.Code, Name: names[t.ID], SortOrder: t.SortOrder,
		SystemId: t.SystemID, CategoryId: t.CategoryID, ParentTypeId: strPtrOrNil(t.ParentTypeID),
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
		GradeCode: strPtrOrNil(r.GradeCode), SortOrder: r.SortOrder, SystemId: r.SystemID, TypeId: r.TypeID,
	}
}

func systemDefaults(ss []domain.System) map[string]string {
	m := make(map[string]string, len(ss))
	for _, sys := range ss {
		m[sys.ID] = sys.Name
	}
	return m
}

// PresetFromRequest maps a Conjure import request to the application preset tree (exported so tests can
// load the bundled deploy/rank-presets/*.json through the same path the endpoint uses).
func PresetFromRequest(req rankapi.ImportRankSchemeRequest) application.Preset { return toPreset(req) }

// toPreset maps the Conjure import request to the application-layer preset tree (D-RankSystems).
func toPreset(req rankapi.ImportRankSchemeRequest) application.Preset {
	sys := req.System
	out := application.PresetSystem{Code: sys.Code, Name: sys.Name, Country: sys.Country, SortOrder: sys.SortOrder}
	for _, c := range sys.Categories {
		pc := application.PresetCategory{Code: c.Code, Name: c.Name, SortOrder: c.SortOrder}
		for _, t := range c.Types {
			pc.Types = append(pc.Types, toPresetType(t))
		}
		out.Categories = append(out.Categories, pc)
	}
	return application.Preset{System: out}
}

func toPresetType(t rankapi.ImportType) application.PresetType {
	pt := application.PresetType{Code: t.Code, Name: t.Name, SortOrder: t.SortOrder}
	for _, child := range t.Children {
		pt.Children = append(pt.Children, toPresetType(child))
	}
	for _, r := range t.Ranks {
		pt.Ranks = append(pt.Ranks, application.PresetRank{
			Code: r.Code, Name: r.Name, Abbreviation: r.Abbreviation, GradeCode: r.GradeCode, SortOrder: r.SortOrder,
		})
	}
	return pt
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
	systemID   string
	categoryID string
	typeID     string
	rankID     string
}

// mapError translates domain/application errors into the Conjure SerializableError contract. An unknown
// standardized grade is a client input error (RankInvalid), not a 404 on a scheme node.
func (s Service) mapError(ctx context.Context, err error, c errCtx) error {
	switch {
	case errors.Is(err, domain.ErrSystemNotFound):
		return rankapi.NewRankSystemNotFound(c.systemID)
	case errors.Is(err, domain.ErrCategoryNotFound):
		return rankapi.NewRankCategoryNotFound(c.categoryID)
	case errors.Is(err, domain.ErrTypeNotFound):
		return rankapi.NewRankTypeNotFound(c.typeID)
	case errors.Is(err, domain.ErrRankNotFound):
		return rankapi.NewRankNotFound(c.rankID)
	case errors.Is(err, domain.ErrGradeNotFound):
		return rankapi.NewRankInvalid(err.Error())
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
