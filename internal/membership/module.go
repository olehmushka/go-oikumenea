// Package membership is the composition seam for the membership module
// (docs/modules/membership.md): it wires the pgx/sqlc repository, the application service, and the
// transport, then registers the MembershipService Conjure routes. Register returns the application
// service so later milestones' modules call it in-process — order (M10) applies appointment/removal
// effects by calling fill/end in the issue transaction, citing order_item_id provenance
// (cross-module path, overview.md).
//
// Positions are unit-owned billets that exist while vacant; memberships are the person->unit
// belonging/filling Link. The owning unit/person/rank are validated through the database FKs; the
// localization service is taken to assemble the `locale -> text` position title maps responses
// return.
package membership

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	membershipapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/membership"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/membership/adapters"
	"github.com/olegamysk/go-oikumenea/internal/membership/application"
	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
	"github.com/olegamysk/go-oikumenea/internal/membership/transport"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the membership module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), and the localization service (title-map assembly), and registers its
// routes onto the witchcraft router. It seeds nothing (positions/memberships are created through the
// API) and owns no resources of its own (the pool is owned by platform), so there is no
// module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit)

	if err := membershipapi.RegisterRoutesMembershipService(info.Router, transport.NewService(svc, loc)); err != nil {
		return nil, werror.Wrap(err, "register membership service routes")
	}
	return svc, nil
}
