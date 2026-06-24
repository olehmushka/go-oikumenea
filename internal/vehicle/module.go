// Package vehicle is the composition seam for the vehicle module (docs/modules/vehicle.md /
// D-Vehicles): it wires the pgx repository, the audited application service, and the transport, then
// registers the VehicleService Conjure routes. The type/number-type reference catalogs are
// migration-seeded, so there is no boot-time seeding here. Register returns the application service so
// a later PersonPurged subscriber can call ErasePersonRegistrations once the event bus carries it.
package vehicle

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	vehicleapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/vehicle"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/adapters"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/application"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/domain"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/transport"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the vehicle module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), and the localization service (translatable catalog name maps), then
// registers the VehicleService onto the witchcraft router. It owns no resources of its own.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit)
	if err := vehicleapi.RegisterRoutesVehicleService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register vehicle service routes")
	}
	return svc, nil
}
