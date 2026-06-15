// Package geo is the composition seam for the geo module (docs/modules/location.md): it wires the
// pgx/sqlc repository, the read-only application service, and the transport, then registers the
// GeoService Conjure routes. Register returns the application service so later milestones' modules
// can call its in-process ListCountries(...) (cross-module query path, overview.md). The country
// registry itself is written by the hermenea import pipeline (geo-countries / WOF), not here.
package geo

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	geoapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/geo"
	"github.com/olegamysk/go-oikumenea/internal/geo/adapters"
	"github.com/olegamysk/go-oikumenea/internal/geo/application"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	"github.com/olegamysk/go-oikumenea/internal/geo/transport"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the geo module over the platform pool and registers its routes onto the witchcraft
// router. The module is read-only (no audit, no writes); it owns no resources of its own (the pool
// is owned by platform), so there is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }

	svc := application.NewService(pool, repoFor)

	if err := geoapi.RegisterRoutesGeoService(info.Router, transport.NewService(svc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register geo service routes")
	}
	return svc, nil
}
