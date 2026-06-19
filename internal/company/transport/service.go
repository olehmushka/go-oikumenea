// Package transport implements the generated companyapi.CompanyService (D-Companies, M21). It PEP-gates
// each op (company entities are instance-global external reference data, so reads/writes are satisfied
// anywhere), assembles translatable labels (company legal names, legal-form/scheme/industry names,
// position titles) as locale->text maps via the localization service, and maps domain sentinels to the
// Conjure Company:* SerializableErrors. Generated code is never hand-edited.
package transport

import (
	"context"
	"encoding/base64"
	"errors"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	"github.com/olegamysk/go-oikumenea/internal/company/application"
	"github.com/olegamysk/go-oikumenea/internal/company/domain"
	companyapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/company"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// i18n entity types the translatable names are stored under (localization store).
const (
	entCompany   = "company_company"
	entLegalForm = "company_legal_form"
	entScheme    = "company_registration_scheme"
	entIndustry  = "company_industry_class"
	entPosition  = "company_position"
)

const (
	readPerm     = string(authzdomain.PermCompanyRead)
	managePerm   = string(authzdomain.PermCompanyManage)
	positionPerm = string(authzdomain.PermCompanyPositionManage)
	catalogPerm  = string(authzdomain.PermCompanyCatalogManage)
)

// CompanyService adapts *application.Service to the generated companyapi.CompanyService interface.
type CompanyService struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the company application service, the localization
// service (name maps), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) CompanyService {
	return CompanyService{app: app, loc: loc, pep: enforcer}
}

var _ companyapi.CompanyService = CompanyService{}

// ============================ catalogs ============================

func (s CompanyService) ListLegalForms(ctx context.Context, token bearertoken.Token) (companyapi.LegalFormList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.LegalFormList{}, err
	}
	rows, err := s.app.ListLegalForms(ctx)
	if err != nil {
		return companyapi.LegalFormList{}, s.mapError(ctx, err)
	}
	names, err := s.namesByID(ctx, entLegalForm, defaultsLegalForms(rows))
	if err != nil {
		return companyapi.LegalFormList{}, s.mapError(ctx, err)
	}
	out := make([]companyapi.LegalForm, 0, len(rows))
	for _, k := range rows {
		out = append(out, legalFormAPI(k, names[k.ID]))
	}
	return companyapi.LegalFormList{LegalForms: out}, nil
}

func (s CompanyService) UpsertLegalForm(ctx context.Context, token bearertoken.Token, req companyapi.UpsertLegalFormRequest) (companyapi.LegalForm, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return companyapi.LegalForm{}, err
	}
	k, err := s.app.UpsertLegalForm(ctx, req.Code, req.Name, req.Abbreviation, req.CountryId, req.SortOrder)
	if err != nil {
		return companyapi.LegalForm{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entLegalForm, k.ID, k.Name)
	if err != nil {
		return companyapi.LegalForm{}, s.mapError(ctx, err)
	}
	return legalFormAPI(k, name), nil
}

func (s CompanyService) ListRegistrationSchemes(ctx context.Context, token bearertoken.Token) (companyapi.RegistrationSchemeList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.RegistrationSchemeList{}, err
	}
	rows, err := s.app.ListRegistrationSchemes(ctx)
	if err != nil {
		return companyapi.RegistrationSchemeList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(rows))
	for _, r := range rows {
		defaults[r.ID] = r.Name
	}
	names, err := s.namesByID(ctx, entScheme, defaults)
	if err != nil {
		return companyapi.RegistrationSchemeList{}, s.mapError(ctx, err)
	}
	out := make([]companyapi.RegistrationScheme, 0, len(rows))
	for _, r := range rows {
		out = append(out, schemeAPI(r, names[r.ID]))
	}
	return companyapi.RegistrationSchemeList{RegistrationSchemes: out}, nil
}

