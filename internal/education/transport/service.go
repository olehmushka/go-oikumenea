// Package transport implements the generated educationapi.EducationService (D-Education, M20). It
// PEP-gates each op (education entities are instance-global external reference data, so reads/writes are
// satisfied anywhere), assembles translatable labels (institution/unit/group/building names, position
// titles, catalog names) as locale->text maps via the localization service, and maps domain sentinels to
// the Conjure Education:* SerializableErrors. Generated code is never hand-edited.
package transport

import (
	"context"
	"encoding/base64"
	"errors"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	educationapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/education"
	"github.com/olegamysk/go-oikumenea/internal/education/application"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// i18n entity types the translatable names are stored under (localization store).
const (
	entInstitution     = "education_institution"
	entUnit            = "education_unit"
	entGroup           = "education_group"
	entBuilding        = "education_building"
	entPosition        = "education_position"
	entInstitutionKind = "education_institution_kind"
	entUnitKind        = "education_unit_kind"
	entDegreeLevel     = "education_degree_level"
)

const (
	readPerm          = string(authzdomain.PermEducationRead)
	managePerm        = string(authzdomain.PermEducationManage)
	positionPerm      = string(authzdomain.PermEducationPositionManage)
	enrollmentPerm    = string(authzdomain.PermEducationEnrollmentManage)
	catalogManagePerm = string(authzdomain.PermEducationCatalogManage)
)

// EducationService adapts *application.Service to the generated educationapi.EducationService interface.
type EducationService struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the education application service, the localization
// service (name maps), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) EducationService {
	return EducationService{app: app, loc: loc, pep: enforcer}
}

var _ educationapi.EducationService = EducationService{}

// ============================ catalogs ============================

func (s EducationService) ListInstitutionKinds(ctx context.Context, token bearertoken.Token) (educationapi.InstitutionKindList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.InstitutionKindList{}, err
	}
	kinds, err := s.app.ListInstitutionKinds(ctx)
	if err != nil {
		return educationapi.InstitutionKindList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(kinds))
	for _, k := range kinds {
		defaults[k.ID] = k.Name
	}
	names, err := s.loc.NamesByID(ctx, entInstitutionKind, defaults)
	if err != nil {
		return educationapi.InstitutionKindList{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.InstitutionKind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, educationapi.InstitutionKind{Id: k.ID, Code: k.Code, Name: names[k.ID], Status: k.Status, SortOrder: k.SortOrder})
	}
	return educationapi.InstitutionKindList{InstitutionKinds: out}, nil
}

func (s EducationService) UpsertInstitutionKind(ctx context.Context, token bearertoken.Token, req educationapi.UpsertCatalogKindRequest) (educationapi.InstitutionKind, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogManagePerm); err != nil {
		return educationapi.InstitutionKind{}, err
	}
	k, err := s.app.UpsertInstitutionKind(ctx, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return educationapi.InstitutionKind{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entInstitutionKind, k.ID, k.Name)
	if err != nil {
		return educationapi.InstitutionKind{}, s.mapError(ctx, err)
	}
	return educationapi.InstitutionKind{Id: k.ID, Code: k.Code, Name: name, Status: k.Status, SortOrder: k.SortOrder}, nil
}

func (s EducationService) ListUnitKinds(ctx context.Context, token bearertoken.Token) (educationapi.UnitKindList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.UnitKindList{}, err
	}
	kinds, err := s.app.ListUnitKinds(ctx)
	if err != nil {
		return educationapi.UnitKindList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(kinds))
	for _, k := range kinds {
		defaults[k.ID] = k.Name
	}
	names, err := s.loc.NamesByID(ctx, entUnitKind, defaults)
	if err != nil {
		return educationapi.UnitKindList{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.UnitKind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, educationapi.UnitKind{Id: k.ID, Code: k.Code, Name: names[k.ID], Status: k.Status, SortOrder: k.SortOrder})
	}
	return educationapi.UnitKindList{UnitKinds: out}, nil
}

func (s EducationService) ListDegreeLevels(ctx context.Context, token bearertoken.Token) (educationapi.DegreeLevelList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.DegreeLevelList{}, err
	}
	levels, err := s.app.ListDegreeLevels(ctx)
	if err != nil {
		return educationapi.DegreeLevelList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(levels))
	for _, d := range levels {
		defaults[d.ID] = d.Name
	}
	names, err := s.loc.NamesByID(ctx, entDegreeLevel, defaults)
	if err != nil {
		return educationapi.DegreeLevelList{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.DegreeLevel, 0, len(levels))
	for _, d := range levels {
		out = append(out, educationapi.DegreeLevel{Id: d.ID, Code: d.Code, Name: names[d.ID], IscedLevel: d.IscedLevel, Status: d.Status, SortOrder: d.SortOrder})
	}
	return educationapi.DegreeLevelList{DegreeLevels: out}, nil
}

