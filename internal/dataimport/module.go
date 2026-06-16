// Package dataimport is the composition seam for the data-import module (M16 / D-Hermenea): it wires
// the application service over the platform pool + audit, registers the available object-type upsert
// handlers (geo-countries first), and registers the generic ImportService Conjure route. This is the
// oikumenea SIDE of the ingestion pipeline — the connectors/mappers/scheduler live out of process in
// the hermenea companion (docs/modules/hermenea.md), which calls POST /import/{objectType} here.
package dataimport

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	dataimportapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/dataimport"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/adapters"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/application"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/transport"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the data-import module over the platform pool + audit service (writes record
// in-transaction — D-Audit), registers the geo-countries upsert handler, and registers the
// ImportService routes onto the witchcraft router. It owns no resources of its own.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	svc := application.NewService(pool, audit)

	// geo-countries: the first importable catalog (M16). The store factory binds the sqlc adapter to
	// the caller's transaction.
	svc.Register(domain.ObjectTypeGeoCountries, application.GeoCountriesHandler(
		func(conn db.DBTX) domain.GeoCountryStore { return adapters.NewGeoCountryRepo(conn) },
	))

	// geo-places: the Who's-On-First administrative gazetteer (D-GeoPlaces) — the first real connector's
	// load target. A placetype=country record also enriches the geo_countries row in the same tx.
	svc.Register(domain.ObjectTypeGeoPlaces, application.GeoPlacesHandler(
		func(conn db.DBTX) domain.GeoPlaceStore { return adapters.NewGeoPlaceRepo(conn) },
	))

	// language-scheme: the Glottolog languoid forest (D-Languages, M18) — the first NEW import consumer.
	// Parent-first upsert; the handler rebuilds the closure + family_code at the end of the batch.
	svc.Register(domain.ObjectTypeLanguageScheme, application.LanguageSchemeHandler(
		func(conn db.DBTX) domain.LanguoidStore { return adapters.NewLanguoidRepo(conn) },
	))

	// language-scripts: the CLDR language→writing-system links (D-Languages). Resolves languoid (by ISO
	// 639-3) + writing system (by ISO-15924) and upserts the link; unresolved records are skipped.
	svc.Register(domain.ObjectTypeLanguageScripts, application.LanguageScriptsHandler(
		func(conn db.DBTX) domain.LanguageScriptStore { return adapters.NewLanguageScriptRepo(conn) },
	))

	if err := dataimportapi.RegisterRoutesImportService(info.Router, transport.NewService(svc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register import service routes")
	}
	return svc, nil
}
