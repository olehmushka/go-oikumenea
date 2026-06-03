// Package localization is the composition seam for the localization module
// (docs/modules/localization.md): it wires the pgx/sqlc repository, the application service, and the
// transport, then registers the LocalizationService Conjure routes. Register returns the application
// service so later milestones' modules can call its in-process TranslationsFor(...) to assemble
// localized responses (cross-module query path, overview.md).
package localization

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	locapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/localization"
	"github.com/olegamysk/go-oikumenea/internal/localization/adapters"
	"github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/localization/domain"
	"github.com/olegamysk/go-oikumenea/internal/localization/transport"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the localization module over the platform pool and the audit service, and
// registers its routes onto the witchcraft router. Writes record audit rows in their own
// transaction via the supplied audit service (D-Audit). The returned *application.Service exposes
// TranslationsFor(...) for later modules; it owns no resources of its own (the pool is owned by
// platform), so there is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }

	svc := application.NewService(pool, repoFor, audit)

	if err := locapi.RegisterRoutesLocalizationService(info.Router, transport.NewService(svc)); err != nil {
		return nil, werror.Wrap(err, "register localization service routes")
	}
	return svc, nil
}