func (s CompanyService) UpsertRegistrationScheme(ctx context.Context, token bearertoken.Token, req companyapi.UpsertSchemeRequest) (companyapi.RegistrationScheme, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return companyapi.RegistrationScheme{}, err
	}
	isGlobal := req.IsGlobal != nil && *req.IsGlobal
	k, err := s.app.UpsertRegistrationScheme(ctx, req.Code, req.Name, req.ValidatorPattern, isGlobal, req.SortOrder)
	if err != nil {
		return companyapi.RegistrationScheme{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entScheme, k.ID, k.Name)
	if err != nil {
		return companyapi.RegistrationScheme{}, s.mapError(ctx, err)
	}
	return schemeAPI(k, name), nil
}

func (s CompanyService) ListIndustryClasses(ctx context.Context, token bearertoken.Token) (companyapi.IndustryClassList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.IndustryClassList{}, err
	}
	rows, err := s.app.ListIndustryClasses(ctx)
	if err != nil {
		return companyapi.IndustryClassList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(rows))
	for _, r := range rows {
		defaults[r.ID] = r.Name
	}
	names, err := s.namesByID(ctx, entIndustry, defaults)
	if err != nil {
		return companyapi.IndustryClassList{}, s.mapError(ctx, err)
	}
	out := make([]companyapi.IndustryClass, 0, len(rows))
	for _, r := range rows {
		out = append(out, industryClassAPI(r, names[r.ID]))
	}
	return companyapi.IndustryClassList{IndustryClasses: out}, nil
}

func (s CompanyService) UpsertIndustryClass(ctx context.Context, token bearertoken.Token, req companyapi.UpsertIndustryClassRequest) (companyapi.IndustryClass, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return companyapi.IndustryClass{}, err
	}
	k, err := s.app.UpsertIndustryClass(ctx, req.Code, req.Name, strOr(req.System), req.SortOrder)
	if err != nil {
		return companyapi.IndustryClass{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entIndustry, k.ID, k.Name)
	if err != nil {
		return companyapi.IndustryClass{}, s.mapError(ctx, err)
	}
	return industryClassAPI(k, name), nil
}

// ============================ companies ============================

func (s CompanyService) CreateCompany(ctx context.Context, token bearertoken.Token, req companyapi.CreateCompanyRequest) (companyapi.Company, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.Company{}, err
	}
	created, err := s.app.CreateCompany(ctx, domain.CompanyInput{
		Code: req.Code, LegalName: req.LegalName, LegalFormID: req.LegalFormId,
		ShortName: req.ShortName, OwnershipCategory: req.OwnershipCategory, CountryID: req.CountryId, FoundedOn: req.FoundedOn,
	})
	if err != nil {
		return companyapi.Company{}, s.mapError(ctx, err)
	}
	return s.toAPICompany(ctx, created)
}

func (s CompanyService) ListCompanies(ctx context.Context, token bearertoken.Token, query *string, pageSize *int, pageToken *string) (companyapi.CompanyPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.CompanyPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListCompanies(ctx, strOr(query), decodeToken(pageToken), limit)
	if err != nil {
		return companyapi.CompanyPage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	defaults := make(map[string]string, len(rows))
	for _, r := range rows {
		defaults[r.ID] = r.LegalName
	}
	names, err := s.namesByID(ctx, entCompany, defaults)
	if err != nil {
		return companyapi.CompanyPage{}, s.mapError(ctx, err)
	}
	out := make([]companyapi.Company, 0, len(rows))
	for _, r := range rows {
		out = append(out, companyAPI(r, names[r.ID]))
	}
	page := companyapi.CompanyPage{Companies: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s CompanyService) GetCompany(ctx context.Context, token bearertoken.Token, id string) (companyapi.Company, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.Company{}, err
	}
	c, err := s.app.GetCompany(ctx, id)
	if err != nil {
		return companyapi.Company{}, s.mapError(ctx, err)
	}
	return s.toAPICompany(ctx, c)
}

func (s CompanyService) UpdateCompany(ctx context.Context, token bearertoken.Token, id string, req companyapi.UpdateCompanyRequest) (companyapi.Company, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.Company{}, err
	}
	updated, err := s.app.UpdateCompany(ctx, id, domain.CompanyUpdate{
		LegalName: req.LegalName, ShortName: req.ShortName, LegalFormID: req.LegalFormId,
		OwnershipCategory: req.OwnershipCategory, CountryID: req.CountryId,
		FoundedOn: req.FoundedOn, DissolvedOn: req.DissolvedOn, State: req.State,
	})
	if err != nil {
		return companyapi.Company{}, s.mapError(ctx, err)
	}
	return s.toAPICompany(ctx, updated)
}

