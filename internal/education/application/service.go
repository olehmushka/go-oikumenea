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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

const (
	auditSubsystem  = "education-admin"
	defaultPageSize = 50
	maxPageSize     = 200
)

// RepositoryFactory binds a domain.Repository to a command surface (pool for reads, tx for writes).
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the education application service.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
}

// NewService wires the service with the pool, repository factory, and the audit service.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit}
}

// ============================ catalogs ============================

func (s *Service) ListInstitutionKinds(ctx context.Context) ([]domain.InstitutionKind, error) {
	return s.newRepo(s.pool).ListInstitutionKinds(ctx)
}

func (s *Service) ListUnitKinds(ctx context.Context) ([]domain.UnitKind, error) {
	return s.newRepo(s.pool).ListUnitKinds(ctx)
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

func (s *Service) UpsertUnitKind(ctx context.Context, code, name string, sortOrder *int) (domain.UnitKind, error) {
	var out domain.UnitKind
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		k, err := s.newRepo(tx).UpsertUnitKind(ctx, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = k
		return s.record(ctx, tx, "education.unit-kind.upsert", k.ID, k)
	})
	return out, err
}

// ============================ institutions ============================

func (s *Service) CreateInstitution(ctx context.Context, in domain.InstitutionInput) (domain.Institution, error) {
	if err := in.Validate(); err != nil {
		return domain.Institution{}, err
	}
	var out domain.Institution
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertInstitution(ctx, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "education.institution.create", created.ID, created)
	})
	return out, err
}

func (s *Service) GetInstitution(ctx context.Context, id string) (domain.Institution, error) {
	return s.newRepo(s.pool).GetInstitution(ctx, id)
}

func (s *Service) ListInstitutions(ctx context.Context, query, after string, pageSize int) ([]domain.Institution, error) {
	return s.newRepo(s.pool).ListInstitutions(ctx, query, after, clampPageSize(pageSize)+1)
}

func (s *Service) UpdateInstitution(ctx context.Context, id string, up domain.InstitutionUpdate) (domain.Institution, error) {
	var out domain.Institution
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateInstitution(ctx, id, up)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "education.institution.update", id, updated)
	})
	return out, err
}

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

// ============================ units + closure ============================

// CreateUnit inserts a unit (validating the parent belongs to the same institution) and recomputes the
// institution's closure in the same transaction.
func (s *Service) CreateUnit(ctx context.Context, institutionID string, in domain.UnitInput) (domain.Unit, error) {
	if err := in.Validate(); err != nil {
		return domain.Unit{}, err
	}
	var out domain.Unit
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetInstitution(ctx, institutionID); err != nil {
			return err
		}
		if in.ParentID != nil {
			parent, err := repo.GetUnit(ctx, *in.ParentID)
			if err != nil {
				return err
			}
			if parent.InstitutionID != institutionID {
				return domain.ErrInvalid
			}
		}
		created, err := repo.InsertUnit(ctx, institutionID, in)
		if err != nil {
			return err
		}
		if err := repo.RecomputeClosure(ctx, institutionID); err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "education.unit.create", created.ID, created)
	})
	return out, err
}

func (s *Service) GetUnit(ctx context.Context, id string) (domain.Unit, error) {
	return s.newRepo(s.pool).GetUnit(ctx, id)
}

func (s *Service) ListUnits(ctx context.Context, institutionID string) ([]domain.Unit, error) {
	return s.newRepo(s.pool).ListUnitsByInstitution(ctx, institutionID)
}

func (s *Service) UpdateUnit(ctx context.Context, id string, up domain.UnitUpdate) (domain.Unit, error) {
	var out domain.Unit
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateUnit(ctx, id, up)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "education.unit.update", id, updated)
	})
	return out, err
}

// ReparentUnit moves a unit under a new parent (same institution), guarding against cycles, then
// recomputes the closure.
func (s *Service) ReparentUnit(ctx context.Context, id string, parentID *string) (domain.Unit, error) {
	var out domain.Unit
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		unit, err := repo.GetUnit(ctx, id)
		if err != nil {
			return err
		}
		if parentID != nil {
			if *parentID == id {
				return domain.ErrUnitCycle
			}
			parent, err := repo.GetUnit(ctx, *parentID)
			if err != nil {
				return err
			}
			if parent.InstitutionID != unit.InstitutionID {
				return domain.ErrInvalid
			}
			// Cycle: the proposed parent must not already be in this unit's subtree.
			cyclic, err := repo.ClosureHasPath(ctx, id, *parentID)
			if err != nil {
				return err
			}
			if cyclic {
				return domain.ErrUnitCycle
			}
		}
		updated, err := repo.SetUnitParent(ctx, id, parentID)
		if err != nil {
			return err
		}
		if err := repo.RecomputeClosure(ctx, unit.InstitutionID); err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "education.unit.reparent", id, updated)
	})
	return out, err
}

func (s *Service) VerifyClosure(ctx context.Context, institutionID string) (domain.ClosureReport, error) {
	var rep domain.ClosureReport
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetInstitution(ctx, institutionID); err != nil {
			return err
		}
		missing, extra, err := repo.VerifyClosure(ctx, institutionID)
		if err != nil {
			return err
		}
		rep = domain.ClosureReport{InstitutionID: institutionID, Missing: missing, Extra: extra, InDrift: missing > 0 || extra > 0}
		return s.record(ctx, tx, "education.closure.verify", institutionID, rep)
	})
	return rep, err
}

func (s *Service) RebuildClosure(ctx context.Context, institutionID string) (domain.ClosureReport, error) {
	var rep domain.ClosureReport
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetInstitution(ctx, institutionID); err != nil {
			return err
		}
		if err := repo.RecomputeClosure(ctx, institutionID); err != nil {
			return err
		}
		rep = domain.ClosureReport{InstitutionID: institutionID}
		return s.record(ctx, tx, "education.closure.rebuild", institutionID, rep)
	})
	return rep, err
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
