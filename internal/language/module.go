// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package language is the composition seam for the language module (docs/modules/language.md;
// D-Languages, M18): it wires the pgx/sqlc repository, the read-only application service, and the
// transport, then registers the LanguageService Conjure routes. Register returns the application
// service so later modules can call its in-process ListLanguoids/GetLanguoid (cross-module query path,
// overview.md). The languoid + writing-system registry itself is written by the hermenea import
// pipeline (language-scheme / language-scripts), not here.
package language

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	languageapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/language"
	"github.com/olegamysk/go-oikumenea/internal/language/adapters"
	"github.com/olegamysk/go-oikumenea/internal/language/application"
	"github.com/olegamysk/go-oikumenea/internal/language/domain"
	"github.com/olegamysk/go-oikumenea/internal/language/transport"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the language module over the platform pool and registers its routes onto the
// witchcraft router. The module is read-only (no audit, no writes); it owns no resources of its own.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, loc *locapp.Service, enforcer *pep.Enforcer) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }

	svc := application.NewService(pool, repoFor)

	if err := languageapi.RegisterRoutesLanguageService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register language service routes")
	}
	return svc, nil
}