func (s CompanyService) DeleteCompany(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteCompany(ctx, id))
}

// ============================ registrations ============================

func (s CompanyService) ListRegistrations(ctx context.Context, token bearertoken.Token, companyID string) (companyapi.RegistrationList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.RegistrationList{}, err
	}
	rows, err := s.app.ListRegistrations(ctx, companyID)
	if err != nil {
		return companyapi.RegistrationList{}, s.mapError(ctx, err)
	}
	out := make([]companyapi.Registration, 0, len(rows))
	for _, r := range rows {
		out = append(out, registrationAPI(r))
	}
	return companyapi.RegistrationList{Registrations: out}, nil
}

func (s CompanyService) AddRegistration(ctx context.Context, token bearertoken.Token, companyID string, req companyapi.UpsertRegistrationRequest) (companyapi.Registration, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.Registration{}, err
	}
	r, err := s.app.AddRegistration(ctx, companyID, domain.RegistrationInput{SchemeID: req.SchemeId, Identifier: req.Identifier})
	if err != nil {
		return companyapi.Registration{}, s.mapError(ctx, err)
	}
	return registrationAPI(r), nil
}

func (s CompanyService) UpdateRegistration(ctx context.Context, token bearertoken.Token, id string, req companyapi.UpsertRegistrationRequest) (companyapi.Registration, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.Registration{}, err
	}
	r, err := s.app.UpdateRegistration(ctx, id, domain.RegistrationInput{SchemeID: req.SchemeId, Identifier: req.Identifier})
	if err != nil {
		return companyapi.Registration{}, s.mapError(ctx, err)
	}
	return registrationAPI(r), nil
}

func (s CompanyService) DeleteRegistration(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteRegistration(ctx, id))
}

// ============================ industries ============================

func (s CompanyService) ListIndustries(ctx context.Context, token bearertoken.Token, companyID string) (companyapi.IndustryAssignmentList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.IndustryAssignmentList{}, err
	}
	rows, err := s.app.ListIndustries(ctx, companyID)
	if err != nil {
		return companyapi.IndustryAssignmentList{}, s.mapError(ctx, err)
	}
	out := make([]companyapi.IndustryAssignment, 0, len(rows))
	for _, r := range rows {
		out = append(out, industryAssignmentAPI(r))
	}
	return companyapi.IndustryAssignmentList{Industries: out}, nil
}

func (s CompanyService) AssignIndustry(ctx context.Context, token bearertoken.Token, companyID string, req companyapi.AssignIndustryRequest) (companyapi.IndustryAssignment, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.IndustryAssignment{}, err
	}
	a, err := s.app.AssignIndustry(ctx, companyID, domain.IndustryInput{IndustryClassID: req.IndustryClassId, IsPrimary: req.IsPrimary != nil && *req.IsPrimary})
	if err != nil {
		return companyapi.IndustryAssignment{}, s.mapError(ctx, err)
	}
	return industryAssignmentAPI(a), nil
}

func (s CompanyService) RemoveIndustry(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveIndustry(ctx, id))
}

// ============================ locations ============================

func (s CompanyService) ListCompanyLocations(ctx context.Context, token bearertoken.Token, companyID string) (companyapi.CompanyLocationList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.CompanyLocationList{}, err
	}
	rows, err := s.app.ListCompanyLocations(ctx, companyID)
	if err != nil {
		return companyapi.CompanyLocationList{}, s.mapError(ctx, err)
	}
	out := make([]companyapi.CompanyLocation, 0, len(rows))
	for _, r := range rows {
		out = append(out, companyLocationAPI(r))
	}
	return companyapi.CompanyLocationList{Locations: out}, nil
}

func (s CompanyService) AddCompanyLocation(ctx context.Context, token bearertoken.Token, companyID string, req companyapi.AddCompanyLocationRequest) (companyapi.CompanyLocation, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.CompanyLocation{}, err
	}
	l, err := s.app.AddCompanyLocation(ctx, companyID, domain.CompanyLocationInput{LocationID: req.LocationId, Role: req.Role})
	if err != nil {
		return companyapi.CompanyLocation{}, s.mapError(ctx, err)
	}
	return companyLocationAPI(l), nil
}

