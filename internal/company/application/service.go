// Package application is the company module's orchestrator (D-Companies): audited writes, the
// positions/appointments one-holder rule (mirrors membership), registration-identifier validation, and
// the ownership/affiliation graph assembly. Every write runs in a transaction that also records the
// audit Action (D-Audit); reads run on the pool. Company entities are instance-global external
// reference data (no unit scope), so writes are recorded as a `system` action with no unit attributed.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/company/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

const (
	auditSubsystem  = "company-admin"
	defaultPageSize = 50
	maxPageSize     = 200
	// companyDomainCode is the tenant domain a company org belongs to (M41 / D-UnifiedOrgGraph). Companies
	// have no internal unit tree, so (unlike education) no per-org graph is seeded.
	companyDomainCode = "company"
)

// RepositoryFactory binds a domain.Repository to a command surface (pool for reads, tx for writes).
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the company application service. A company is a `company`-domain tenant organization; the
// org (code/name) is owned by the tenant service, while this service owns the company_org_profiles
// sidecar, the registrations/positions/locations, and the ownership/affiliation graph.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
	tenant  *tenantapp.Service

	domMu sync.Mutex
	domID string // cached `company` domain RID (seeded at boot, stable)
}

// NewService wires the service with the pool, repository factory, the audit service, and the tenant
// service (a company = a `company`-domain org — M41).
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, tenant *tenantapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, tenant: tenant}
}

// companyDomainID resolves (and caches) the `company` tenant domain RID.
func (s *Service) companyDomainID(ctx context.Context) (string, error) {
	s.domMu.Lock()
	defer s.domMu.Unlock()
	if s.domID != "" {
		return s.domID, nil
	}
	doms, err := s.tenant.ListDomains(ctx)
	if err != nil {
		return "", err
	}
	for _, d := range doms {
		if d.Code == companyDomainCode {
			s.domID = d.ID
			return d.ID, nil
		}
	}
	return "", domain.ErrInvalid
}

// ============================ catalogs ============================

func (s *Service) ListLegalForms(ctx context.Context) ([]domain.LegalForm, error) {
	return s.newRepo(s.pool).ListLegalForms(ctx)
}

func (s *Service) ListRegistrationSchemes(ctx context.Context) ([]domain.RegistrationScheme, error) {
	return s.newRepo(s.pool).ListRegistrationSchemes(ctx)
}

func (s *Service) ListIndustryClasses(ctx context.Context) ([]domain.IndustryClass, error) {
	return s.newRepo(s.pool).ListIndustryClasses(ctx)
}

func (s *Service) UpsertLegalForm(ctx context.Context, code, name string, abbreviation, countryID *string, sortOrder *int) (domain.LegalForm, error) {
	var out domain.LegalForm
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		k, err := s.newRepo(tx).UpsertLegalForm(ctx, code, name, abbreviation, countryID, sortOrder)
		if err != nil {
			return err
		}
		out = k
		return s.record(ctx, tx, "company.legal-form.upsert", k.ID, k)
	})
	return out, err
}

func (s *Service) UpsertRegistrationScheme(ctx context.Context, code, name string, validatorPattern *string, isGlobal bool, sortOrder *int) (domain.RegistrationScheme, error) {
	var out domain.RegistrationScheme
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		k, err := s.newRepo(tx).UpsertRegistrationScheme(ctx, code, name, validatorPattern, isGlobal, sortOrder)
		if err != nil {
			return err
		}
		out = k
		return s.record(ctx, tx, "company.registration-scheme.upsert", k.ID, k)
	})
	return out, err
}

func (s *Service) UpsertIndustryClass(ctx context.Context, code, name, system string, sortOrder *int) (domain.IndustryClass, error) {
	if system == "" {
		system = "nace"
	}
	var out domain.IndustryClass
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		k, err := s.newRepo(tx).UpsertIndustryClass(ctx, code, name, system, sortOrder)
		if err != nil {
			return err
		}
		out = k
		return s.record(ctx, tx, "company.industry-class.upsert", k.ID, k)
	})
	return out, err
}

// ============================ companies ============================

