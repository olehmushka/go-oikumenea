// Package adapters is the company module's pgx/sqlc-backed persistence adapter (implements
// domain.Repository). It maps domain values to the generated companysql params/rows and translates
// Postgres constraint violations (23505 unique / 23503 FK) into domain sentinels.
package adapters

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/company/adapters/companysql"
	"github.com/olegamysk/go-oikumenea/internal/company/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository implements domain.Repository over a single command surface (pool or tx).
type Repository struct {
	q *companysql.Queries
}

// NewRepository binds a repository to the given command surface (a db.DBTX value).
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: companysql.New(conn)}
}

var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- catalogs

func (r *Repository) ListLegalForms(ctx context.Context) ([]domain.LegalForm, error) {
	rows, err := r.q.ListLegalForms(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LegalForm, 0, len(rows))
	for _, k := range rows {
		out = append(out, toLegalForm(k))
	}
	return out, nil
}

func (r *Repository) UpsertLegalForm(ctx context.Context, code, name string, abbreviation, countryID *string, sortOrder *int) (domain.LegalForm, error) {
	k, err := r.q.UpsertLegalForm(ctx, companysql.UpsertLegalFormParams{
		Code: code, Name: name, Abbreviation: text(abbreviation), CountryID: text(countryID), SortOrder: int4(sortOrder),
	})
	if err != nil {
		return domain.LegalForm{}, mapErr(err)
	}
	return toLegalForm(k), nil
}

func (r *Repository) ListRegistrationSchemes(ctx context.Context) ([]domain.RegistrationScheme, error) {
	rows, err := r.q.ListRegistrationSchemes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RegistrationScheme, 0, len(rows))
	for _, s := range rows {
		out = append(out, toScheme(s))
	}
	return out, nil
}

func (r *Repository) GetRegistrationScheme(ctx context.Context, id string) (domain.RegistrationScheme, error) {
	s, err := r.q.GetRegistrationScheme(ctx, id)
	if err != nil {
		return domain.RegistrationScheme{}, notFound(err, domain.ErrInvalid)
	}
	return toScheme(s), nil
}

func (r *Repository) UpsertRegistrationScheme(ctx context.Context, code, name string, validatorPattern *string, isGlobal bool, sortOrder *int) (domain.RegistrationScheme, error) {
	s, err := r.q.UpsertRegistrationScheme(ctx, companysql.UpsertRegistrationSchemeParams{
		Code: code, Name: name, ValidatorPattern: text(validatorPattern), IsGlobal: isGlobal, SortOrder: int4(sortOrder),
	})
	if err != nil {
		return domain.RegistrationScheme{}, mapErr(err)
	}
	return toScheme(s), nil
}

func (r *Repository) ListIndustryClasses(ctx context.Context) ([]domain.IndustryClass, error) {
	rows, err := r.q.ListIndustryClasses(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.IndustryClass, 0, len(rows))
	for _, c := range rows {
		out = append(out, toIndustryClass(c))
	}
	return out, nil
}

func (r *Repository) UpsertIndustryClass(ctx context.Context, code, name, system string, sortOrder *int) (domain.IndustryClass, error) {
	c, err := r.q.UpsertIndustryClass(ctx, companysql.UpsertIndustryClassParams{Code: code, Name: name, System: system, SortOrder: int4(sortOrder)})
	if err != nil {
		return domain.IndustryClass{}, mapErr(err)
	}
	return toIndustryClass(c), nil
}

// ---------------------------------------------------------------- companies

func (r *Repository) InsertCompany(ctx context.Context, in domain.CompanyInput) (domain.Company, error) {
	row, err := r.q.InsertCompany(ctx, companysql.InsertCompanyParams{
		Code: in.Code, LegalName: in.LegalName, ShortName: text(in.ShortName), LegalFormID: in.LegalFormID,
		OwnershipCategory: text(in.OwnershipCategory), CountryID: text(in.CountryID), FoundedOn: datePtr(in.FoundedOn),
	})
	if err != nil {
		return domain.Company{}, mapErr(err)
	}
	return toCompany(row), nil
}