func (s CompanyService) RemoveCompanyLocation(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveCompanyLocation(ctx, id))
}

// ============================ positions + appointments ============================

func (s CompanyService) CreatePosition(ctx context.Context, token bearertoken.Token, companyID string, req companyapi.CreatePositionRequest) (companyapi.CompanyPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return companyapi.CompanyPosition{}, err
	}
	p, err := s.app.CreatePosition(ctx, companyID, domain.PositionInput{Code: req.Code, Title: req.Title, SortOrder: req.SortOrder})
	if err != nil {
		return companyapi.CompanyPosition{}, s.mapError(ctx, err)
	}
	return s.toAPIPosition(ctx, p)
}

func (s CompanyService) ListPositions(ctx context.Context, token bearertoken.Token, companyID string, state *string, pageSize *int, pageToken *string) (companyapi.CompanyPositionPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.CompanyPositionPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListPositions(ctx, companyID, strOr(state), decodeToken(pageToken), limit)
	if err != nil {
		return companyapi.CompanyPositionPage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	defaults := make(map[string]string, len(rows))
	for _, r := range rows {
		defaults[r.ID] = r.Title
	}
	names, err := s.namesByID(ctx, entPosition, defaults)
	if err != nil {
		return companyapi.CompanyPositionPage{}, s.mapError(ctx, err)
	}
	out := make([]companyapi.CompanyPosition, 0, len(rows))
	for _, r := range rows {
		out = append(out, positionAPI(r, names[r.ID]))
	}
	page := companyapi.CompanyPositionPage{Positions: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s CompanyService) GetPosition(ctx context.Context, token bearertoken.Token, id string) (companyapi.CompanyPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.CompanyPosition{}, err
	}
	p, err := s.app.GetPosition(ctx, id)
	if err != nil {
		return companyapi.CompanyPosition{}, s.mapError(ctx, err)
	}
	return s.toAPIPosition(ctx, p)
}

func (s CompanyService) UpdatePosition(ctx context.Context, token bearertoken.Token, id string, req companyapi.UpdatePositionRequest) (companyapi.CompanyPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return companyapi.CompanyPosition{}, err
	}
	p, err := s.app.UpdatePosition(ctx, id, domain.PositionUpdate{Title: req.Title, SortOrder: req.SortOrder})
	if err != nil {
		return companyapi.CompanyPosition{}, s.mapError(ctx, err)
	}
	return s.toAPIPosition(ctx, p)
}

func (s CompanyService) AbolishPosition(ctx context.Context, token bearertoken.Token, id string) (companyapi.CompanyPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return companyapi.CompanyPosition{}, err
	}
	p, err := s.app.AbolishPosition(ctx, id)
	if err != nil {
		return companyapi.CompanyPosition{}, s.mapError(ctx, err)
	}
	return s.toAPIPosition(ctx, p)
}

func (s CompanyService) FillPosition(ctx context.Context, token bearertoken.Token, id string, req companyapi.FillPositionRequest) (companyapi.Appointment, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return companyapi.Appointment{}, err
	}
	a, err := s.app.FillPosition(ctx, id, req.PersonId, fromDateTime(req.EffectiveFrom))
	if err != nil {
		return companyapi.Appointment{}, s.mapError(ctx, err)
	}
	return appointmentAPI(a), nil
}

func (s CompanyService) EndAppointment(ctx context.Context, token bearertoken.Token, id string, req companyapi.EndAppointmentRequest) (companyapi.Appointment, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return companyapi.Appointment{}, err
	}
	a, err := s.app.EndAppointment(ctx, id, fromDateTime(req.EffectiveTo))
	if err != nil {
		return companyapi.Appointment{}, s.mapError(ctx, err)
	}
	return appointmentAPI(a), nil
}

// ============================ ownership / affiliation graph ============================

