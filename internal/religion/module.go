// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package religion is the composition seam for the religion module (docs/modules/religion.md /
// D-Religion, M22): it wires the raw-pgx repository, the audited application service (which reuses the
// tenant service to build canonical-graph governance trees), and the transport, then registers the
// ReligionService Conjure routes. The taxonomy + per-faith catalogs are migration-seeded
// (deploy/religion-presets), so there is no boot-time seeding here. Register returns the application
// service so later milestones (M23–M25) can call it in-process.
package religion

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	religionapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/religion"
	locapp "github.com/olehmushka/go-oikumenea/internal/localization/application"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/olehmushka/go-oikumenea/internal/religion/adapters"
	"github.com/olehmushka/go-oikumenea/internal/religion/application"
	"github.com/olehmushka/go-oikumenea/internal/religion/transport"
	tenantapp "github.com/olehmushka/go-oikumenea/internal/tenant/application"
	"github.com/olehmushka/go-oikumenea/pkg/crypto"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the religion module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), the localization service (translatable name maps), and the tenant service
// (canonical-graph org placement for createChildOrg), and the envelope cipher (D-SpecialPII — seals lay
// affiliation belief values), then registers the ReligionService routes.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, tenant *tenantapp.Service, enforcer *pep.Enforcer, cipher *crypto.Cipher) (*application.Service, error) {
	repoFor := func(conn db.DBTX) application.Repo { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, tenant, cipher)
	if err := religionapi.RegisterRoutesReligionService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register religion service routes")
	}
	return svc, nil
}
