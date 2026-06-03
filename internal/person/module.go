// Package person is the composition seam for the person module (docs/modules/person.md): it wires the
// pgx/sqlc repository, the application service, and the transport, then registers the PersonService
// Conjure routes. Register returns the application service so later modules call it in-process
// (membership references a person, M6; identity-federation links an account, M8; authorization's
// assignment subject is a person id, M7 — cross-module query path, overview.md).
//
// Person is the directory's core aggregate (account-optional, instance-global, holding one rank as a
// directory attribute). It validates name-variant locales, rank, and country codes through the
// database FKs (i18n_locales / rank_ranks / geo_countries); the localization and rank services are
// taken for forward use and validation symmetry.
package person

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/person/adapters"
	"github.com/olegamysk/go-oikumenea/internal/person/application"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/internal/person/transport"
	pconfig "github.com/olegamysk/go-oikumenea/internal/platform/config"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	rankapp "github.com/olegamysk/go-oikumenea/internal/rank/application"
	pkgconfig "github.com/olegamysk/go-oikumenea/pkg/config"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// defaultPurgeGraceHours is the deactivate->purge reversibility window when the runtime config omits
// person-purge-grace-hours (720h = 30 days; D-PersonReadScope).
const defaultPurgeGraceHours = 720

// Register builds the person module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), the localization service, and the rank service, and registers its
// routes onto the witchcraft router. The purge-grace window is read from the (refreshable) runtime
// config. It owns no resources of its own (the pool is owned by platform), so there is no
// module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, _ *locapp.Service, _ *rankapp.Service) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }

	graceRef := pkgconfig.IntOrDefault(info.RuntimeConfig, defaultPurgeGraceHours, func(v any) int {
		if rt, ok := v.(pconfig.Runtime); ok && rt.PersonPurgeGraceHours > 0 {
			return rt.PersonPurgeGraceHours
		}
		return 0
	})
	graceHours := func() int {
		if n, ok := graceRef.Current().(int); ok {
			return n
		}
		return defaultPurgeGraceHours
	}

	svc := application.NewService(pool, repoFor, audit, graceHours)

	if err := personapi.RegisterRoutesPersonService(info.Router, transport.NewService(svc)); err != nil {
		return nil, werror.Wrap(err, "register person service routes")
	}
	return svc, nil
}
