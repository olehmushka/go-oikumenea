// Package application is the vehicle module's orchestrator (D-Vehicles, M26): audited writes over the
// brand/model/type catalogs, the vehicle object, the brand→manufacturer link, and the ownership+plate
// registration record. Every write runs in a transaction that also records the audit Action (D-Audit);
// reads run on the pool. Vehicle entities are instance-global external reference data, so writes are
// recorded under a `system` actor (mirroring company). A registration's plate region is validated
// against the WOF geo_places gazetteer (placetype=region, D-GeoPlaces).
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
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/domain"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

const (
	auditSubsystem  = "vehicle-admin"
	defaultPageSize = 50
	maxPageSize     = 200
)

// RepositoryFactory binds a domain.Repository to a command surface (pool for reads, tx for writes).
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the vehicle application service.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
	colors  domain.ColorLookup
}

// NewService wires the service with the pool, repository factory, audit service, and the color catalog
// lookup (D-Color: hard-FK palette enforcement for the vehicle color).
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, colors domain.ColorLookup) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, colors: colors}
}

// checkColor enforces the hard FK's palette: a non-empty color id must resolve to a domain='vehicle'
// color (D-Color). Returns domain.ErrColorMismatch otherwise (unknown id maps there too).
func (s *Service) checkColor(ctx context.Context, colorID string) error {
	if colorID == "" || s.colors == nil {
		return nil
	}
	d, err := s.colors.ColorDomain(ctx, colorID)
	if err != nil {
		return domain.ErrColorMismatch
	}
	if d != "vehicle" {
		return domain.ErrColorMismatch
	}
	return nil
}

// ============================ catalogs ============================

func (s *Service) ListVehicleTypes(ctx context.Context) ([]domain.VehicleType, error) {
	return s.newRepo(s.querier(ctx)).ListVehicleTypes(ctx)
}

func (s *Service) UpsertVehicleType(ctx context.Context, code, name string, parentID *string, sortOrder *int) (domain.VehicleType, error) {
	var out domain.VehicleType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertVehicleType(ctx, code, name, parentID, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "vehicle.type.upsert", v.ID, v)
	})
	return out, err
}

func (s *Service) ListBrands(ctx context.Context, query string) ([]domain.Brand, error) {
	return s.newRepo(s.querier(ctx)).ListBrands(ctx, query)
}

func (s *Service) UpsertBrand(ctx context.Context, code, name string, countryID *string, sortOrder *int) (domain.Brand, error) {
	var out domain.Brand
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertBrand(ctx, code, name, countryID, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "vehicle.brand.upsert", v.ID, v)
	})
	return out, err
}

func (s *Service) ListModelsByBrand(ctx context.Context, brandID string) ([]domain.Model, error) {
	if _, err := s.newRepo(s.querier(ctx)).GetBrand(ctx, brandID); err != nil {
		return nil, mapNotFound(err, domain.ErrBrandNotFound)
	}
	return s.newRepo(s.querier(ctx)).ListModelsByBrand(ctx, brandID)
}

func (s *Service) UpsertModel(ctx context.Context, brandID, code, name string, generation, manufactureStart, manufactureEnd *string, sortOrder *int) (domain.Model, error) {
	var out domain.Model
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetBrand(ctx, brandID); err != nil {
			return mapNotFound(err, domain.ErrBrandNotFound)
		}
		v, err := repo.UpsertModel(ctx, brandID, code, name, generation, manufactureStart, manufactureEnd, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "vehicle.model.upsert", v.ID, v)
	})
	return out, err
}

func (s *Service) ListNumberTypes(ctx context.Context) ([]domain.NumberType, error) {
	return s.newRepo(s.querier(ctx)).ListNumberTypes(ctx)
}

func (s *Service) UpsertNumberType(ctx context.Context, code, name string, sortOrder *int) (domain.NumberType, error) {
	var out domain.NumberType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertNumberType(ctx, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "vehicle.number-type.upsert", v.ID, v)
	})
	return out, err
}

