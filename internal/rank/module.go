// Package rank is the composition seam for the rank module (docs/modules/rank.md): it wires the
// pgx/sqlc repository, the application service, and the transport, then registers the RankService
// Conjure routes. Register returns the application service so a later milestone's module (person, M5,
// validates a person's rank_id against the scheme) can call it in-process (cross-module query path,
// overview.md). The rank scheme seeds NO rows: it is deployment-specific (L-SingleDomain) and
// populated by the instance admin through the API, so there is no boot seed (unlike tenant's graphs).
package rank

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	rankapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/rank"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/rank/adapters"
	"github.com/olegamysk/go-oikumenea/internal/rank/application"
	"github.com/olegamysk/go-oikumenea/internal/rank/domain"
	"github.com/olegamysk/go-oikumenea/internal/rank/transport"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the rank module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), and the localization service (name-map assembly), and registers its
// routes onto the witchcraft router. It owns no resources of its own (the pool is owned by platform),
// so there is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit)

	if err := rankapi.RegisterRoutesRankService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register rank service routes")
	}
	return svc, nil
}