// CreateCompany creates a `company`-domain tenant organization (the legal entity; code/registered name =
// org code/name) and writes the company_org_profiles sidecar. Cross-module: the org create runs in the
// tenant service's own transaction; the sidecar + audit run in a company transaction (sequential, like
// education's CreateInstitution). Companies have no internal unit tree, so no graph is seeded.
func (s *Service) CreateCompany(ctx context.Context, in domain.CompanyInput) (domain.Company, error) {
	if err := in.Validate(); err != nil {
		return domain.Company{}, err
	}
	dom, err := s.companyDomainID(ctx)
	if err != nil {
		return domain.Company{}, err
	}
	org, err := s.tenant.CreateOrganization(ctx, tenantdomain.Organization{
		Code: in.Code, Name: in.LegalName, DomainID: dom, Visibility: tenantdomain.VisibilityPublic,
	})
	if err != nil {
		return domain.Company{}, err
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).InsertOrgProfile(ctx, org.ID, in); err != nil {
			return err
		}
		return s.record(ctx, tx, "company.create", org.ID, org)
	})
	if err != nil {
		return domain.Company{}, err
	}
	return s.GetCompany(ctx, org.ID)
}

func (s *Service) GetCompany(ctx context.Context, id string) (domain.Company, error) {
	return s.newRepo(s.pool).GetCompany(ctx, id)
}

func (s *Service) ListCompanies(ctx context.Context, query, after string, pageSize int) ([]domain.Company, error) {
	return s.newRepo(s.pool).ListCompanies(ctx, query, after, clampPageSize(pageSize)+1)
}

// UpdateCompany applies a partial change: the org name (registered name) via the tenant service (if set)
// and the company sidecar fields (short name/legal form/ownership/country/dates/state) via the repository.
func (s *Service) UpdateCompany(ctx context.Context, id string, up domain.CompanyUpdate) (domain.Company, error) {
	if _, err := s.newRepo(s.pool).GetCompany(ctx, id); err != nil {
		return domain.Company{}, err
	}
	if up.LegalName != nil {
		if _, err := s.tenant.UpdateOrganization(ctx, id, tenantdomain.OrgPatch{Name: up.LegalName}); err != nil {
			return domain.Company{}, mapOrgNotFound(err)
		}
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).UpdateOrgProfile(ctx, id, up); err != nil {
			return err
		}
		return s.record(ctx, tx, "company.update", id, up)
	})
	if err != nil {
		return domain.Company{}, err
	}
	return s.GetCompany(ctx, id)
}

// mapOrgNotFound translates the tenant service's org-not-found sentinel into the company one.
func mapOrgNotFound(err error) error {
	if errors.Is(err, tenantdomain.ErrOrgNotFound) {
		return domain.ErrCompanyNotFound
	}
	return err
}

func (s *Service) DeleteCompany(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteCompany(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrCompanyNotFound
		}
		return s.record(ctx, tx, "company.delete", id, map[string]string{"id": id})
	})
}

// ============================ registrations ============================

func (s *Service) ListRegistrations(ctx context.Context, companyID string) ([]domain.Registration, error) {
	return s.newRepo(s.pool).ListRegistrationsByCompany(ctx, companyID)
}

// AddRegistration validates the identifier against the scheme's pattern (recording `validated`) and
// stores it. A scheme+identifier collision surfaces as a conflict.
func (s *Service) AddRegistration(ctx context.Context, companyID string, in domain.RegistrationInput) (domain.Registration, error) {
	if err := in.Validate(); err != nil {
		return domain.Registration{}, err
	}
	var out domain.Registration
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCompany(ctx, companyID); err != nil {
			return err
		}
		scheme, err := repo.GetRegistrationScheme(ctx, in.SchemeID)
		if err != nil {
			return err
		}
		validated := domain.ValidatesIdentifier(scheme.ValidatorPattern, in.Identifier)
		created, err := repo.InsertRegistration(ctx, companyID, in.SchemeID, in.Identifier, validated)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "company.registration.add", created.ID, created)
	})
	return out, err
}

func (s *Service) UpdateRegistration(ctx context.Context, id string, in domain.RegistrationInput) (domain.Registration, error) {
	if err := in.Validate(); err != nil {
		return domain.Registration{}, err
	}
	var out domain.Registration
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		scheme, err := repo.GetRegistrationScheme(ctx, in.SchemeID)
		if err != nil {
			return err
		}
		validated := domain.ValidatesIdentifier(scheme.ValidatorPattern, in.Identifier)
		updated, err := repo.UpdateRegistration(ctx, id, in.SchemeID, in.Identifier, validated)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "company.registration.update", id, updated)
	})
	return out, err
}

