package domain

import "context"

// Repository is the company module's persistence port (implemented by adapters over pgx/sqlc). It is
// bound to a single command surface — the pool for reads, or a caller's transaction for an audited
// write (D-Audit). The application layer owns transaction boundaries; the repository never opens its own.
type Repository interface {
	// catalogs
	ListLegalForms(ctx context.Context) ([]LegalForm, error)
	UpsertLegalForm(ctx context.Context, code, name string, abbreviation, countryID *string, sortOrder *int) (LegalForm, error)
	ListRegistrationSchemes(ctx context.Context) ([]RegistrationScheme, error)
	GetRegistrationScheme(ctx context.Context, id string) (RegistrationScheme, error)
	UpsertRegistrationScheme(ctx context.Context, code, name string, validatorPattern *string, isGlobal bool, sortOrder *int) (RegistrationScheme, error)
	ListIndustryClasses(ctx context.Context) ([]IndustryClass, error)
	UpsertIndustryClass(ctx context.Context, code, name, system string, sortOrder *int) (IndustryClass, error)

	// companies (tenant org + company_org_profiles sidecar — M41). The org row (code/name) is owned by
	// the tenant service; these own the sidecar and the joined read view.
	InsertOrgProfile(ctx context.Context, companyID string, in CompanyInput) error
	GetCompany(ctx context.Context, id string) (Company, error)
	UpdateOrgProfile(ctx context.Context, id string, up CompanyUpdate) error
	ListCompanies(ctx context.Context, query, after string, lim int) ([]Company, error)
	SoftDeleteCompany(ctx context.Context, id string) (int64, error)
	// CompanyNamesByIDs returns default-locale legal names for a set of company ids (label resolution).
	CompanyNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)

	// registrations
	InsertRegistration(ctx context.Context, companyID, schemeID, identifier string, validated bool) (Registration, error)
	GetRegistration(ctx context.Context, id string) (Registration, error)
	UpdateRegistration(ctx context.Context, id, schemeID, identifier string, validated bool) (Registration, error)
	ListRegistrationsByCompany(ctx context.Context, companyID string) ([]Registration, error)
	SoftDeleteRegistration(ctx context.Context, id string) (int64, error)

	// industry assignments
	InsertIndustryAssignment(ctx context.Context, companyID, classID string, isPrimary bool) (IndustryAssignment, error)
	ClearPrimaryIndustries(ctx context.Context, companyID string) error
	ListIndustriesByCompany(ctx context.Context, companyID string) ([]IndustryAssignment, error)
	SoftDeleteIndustryAssignment(ctx context.Context, id string) (int64, error)

	// locations
	InsertCompanyLocation(ctx context.Context, companyID, locationID, role string) (CompanyLocation, error)
	ListCompanyLocationsByCompany(ctx context.Context, companyID string) ([]CompanyLocation, error)
	SoftDeleteCompanyLocation(ctx context.Context, id string) (int64, error)

	// positions + appointments
	InsertPosition(ctx context.Context, companyID string, in PositionInput) (Position, error)
	GetPosition(ctx context.Context, id string) (Position, error)
	UpdatePosition(ctx context.Context, id string, up PositionUpdate) (Position, error)
	AbolishPosition(ctx context.Context, id string) (Position, error)
	ListPositionsByCompany(ctx context.Context, companyID, state, after string, lim int) ([]Position, error)
	GetActiveAppointmentByPosition(ctx context.Context, positionID string) (Appointment, error)
	InsertAppointment(ctx context.Context, personID, positionID string, effectiveFrom *string) (Appointment, error)
	GetAppointment(ctx context.Context, id string) (Appointment, error)
	EndAppointment(ctx context.Context, id string, effectiveTo *string) (Appointment, error)
	ListAppointmentsByPerson(ctx context.Context, personID string) ([]PersonAppointment, error)

	// foundings
	InsertFounding(ctx context.Context, companyID string, in FoundingInput) (Founding, error)
	GetFounding(ctx context.Context, id string) (Founding, error)
	SoftDeleteFounding(ctx context.Context, id string) (int64, error)
	ListFoundingsByCompany(ctx context.Context, companyID string) ([]Founding, error)
	ListFoundingsByPersonHolder(ctx context.Context, personID string) ([]Founding, error)

	// shareholdings
	InsertShareholding(ctx context.Context, companyID string, in ShareholdingInput) (Shareholding, error)
	GetShareholding(ctx context.Context, id string) (Shareholding, error)
	SoftDeleteShareholding(ctx context.Context, id string) (int64, error)
	ListShareholdersByCompany(ctx context.Context, companyID string) ([]Shareholding, error)   // stakes IN this company
	ListHoldingsByCompanyHolder(ctx context.Context, companyID string) ([]Shareholding, error) // stakes this company holds
	ListShareholdingsByPersonHolder(ctx context.Context, personID string) ([]Shareholding, error)

	// beneficiaries
	InsertBeneficiary(ctx context.Context, companyID string, in BeneficiaryInput) (Beneficiary, error)
	GetBeneficiary(ctx context.Context, id string) (Beneficiary, error)
	SoftDeleteBeneficiary(ctx context.Context, id string) (int64, error)
	ListBeneficiariesByCompany(ctx context.Context, companyID string) ([]Beneficiary, error)
	ListBeneficiariesByPerson(ctx context.Context, personID string) ([]Beneficiary, error)

	// successions
	InsertSuccession(ctx context.Context, predecessorID string, in SuccessionInput) (Succession, error)
	GetSuccession(ctx context.Context, id string) (Succession, error)
	SoftDeleteSuccession(ctx context.Context, id string) (int64, error)
	ListSuccessionsByCompany(ctx context.Context, companyID string) ([]Succession, error)

	// branches
	InsertBranch(ctx context.Context, parentID, branchID string) (Branch, error)
	GetBranch(ctx context.Context, id string) (Branch, error)
	SoftDeleteBranch(ctx context.Context, id string) (int64, error)
	ListBranchesByParent(ctx context.Context, parentID string) ([]Branch, error)
}