func (r *Repository) GetCompany(ctx context.Context, id string) (domain.Company, error) {
	row, err := r.q.GetCompany(ctx, id)
	if err != nil {
		return domain.Company{}, notFound(err, domain.ErrCompanyNotFound)
	}
	return toCompany(row), nil
}

func (r *Repository) UpdateCompany(ctx context.Context, id string, up domain.CompanyUpdate) (domain.Company, error) {
	row, err := r.q.UpdateCompany(ctx, companysql.UpdateCompanyParams{
		LegalName: text(up.LegalName), ShortName: text(up.ShortName), LegalFormID: text(up.LegalFormID),
		OwnershipCategory: text(up.OwnershipCategory), CountryID: text(up.CountryID),
		FoundedOn: datePtr(up.FoundedOn), DissolvedOn: datePtr(up.DissolvedOn), State: text(up.State), ID: id,
	})
	if err != nil {
		return domain.Company{}, notFound(mapErr(err), domain.ErrCompanyNotFound)
	}
	return toCompany(row), nil
}

func (r *Repository) ListCompanies(ctx context.Context, query, after string, lim int) ([]domain.Company, error) {
	rows, err := r.q.ListCompanies(ctx, companysql.ListCompaniesParams{Query: query, After: after, Lim: int32(lim)})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Company, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCompany(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteCompany(ctx context.Context, id string) (int64, error) {
	n, err := r.q.SoftDeleteCompany(ctx, id)
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

func (r *Repository) CompanyNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.q.CompanyNamesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.LegalName
	}
	return out, nil
}

// ---------------------------------------------------------------- registrations

func (r *Repository) InsertRegistration(ctx context.Context, companyID, schemeID, identifier string, validated bool) (domain.Registration, error) {
	row, err := r.q.InsertRegistration(ctx, companysql.InsertRegistrationParams{CompanyID: companyID, SchemeID: schemeID, Identifier: identifier, Validated: validated})
	if err != nil {
		return domain.Registration{}, mapErr(err)
	}
	return toRegistration(row), nil
}

func (r *Repository) GetRegistration(ctx context.Context, id string) (domain.Registration, error) {
	row, err := r.q.GetRegistration(ctx, id)
	if err != nil {
		return domain.Registration{}, notFound(err, domain.ErrLinkNotFound)
	}
	return toRegistration(row), nil
}

func (r *Repository) UpdateRegistration(ctx context.Context, id, schemeID, identifier string, validated bool) (domain.Registration, error) {
	row, err := r.q.UpdateRegistration(ctx, companysql.UpdateRegistrationParams{SchemeID: schemeID, Identifier: identifier, Validated: validated, ID: id})
	if err != nil {
		return domain.Registration{}, notFound(mapErr(err), domain.ErrLinkNotFound)
	}
	return toRegistration(row), nil
}

func (r *Repository) ListRegistrationsByCompany(ctx context.Context, companyID string) ([]domain.Registration, error) {
	rows, err := r.q.ListRegistrationsByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Registration, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRegistration(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteRegistration(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteRegistration(ctx, id)
}

// ---------------------------------------------------------------- industry assignments

func (r *Repository) InsertIndustryAssignment(ctx context.Context, companyID, classID string, isPrimary bool) (domain.IndustryAssignment, error) {
	row, err := r.q.InsertIndustryAssignment(ctx, companysql.InsertIndustryAssignmentParams{CompanyID: companyID, IndustryClassID: classID, IsPrimary: isPrimary})
	if err != nil {
		return domain.IndustryAssignment{}, mapErr(err)
	}
	return toIndustryAssignment(row), nil
}

func (r *Repository) ClearPrimaryIndustries(ctx context.Context, companyID string) error {
	return r.q.ClearPrimaryIndustries(ctx, companyID)
}

func (r *Repository) ListIndustriesByCompany(ctx context.Context, companyID string) ([]domain.IndustryAssignment, error) {
	rows, err := r.q.ListIndustriesByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.IndustryAssignment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toIndustryAssignment(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteIndustryAssignment(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteIndustryAssignment(ctx, id)
}

// ---------------------------------------------------------------- locations

func (r *Repository) InsertCompanyLocation(ctx context.Context, companyID, locationID, role string) (domain.CompanyLocation, error) {
	row, err := r.q.InsertCompanyLocation(ctx, companysql.InsertCompanyLocationParams{CompanyID: companyID, LocationID: locationID, Role: role})
	if err != nil {
		return domain.CompanyLocation{}, mapErr(err)
	}
	return toCompanyLocation(row), nil
}

func (r *Repository) ListCompanyLocationsByCompany(ctx context.Context, companyID string) ([]domain.CompanyLocation, error) {
	rows, err := r.q.ListCompanyLocationsByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CompanyLocation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCompanyLocation(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteCompanyLocation(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteCompanyLocation(ctx, id)
}

// ---------------------------------------------------------------- positions + appointments

func (r *Repository) InsertPosition(ctx context.Context, companyID string, in domain.PositionInput) (domain.Position, error) {
	row, err := r.q.InsertPosition(ctx, companysql.InsertPositionParams{CompanyID: companyID, Code: in.Code, Title: in.Title, SortOrder: int4(in.SortOrder)})
	if err != nil {
		return domain.Position{}, mapErr(err)
	}
	return toPosition(row), nil
}

func (r *Repository) GetPosition(ctx context.Context, id string) (domain.Position, error) {
	row, err := r.q.GetPosition(ctx, id)
	if err != nil {
		return domain.Position{}, notFound(err, domain.ErrPositionNotFound)
	}
	return toPosition(row), nil
}

func (r *Repository) UpdatePosition(ctx context.Context, id string, up domain.PositionUpdate) (domain.Position, error) {
	row, err := r.q.UpdatePosition(ctx, companysql.UpdatePositionParams{Title: text(up.Title), SortOrder: int4(up.SortOrder), ID: id})
	if err != nil {
		return domain.Position{}, notFound(mapErr(err), domain.ErrPositionNotFound)
	}
	return toPosition(row), nil
}

func (r *Repository) AbolishPosition(ctx context.Context, id string) (domain.Position, error) {
	row, err := r.q.AbolishPosition(ctx, id)
	if err != nil {
		return domain.Position{}, notFound(err, domain.ErrPositionNotFound)
	}
	return toPosition(row), nil
}

func (r *Repository) ListPositionsByCompany(ctx context.Context, companyID, state, after string, lim int) ([]domain.Position, error) {
	rows, err := r.q.ListPositionsByCompany(ctx, companysql.ListPositionsByCompanyParams{CompanyID: companyID, State: state, After: after, Lim: int32(lim)})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Position, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPosition(row))
	}
	return out, nil
}

func (r *Repository) GetActiveAppointmentByPosition(ctx context.Context, positionID string) (domain.Appointment, error) {
	row, err := r.q.GetActiveAppointmentByPosition(ctx, positionID)
	if err != nil {
		return domain.Appointment{}, notFound(err, domain.ErrAppointmentNotFound)
	}
	return toAppointment(row), nil
}

func (r *Repository) InsertAppointment(ctx context.Context, personID, positionID string, effectiveFrom *string) (domain.Appointment, error) {
	row, err := r.q.InsertAppointment(ctx, companysql.InsertAppointmentParams{PersonID: personID, PositionID: positionID, EffectiveFrom: tsArg(effectiveFrom)})
	if err != nil {
		return domain.Appointment{}, mapErr(err)
	}
	return toAppointment(row), nil
}

func (r *Repository) GetAppointment(ctx context.Context, id string) (domain.Appointment, error) {
	row, err := r.q.GetAppointment(ctx, id)
	if err != nil {
		return domain.Appointment{}, notFound(err, domain.ErrAppointmentNotFound)
	}
	return toAppointment(row), nil
}

func (r *Repository) EndAppointment(ctx context.Context, id string, effectiveTo *string) (domain.Appointment, error) {
	row, err := r.q.EndAppointment(ctx, companysql.EndAppointmentParams{EffectiveTo: tsArg(effectiveTo), ID: id})
	if err != nil {
		return domain.Appointment{}, notFound(err, domain.ErrAppointmentNotFound)
	}
	return toAppointment(row), nil
}

func (r *Repository) ListAppointmentsByPerson(ctx context.Context, personID string) ([]domain.PersonAppointment, error) {
	rows, err := r.q.ListAppointmentsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PersonAppointment, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PersonAppointment{
			ID: row.ID, PersonID: row.PersonID, PositionID: row.PositionID, Status: row.Status,
			PositionTitle: row.PositionTitle, CompanyID: row.CompanyID, CompanyName: row.CompanyName,
			EffectiveFrom: ts(row.EffectiveFrom), EffectiveTo: tsPtr(row.EffectiveTo),
			CreatedAt: ts(row.CreatedAt), UpdatedAt: ts(row.UpdatedAt),
		})
	}
	return out, nil
}

// ---------------------------------------------------------------- foundings

func (r *Repository) InsertFounding(ctx context.Context, companyID string, in domain.FoundingInput) (domain.Founding, error) {
	row, err := r.q.InsertFounding(ctx, companysql.InsertFoundingParams{CompanyID: companyID, HolderKind: in.HolderKind, HolderID: in.HolderID, FoundedOn: datePtr(in.FoundedOn)})
	if err != nil {
		return domain.Founding{}, mapErr(err)
	}
	return toFounding(row), nil
}

func (r *Repository) GetFounding(ctx context.Context, id string) (domain.Founding, error) {
	row, err := r.q.GetFounding(ctx, id)
	if err != nil {
		return domain.Founding{}, notFound(err, domain.ErrLinkNotFound)
	}
	return toFounding(row), nil
}

func (r *Repository) SoftDeleteFounding(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteFounding(ctx, id)
}

func (r *Repository) ListFoundingsByCompany(ctx context.Context, companyID string) ([]domain.Founding, error) {
	rows, err := r.q.ListFoundingsByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	return mapFoundings(rows), nil
}

func (r *Repository) ListFoundingsByPersonHolder(ctx context.Context, personID string) ([]domain.Founding, error) {
	rows, err := r.q.ListFoundingsByPersonHolder(ctx, personID)
	if err != nil {
		return nil, err
	}
	return mapFoundings(rows), nil
}

func mapFoundings(rows []companysql.OikumeneaCompanyFounding) []domain.Founding {
	out := make([]domain.Founding, 0, len(rows))
	for _, row := range rows {
		out = append(out, toFounding(row))
	}
	return out
}

// ---------------------------------------------------------------- shareholdings

func (r *Repository) InsertShareholding(ctx context.Context, companyID string, in domain.ShareholdingInput) (domain.Shareholding, error) {
	row, err := r.q.InsertShareholding(ctx, companysql.InsertShareholdingParams{
		CompanyID: companyID, HolderKind: in.HolderKind, HolderID: in.HolderID,
		StakePct: numFloatArg(in.StakePct), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo),
	})
	if err != nil {
		return domain.Shareholding{}, mapErr(err)
	}
	return toShareholding(row), nil
}

func (r *Repository) GetShareholding(ctx context.Context, id string) (domain.Shareholding, error) {
	row, err := r.q.GetShareholding(ctx, id)
	if err != nil {
		return domain.Shareholding{}, notFound(err, domain.ErrLinkNotFound)
	}
	return toShareholding(row), nil
}

func (r *Repository) SoftDeleteShareholding(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteShareholding(ctx, id)
}

func (r *Repository) ListShareholdersByCompany(ctx context.Context, companyID string) ([]domain.Shareholding, error) {
	rows, err := r.q.ListShareholdersByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	return mapShareholdings(rows), nil
}

func (r *Repository) ListHoldingsByCompanyHolder(ctx context.Context, companyID string) ([]domain.Shareholding, error) {
	rows, err := r.q.ListHoldingsByCompanyHolder(ctx, companyID)
	if err != nil {
		return nil, err
	}
	return mapShareholdings(rows), nil
}

func (r *Repository) ListShareholdingsByPersonHolder(ctx context.Context, personID string) ([]domain.Shareholding, error) {
	rows, err := r.q.ListShareholdingsByPersonHolder(ctx, personID)
	if err != nil {
		return nil, err
	}
	return mapShareholdings(rows), nil
}

func mapShareholdings(rows []companysql.OikumeneaCompanyShareholding) []domain.Shareholding {
	out := make([]domain.Shareholding, 0, len(rows))
	for _, row := range rows {
		out = append(out, toShareholding(row))
	}
	return out
}

// ---------------------------------------------------------------- beneficiaries

func (r *Repository) InsertBeneficiary(ctx context.Context, companyID string, in domain.BeneficiaryInput) (domain.Beneficiary, error) {
	declared := true
	if in.Declared != nil {
		declared = *in.Declared
	}
	row, err := r.q.InsertBeneficiary(ctx, companysql.InsertBeneficiaryParams{CompanyID: companyID, PersonID: in.PersonID, UltimatePct: numFloatArg(in.UltimatePct), Declared: declared})
	if err != nil {
		return domain.Beneficiary{}, mapErr(err)
	}
	return toBeneficiary(row), nil
}

func (r *Repository) GetBeneficiary(ctx context.Context, id string) (domain.Beneficiary, error) {
	row, err := r.q.GetBeneficiary(ctx, id)
	if err != nil {
		return domain.Beneficiary{}, notFound(err, domain.ErrLinkNotFound)
	}
	return toBeneficiary(row), nil
}

func (r *Repository) SoftDeleteBeneficiary(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteBeneficiary(ctx, id)
}

func (r *Repository) ListBeneficiariesByCompany(ctx context.Context, companyID string) ([]domain.Beneficiary, error) {
	rows, err := r.q.ListBeneficiariesByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	return mapBeneficiaries(rows), nil
}

func (r *Repository) ListBeneficiariesByPerson(ctx context.Context, personID string) ([]domain.Beneficiary, error) {
	rows, err := r.q.ListBeneficiariesByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	return mapBeneficiaries(rows), nil
}

func mapBeneficiaries(rows []companysql.OikumeneaCompanyBeneficiary) []domain.Beneficiary {
	out := make([]domain.Beneficiary, 0, len(rows))
	for _, row := range rows {
		out = append(out, toBeneficiary(row))
	}
	return out
}

// ---------------------------------------------------------------- successions

func (r *Repository) InsertSuccession(ctx context.Context, predecessorID string, in domain.SuccessionInput) (domain.Succession, error) {
	row, err := r.q.InsertSuccession(ctx, companysql.InsertSuccessionParams{PredecessorID: predecessorID, SuccessorID: in.SuccessorID, Kind: text(in.Kind), EffectiveOn: datePtr(in.EffectiveOn)})
	if err != nil {
		return domain.Succession{}, mapErr(err)
	}
	return toSuccession(row), nil
}

func (r *Repository) GetSuccession(ctx context.Context, id string) (domain.Succession, error) {
	row, err := r.q.GetSuccession(ctx, id)
	if err != nil {
		return domain.Succession{}, notFound(err, domain.ErrLinkNotFound)
	}
	return toSuccession(row), nil
}

func (r *Repository) SoftDeleteSuccession(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteSuccession(ctx, id)
}

func (r *Repository) ListSuccessionsByCompany(ctx context.Context, companyID string) ([]domain.Succession, error) {
	rows, err := r.q.ListSuccessionsByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Succession, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSuccession(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- branches

func (r *Repository) InsertBranch(ctx context.Context, parentID, branchID string) (domain.Branch, error) {
	row, err := r.q.InsertBranch(ctx, companysql.InsertBranchParams{ParentID: parentID, BranchID: branchID})
	if err != nil {
		return domain.Branch{}, mapErr(err)
	}
	return toBranch(row), nil
}

func (r *Repository) GetBranch(ctx context.Context, id string) (domain.Branch, error) {
	row, err := r.q.GetBranch(ctx, id)
	if err != nil {
		return domain.Branch{}, notFound(err, domain.ErrLinkNotFound)
	}
	return toBranch(row), nil
}

func (r *Repository) SoftDeleteBranch(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteBranch(ctx, id)
}

func (r *Repository) ListBranchesByParent(ctx context.Context, parentID string) ([]domain.Branch, error) {
	rows, err := r.q.ListBranchesByParent(ctx, parentID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Branch, 0, len(rows))
	for _, row := range rows {
		out = append(out, toBranch(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- converters

func toLegalForm(r companysql.OikumeneaCompanyLegalForm) domain.LegalForm {
	return domain.LegalForm{ID: r.ID, Code: r.Code, Name: r.Name, Status: r.Status, Abbreviation: textVal(r.Abbreviation), CountryID: textVal(r.CountryID), SortOrder: int4ptr(r.SortOrder)}
}

func toScheme(r companysql.OikumeneaCompanyRegistrationScheme) domain.RegistrationScheme {
	return domain.RegistrationScheme{ID: r.ID, Code: r.Code, Name: r.Name, Status: r.Status, ValidatorPattern: textVal(r.ValidatorPattern), IsGlobal: r.IsGlobal, SortOrder: int4ptr(r.SortOrder)}
}

func toIndustryClass(r companysql.OikumeneaCompanyIndustryClass) domain.IndustryClass {
	return domain.IndustryClass{ID: r.ID, Code: r.Code, Name: r.Name, System: r.System, Status: r.Status, SortOrder: int4ptr(r.SortOrder)}
}

func toCompany(r companysql.OikumeneaCompanyCompany) domain.Company {
	return domain.Company{
		ID: r.ID, Code: r.Code, LegalName: r.LegalName, ShortName: textVal(r.ShortName), LegalFormID: r.LegalFormID,
		OwnershipCategory: r.OwnershipCategory, CountryID: textVal(r.CountryID),
		FoundedOn: dateStr(r.FoundedOn), DissolvedOn: dateStr(r.DissolvedOn), State: r.State,
		CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toRegistration(r companysql.OikumeneaCompanyRegistration) domain.Registration {
	return domain.Registration{ID: r.ID, CompanyID: r.CompanyID, SchemeID: r.SchemeID, Identifier: r.Identifier, Validated: r.Validated, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

func toIndustryAssignment(r companysql.OikumeneaCompanyIndustryAssignment) domain.IndustryAssignment {
	return domain.IndustryAssignment{ID: r.ID, CompanyID: r.CompanyID, IndustryClassID: r.IndustryClassID, IsPrimary: r.IsPrimary, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

func toCompanyLocation(r companysql.OikumeneaCompanyLocation) domain.CompanyLocation {
	return domain.CompanyLocation{ID: r.ID, CompanyID: r.CompanyID, LocationID: r.LocationID, Role: r.Role, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

func toPosition(r companysql.OikumeneaCompanyPosition) domain.Position {
	return domain.Position{ID: r.ID, CompanyID: r.CompanyID, Code: r.Code, Title: r.Title, Status: r.Status, SortOrder: int4ptr(r.SortOrder), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

func toAppointment(r companysql.OikumeneaCompanyAppointment) domain.Appointment {
	return domain.Appointment{ID: r.ID, PersonID: r.PersonID, PositionID: r.PositionID, Status: r.Status, EffectiveFrom: ts(r.EffectiveFrom), EffectiveTo: tsPtr(r.EffectiveTo), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

func toFounding(r companysql.OikumeneaCompanyFounding) domain.Founding {
	return domain.Founding{ID: r.ID, CompanyID: r.CompanyID, HolderKind: r.HolderKind, HolderID: r.HolderID, FoundedOn: dateStr(r.FoundedOn), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

func toShareholding(r companysql.OikumeneaCompanyShareholding) domain.Shareholding {
	return domain.Shareholding{ID: r.ID, CompanyID: r.CompanyID, HolderKind: r.HolderKind, HolderID: r.HolderID, StakePct: numFloatPtr(r.StakePct), EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

func toBeneficiary(r companysql.OikumeneaCompanyBeneficiary) domain.Beneficiary {
	return domain.Beneficiary{ID: r.ID, CompanyID: r.CompanyID, PersonID: r.PersonID, UltimatePct: numFloatPtr(r.UltimatePct), Declared: r.Declared, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

func toSuccession(r companysql.OikumeneaCompanySuccession) domain.Succession {
	return domain.Succession{ID: r.ID, PredecessorID: r.PredecessorID, SuccessorID: r.SuccessorID, Kind: r.Kind, EffectiveOn: dateStr(r.EffectiveOn), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

func toBranch(r companysql.OikumeneaCompanyBranch) domain.Branch {
	return domain.Branch{ID: r.ID, BranchID: r.BranchID, ParentID: r.ParentID, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt)}
}

// ---------------------------------------------------------------- pgtype helpers

func text(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func textVal(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func int4(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

func int4ptr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	out := int(v.Int32)
	return &out
}

func dateText(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse(domain.ISODate, s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func datePtr(p *string) pgtype.Date {
	if p == nil {
		return pgtype.Date{}
	}
	return dateText(*p)
}

func dateStr(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(domain.ISODate)
}

func ts(t pgtype.Timestamptz) time.Time {
	return t.Time
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	out := t.Time
	return &out
}

func tsArg(p *string) pgtype.Timestamptz {
	if p == nil || *p == "" {
		return pgtype.Timestamptz{}
	}
	if t, err := time.Parse(time.RFC3339, *p); err == nil {
		return pgtype.Timestamptz{Time: t, Valid: true}
	}
	if t, err := time.Parse(domain.ISODate, *p); err == nil {
		return pgtype.Timestamptz{Time: t, Valid: true}
	}
	return pgtype.Timestamptz{}
}

// numFloatArg converts an optional percentage into a pgtype.Numeric (via its decimal string form).
func numFloatArg(p *float64) pgtype.Numeric {
	var n pgtype.Numeric
	if p == nil {
		return n
	}
	if err := n.Scan(strconv.FormatFloat(*p, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// numFloatPtr converts a stored numeric back into an optional float64 (via its string Value()).
func numFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Value()
	if err != nil || v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func notFound(err error, sentinel error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return sentinel
	}
	return err
}

func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505":
		return domain.ErrConflict
	case "23503":
		// A FK violation on insert/update is a bad reference; a violation raised by a delete (RESTRICT
		// from a child) is an in-use conflict.
		if strings.Contains(strings.ToLower(pgErr.Message), "still referenced") {
			return domain.ErrInUse
		}
		return domain.ErrInvalid
	case "23514":
		// CHECK violation (bad enum / out-of-range percentage / predecessor=successor).
		return domain.ErrInvalid
	}
	return err
}