func (s *Service) DeleteRegistration(ctx context.Context, id string) error {
	return s.softDelete(ctx, "company.registration.delete", id, func(repo domain.Repository) (int64, error) {
		return repo.SoftDeleteRegistration(ctx, id)
	})
}

// ============================ industries ============================

func (s *Service) ListIndustries(ctx context.Context, companyID string) ([]domain.IndustryAssignment, error) {
	return s.newRepo(s.pool).ListIndustriesByCompany(ctx, companyID)
}

// AssignIndustry attaches an industry class to a company; if marked primary, the company's existing
// primary is demoted first (at most one primary).
func (s *Service) AssignIndustry(ctx context.Context, companyID string, in domain.IndustryInput) (domain.IndustryAssignment, error) {
	if err := in.Validate(); err != nil {
		return domain.IndustryAssignment{}, err
	}
	var out domain.IndustryAssignment
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCompany(ctx, companyID); err != nil {
			return err
		}
		if in.IsPrimary {
			if err := repo.ClearPrimaryIndustries(ctx, companyID); err != nil {
				return err
			}
		}
		created, err := repo.InsertIndustryAssignment(ctx, companyID, in.IndustryClassID, in.IsPrimary)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "company.industry.assign", created.ID, created)
	})
	return out, err
}

func (s *Service) RemoveIndustry(ctx context.Context, id string) error {
	return s.softDelete(ctx, "company.industry.remove", id, func(repo domain.Repository) (int64, error) {
		return repo.SoftDeleteIndustryAssignment(ctx, id)
	})
}

// ============================ locations ============================

func (s *Service) ListCompanyLocations(ctx context.Context, companyID string) ([]domain.CompanyLocation, error) {
	return s.newRepo(s.pool).ListCompanyLocationsByCompany(ctx, companyID)
}

func (s *Service) AddCompanyLocation(ctx context.Context, companyID string, in domain.CompanyLocationInput) (domain.CompanyLocation, error) {
	if err := in.Validate(); err != nil {
		return domain.CompanyLocation{}, err
	}
	role := "registered"
	if in.Role != nil && *in.Role != "" {
		role = *in.Role
	}
	var out domain.CompanyLocation
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCompany(ctx, companyID); err != nil {
			return err
		}
		created, err := repo.InsertCompanyLocation(ctx, companyID, in.LocationID, role)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "company.location.add", created.ID, created)
	})
	return out, err
}

func (s *Service) RemoveCompanyLocation(ctx context.Context, id string) error {
	return s.softDelete(ctx, "company.location.remove", id, func(repo domain.Repository) (int64, error) {
		return repo.SoftDeleteCompanyLocation(ctx, id)
	})
}

// ============================ positions + appointments ============================

func (s *Service) CreatePosition(ctx context.Context, companyID string, in domain.PositionInput) (domain.Position, error) {
	if err := in.Validate(); err != nil {
		return domain.Position{}, err
	}
	var out domain.Position
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCompany(ctx, companyID); err != nil {
			return err
		}
		created, err := repo.InsertPosition(ctx, companyID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "company.position.create", created.ID, created)
	})
	return out, err
}

func (s *Service) GetPosition(ctx context.Context, id string) (domain.Position, error) {
	repo := s.newRepo(s.pool)
	p, err := repo.GetPosition(ctx, id)
	if err != nil {
		return domain.Position{}, err
	}
	if a, err := repo.GetActiveAppointmentByPosition(ctx, id); err == nil {
		p.Holder = &a
	} else if !errors.Is(err, domain.ErrAppointmentNotFound) {
		return domain.Position{}, err
	}
	return p, nil
}

func (s *Service) ListPositions(ctx context.Context, companyID, state, after string, pageSize int) ([]domain.Position, error) {
	repo := s.newRepo(s.pool)
	positions, err := repo.ListPositionsByCompany(ctx, companyID, state, after, clampPageSize(pageSize)+1)
	if err != nil {
		return nil, err
	}
	for i := range positions {
		if a, err := repo.GetActiveAppointmentByPosition(ctx, positions[i].ID); err == nil {
			positions[i].Holder = &a
		} else if !errors.Is(err, domain.ErrAppointmentNotFound) {
			return nil, err
		}
	}
	return positions, nil
}

