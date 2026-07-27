// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package personsensitive is the composition seam for the person directory's sensitive-data module
// (R-09 split): physical identity, the envelope-encrypted declared ethnicity, the financial/behavioural/
// psychological overlays, the encrypted declared party membership, and watchlist matches + regulatory
// sanctions. Consolidating the crypto Cipher surface here is the R-09 rationale.
//
// It shares the person aggregate's domain kernel (internal/person/domain) and, since PR-2b, owns its own
// persistence adapter + query package (internal/personsensitive/adapters over personsensitivesql)
// implementing the domain.SensitiveRepository port. Its service is composed into the one person transport
// (persontransport.Service), which delegates the sensitive endpoints to it; this module registers no
// routes of its own.
package personsensitive

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/internal/personsensitive/adapters"
	"github.com/olegamysk/go-oikumenea/internal/personsensitive/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
)

// Register builds the personsensitive application service over the platform pool, the audit service
// (writes record in-transaction — D-Audit), and the envelope cipher (D-CryptoProvider). The color and
// watchlist seams are late-bound by the composition root (SetColorLookup / SetWatchlistLookup). It owns
// no resources of its own (the pool is owned by platform).
func Register(pool *pgxpool.Pool, audit *auditapp.Service, cipher *crypto.Cipher) *application.Service {
	repoFor := func(conn db.DBTX) domain.SensitiveRepository { return adapters.NewRepository(conn) }
	return application.NewService(pool, repoFor, audit, cipher)
}
