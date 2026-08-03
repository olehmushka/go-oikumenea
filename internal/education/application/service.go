// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package application is the education module's orchestrator (D-Education): audited writes, the unit
// structure tree + maintained closure (mirrors tenant), and the positions/appointments one-holder rule
// (mirrors membership). Every write runs in a transaction that also records the audit Action (D-Audit);
// reads run on the pool. Education entities are instance-global external reference data (no unit scope),
// so writes are recorded as a `system` action with no unit attributed (interim until M7/M8 resolve a
// person actor).
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
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

const (
	auditSubsystem  = "education-admin"
	defaultPageSize = 50
	maxPageSize     = 200
	// universityDomainCode is the tenant domain an education institution org belongs to (M41 /
	// D-UnifiedOrgGraph). structureGraph is the per-org graph holding the institution's unit tree
	// (campus → faculty → department → chair), lazily seeded since `university` is a reference domain.
	universityDomainCode = "university"
	structureGraph       = "structure"
	structureGraphName   = "Structure"
)

// RepositoryFactory binds a domain.Repository to a command surface (pool for reads, tx for writes).
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the education application service. Institutions are `university`-domain tenant organizations
// and units are tenant units; the structure (orgs/units/closure) is owned by the tenant service, while
// this service owns the education_org_profiles sidecar, the reference layer, and the person links.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
	tenant  *tenantapp.Service

	labeler stats.Labeler

	uniMu     sync.Mutex
	uniDomain string // cached `university` domain RID (seeded at boot, stable)
}

// SetBucketLabeler injects the dashboard's ref-bucket name resolver (M58 ticket 5 / D-ObjectFacets),
// wired at the composition root. Institution declares TWO ref facets (kind, country), so without it
// two of the four charts would be axis-labelled with RID tails.
func (s *Service) SetBucketLabeler(l stats.Labeler) { s.labeler = l }

// NewService wires the service with the pool, repository factory, the audit service, and the tenant
// service (institution = org, unit = tenant unit — M41).
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, tenant *tenantapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, tenant: tenant}
}

// universityDomainID resolves (and caches) the `university` tenant domain RID.
func (s *Service) universityDomainID(ctx context.Context) (string, error) {
	s.uniMu.Lock()
	defer s.uniMu.Unlock()
	if s.uniDomain != "" {
		return s.uniDomain, nil
	}
	doms, err := s.tenant.ListDomains(ctx)
	if err != nil {
		return "", err
	}
	for _, d := range doms {
		if d.Code == universityDomainCode {
			s.uniDomain = d.ID
			return d.ID, nil
		}
	}
	return "", domain.ErrInvalid
}