// ============================ institutions ============================

func (s EducationService) CreateInstitution(ctx context.Context, token bearertoken.Token, req educationapi.CreateInstitutionRequest) (educationapi.Institution, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return educationapi.Institution{}, err
	}
	created, err := s.app.CreateInstitution(ctx, domain.InstitutionInput{
		Code: req.Code, Name: req.Name, KindID: req.KindId,
		CountryID: req.CountryId, FoundedOn: req.FoundedOn, ClosedOn: req.ClosedOn,
	})
	if err != nil {
		return educationapi.Institution{}, s.mapError(ctx, err)
	}
	return s.toAPIInstitution(ctx, created)
}

func (s EducationService) ListInstitutions(ctx context.Context, token bearertoken.Token, query *string, pageSize *int, pageToken *string) (educationapi.InstitutionPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.InstitutionPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListInstitutions(ctx, strOr(query), decodeToken(pageToken), limit)
	if err != nil {
		return educationapi.InstitutionPage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	defaults := make(map[string]string, len(rows))
	for _, r := range rows {
		defaults[r.ID] = r.Name
	}
	names, err := s.loc.NamesByID(ctx, entInstitution, defaults)
	if err != nil {
		return educationapi.InstitutionPage{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.Institution, 0, len(rows))
	for _, r := range rows {
		out = append(out, institutionAPI(r, names[r.ID]))
	}
	page := educationapi.InstitutionPage{Institutions: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s EducationService) GetInstitution(ctx context.Context, token bearertoken.Token, id string) (educationapi.Institution, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.Institution{}, err
	}
	inst, err := s.app.GetInstitution(ctx, id)
	if err != nil {
		return educationapi.Institution{}, s.mapError(ctx, err)
	}
	return s.toAPIInstitution(ctx, inst)
}

func (s EducationService) UpdateInstitution(ctx context.Context, token bearertoken.Token, id string, req educationapi.UpdateInstitutionRequest) (educationapi.Institution, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return educationapi.Institution{}, err
	}
	updated, err := s.app.UpdateInstitution(ctx, id, domain.InstitutionUpdate{
		Name: req.Name, KindID: req.KindId, CountryID: req.CountryId, FoundedOn: req.FoundedOn, ClosedOn: req.ClosedOn, State: req.State,
	})
	if err != nil {
		return educationapi.Institution{}, s.mapError(ctx, err)
	}
	return s.toAPIInstitution(ctx, updated)
}

func (s EducationService) DeleteInstitution(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteInstitution(ctx, id))
}

// ============================ units + closure ============================

func (s EducationService) CreateUnit(ctx context.Context, token bearertoken.Token, institutionID string, req educationapi.CreateUnitRequest) (educationapi.EducationUnit, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return educationapi.EducationUnit{}, err
	}
	created, err := s.app.CreateUnit(ctx, institutionID, domain.UnitInput{
		Code: req.Code, Name: req.Name, KindID: req.KindId, ParentID: req.ParentId, SortOrder: req.SortOrder,
	})
	if err != nil {
		return educationapi.EducationUnit{}, s.mapError(ctx, err)
	}
	return s.toAPIUnit(ctx, created)
}

func (s EducationService) ListUnits(ctx context.Context, token bearertoken.Token, institutionID string) (educationapi.EducationUnitList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.EducationUnitList{}, err
	}
	units, err := s.app.ListUnits(ctx, institutionID)
	if err != nil {
		return educationapi.EducationUnitList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(units))
	for _, u := range units {
		defaults[u.ID] = u.Name
	}
	names, err := s.loc.NamesByID(ctx, entUnit, defaults)
	if err != nil {
		return educationapi.EducationUnitList{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.EducationUnit, 0, len(units))
	for _, u := range units {
		out = append(out, unitAPI(u, names[u.ID]))
	}
	return educationapi.EducationUnitList{Units: out}, nil
}

func (s EducationService) GetUnit(ctx context.Context, token bearertoken.Token, id string) (educationapi.EducationUnit, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.EducationUnit{}, err
	}
	u, err := s.app.GetUnit(ctx, id)
	if err != nil {
		return educationapi.EducationUnit{}, s.mapError(ctx, err)
	}
	return s.toAPIUnit(ctx, u)
}

func (s EducationService) UpdateUnit(ctx context.Context, token bearertoken.Token, id string, req educationapi.UpdateUnitRequest) (educationapi.EducationUnit, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return educationapi.EducationUnit{}, err
	}
	u, err := s.app.UpdateUnit(ctx, id, domain.UnitUpdate{Name: req.Name, KindID: req.KindId, Status: req.Status, SortOrder: req.SortOrder})
	if err != nil {
		return educationapi.EducationUnit{}, s.mapError(ctx, err)
	}
	return s.toAPIUnit(ctx, u)
}

