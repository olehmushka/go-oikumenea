// Location application logic (D-Location, M19): audited CRUD + spatial reads over the shared place
// entity. Writes run in a transaction that also records the audit Action (D-Audit); reads run on the
// pool. Mirrors the rank module's write orchestration. A location is instance-global (no unit scope),
// so writes are recorded as a `system` action with no unit attributed (interim, until M7/M8 resolve a
// person actor — the no-unaudited-mutation ground rule still holds).
package application

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

const (
	locationAuditSubsystem = "location-admin"
	targetLocation         = "location"
)

// maxPageSize caps a spatial page; the application clamps the requested size into [1, maxPageSize].
const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// ---------------------------------------------------------------- reads

// GetLocation reads one location by RID (domain.ErrLocationNotFound when absent/soft-deleted).
func (s *Service) GetLocation(ctx context.Context, id string) (domain.Location, error) {
	return s.newRepo(s.pool).GetLocation(ctx, id)
}

// LocationExists reports whether a location RID resolves to a live row (domain.ErrLocationNotFound
// otherwise). It is the cross-module verification seam consumers use before FK-referencing a location
// (D-PersonAddresses, M32 — the person module's domain.LocationLookup port is satisfied by this method).
func (s *Service) LocationExists(ctx context.Context, id string) error {
	_, err := s.newRepo(s.pool).GetLocation(ctx, id)
	return err
}

// ListLocationTypes reads the active place-type catalog.
func (s *Service) ListLocationTypes(ctx context.Context) ([]domain.LocationType, error) {
	return s.newRepo(s.pool).ListLocationTypes(ctx)
}

// ListLocationsNear returns locations within radiusM metres of (lat,lng), nearest first, plus a flag
// for whether another page exists (the caller encodes the page token). Keyset-paginated on the
// (distance, id) sort key: afterDist/afterID resume strictly after the last row of the previous page
// (empty afterID starts at the nearest — review R-21, replacing OFFSET).
func (s *Service) ListLocationsNear(ctx context.Context, lat, lng, radiusM, afterDist float64, afterID string, pageSize int) ([]domain.Location, bool, error) {
	limit := clampPageSize(pageSize)
	rows, err := s.newRepo(s.pool).ListLocationsNear(ctx, lat, lng, radiusM, afterDist, afterID, limit+1)
	if err != nil {
		return nil, false, err
	}
	return trimPage(rows, limit)
}

// ListLocationsInBbox returns locations whose coordinate falls inside the box, ordered by id,
// keyset-paginated on id (review R-21, replacing OFFSET).
func (s *Service) ListLocationsInBbox(ctx context.Context, minLat, minLng, maxLat, maxLng float64, after string, pageSize int) ([]domain.Location, bool, error) {
	limit := clampPageSize(pageSize)
	rows, err := s.newRepo(s.pool).ListLocationsInBbox(ctx, minLat, minLng, maxLat, maxLng, after, limit+1)
	if err != nil {
		return nil, false, err
	}
	return trimPage(rows, limit)
}

// SearchLocations returns locations whose address fields match a case-insensitive text query, ordered
// by id, keyset-paginated on id (no spatial window required) — backs the typeahead picker (review R-21).
func (s *Service) SearchLocations(ctx context.Context, query, after string, pageSize int) ([]domain.Location, bool, error) {
	limit := clampPageSize(pageSize)
	rows, err := s.newRepo(s.pool).SearchLocationsByText(ctx, query, after, limit+1)
	if err != nil {
		return nil, false, err
	}
	return trimPage(rows, limit)
}

// ---------------------------------------------------------------- writes

// CreateLocation resolves the supplied coordinate to canonical WGS84, derives the MGRS, inserts the
// location, then records the action. ErrCoordinateInvalid/ErrCoordinateOutOfRange on a bad coordinate;
// a bad country FK surfaces as ErrInvalidLocation.
func (s *Service) CreateLocation(ctx context.Context, in domain.LocationInput) (domain.Location, error) {
	w, err := in.ToWrite()
	if err != nil {
		return domain.Location{}, err
	}
	var out domain.Location
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertLocation(ctx, w)
		if err != nil {
			return mapWriteErr(err)
		}
		out = created
		return s.record(ctx, tx, "location.create", created.ID, created)
	})
	return out, err
}

// UpdateLocation resolves the supplied coordinate and replaces a location's coordinate/address/type
// (re-deriving the MGRS), then records the action. ErrLocationNotFound when absent.
func (s *Service) UpdateLocation(ctx context.Context, id string, in domain.LocationInput) (domain.Location, error) {
	w, err := in.ToWrite()
	if err != nil {
		return domain.Location{}, err
	}
	var out domain.Location
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateLocation(ctx, id, w)
		if err != nil {
			return mapWriteErr(err)
		}
		out = updated
		return s.record(ctx, tx, "location.update", id, updated)
	})
	return out, err
}

// DeleteLocation soft-deletes a location and records the action. ErrLocationNotFound when absent;
// ErrLocationInUse if an owning link still references it (defence in depth — owner tables arrive in
// M20/M21).
func (s *Service) DeleteLocation(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteLocation(ctx, id)
		if err != nil {
			return mapWriteErr(err)
		}
		if n == 0 {
			return domain.ErrLocationNotFound
		}
		return s.record(ctx, tx, "location.delete", id, map[string]string{"id": id})
	})
}

// ---------------------------------------------------------------- helpers

// pageSizePolicy is this module's page-size policy, clamped through the shared kernel (M56 /
// pkg/listing) instead of a local copy.
var pageSizePolicy = listing.PageSize{Default: defaultPageSize, Max: maxPageSize}

func clampPageSize(n int) int { return pageSizePolicy.Resolve(n) }

// trimPage drops the sentinel (limit+1) row used to detect a next page and reports whether one exists.
func trimPage(rows []domain.Location, limit int) ([]domain.Location, bool, error) {
	if len(rows) > limit {
		return rows[:limit], true, nil
	}
	return rows, false, nil
}

// mapWriteErr maps a country/type FK violation (23503) to ErrInvalidLocation (bad reference) and a
// reference-from-owner violation to ErrLocationInUse; everything else passes through.
func mapWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		// A violation on the location's own FKs (country/type) is a bad request; a violation from an
		// owner table referencing this location is an in-use conflict.
		if pgErr.TableName == "location_locations" {
			return domain.ErrInvalidLocation
		}
		return domain.ErrLocationInUse
	}
	return err
}

// inTx runs fn in a transaction, committing on success and rolling back on error.
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

// record mints an Action RID in the caller's transaction and writes the audit row on it, so the audit
// entry commits iff the change commits (D-Audit). Location writes are instance-global, so no unit.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(12, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  locationAuditSubsystem,
		Action:     action,
		TargetType: targetLocation,
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