// toUnitView maps a tenant unit to the education Unit view, deriving its parent + depth from the
// structure graph closure (nearest ancestor = parent; ancestor count = depth).
func (s *Service) toUnitView(ctx context.Context, u tenantdomain.Unit) (domain.Unit, error) {
	anc, err := s.tenant.Ancestors(ctx, u.ID, structureGraph)
	if err != nil {
		return domain.Unit{}, err
	}
	parent := ""
	if len(anc) > 0 {
		parent = anc[0].ID
	}
	return domain.Unit{
		ID: u.ID, InstitutionID: u.OrgID, ParentID: parent, KindID: deref(u.KindID),
		Code: deref(u.Code), Name: u.Name, Status: string(u.State), Depth: len(anc),
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// mapOrgNotFound / mapUnitNotFound translate the tenant service's not-found sentinels into the
// education ones, so the transport keeps mapping to the same Conjure errors (M41 delegation).
func mapOrgNotFound(err error) error {
	if errors.Is(err, tenantdomain.ErrOrgNotFound) {
		return domain.ErrInstitutionNotFound
	}
	return err
}

func mapUnitNotFound(err error) error {
	if errors.Is(err, tenantdomain.ErrUnitNotFound) {
		return domain.ErrUnitNotFound
	}
	return err
}

// mapEdgeErr translates a tenant edge error (cycle / missing unit) into the education sentinels so the
// transport keeps mapping reparent failures to Education:UnitCycleDetected / Education:UnitNotFound.
func mapEdgeErr(err error) error {
	switch {
	case errors.Is(err, tenantdomain.ErrUnitCycle):
		return domain.ErrUnitCycle
	case errors.Is(err, tenantdomain.ErrUnitNotFound):
		return domain.ErrUnitNotFound
	}
	return err
}

// ============================ catalogs ============================

func (s *Service) ListInstitutionKinds(ctx context.Context) ([]domain.InstitutionKind, error) {
	return s.newRepo(s.pool).ListInstitutionKinds(ctx)
}

// ListUnitKinds returns the `university` domain's unit-kind catalog (campus/faculty/department/chair),
// owned by the tenant service (M41).
func (s *Service) ListUnitKinds(ctx context.Context) ([]domain.UnitKind, error) {
	uni, err := s.universityDomainID(ctx)
	if err != nil {
		return nil, err
	}
	kinds, err := s.tenant.ListUnitKinds(ctx, uni)
	if err != nil {
		return nil, err
	}
	out := make([]domain.UnitKind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, domain.UnitKind{ID: k.ID, Code: k.Code, Name: k.Name, Status: string(k.Status), SortOrder: k.SortOrder})
	}
	return out, nil
}

func (s *Service) ListDegreeLevels(ctx context.Context) ([]domain.DegreeLevel, error) {
	return s.newRepo(s.pool).ListDegreeLevels(ctx)
}

func (s *Service) UpsertInstitutionKind(ctx context.Context, code, name string, sortOrder *int) (domain.InstitutionKind, error) {
	var out domain.InstitutionKind
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		k, err := s.newRepo(tx).UpsertInstitutionKind(ctx, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = k
		return s.record(ctx, tx, "education.institution-kind.upsert", k.ID, k)
	})
	return out, err
}

// ============================ institutions (tenant org + sidecar — M41) ============================

// CreateInstitution creates a `university`-domain tenant organization (the institution), seeds its
// `structure` graph, and writes the education_org_profiles sidecar. Cross-module: the org create runs in
// the tenant service's own transaction; the sidecar + audit run in an education transaction (sequential,
// like religion's createChildOrg).
func (s *Service) CreateInstitution(ctx context.Context, in domain.InstitutionInput) (domain.Institution, error) {
	if err := in.Validate(); err != nil {
		return domain.Institution{}, err
	}
	uni, err := s.universityDomainID(ctx)
	if err != nil {
		return domain.Institution{}, err
	}
	org, err := s.tenant.CreateOrganization(ctx, tenantdomain.Organization{
		Code: in.Code, Name: in.Name, DomainID: uni, Visibility: tenantdomain.VisibilityPublic,
	})
	if err != nil {
		return domain.Institution{}, err
	}
	if _, err := s.tenant.EnsureGraph(ctx, org.ID, structureGraph, structureGraphName, false); err != nil {
		return domain.Institution{}, err
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).InsertOrgProfile(ctx, org.ID, in.KindID, in.CountryID, in.FoundedOn, in.ClosedOn); err != nil {
			return err
		}
		return s.record(ctx, tx, "education.institution.create", org.ID, org)
	})
	if err != nil {
		return domain.Institution{}, err
	}
	return s.GetInstitution(ctx, org.ID)
}

func (s *Service) GetInstitution(ctx context.Context, id string) (domain.Institution, error) {
	return s.newRepo(s.pool).GetInstitution(ctx, id)
}

func (s *Service) ListInstitutions(ctx context.Context, f domain.InstitutionFilter, after string, pageSize int) ([]domain.Institution, error) {
	return s.newRepo(s.pool).ListInstitutions(ctx, f, after, clampPageSize(pageSize)+1)
}

// InstitutionStats answers the institution dashboard (M58 ticket 5 / D-ObjectFacets). It takes BOTH
// the subject and the admin flag rather than deriving one from the other: stats.Compute owns the arm
// convention, so a machine principal (no subject, not an admin) reads nothing rather than everything.
func (s *Service) InstitutionStats(ctx context.Context, subjectPersonID string, isAdmin bool, f domain.InstitutionFilter, sel stats.Selection) (stats.Result, error) {
	repo := s.newRepo(s.pool)
	return stats.Compute(ctx, s.labeler, sel, isAdmin, subjectPersonID, func(subject string) ([]stats.Group, error) {
		return repo.InstitutionStats(ctx, subject, f, sel)
	})
}

