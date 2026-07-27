// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package links is the composition seam for the generic link-traversal module (docs/modules/links.md
// / D-LinkTraversal, review-2026-09 R-27). The module owns no tables and no RIDs — it is a fan-in
// over the other modules' reified link tables. Register builds the engine over the PEP's authority +
// permission probes and registers the Conjure routes; the DESCRIPTORS (and per-neighbor-type
// visibility scopes + labelers) are wired afterwards by the composition root
// (cmd/oikumenea/link_descriptors.go), closing over the module services, and the engine's
// MustBeBound joins main.go's boot seam loop so an incomplete/undrifted descriptor set fails startup.
package links

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	linksapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/links"
	"github.com/olegamysk/go-oikumenea/internal/links/application"
	"github.com/olegamysk/go-oikumenea/internal/links/transport"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the link-traversal module over the shared pool + PEP enforcer. The bearer token
// param on the pep probe is a call-site-stability vestige (the subject rides the request context).
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, enforcer *pep.Enforcer) (*application.Service, error) {
	svc := application.NewService(
		pool,
		func(ctx context.Context) (string, bool, error) { return enforcer.SubjectAuthority(ctx) },
		func(ctx context.Context, action string) (bool, error) {
			return enforcer.AllowedAnywhere(ctx, bearertoken.Token(""), action)
		},
	)
	if err := linksapi.RegisterRoutesLinkService(info.Router, transport.NewService(svc)); err != nil {
		return nil, werror.Wrap(err, "register link service routes")
	}
	return svc, nil
}
