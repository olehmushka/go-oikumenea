// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package education is the composition seam for the education module (docs/modules/education.md /
// D-Education): it wires the pgx/sqlc repository, the audited application service, and the transport,
// then registers the EducationService Conjure routes. The reference catalogs (institution/unit kinds,
// ISCED degree levels) are migration-seeded, so there is no boot-time seeding here. Register returns the
// application service so later milestones could call it in-process.
package education

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	educationapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/education"
	educationrefapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/educationref"
	"github.com/olegamysk/go-oikumenea/internal/education/adapters"
	"github.com/olegamysk/go-oikumenea/internal/education/application"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	"github.com/olegamysk/go-oikumenea/internal/education/transport"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the education module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), the localization service (translatable name maps), and the tenant service
// (institutions are `university`-domain orgs and units are tenant units — M41 / D-UnifiedOrgGraph), then
// registers the EducationService onto the witchcraft router. It owns no resources of its own.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, tenant *tenantapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, tenant)
	if err := educationapi.RegisterRoutesEducationService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register education service routes")
	}
	if err := educationrefapi.RegisterRoutesEducationReferenceService(info.Router, transport.NewReferenceService(svc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register education reference service routes")
	}
	return svc, nil
}