// UpdateInstitution applies a partial change: the org name via the tenant service (if set) and the
// education sidecar fields (kind/country/dates/state) via the repository.
func (s *Service) UpdateInstitution(ctx context.Context, id string, up domain.InstitutionUpdate) (domain.Institution, error) {
	if _, err := s.newRepo(s.pool).GetInstitution(ctx, id); err != nil {
		return domain.Institution{}, err
	}
	if up.Name != nil {
		if _, err := s.tenant.UpdateOrganization(ctx, id, tenantdomain.OrgPatch{Name: up.Name}); err != nil {
			return domain.Institution{}, err
		}
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).UpdateOrgProfile(ctx, id, up); err != nil {
			return err
		}
		return s.record(ctx, tx, "education.institution.update", id, up)
	})
	if err != nil {
		return domain.Institution{}, err
	}
	return s.GetInstitution(ctx, id)
}

// DeleteInstitution soft-deletes the education profile (the org itself stays in the tenant directory).
func (s *Service) DeleteInstitution(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteInstitution(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrInstitutionNotFound
		}
		return s.record(ctx, tx, "education.institution.delete", id, map[string]string{"id": id})
	})
}

// ============================ units (tenant units in the structure graph — M41) ============================

// CreateUnit creates a tenant unit under the institution org (in its `structure` graph) and, if a parent
// is given, the parent→child edge. The tenant service owns the closure (no second closure engine).
func (s *Service) CreateUnit(ctx context.Context, institutionID string, in domain.UnitInput) (domain.Unit, error) {
	if err := in.Validate(); err != nil {
		return domain.Unit{}, err
	}
	org, err := s.tenant.GetOrganization(ctx, institutionID)
	if err != nil {
		return domain.Unit{}, mapOrgNotFound(err)
	}
	if _, err := s.tenant.EnsureGraph(ctx, institutionID, structureGraph, structureGraphName, false); err != nil {
		return domain.Unit{}, err
	}
	kindID := in.KindID
	code := in.Code
	u, err := s.tenant.CreateUnit(ctx, tenantdomain.Unit{
		OrgID: institutionID, DomainID: org.DomainID, KindID: &kindID, Code: &code, Name: in.Name,
		Visibility: tenantdomain.VisibilityPublic,
	})
	if err != nil {
		return domain.Unit{}, err
	}
	if in.ParentID != nil && *in.ParentID != "" {
		if _, err := s.tenant.AddEdge(ctx, u.ID, *in.ParentID, structureGraph); err != nil {
			return domain.Unit{}, mapEdgeErr(err)
		}
	}
	return s.toUnitView(ctx, u)
}

func (s *Service) GetUnit(ctx context.Context, id string) (domain.Unit, error) {
	u, err := s.tenant.GetUnit(ctx, id)
	if err != nil {
		return domain.Unit{}, mapUnitNotFound(err)
	}
	return s.toUnitView(ctx, u)
}