func (s CompanyService) GetOwnershipGraph(ctx context.Context, token bearertoken.Token, companyID string) (companyapi.OwnershipGraph, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.OwnershipGraph{}, err
	}
	g, err := s.app.GetOwnershipGraph(ctx, companyID)
	if err != nil {
		return companyapi.OwnershipGraph{}, s.mapError(ctx, err)
	}
	return ownershipGraphAPI(g), nil
}

func (s CompanyService) RecordFounding(ctx context.Context, token bearertoken.Token, companyID string, req companyapi.RecordFoundingRequest) (companyapi.Founding, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.Founding{}, err
	}
	f, err := s.app.RecordFounding(ctx, companyID, domain.FoundingInput{HolderKind: req.HolderKind, HolderID: req.HolderId, FoundedOn: req.FoundedOn})
	if err != nil {
		return companyapi.Founding{}, s.mapError(ctx, err)
	}
	return foundingAPI(f), nil
}

func (s CompanyService) RemoveFounding(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveFounding(ctx, id))
}

func (s CompanyService) RecordShareholding(ctx context.Context, token bearertoken.Token, companyID string, req companyapi.RecordShareholdingRequest) (companyapi.Shareholding, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.Shareholding{}, err
	}
	sh, err := s.app.RecordShareholding(ctx, companyID, domain.ShareholdingInput{
		HolderKind: req.HolderKind, HolderID: req.HolderId, StakePct: req.StakePct, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo,
	})
	if err != nil {
		return companyapi.Shareholding{}, s.mapError(ctx, err)
	}
	return shareholdingAPI(sh), nil
}

func (s CompanyService) RemoveShareholding(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveShareholding(ctx, id))
}

func (s CompanyService) RecordBeneficiary(ctx context.Context, token bearertoken.Token, companyID string, req companyapi.RecordBeneficiaryRequest) (companyapi.Beneficiary, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.Beneficiary{}, err
	}
	b, err := s.app.RecordBeneficiary(ctx, companyID, domain.BeneficiaryInput{PersonID: req.PersonId, UltimatePct: req.UltimatePct, Declared: req.Declared})
	if err != nil {
		return companyapi.Beneficiary{}, s.mapError(ctx, err)
	}
	return beneficiaryAPI(b), nil
}

func (s CompanyService) RemoveBeneficiary(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveBeneficiary(ctx, id))
}

func (s CompanyService) RecordSuccession(ctx context.Context, token bearertoken.Token, companyID string, req companyapi.RecordSuccessionRequest) (companyapi.Succession, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.Succession{}, err
	}
	su, err := s.app.RecordSuccession(ctx, companyID, domain.SuccessionInput{SuccessorID: req.SuccessorId, Kind: req.Kind, EffectiveOn: req.EffectiveOn})
	if err != nil {
		return companyapi.Succession{}, s.mapError(ctx, err)
	}
	return successionAPI(su), nil
}

func (s CompanyService) RemoveSuccession(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveSuccession(ctx, id))
}

func (s CompanyService) RecordBranch(ctx context.Context, token bearertoken.Token, companyID string, req companyapi.RecordBranchRequest) (companyapi.Branch, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return companyapi.Branch{}, err
	}
	b, err := s.app.RecordBranch(ctx, companyID, req.BranchId)
	if err != nil {
		return companyapi.Branch{}, s.mapError(ctx, err)
	}
	return branchAPI(b), nil
}

func (s CompanyService) RemoveBranch(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveBranch(ctx, id))
}

// ============================ person view ============================

func (s CompanyService) ListPersonCompanyAffiliations(ctx context.Context, token bearertoken.Token, personID string) (companyapi.PersonCompanyAffiliations, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.PersonCompanyAffiliations{}, err
	}
	a, err := s.app.ListPersonAffiliations(ctx, personID)
	if err != nil {
		return companyapi.PersonCompanyAffiliations{}, s.mapError(ctx, err)
	}
	out := companyapi.PersonCompanyAffiliations{
		Appointments:  make([]companyapi.PersonCompanyAppointment, 0, len(a.Appointments)),
		Foundings:     make([]companyapi.Founding, 0, len(a.Foundings)),
		Shareholdings: make([]companyapi.Shareholding, 0, len(a.Shareholdings)),
		BeneficiaryOf: make([]companyapi.Beneficiary, 0, len(a.BeneficiaryOf)),
	}
	for _, ap := range a.Appointments {
		out.Appointments = append(out.Appointments, personAppointmentAPI(ap))
	}
	for _, f := range a.Foundings {
		out.Foundings = append(out.Foundings, foundingAPI(f))
	}
	for _, sh := range a.Shareholdings {
		out.Shareholdings = append(out.Shareholdings, shareholdingAPI(sh))
	}
	for _, b := range a.BeneficiaryOf {
		out.BeneficiaryOf = append(out.BeneficiaryOf, beneficiaryAPI(b))
	}
	return out, nil
}