func (s *Service) UpdatePosition(ctx context.Context, id string, up domain.PositionUpdate) (domain.Position, error) {
	var out domain.Position
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdatePosition(ctx, id, up)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "company.position.update", id, updated)
	})
	return out, err
}

func (s *Service) AbolishPosition(ctx context.Context, id string) (domain.Position, error) {
	var out domain.Position
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetActiveAppointmentByPosition(ctx, id); err == nil {
			return domain.ErrInUse
		} else if !errors.Is(err, domain.ErrAppointmentNotFound) {
			return err
		}
		updated, err := repo.AbolishPosition(ctx, id)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "company.position.abolish", id, updated)
	})
	return out, err
}

// FillPosition appoints a person to a vacant position (one holder per billet, enforced by the DB index
// and surfaced as ErrPositionAlreadyFilled).
func (s *Service) FillPosition(ctx context.Context, positionID, personID string, effectiveFrom *string) (domain.Appointment, error) {
	var out domain.Appointment
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		pos, err := repo.GetPosition(ctx, positionID)
		if err != nil {
			return err
		}
		if pos.Status != "active" {
			return domain.ErrLifecycle
		}
		created, err := repo.InsertAppointment(ctx, personID, positionID, effectiveFrom)
		if err != nil {
			if errors.Is(err, domain.ErrConflict) {
				return domain.ErrPositionAlreadyFilled
			}
			return err
		}
		out = created
		return s.record(ctx, tx, "company.appointment.fill", created.ID, created)
	})
	return out, err
}

func (s *Service) EndAppointment(ctx context.Context, id string, effectiveTo *string) (domain.Appointment, error) {
	var out domain.Appointment
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		ended, err := s.newRepo(tx).EndAppointment(ctx, id, effectiveTo)
		if err != nil {
			return err
		}
		out = ended
		return s.record(ctx, tx, "company.appointment.end", id, ended)
	})
	return out, err
}

// ============================ ownership / affiliation graph ============================

func (s *Service) RecordFounding(ctx context.Context, companyID string, in domain.FoundingInput) (domain.Founding, error) {
	if err := in.Validate(); err != nil {
		return domain.Founding{}, err
	}
	var out domain.Founding
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCompany(ctx, companyID); err != nil {
			return err
		}
		if err := s.requireHolder(ctx, repo, in.HolderKind, in.HolderID); err != nil {
			return err
		}
		created, err := repo.InsertFounding(ctx, companyID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "company.founding.record", created.ID, created)
	})
	return out, err
}

func (s *Service) RemoveFounding(ctx context.Context, id string) error {
	return s.softDelete(ctx, "company.founding.remove", id, func(repo domain.Repository) (int64, error) {
		return repo.SoftDeleteFounding(ctx, id)
	})
}

func (s *Service) RecordShareholding(ctx context.Context, companyID string, in domain.ShareholdingInput) (domain.Shareholding, error) {
	if err := in.Validate(); err != nil {
		return domain.Shareholding{}, err
	}
	var out domain.Shareholding
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCompany(ctx, companyID); err != nil {
			return err
		}
		if err := s.requireHolder(ctx, repo, in.HolderKind, in.HolderID); err != nil {
			return err
		}
		created, err := repo.InsertShareholding(ctx, companyID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "company.shareholding.record", created.ID, created)
	})
	return out, err
}

func (s *Service) RemoveShareholding(ctx context.Context, id string) error {
	return s.softDelete(ctx, "company.shareholding.remove", id, func(repo domain.Repository) (int64, error) {
		return repo.SoftDeleteShareholding(ctx, id)
	})
}

func (s *Service) RecordBeneficiary(ctx context.Context, companyID string, in domain.BeneficiaryInput) (domain.Beneficiary, error) {
	if err := in.Validate(); err != nil {
		return domain.Beneficiary{}, err
	}
	var out domain.Beneficiary
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCompany(ctx, companyID); err != nil {
			return err
		}
		created, err := repo.InsertBeneficiary(ctx, companyID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "company.beneficiary.record", created.ID, created)
	})
	return out, err
}

func (s *Service) RemoveBeneficiary(ctx context.Context, id string) error {
	return s.softDelete(ctx, "company.beneficiary.remove", id, func(repo domain.Repository) (int64, error) {
		return repo.SoftDeleteBeneficiary(ctx, id)
	})
}