func (s *Service) ListUnits(ctx context.Context, institutionID string) ([]domain.Unit, error) {
	page, err := s.tenant.ListUnits(ctx, tenantdomain.UnitFilter{OrgID: institutionID}, "", nil, false, maxPageSize, "")
	if err != nil {
		return nil, err
	}
	out := make([]domain.Unit, 0, len(page.Units))
	for _, u := range page.Units {
		v, err := s.toUnitView(ctx, u)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// UpdateUnit applies name/kind via the tenant unit patch and, when a status is given, a tenant lifecycle
// transition (education active/archived → tenant active/archived).
func (s *Service) UpdateUnit(ctx context.Context, id string, up domain.UnitUpdate) (domain.Unit, error) {
	if up.Name != nil || up.KindID != nil {
		if _, err := s.tenant.UpdateUnit(ctx, id, tenantdomain.UnitPatch{Name: up.Name, KindID: up.KindID}); err != nil {
			return domain.Unit{}, mapUnitNotFound(err)
		}
	}
	if up.Status != nil {
		to := tenantdomain.State(*up.Status)
		if *up.Status == "archived" {
			to = tenantdomain.StateArchived
		} else if *up.Status == "active" {
			to = tenantdomain.StateActive
		}
		if _, err := s.tenant.TransitionUnit(ctx, id, to, "education.unit.update"); err != nil {
			return domain.Unit{}, mapUnitNotFound(err)
		}
	}
	return s.GetUnit(ctx, id)
}

// ReparentUnit re-points a unit's structure-graph parent: drop the current parent edge (if any) and add
// the new one. The tenant AddEdge enforces the cycle guard and maintains the closure.
func (s *Service) ReparentUnit(ctx context.Context, id string, parentID *string) (domain.Unit, error) {
	if _, err := s.tenant.GetUnit(ctx, id); err != nil {
		return domain.Unit{}, mapUnitNotFound(err)
	}
	cur, err := s.tenant.Ancestors(ctx, id, structureGraph)
	if err != nil {
		return domain.Unit{}, err
	}
	if len(cur) > 0 {
		if err := s.tenant.RemoveEdge(ctx, id, cur[0].ID, structureGraph); err != nil {
			return domain.Unit{}, err
		}
	}
	if parentID != nil && *parentID != "" {
		if _, err := s.tenant.AddEdge(ctx, id, *parentID, structureGraph); err != nil {
			return domain.Unit{}, mapEdgeErr(err)
		}
	}
	return s.GetUnit(ctx, id)
}

// ============================ buildings ============================

func (s *Service) CreateBuilding(ctx context.Context, institutionID string, in domain.BuildingInput) (domain.Building, error) {
	if err := in.Validate(); err != nil {
		return domain.Building{}, err
	}
	var out domain.Building
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertBuilding(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "education.building.create", created.ID, created)
	})
	return out, err
}

func (s *Service) GetBuilding(ctx context.Context, id string) (domain.Building, error) {
	return s.newRepo(s.pool).GetBuilding(ctx, id)
}

func (s *Service) ListBuildings(ctx context.Context, institutionID string) ([]domain.Building, error) {
	return s.newRepo(s.pool).ListBuildingsByInstitution(ctx, institutionID)
}

func (s *Service) UpdateBuilding(ctx context.Context, id string, up domain.BuildingUpdate) (domain.Building, error) {
	var out domain.Building
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateBuilding(ctx, id, up)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "education.building.update", id, updated)
	})
	return out, err
}

func (s *Service) DeleteBuilding(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteBuilding(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrBuildingNotFound
		}
		return s.record(ctx, tx, "education.building.delete", id, map[string]string{"id": id})
	})
}

// ============================ groups ============================

func (s *Service) CreateGroup(ctx context.Context, unitID string, in domain.GroupInput) (domain.Group, error) {
	if err := in.Validate(); err != nil {
		return domain.Group{}, err
	}
	var out domain.Group
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertGroup(ctx, unitID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "education.group.create", created.ID, created)
	})
	return out, err
}

func (s *Service) GetGroup(ctx context.Context, id string) (domain.Group, error) {
	return s.newRepo(s.pool).GetGroup(ctx, id)
}

func (s *Service) ListGroups(ctx context.Context, unitID string) ([]domain.Group, error) {
	return s.newRepo(s.pool).ListGroupsByUnit(ctx, unitID)
}

func (s *Service) UpdateGroup(ctx context.Context, id string, up domain.GroupUpdate) (domain.Group, error) {
	var out domain.Group
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateGroup(ctx, id, up)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "education.group.update", id, updated)
	})
	return out, err
}

func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteGroup(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrGroupNotFound
		}
		return s.record(ctx, tx, "education.group.delete", id, map[string]string{"id": id})
	})
}

// ============================ positions + appointments ============================