// ============================ vehicles ============================

func (s *Service) GetVehicle(ctx context.Context, id string) (domain.Vehicle, error) {
	v, err := s.newRepo(s.querier(ctx)).GetVehicle(ctx, id)
	return v, mapNotFound(err, domain.ErrVehicleNotFound)
}

func (s *Service) ListVehicles(ctx context.Context, query, after string, pageSize int) ([]domain.Vehicle, error) {
	return s.newRepo(s.querier(ctx)).ListVehicles(ctx, query, after, clampPageSize(pageSize)+1)
}

func (s *Service) CreateVehicle(ctx context.Context, in domain.VehicleInput) (domain.Vehicle, error) {
	if err := in.Validate(); err != nil {
		return domain.Vehicle{}, err
	}
	if err := s.checkColor(ctx, in.ColorID); err != nil {
		return domain.Vehicle{}, err
	}
	var out domain.Vehicle
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertVehicle(ctx, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "vehicle.create", created.ID, created)
	})
	return out, err
}

func (s *Service) UpdateVehicle(ctx context.Context, id string, up domain.VehicleUpdate) (domain.Vehicle, error) {
	if up.ColorID != nil {
		if err := s.checkColor(ctx, *up.ColorID); err != nil {
			return domain.Vehicle{}, err
		}
	}
	var out domain.Vehicle
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateVehicle(ctx, id, up)
		if err != nil {
			return mapNotFound(err, domain.ErrVehicleNotFound)
		}
		out = v
		return s.record(ctx, tx, "vehicle.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteVehicle(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteVehicle(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrVehicleNotFound
		}
		return s.record(ctx, tx, "vehicle.delete", id, nil)
	})
}

// ============================ registrations ============================

func (s *Service) ListRegistrationsByVehicle(ctx context.Context, vehicleID string) ([]domain.Registration, error) {
	if _, err := s.newRepo(s.querier(ctx)).GetVehicle(ctx, vehicleID); err != nil {
		return nil, mapNotFound(err, domain.ErrVehicleNotFound)
	}
	return s.newRepo(s.querier(ctx)).ListRegistrationsByVehicle(ctx, vehicleID)
}

// RegisterVehicle registers (or transfers) a vehicle to a new owner: it closes any active registration
// for the vehicle first (registration is the ownership history), validating the owner and the optional
// plate region (a geo_places region).
func (s *Service) RegisterVehicle(ctx context.Context, vehicleID string, in domain.RegistrationInput) (domain.Registration, error) {
	if err := in.Validate(); err != nil {
		return domain.Registration{}, err
	}
	var out domain.Registration
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetVehicle(ctx, vehicleID); err != nil {
			return mapNotFound(err, domain.ErrVehicleNotFound)
		}
		if in.OwnerKind == domain.OwnerCompany {
			names, err := repo.CompanyNamesByIDs(ctx, []string{in.OwnerID})
			if err != nil {
				return err
			}
			if _, ok := names[in.OwnerID]; !ok {
				return domain.ErrInvalid
			}
		}
		if in.SubdivisionID != "" {
			ok, err := repo.IsRegion(ctx, in.SubdivisionID)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrRegionInvalid
			}
		}
		if err := repo.CloseActiveRegistrationsForVehicle(ctx, vehicleID); err != nil {
			return err
		}
		reg, err := repo.InsertRegistration(ctx, vehicleID, in)
		if err != nil {
			return err
		}
		out = reg
		return s.record(ctx, tx, "vehicle.register", reg.ID, reg)
	})
	return out, err
}

func (s *Service) CloseRegistration(ctx context.Context, id string) (domain.Registration, error) {
	var out domain.Registration
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		reg, err := s.newRepo(tx).CloseRegistration(ctx, id)
		if err != nil {
			return mapNotFound(err, domain.ErrLinkNotFound)
		}
		out = reg
		return s.record(ctx, tx, "vehicle.registration.close", id, reg)
	})
	return out, err
}