func (s *Service) RecordSuccession(ctx context.Context, predecessorID string, in domain.SuccessionInput) (domain.Succession, error) {
	if err := in.Validate(); err != nil {
		return domain.Succession{}, err
	}
	var out domain.Succession
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCompany(ctx, predecessorID); err != nil {
			return err
		}
		if _, err := repo.GetCompany(ctx, in.SuccessorID); err != nil {
			return err
		}
		created, err := repo.InsertSuccession(ctx, predecessorID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "company.succession.record", created.ID, created)
	})
	return out, err
}

func (s *Service) RemoveSuccession(ctx context.Context, id string) error {
	return s.softDelete(ctx, "company.succession.remove", id, func(repo domain.Repository) (int64, error) {
		return repo.SoftDeleteSuccession(ctx, id)
	})
}

func (s *Service) RecordBranch(ctx context.Context, parentID, branchID string) (domain.Branch, error) {
	if branchID == "" {
		return domain.Branch{}, domain.ErrInvalid
	}
	var out domain.Branch
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCompany(ctx, parentID); err != nil {
			return err
		}
		if _, err := repo.GetCompany(ctx, branchID); err != nil {
			return err
		}
		created, err := repo.InsertBranch(ctx, parentID, branchID)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "company.branch.record", created.ID, created)
	})
	return out, err
}

func (s *Service) RemoveBranch(ctx context.Context, id string) error {
	return s.softDelete(ctx, "company.branch.remove", id, func(repo domain.Repository) (int64, error) {
		return repo.SoftDeleteBranch(ctx, id)
	})
}

// GetOwnershipGraph assembles a company's one-hop ownership/affiliation neighbourhood, enriching
// company labels (best-effort default-locale legal names).
func (s *Service) GetOwnershipGraph(ctx context.Context, companyID string) (domain.OwnershipGraph, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetCompany(ctx, companyID); err != nil {
		return domain.OwnershipGraph{}, err
	}
	g := domain.OwnershipGraph{CompanyID: companyID}
	var err error
	if g.Shareholders, err = repo.ListShareholdersByCompany(ctx, companyID); err != nil {
		return domain.OwnershipGraph{}, err
	}
	if g.Holdings, err = repo.ListHoldingsByCompanyHolder(ctx, companyID); err != nil {
		return domain.OwnershipGraph{}, err
	}
	if g.Beneficiaries, err = repo.ListBeneficiariesByCompany(ctx, companyID); err != nil {
		return domain.OwnershipGraph{}, err
	}
	if g.Founders, err = repo.ListFoundingsByCompany(ctx, companyID); err != nil {
		return domain.OwnershipGraph{}, err
	}
	if g.Successions, err = repo.ListSuccessionsByCompany(ctx, companyID); err != nil {
		return domain.OwnershipGraph{}, err
	}
	if g.Branches, err = repo.ListBranchesByParent(ctx, companyID); err != nil {
		return domain.OwnershipGraph{}, err
	}
	// Resolve every referenced company id to a label in one batch.
	ids := map[string]struct{}{}
	for _, sh := range g.Shareholders {
		ids[sh.CompanyID] = struct{}{}
		if sh.HolderKind == domain.HolderCompany {
			ids[sh.HolderID] = struct{}{}
		}
	}
	for _, sh := range g.Holdings {
		ids[sh.CompanyID] = struct{}{}
		ids[sh.HolderID] = struct{}{}
	}
	for _, b := range g.Beneficiaries {
		ids[b.CompanyID] = struct{}{}
	}
	for _, f := range g.Founders {
		ids[f.CompanyID] = struct{}{}
		if f.HolderKind == domain.HolderCompany {
			ids[f.HolderID] = struct{}{}
		}
	}
	for _, su := range g.Successions {
		ids[su.PredecessorID] = struct{}{}
		ids[su.SuccessorID] = struct{}{}
	}
	for _, br := range g.Branches {
		ids[br.BranchID] = struct{}{}
		ids[br.ParentID] = struct{}{}
	}
	names, err := repo.CompanyNamesByIDs(ctx, keys(ids))
	if err != nil {
		return domain.OwnershipGraph{}, err
	}
	for i := range g.Shareholders {
		g.Shareholders[i].CompanyLabel = names[g.Shareholders[i].CompanyID]
		if g.Shareholders[i].HolderKind == domain.HolderCompany {
			g.Shareholders[i].HolderLabel = names[g.Shareholders[i].HolderID]
		}
	}
	for i := range g.Holdings {
		g.Holdings[i].CompanyLabel = names[g.Holdings[i].CompanyID]
		g.Holdings[i].HolderLabel = names[g.Holdings[i].HolderID]
	}
	for i := range g.Beneficiaries {
		g.Beneficiaries[i].CompanyLabel = names[g.Beneficiaries[i].CompanyID]
	}
	for i := range g.Founders {
		g.Founders[i].CompanyLabel = names[g.Founders[i].CompanyID]
		if g.Founders[i].HolderKind == domain.HolderCompany {
			g.Founders[i].HolderLabel = names[g.Founders[i].HolderID]
		}
	}
	for i := range g.Successions {
		g.Successions[i].PredecessorLabel = names[g.Successions[i].PredecessorID]
		g.Successions[i].SuccessorLabel = names[g.Successions[i].SuccessorID]
	}
	for i := range g.Branches {
		g.Branches[i].BranchLabel = names[g.Branches[i].BranchID]
		g.Branches[i].ParentLabel = names[g.Branches[i].ParentID]
	}
	return g, nil
}