// ============================ mappers ============================

func (s CompanyService) toAPICompany(ctx context.Context, c domain.Company) (companyapi.Company, error) {
	name, err := s.nameMap(ctx, entCompany, c.ID, c.LegalName)
	if err != nil {
		return companyapi.Company{}, s.mapError(ctx, err)
	}
	return companyAPI(c, name), nil
}

func companyAPI(c domain.Company, legalName map[string]string) companyapi.Company {
	return companyapi.Company{
		Id: c.ID, Code: c.Code, LegalName: legalName, ShortName: emptyToNil(c.ShortName), LegalFormId: c.LegalFormID,
		OwnershipCategory: c.OwnershipCategory, CountryId: emptyToNil(c.CountryID),
		FoundedOn: emptyToNil(c.FoundedOn), DissolvedOn: emptyToNil(c.DissolvedOn), State: c.State,
		CreatedAt: datetime.DateTime(c.CreatedAt), UpdatedAt: datetime.DateTime(c.UpdatedAt),
	}
}

func legalFormAPI(k domain.LegalForm, name map[string]string) companyapi.LegalForm {
	return companyapi.LegalForm{Id: k.ID, Code: k.Code, Name: name, Abbreviation: emptyToNil(k.Abbreviation), CountryId: emptyToNil(k.CountryID), Status: k.Status, SortOrder: k.SortOrder}
}

func schemeAPI(k domain.RegistrationScheme, name map[string]string) companyapi.RegistrationScheme {
	return companyapi.RegistrationScheme{Id: k.ID, Code: k.Code, Name: name, ValidatorPattern: emptyToNil(k.ValidatorPattern), IsGlobal: k.IsGlobal, Status: k.Status, SortOrder: k.SortOrder}
}

func industryClassAPI(k domain.IndustryClass, name map[string]string) companyapi.IndustryClass {
	return companyapi.IndustryClass{Id: k.ID, Code: k.Code, Name: name, System: k.System, Status: k.Status, SortOrder: k.SortOrder}
}

func registrationAPI(r domain.Registration) companyapi.Registration {
	return companyapi.Registration{Id: r.ID, CompanyId: r.CompanyID, SchemeId: r.SchemeID, Identifier: r.Identifier, Validated: r.Validated, CreatedAt: datetime.DateTime(r.CreatedAt), UpdatedAt: datetime.DateTime(r.UpdatedAt)}
}

func industryAssignmentAPI(a domain.IndustryAssignment) companyapi.IndustryAssignment {
	return companyapi.IndustryAssignment{Id: a.ID, CompanyId: a.CompanyID, IndustryClassId: a.IndustryClassID, IsPrimary: a.IsPrimary, CreatedAt: datetime.DateTime(a.CreatedAt), UpdatedAt: datetime.DateTime(a.UpdatedAt)}
}

func companyLocationAPI(l domain.CompanyLocation) companyapi.CompanyLocation {
	return companyapi.CompanyLocation{Id: l.ID, CompanyId: l.CompanyID, LocationId: l.LocationID, Role: l.Role, CreatedAt: datetime.DateTime(l.CreatedAt), UpdatedAt: datetime.DateTime(l.UpdatedAt)}
}

func (s CompanyService) toAPIPosition(ctx context.Context, p domain.Position) (companyapi.CompanyPosition, error) {
	title, err := s.nameMap(ctx, entPosition, p.ID, p.Title)
	if err != nil {
		return companyapi.CompanyPosition{}, s.mapError(ctx, err)
	}
	return positionAPI(p, title), nil
}