func (s *Service) ListPersonVehicles(ctx context.Context, personID string) ([]domain.PersonRegistration, error) {
	return s.newRepo(s.querier(ctx)).ListRegistrationsByPersonOwner(ctx, personID)
}

// ErasePersonRegistrations is the person-purge erasure path (D-Vehicles): it soft-deletes a person's
// owned registrations. Triggered by the PersonPurged event (SubscribePersonPurge); also exercised directly.
func (s *Service) ErasePersonRegistrations(ctx context.Context, personID string) (int64, error) {
	var n int64
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.erasePersonRegistrationsTx(ctx, tx, personID)
		n = v
		return err
	})
	return n, err
}

// erasePersonRegistrationsTx is the body of the person-purge erasure, run in a caller-supplied transaction
// so it executes either standalone (ErasePersonRegistrations) or inside the person-purge tx as the
// PersonPurged subscriber (SubscribePersonPurge). The audit row is written only when something was erased.
func (s *Service) erasePersonRegistrationsTx(ctx context.Context, tx pgx.Tx, personID string) (int64, error) {
	n, err := s.newRepo(tx).ErasePersonRegistrations(ctx, personID)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		if err := s.record(ctx, tx, "vehicle.registrations.erase", personID, map[string]int64{"erased": n}); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// ============================ brand manufacturers ============================

func (s *Service) ListManufacturersByBrand(ctx context.Context, brandID string) ([]domain.Manufacturer, error) {
	if _, err := s.newRepo(s.querier(ctx)).GetBrand(ctx, brandID); err != nil {
		return nil, mapNotFound(err, domain.ErrBrandNotFound)
	}
	return s.newRepo(s.querier(ctx)).ListManufacturersByBrand(ctx, brandID)
}

func (s *Service) AddManufacturer(ctx context.Context, brandID string, in domain.ManufacturerInput) (domain.Manufacturer, error) {
	var out domain.Manufacturer
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetBrand(ctx, brandID); err != nil {
			return mapNotFound(err, domain.ErrBrandNotFound)
		}
		m, err := repo.InsertManufacturer(ctx, brandID, in)
		if err != nil {
			return err
		}
		out = m
		return s.record(ctx, tx, "vehicle.manufacturer.add", m.ID, m)
	})
	return out, err
}

func (s *Service) RemoveManufacturer(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteManufacturer(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrLinkNotFound
		}
		return s.record(ctx, tx, "vehicle.manufacturer.remove", id, nil)
	})
}

// ============================ label helpers (transport) ============================

func (s *Service) TypeNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).TypeNamesByIDs(ctx, ids)
}

func (s *Service) BrandNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).BrandNamesByIDs(ctx, ids)
}

func (s *Service) ModelNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).ModelNamesByIDs(ctx, ids)
}

func (s *Service) CompanyNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).CompanyNamesByIDs(ctx, ids)
}

func (s *Service) PlaceNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).PlaceNamesByIDs(ctx, ids)
}

// ============================ infra ============================

// pageSizePolicy is this module's page-size policy, clamped through the shared kernel (M56 /
// pkg/listing) instead of a local copy.
var pageSizePolicy = listing.PageSize{Default: defaultPageSize, Max: maxPageSize}

func clampPageSize(n int) int { return pageSizePolicy.Resolve(n) }

func (s *Service) querier(ctx context.Context) db.Querier {
	return db.RequestQuerier(ctx, s.pool)
}

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.querier(ctx).Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record mints an Action RID (vehicle service=17, kind=action=3, type=0) in the caller's transaction
// and writes the audit row on it (D-Audit). Vehicle data is instance-global reference data → system actor.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(17, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: "vehicle",
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mapNotFound translates a bare pgx.ErrNoRows into the given module sentinel.
func mapNotFound(err error, sentinel error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sentinel
	}
	return err
}

func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