func (s *Service) CreatePosition(ctx context.Context, institutionID string, in domain.PositionInput) (domain.Position, error) {
	if err := in.Validate(); err != nil {
		return domain.Position{}, err
	}
	var out domain.Position
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertPosition(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "education.position.create", created.ID, created)
	})
	return out, err
}

// GetPosition reads a position and attaches its current holder (if any).
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

func (s *Service) ListPositions(ctx context.Context, institutionID, state, after string, pageSize int) ([]domain.Position, error) {
	repo := s.newRepo(s.pool)
	positions, err := repo.ListPositionsByInstitution(ctx, institutionID, state, after, clampPageSize(pageSize)+1)
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
		return s.record(ctx, tx, "education.position.update", id, updated)
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
		return s.record(ctx, tx, "education.position.abolish", id, updated)
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
		return s.record(ctx, tx, "education.appointment.fill", created.ID, created)
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
		return s.record(ctx, tx, "education.appointment.end", id, ended)
	})
	return out, err
}

func (s *Service) ListPersonAppointments(ctx context.Context, personID string) ([]domain.PersonAppointment, error) {
	return s.newRepo(s.pool).ListAppointmentsByPerson(ctx, personID)
}

// ============================ person bindings ============================

func (s *Service) ListEnrollments(ctx context.Context, personID string) ([]domain.Enrollment, error) {
	return s.newRepo(s.pool).ListEnrollmentsByPerson(ctx, personID)
}

func (s *Service) CreateEnrollment(ctx context.Context, personID string, in domain.EnrollmentInput) (domain.Enrollment, error) {
	if err := in.Validate(); err != nil {
		return domain.Enrollment{}, err
	}
	var out domain.Enrollment
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertEnrollment(ctx, personID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "education.enrollment.create", created.ID, created)
	})
	return out, err
}

func (s *Service) UpdateEnrollment(ctx context.Context, personID, id string, in domain.EnrollmentInput) (domain.Enrollment, error) {
	var out domain.Enrollment
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateEnrollment(ctx, personID, id, in)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "education.enrollment.update", id, updated)
	})
	return out, err
}

func (s *Service) DeleteEnrollment(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteEnrollment(ctx, personID, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrEnrollmentNotFound
		}
		return s.record(ctx, tx, "education.enrollment.delete", id, map[string]string{"id": id})
	})
}

func (s *Service) ListDormitoryStays(ctx context.Context, personID string) ([]domain.DormitoryStay, error) {
	return s.newRepo(s.pool).ListDormitoryStaysByPerson(ctx, personID)
}

func (s *Service) CreateDormitoryStay(ctx context.Context, personID string, in domain.DormInput) (domain.DormitoryStay, error) {
	if err := in.Validate(); err != nil {
		return domain.DormitoryStay{}, err
	}
	var out domain.DormitoryStay
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertDormitoryStay(ctx, personID, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "education.dormitory-stay.create", created.ID, created)
	})
	return out, err
}

func (s *Service) UpdateDormitoryStay(ctx context.Context, personID, id string, in domain.DormInput) (domain.DormitoryStay, error) {
	var out domain.DormitoryStay
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateDormitoryStay(ctx, personID, id, in)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "education.dormitory-stay.update", id, updated)
	})
	return out, err
}

func (s *Service) DeleteDormitoryStay(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteDormitoryStay(ctx, personID, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrDormNotFound
		}
		return s.record(ctx, tx, "education.dormitory-stay.delete", id, map[string]string{"id": id})
	})
}

// ============================ helpers ============================

// pageSizePolicy is this module's page-size policy, clamped through the shared kernel (M56 /
// pkg/listing) instead of a local copy.
var pageSizePolicy = listing.PageSize{Default: defaultPageSize, Max: maxPageSize}

func clampPageSize(n int) int { return pageSizePolicy.Resolve(n) }

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

// record mints an Action RID (education service=14, kind=action=3, generic type=0) in the caller's
// transaction and writes the audit row on it, so the audit entry commits iff the change commits (D-Audit).
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(14, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: targetType(action),
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// targetType derives a coarse audit target type from the action verb prefix (education.<thing>.<verb>).
func targetType(action string) string {
	return "education"
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
