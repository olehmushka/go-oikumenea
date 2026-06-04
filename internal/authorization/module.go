// Package authorization is the composition seam for the authorization module
// (docs/modules/authorization.md) — the centerpiece: RBAC + the PDP. Register builds the PDP over
// the tenant graph closure (cross-module query), wires the pgx/sqlc repository, the application
// service, and the transport, seeds the four base roles at boot (D-BaseRoles / D-RIDSeeding), binds
// the shared PEP enforcer the other modules' transports use, and registers the AuthorizationService
// routes.
//
// Authority comes ONLY from assignments here; the PDP reads no rank/position. The returned service is
// the in-process decision seam (Enforce/Decide/EffectiveReach) other modules call before guarded
// ops; the bound enforcer is the transport-layer PEP. Ordering note: the PDP needs tenant's closure,
// so tenant builds first — hence the enforcer is created UNBOUND in the composition root and bound
// here once the service exists (see internal/authorization/pep).
package authorization

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/adapters"
	"github.com/olegamysk/go-oikumenea/internal/authorization/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	"github.com/olegamysk/go-oikumenea/internal/authorization/transport"
	authzapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/authorization"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the authorization module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), the localization service (role name/description maps), and the tenant
// service (the PDP closure + graph resolution). It seeds the base roles, binds the shared enforcer,
// and registers the AuthorizationService routes. It owns no resources of its own (the pool is owned
// by platform), so there is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, tenantSvc *tenantapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	pdp := domain.NewPDP(tenantSvc) // tenant.Service implements domain.ClosurePort
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, pdp, tenantSvc) // tenant.Service implements GraphPort

	if err := svc.SeedBaseRoles(context.Background()); err != nil {
		return nil, werror.Wrap(err, "seed authorization base roles")
	}
	enforcer.Bind(svc)

	if err := authzapi.RegisterRoutesAuthorizationService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register authorization service routes")
	}
	return svc, nil
}
