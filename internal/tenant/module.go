// Package tenant is the composition seam for the tenant module (docs/modules/tenant.md): it seeds
// the graph registry, wires the pgx/sqlc repository, the application service, and the transport,
// then registers the TenantService Conjure routes. Register returns the application service so
// later milestones' modules (authorization's PDP closure reads, membership's unit validation) can
// call it in-process (cross-module query path, overview.md).
package tenant

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
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

// seedGraphsSQL idempotently seeds the graph registry (D-Graphs): command (default + undeletable +
// locked authority-bearing) and operational (authority-bearing). The RID PKs default via new_rid(),
// which reads the per-connection app.environment GUC — set by db.NewPool but NOT by atlas's
// migration connection — so RID-keyed seed rows are inserted at BOOT here, on the GUC-bearing pool,
// rather than in the migration. ON CONFLICT on the partial-unique code index makes this safe to run
// on every boot (and after the operator changes the default). See docs/architecture/decisions.md
// (boot-time idempotent seeding of RID-keyed reference rows). Precedent for M7 base-roles / M8.
const seedGraphsSQL = `
INSERT INTO oikumenea.tenant_graphs (code, name, is_default, is_authority_bearing) VALUES
  ('command',     'Command',     true,  true),
  ('operational', 'Operational', false, true)
ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`

// Register seeds the graph registry, builds the tenant module over the platform pool, the audit
// service (writes record in-transaction — D-Audit), and the localization service (name-map
// assembly), and registers its routes onto the witchcraft router. It owns no resources of its own
// (the pool is owned by platform), so there is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service) (*application.Service, error) {
	if _, err := pool.Exec(context.Background(), seedGraphsSQL); err != nil {
		return nil, werror.Wrap(err, "seed tenant graph registry")
	}

	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit)

	if err := tenantapi.RegisterRoutesTenantService(info.Router, transport.NewService(svc, loc)); err != nil {
		return nil, werror.Wrap(err, "register tenant service routes")
	}
	return svc, nil
}