func (s EducationService) ReparentUnit(ctx context.Context, token bearertoken.Token, id string, req educationapi.ReparentUnitRequest) (educationapi.EducationUnit, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return educationapi.EducationUnit{}, err
	}
	u, err := s.app.ReparentUnit(ctx, id, req.ParentId)
	if err != nil {
		return educationapi.EducationUnit{}, s.mapError(ctx, err)
	}
	return s.toAPIUnit(ctx, u)
}

// ============================ mappers + helpers ============================

func (s EducationService) toAPIInstitution(ctx context.Context, inst domain.Institution) (educationapi.Institution, error) {
	name, err := s.nameMap(ctx, entInstitution, inst.ID, inst.Name)
	if err != nil {
		return educationapi.Institution{}, s.mapError(ctx, err)
	}
	return institutionAPI(inst, name), nil
}

func institutionAPI(inst domain.Institution, name map[string]string) educationapi.Institution {
	return educationapi.Institution{
		Id: inst.ID, Code: inst.Code, Name: name, KindId: inst.KindID,
		CountryId: emptyToNil(inst.CountryID), FoundedOn: emptyToNil(inst.FoundedOn), ClosedOn: emptyToNil(inst.ClosedOn),
		State: inst.State, CreatedAt: datetime.DateTime(inst.CreatedAt), UpdatedAt: datetime.DateTime(inst.UpdatedAt),
	}
}

func (s EducationService) toAPIUnit(ctx context.Context, u domain.Unit) (educationapi.EducationUnit, error) {
	name, err := s.nameMap(ctx, entUnit, u.ID, u.Name)
	if err != nil {
		return educationapi.EducationUnit{}, s.mapError(ctx, err)
	}
	return unitAPI(u, name), nil
}

func unitAPI(u domain.Unit, name map[string]string) educationapi.EducationUnit {
	var depth *int
	if u.Depth > 0 || u.ParentID == "" {
		d := u.Depth
		depth = &d
	}
	return educationapi.EducationUnit{
		Id: u.ID, InstitutionId: u.InstitutionID, ParentId: emptyToNil(u.ParentID), KindId: u.KindID,
		Code: u.Code, Name: name, Status: u.Status, SortOrder: u.SortOrder, Depth: depth,
		CreatedAt: datetime.DateTime(u.CreatedAt), UpdatedAt: datetime.DateTime(u.UpdatedAt),
	}
}

// nameMap assembles one entity's translatable name as a locale->text map (default + i18n overlay).
func (s EducationService) nameMap(ctx context.Context, entityType, id, def string) (map[string]string, error) {
	m, err := s.loc.NamesByID(ctx, entityType, map[string]string{id: def})
	if err != nil {
		return nil, err
	}
	return m[id], nil
}

// mapError translates education domain sentinels into the Conjure Education:* errors.
func (s EducationService) mapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrInstitutionNotFound):
		return educationapi.NewInstitutionNotFound("")
	case errors.Is(err, domain.ErrUnitNotFound):
		return educationapi.NewUnitNotFound("")
	case errors.Is(err, domain.ErrBuildingNotFound):
		return educationapi.NewBuildingNotFound("")
	case errors.Is(err, domain.ErrGroupNotFound):
		return educationapi.NewGroupNotFound("")
	case errors.Is(err, domain.ErrPositionNotFound):
		return educationapi.NewPositionNotFound("")
	case errors.Is(err, domain.ErrEnrollmentNotFound):
		return educationapi.NewEnrollmentNotFound("")
	case errors.Is(err, domain.ErrDormNotFound):
		return educationapi.NewDormitoryStayNotFound("")
	case errors.Is(err, domain.ErrAppointmentNotFound):
		return educationapi.NewPositionNotFound("")
	case errors.Is(err, domain.ErrConflict):
		return educationapi.NewConflict("code already exists in scope")
	case errors.Is(err, domain.ErrUnitCycle):
		return educationapi.NewUnitCycleDetected("")
	case errors.Is(err, domain.ErrPositionAlreadyFilled):
		return educationapi.NewPositionAlreadyFilled("")
	case errors.Is(err, domain.ErrInUse):
		return educationapi.NewInUse("entity still referenced")
	case errors.Is(err, domain.ErrLifecycle):
		return educationapi.NewLifecycleConflict("invalid lifecycle transition")
	case errors.Is(err, domain.ErrInvalid):
		return educationapi.NewInvalid("invalid request or unknown reference")
	}
	return werror.WrapWithContextParams(ctx, err, "education operation failed")
}

// ---- pagination tokens (opaque base64 of the last id) ----

func decodeToken(p *string) string {
	if p == nil || *p == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(*p)
	if err != nil {
		return ""
	}
	return string(raw)
}

func encodeToken(id string) string {
	return base64.StdEncoding.EncodeToString([]byte(id))
}

func pageSizeOr(p *int) int {
	if p == nil || *p <= 0 {
		return 50
	}
	if *p > 200 {
		return 200
	}
	return *p
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
