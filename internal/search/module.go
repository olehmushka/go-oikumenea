// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package search is the composition seam for the unified cross-type search module
// (docs/modules/search.md / D-UnifiedSearch, review-2026-09 R-26). The module owns no tables and
// no RIDs — it is a fan-in over the other modules' trigram search queries. Register builds the
// engine over the PEP's authority + permission probes and registers the Conjure routes; the
// PROVIDERS are wired afterwards by the composition root (cmd/oikumenea/search_providers.go),
// closing over the module services, and the engine's MustBeBound joins main.go's boot seam loop so
// an empty provider set fails startup.
package search

import (
	"context"

	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	searchapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/search"
	"github.com/olegamysk/go-oikumenea/internal/search/application"
	"github.com/olegamysk/go-oikumenea/internal/search/transport"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the search module over the shared PEP enforcer. The bearer token params on the
// pep probes are call-site-stability vestiges (the subject rides the request context), hence the
// zero tokens here.
func Register(info witchcraft.InitInfo, enforcer *pep.Enforcer) (*application.Service, error) {
	svc := application.NewService(
		func(ctx context.Context) (string, bool, error) { return enforcer.SubjectAuthority(ctx) },
		func(ctx context.Context, action string) (bool, error) {
			return enforcer.AllowedAnywhere(ctx, bearertoken.Token(""), action)
		},
	)
	if err := searchapi.RegisterRoutesSearchService(info.Router, transport.NewService(svc)); err != nil {
		return nil, werror.Wrap(err, "register search service routes")
	}
	return svc, nil
}
