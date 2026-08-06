// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport implements the generated educationapi.EducationService (D-Education, M20). It
// PEP-gates each op (education entities are instance-global external reference data, so reads/writes are
// satisfied anywhere), assembles translatable labels (institution/unit/group/building names, position
// titles, catalog names) as locale->text maps via the localization service, and maps domain sentinels to
// the Conjure Education:* SerializableErrors. Generated code is never hand-edited.
package transport

import (
	"context"
	"errors"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	educationapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/education"
	"github.com/olegamysk/go-oikumenea/internal/education/application"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
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

// PersonReader is the read-scope seam (D-PersonReadScope): may this subject read this person? The
// same one-method interface the document module declares, and answered by the same person-service SQL
// point probe (R-02.1) — education asks the question, it does not own the answer.
type PersonReader interface {
	ReadablePerson(ctx context.Context, subjectPersonID, personID string) (bool, error)
}

// EducationService adapts *application.Service to the generated educationapi.EducationService interface.
type EducationService struct {
	app    *application.Service
	loc    *locapp.Service
	pep    *pep.Enforcer
	person PersonReader
}

// NewService builds the transport adapter over the education application service, the localization
// service (name maps), the PEP enforcer, and the person reader the holder read scope is answered by.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer, person PersonReader) EducationService {
	return EducationService{app: app, loc: loc, pep: enforcer, person: person}
}

// holderReadable reports whether the request subject may read the given holder person under the
// read-scope projection (D-PersonReadScope): instance admins pass; anyone else is answered by the
// person reader's SQL reach point probe (R-02.1).
//
// Used by EVERY person-binding read in this module (M58 ticket 7). Until then education applied no
// holder scope at all: each of those endpoints gated `education.read` ANYWHERE and then returned the
// rows, so a single grant anywhere enumerated any person's education history instance-wide — the
// decision has required this projection since D-PersonReadScope, and only documents implemented it. A
// caller who fails the probe gets an EMPTY list, never a 403: a permission error would confirm the
// person exists, which is the same reasoning that makes a gated-out shadow row a NotFound.
func (s EducationService) holderReadable(ctx context.Context, personID string) (bool, error) {
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}
	return s.person.ReadablePerson(ctx, subject, personID)
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

func (s EducationService) ListInstitutions(ctx context.Context, token bearertoken.Token, query *string, kindId *string, countryId *string, foundedOnFrom *string, foundedOnTo *string, state *string, pageSize *int, pageToken *string) (educationapi.InstitutionPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.InstitutionPage{}, err
	}
	limit := pageSizeOr(pageSize)
	filter := institutionFilterFrom(query, kindId, countryId, foundedOnFrom, foundedOnTo, state)
	rows, err := s.app.ListInstitutions(ctx, filter, decodeToken(pageToken), limit)
	if err != nil {
		return educationapi.InstitutionPage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	// The shadow gate, AFTER the page is cut and the token taken from the last row read — the same
	// order listOrganizations uses, so paging stays stable when the gate trims a row (D-VisibilityScope).
	if rows, err = gateInstitutions(ctx, s.pep, rows); err != nil {
		return educationapi.InstitutionPage{}, s.mapError(ctx, err)
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
	// The point read runs the SAME gate as the list, through the same helper. Before M58 ticket 5 it
	// ran none at all, so a caller holding education.read could fetch a shadow institution by RID that
	// the list refused to name — the getOrganization leak, in the module that inherited its rows.
	visible, err := gateInstitutions(ctx, s.pep, []domain.Institution{inst})
	if err != nil {
		return educationapi.Institution{}, s.mapError(ctx, err)
	}
	if len(visible) == 0 {
		// NOT a permission error: `shadow` hides EXISTENCE, and a 403 would confirm the RID names a
		// real institution — exactly what the list refuses to say by omitting the row.
		return educationapi.Institution{}, s.mapError(ctx, domain.ErrInstitutionNotFound)
	}
	return s.toAPIInstitution(ctx, visible[0])
}

// gateInstitutions applies the organization shadow gate to institution rows (M58 ticket 5). An
// institution IS a `university`-domain tenant organization (M41 / D-UnifiedOrgGraph), so it carries
// that organization's public/shadow bit and must be trimmed by exactly the rule listOrganizations
// applies — including the M58 ticket-4 amendment that DERIVES organization reach from unit reach.
//
// It is a sibling of tenant's gateOrgs rather than a call into it: the reach decision itself lives in
// one place (pep.FilterVisibleOrgs → authz), and what is repeated here is only the shape of the
// candidate list. Every shadow-bearing read in this module routes through this one helper, which is
// what transport/shadowgate_test.go asserts structurally.
func gateInstitutions(ctx context.Context, enf *pep.Enforcer, items []domain.Institution) ([]domain.Institution, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]string, len(items))
	shadow := make(map[string]bool, len(items))
	for i, inst := range items {
		ids[i] = inst.ID
		shadow[inst.ID] = inst.Visibility == domain.VisibilityShadow
	}
	visible, err := enf.FilterVisibleOrgs(ctx, ids, shadow)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(visible))
	for _, id := range visible {
		allowed[id] = struct{}{}
	}
	out := make([]domain.Institution, 0, len(items))
	for _, inst := range items {
		if _, ok := allowed[inst.ID]; ok {
			out = append(out, inst)
		}
	}
	return out, nil
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

// decodeToken/encodeToken are the opaque keyset cursor over the last row's RID, delegated to the
// shared pkg/listing codec (M56). These endpoints previously emitted base64 StdEncoding, whose
// `+`, `/` and `=` are NOT URL-safe in a query parameter (a `+` decodes to a space, corrupting the
// cursor); listing.EncodeCursor emits RawURL, and its decode stays tolerant of the old alphabet so
// tokens issued before the upgrade keep working. An undecodable token still yields "" — restarting
// at the first page — preserving this transport's existing behaviour.
func decodeToken(p *string) string {
	id, err := listing.DecodeCursorPtr(p)
	if err != nil {
		return ""
	}
	return id
}

func encodeToken(id string) string { return listing.EncodeCursor(id) }

// pageSizePolicy mirrors the owning application service's clamp, applied at the wire edge over the
// optional Conjure arg (M56 / pkg/listing).
var pageSizePolicy = listing.PageSize{Default: 50, Max: 200}

func pageSizeOr(p *int) int { return pageSizePolicy.ResolvePtr(p) }

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
