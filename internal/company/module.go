// Package company is the composition seam for the company module (docs/modules/company.md /
// D-Companies): it wires the pgx/sqlc repository, the audited application service, and the transport,
// then registers the CompanyService Conjure routes. The reference catalogs (legal forms, registration
// schemes, industry classes) are migration-seeded, so there is no boot-time seeding here. Register
// returns the application service so later milestones could call it in-process.
package company

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	"github.com/olegamysk/go-oikumenea/internal/company/adapters"
	"github.com/olegamysk/go-oikumenea/internal/company/application"
	"github.com/olegamysk/go-oikumenea/internal/company/domain"
	"github.com/olegamysk/go-oikumenea/internal/company/transport"
	companyapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/company"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the company module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), the localization service (translatable name maps), and the tenant service
// (a company = a `company`-domain org — M41), then registers the CompanyService onto the witchcraft
// router. It owns no resources of its own.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, tenant *tenantapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, tenant)
	if err := companyapi.RegisterRoutesCompanyService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register company service routes")
	}
	return svc, nil
}
