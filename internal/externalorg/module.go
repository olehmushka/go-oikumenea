// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package externalorg is the composition seam for the external-organizations module
// (docs/modules/external-organizations.md / D-ExternalOrgs, M30): it wires the pgx repository, the
// audited application service, and the transport, then registers the ExternalOrganizationService Conjure
// routes. The kind catalog is migration-seeded, so there is no boot-time seeding here. Register returns
// the application service so callers/tests can reach it directly.
package externalorg

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	externalorgapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/externalorg"
	"github.com/olehmushka/go-oikumenea/internal/externalorg/adapters"
	"github.com/olehmushka/go-oikumenea/internal/externalorg/application"
	"github.com/olehmushka/go-oikumenea/internal/externalorg/domain"
	"github.com/olehmushka/go-oikumenea/internal/externalorg/transport"
	locapp "github.com/olehmushka/go-oikumenea/internal/localization/application"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the external-organizations module over the platform pool, the audit service (writes
// record in-transaction — D-Audit), and the localization service (translatable name maps), then
// registers the ExternalOrganizationService onto the witchcraft router. It owns no resources of its own.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit)
	if err := externalorgapi.RegisterRoutesExternalOrganizationService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register external-organization service routes")
	}
	return svc, nil
}
