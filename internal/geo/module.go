// Package geo is the composition seam for the geo module (docs/modules/location.md): it wires the
// pgx/sqlc repository, the read-only application service, and the transport, then registers the
// GeoService Conjure routes. Register returns the application service so later milestones' modules
// can call its in-process ListCountries(...) (cross-module query path, overview.md). The country
// registry itself is written by the hermenea import pipeline (geo-countries / WOF), not here.
package geo

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	geoapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/geo"
	locationapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/location"
	"github.com/olegamysk/go-oikumenea/internal/geo/adapters"
	"github.com/olegamysk/go-oikumenea/internal/geo/application"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	"github.com/olegamysk/go-oikumenea/internal/geo/transport"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the geo/location module over the platform pool, the audit service (Location writes
// record in-transaction — D-Audit), and the localization service (place-type name maps), then
// registers the read-only GeoService (country registry) + the audited LocationService (D-Location, M19)
// onto the witchcraft router. It owns no resources of its own (the pool is owned by platform), so there
// is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }

	svc := application.NewService(pool, repoFor, audit)

	if err := geoapi.RegisterRoutesGeoService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register geo service routes")
	}
	if err := locationapi.RegisterRoutesLocationService(info.Router, transport.NewLocationService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register location service routes")
	}
	return svc, nil
}
