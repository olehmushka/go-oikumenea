// Package audit is the composition seam for the audit module (docs/modules/audit.md): it wires the
// pgx/sqlc repository, the application service, and the transport, then registers the AuditService
// Conjure routes. Register returns the application service so later milestones' modules can call
// its in-transaction Record(...) (D-Audit) directly (cross-module query path, overview.md).
package audit

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	"github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/audit/transport"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	auditapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/audit"
	pconfig "github.com/olegamysk/go-oikumenea/internal/platform/config"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	pkgconfig "github.com/olegamysk/go-oikumenea/pkg/config"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// defaultPageSize is the fallback when runtime config supplies no positive default-page-size.
const defaultPageSize = 50

// Register builds the audit module over the platform pool and registers its read routes onto the
// witchcraft router. The returned *application.Service is the in-process Record(...) entry point
// every write-bearing module will use; it carries no resources of its own (the pool is owned by
// platform), so there is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }

	sizeRef := pkgconfig.IntOrDefault(info.RuntimeConfig, defaultPageSize, func(v any) int {
		if rt, ok := v.(pconfig.Runtime); ok {
			return rt.DefaultPageSize
		}
		return 0
	})
	pageSize := func() int {
		if n, ok := sizeRef.Current().(int); ok {
			return n
		}
		return defaultPageSize
	}

	svc := application.NewService(pool, repoFor, pageSize)

	if err := auditapi.RegisterRoutesAuditService(info.Router, transport.NewService(svc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register audit service routes")
	}
	return svc, nil
}
