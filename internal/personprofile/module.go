// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package personprofile is the composition seam for the person directory's non-encrypted, person-owned
// directory data (R-09 split): citizenships, residences, addresses, contact channels, SPEAKS languages,
// person↔person relationships, and the non-encrypted institutional ties.
//
// It shares the person aggregate's domain kernel (internal/person/domain) and, since PR-2b, owns its own
// persistence adapter + query package (internal/personprofile/adapters over personprofilesql) implementing
// the domain.ProfileRepository port. Its service is composed into the one person transport
// (persontransport.Service), which delegates the profile endpoints to it; this module registers no routes
// of its own.
package personprofile

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	"github.com/olehmushka/go-oikumenea/internal/personprofile/adapters"
	"github.com/olehmushka/go-oikumenea/internal/personprofile/application"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// Register builds the personprofile application service over the platform pool and the audit service
// (writes record in-transaction — D-Audit). The location lookup seam is late-bound by the composition
// root (SetLocationLookup). It owns no resources of its own (the pool is owned by platform).
func Register(pool *pgxpool.Pool, audit *auditapp.Service) *application.Service {
	repoFor := func(conn db.DBTX) domain.ProfileRepository { return adapters.NewRepository(conn) }
	return application.NewService(pool, repoFor, audit)
}