func positionAPI(p domain.Position, title map[string]string) companyapi.CompanyPosition {
	out := companyapi.CompanyPosition{
		Id: p.ID, CompanyId: p.CompanyID, Code: p.Code, Title: title, Status: p.Status, SortOrder: p.SortOrder,
		CreatedAt: datetime.DateTime(p.CreatedAt), UpdatedAt: datetime.DateTime(p.UpdatedAt),
	}
	if p.Holder != nil {
		h := appointmentAPI(*p.Holder)
		out.Holder = &h
	}
	return out
}

func appointmentAPI(a domain.Appointment) companyapi.Appointment {
	out := companyapi.Appointment{
		Id: a.ID, PersonId: a.PersonID, PositionId: a.PositionID, Status: a.Status,
		EffectiveFrom: datetime.DateTime(a.EffectiveFrom), CreatedAt: datetime.DateTime(a.CreatedAt), UpdatedAt: datetime.DateTime(a.UpdatedAt),
	}
	if a.EffectiveTo != nil {
		t := datetime.DateTime(*a.EffectiveTo)
		out.EffectiveTo = &t
	}
	return out
}

func personAppointmentAPI(a domain.PersonAppointment) companyapi.PersonCompanyAppointment {
	out := companyapi.PersonCompanyAppointment{
		Id: a.ID, PersonId: a.PersonID, PositionId: a.PositionID, PositionTitle: a.PositionTitle,
		CompanyId: a.CompanyID, CompanyName: a.CompanyName, Status: a.Status,
		EffectiveFrom: datetime.DateTime(a.EffectiveFrom), CreatedAt: datetime.DateTime(a.CreatedAt), UpdatedAt: datetime.DateTime(a.UpdatedAt),
	}
	if a.EffectiveTo != nil {
		t := datetime.DateTime(*a.EffectiveTo)
		out.EffectiveTo = &t
	}
	return out
}

func foundingAPI(f domain.Founding) companyapi.Founding {
	return companyapi.Founding{
		Id: f.ID, CompanyId: f.CompanyID, CompanyLabel: emptyToNil(f.CompanyLabel),
		HolderKind: f.HolderKind, HolderId: f.HolderID, HolderLabel: emptyToNil(f.HolderLabel),
		FoundedOn: emptyToNil(f.FoundedOn), CreatedAt: datetime.DateTime(f.CreatedAt), UpdatedAt: datetime.DateTime(f.UpdatedAt),
	}
}

func shareholdingAPI(sh domain.Shareholding) companyapi.Shareholding {
	return companyapi.Shareholding{
		Id: sh.ID, CompanyId: sh.CompanyID, CompanyLabel: emptyToNil(sh.CompanyLabel),
		HolderKind: sh.HolderKind, HolderId: sh.HolderID, HolderLabel: emptyToNil(sh.HolderLabel),
		StakePct: sh.StakePct, EffectiveFrom: emptyToNil(sh.EffectiveFrom), EffectiveTo: emptyToNil(sh.EffectiveTo),
		CreatedAt: datetime.DateTime(sh.CreatedAt), UpdatedAt: datetime.DateTime(sh.UpdatedAt),
	}
}

func beneficiaryAPI(b domain.Beneficiary) companyapi.Beneficiary {
	return companyapi.Beneficiary{
		Id: b.ID, CompanyId: b.CompanyID, CompanyLabel: emptyToNil(b.CompanyLabel), PersonId: b.PersonID,
		UltimatePct: b.UltimatePct, Declared: b.Declared, CreatedAt: datetime.DateTime(b.CreatedAt), UpdatedAt: datetime.DateTime(b.UpdatedAt),
	}
}

func successionAPI(su domain.Succession) companyapi.Succession {
	return companyapi.Succession{
		Id: su.ID, PredecessorId: su.PredecessorID, PredecessorLabel: emptyToNil(su.PredecessorLabel),
		SuccessorId: su.SuccessorID, SuccessorLabel: emptyToNil(su.SuccessorLabel), Kind: su.Kind,
		EffectiveOn: emptyToNil(su.EffectiveOn), CreatedAt: datetime.DateTime(su.CreatedAt), UpdatedAt: datetime.DateTime(su.UpdatedAt),
	}
}