// ListPersonAffiliations assembles a person's company links (employment, founding, ownership, UBO),
// enriching the "other end" company label.
func (s *Service) ListPersonAffiliations(ctx context.Context, personID string) (domain.PersonAffiliations, error) {
	repo := s.newRepo(s.pool)
	out := domain.PersonAffiliations{}
	var err error
	if out.Appointments, err = repo.ListAppointmentsByPerson(ctx, personID); err != nil {
		return domain.PersonAffiliations{}, err
	}
	if out.Foundings, err = repo.ListFoundingsByPersonHolder(ctx, personID); err != nil {
		return domain.PersonAffiliations{}, err
	}
	if out.Shareholdings, err = repo.ListShareholdingsByPersonHolder(ctx, personID); err != nil {
		return domain.PersonAffiliations{}, err
	}
	if out.BeneficiaryOf, err = repo.ListBeneficiariesByPerson(ctx, personID); err != nil {
		return domain.PersonAffiliations{}, err
	}
	ids := map[string]struct{}{}
	for _, f := range out.Foundings {
		ids[f.CompanyID] = struct{}{}
	}
	for _, sh := range out.Shareholdings {
		ids[sh.CompanyID] = struct{}{}
	}
	for _, b := range out.BeneficiaryOf {
		ids[b.CompanyID] = struct{}{}
	}
	names, err := repo.CompanyNamesByIDs(ctx, keys(ids))
	if err != nil {
		return domain.PersonAffiliations{}, err
	}
	for i := range out.Foundings {
		out.Foundings[i].CompanyLabel = names[out.Foundings[i].CompanyID]
	}
	for i := range out.Shareholdings {
		out.Shareholdings[i].CompanyLabel = names[out.Shareholdings[i].CompanyID]
	}
	for i := range out.BeneficiaryOf {
		out.BeneficiaryOf[i].CompanyLabel = names[out.BeneficiaryOf[i].CompanyID]
	}
	return out, nil
}

// ============================ helpers ============================

// requireHolder validates a polymorphic holder: a company holder must exist; a person holder is
// accepted as-is (no cross-module person reader — best-effort registry data).
func (s *Service) requireHolder(ctx context.Context, repo domain.Repository, kind, id string) error {
	if kind == domain.HolderCompany {
		if _, err := repo.GetCompany(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// softDelete runs a soft-delete repo call in an audited transaction, returning ErrLinkNotFound when no
// row matched.
func (s *Service) softDelete(ctx context.Context, action, id string, del func(domain.Repository) (int64, error)) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := del(s.newRepo(tx))
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrLinkNotFound
		}
		return s.record(ctx, tx, action, id, map[string]string{"id": id})
	})
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

func clampPageSize(n int) int {
	if n <= 0 {
		return defaultPageSize
	}
	if n > maxPageSize {
		return maxPageSize
	}
	return n
}

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record mints an Action RID (company service=15, kind=action=3, generic type=0) in the caller's
// transaction and writes the audit row on it, so the audit entry commits iff the change commits (D-Audit).
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(15, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: "company",
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
