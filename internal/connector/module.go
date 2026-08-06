// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package connector is the composition seam for the connector-plane module (M53 / D-ConnectorPlane):
// it wires the pgx repository, the audited application service, and the transport, then registers the
// ConnectorService Conjure routes. The registry has no boot-time seeding — connectors self-register at
// their own boot. Register returns the application service so callers/tests can reach it directly.
package connector

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	connectorapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/connector"
	wiringapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/wiring"
	"github.com/olehmushka/go-oikumenea/internal/connector/adapters"
	"github.com/olehmushka/go-oikumenea/internal/connector/application"
	"github.com/olehmushka/go-oikumenea/internal/connector/domain"
	"github.com/olehmushka/go-oikumenea/internal/connector/transport"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the connector-plane module over the platform pool, the audit service (writes record
// in-transaction — D-Audit) and the PEP enforcer, then registers the ConnectorService onto the
// witchcraft router. It returns the application service so main.go can register the WiringService over
// it (the wiring `self` surface reads the same registry) via RegisterWiring.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit)
	if err := connectorapi.RegisterRoutesConnectorService(info.Router, transport.NewService(svc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register connector service routes")
	}
	return svc, nil
}

// RegisterWiring wires the pull-wiring read API (M53 / D-ConnectorPlane) — resolve, reference-catalog
// reads, and a connector's own cursors — over the connector registry plus the reference-catalog
// readers (geo/language/legal-basis) and the country namer (localization). It is separate from Register
// because it composes OTHER modules that must be constructed first; main.go calls it once those exist.
func RegisterWiring(info witchcraft.InitInfo, svc *application.Service, countries transport.CountryReader, langs transport.LanguageReader, legal transport.LegalBasisReader, names transport.CountryNamer, enforcer *pep.Enforcer) error {
	w := transport.NewWiringService(svc, countries, langs, legal, names, enforcer)
	if err := wiringapi.RegisterRoutesWiringService(info.Router, w); err != nil {
		return werror.Wrap(err, "register wiring service routes")
	}
	return nil
}
