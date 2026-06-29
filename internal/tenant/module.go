// Package tenant is the composition seam for the tenant module (docs/modules/tenant.md): it seeds
// the graph registry, wires the pgx/sqlc repository, the application service, and the transport,
// then registers the TenantService Conjure routes. Register returns the application service so
// later milestones' modules (authorization's PDP closure reads, membership's unit validation) can
// call it in-process (cross-module query path, overview.md).
package tenant

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	tenantapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/tenant"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	"github.com/olegamysk/go-oikumenea/internal/tenant/application"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/internal/tenant/transport"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the tenant module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), and the localization service (name-map assembly), and registers its routes
// onto the witchcraft router. The domain + unit-kind reference catalogs are seeded by migration
// 0003_tenant (RID seeding in migrations is valid post-F-014 — new_id reads no GUC), not at boot. Per-org
// command/operational graphs are created with each organization (application.CreateOrganization). It owns
// no resources of its own (the pool is owned by platform), so there is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit)

	if err := tenantapi.RegisterRoutesTenantService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register tenant service routes")
	}
	return svc, nil
}