func branchAPI(b domain.Branch) companyapi.Branch {
	return companyapi.Branch{
		Id: b.ID, BranchId: b.BranchID, BranchLabel: emptyToNil(b.BranchLabel),
		ParentId: b.ParentID, ParentLabel: emptyToNil(b.ParentLabel), CreatedAt: datetime.DateTime(b.CreatedAt), UpdatedAt: datetime.DateTime(b.UpdatedAt),
	}
}

func ownershipGraphAPI(g domain.OwnershipGraph) companyapi.OwnershipGraph {
	out := companyapi.OwnershipGraph{
		CompanyId:     g.CompanyID,
		Shareholders:  make([]companyapi.Shareholding, 0, len(g.Shareholders)),
		Holdings:      make([]companyapi.Shareholding, 0, len(g.Holdings)),
		Beneficiaries: make([]companyapi.Beneficiary, 0, len(g.Beneficiaries)),
		Founders:      make([]companyapi.Founding, 0, len(g.Founders)),
		Successions:   make([]companyapi.Succession, 0, len(g.Successions)),
		Branches:      make([]companyapi.Branch, 0, len(g.Branches)),
	}
	for _, sh := range g.Shareholders {
		out.Shareholders = append(out.Shareholders, shareholdingAPI(sh))
	}
	for _, sh := range g.Holdings {
		out.Holdings = append(out.Holdings, shareholdingAPI(sh))
	}
	for _, b := range g.Beneficiaries {
		out.Beneficiaries = append(out.Beneficiaries, beneficiaryAPI(b))
	}
	for _, f := range g.Founders {
		out.Founders = append(out.Founders, foundingAPI(f))
	}
	for _, su := range g.Successions {
		out.Successions = append(out.Successions, successionAPI(su))
	}
	for _, br := range g.Branches {
		out.Branches = append(out.Branches, branchAPI(br))
	}
	return out
}

// ============================ helpers ============================

func defaultsLegalForms(rows []domain.LegalForm) map[string]string {
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.ID] = r.Name
	}
	return m
}

// namesByID assembles a set of entities' translatable names as locale->text maps (default + i18n overlay).
func (s CompanyService) namesByID(ctx context.Context, entityType string, defaults map[string]string) (map[string]map[string]string, error) {
	return s.loc.NamesByID(ctx, entityType, defaults)
}

// nameMap assembles one entity's translatable name as a locale->text map (default + i18n overlay).
func (s CompanyService) nameMap(ctx context.Context, entityType, id, def string) (map[string]string, error) {
	m, err := s.loc.NamesByID(ctx, entityType, map[string]string{id: def})
	if err != nil {
		return nil, err
	}
	return m[id], nil
}

// mapError translates company domain sentinels into the Conjure Company:* errors.
func (s CompanyService) mapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrCompanyNotFound):
		return companyapi.NewCompanyNotFound("")
	case errors.Is(err, domain.ErrPositionNotFound):
		return companyapi.NewPositionNotFound("")
	case errors.Is(err, domain.ErrAppointmentNotFound):
		return companyapi.NewPositionNotFound("")
	case errors.Is(err, domain.ErrLinkNotFound):
		return companyapi.NewLinkNotFound("")
	case errors.Is(err, domain.ErrConflict):
		return companyapi.NewConflict("code or identifier already exists in scope")
	case errors.Is(err, domain.ErrPositionAlreadyFilled):
		return companyapi.NewPositionAlreadyFilled("")
	case errors.Is(err, domain.ErrInUse):
		return companyapi.NewInUse("entity still referenced")
	case errors.Is(err, domain.ErrLifecycle):
		return companyapi.NewLifecycleConflict("invalid lifecycle transition")
	case errors.Is(err, domain.ErrInvalid):
		return companyapi.NewInvalid("invalid request or unknown reference")
	}
	return werror.WrapWithContextParams(ctx, err, "company operation failed")
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

func fromDateTime(p *datetime.DateTime) *string {
	if p == nil {
		return nil
	}
	s := p.String()
	return &s
}
