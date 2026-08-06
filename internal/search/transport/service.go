// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport adapts the generated SearchService Conjure interface onto the fan-in engine
// (D-UnifiedSearch). There is no endpoint-level Require here BY DESIGN: authorization is entirely
// per-provider (read-permission gate) + per-row (D-VisibilityScope trim) inside the engine — an
// authenticated subject with no read grants gets an empty page, not a 403. Reads are not audited
// (matches every other read path).
package transport

import (
	"context"
	"errors"

	authzapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/authorization"
	searchapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/search"
	"github.com/olehmushka/go-oikumenea/internal/search/application"
	"github.com/olehmushka/go-oikumenea/internal/search/domain"
	"github.com/palantir/pkg/bearertoken"
)

type Service struct {
	app *application.Service
}

func NewService(app *application.Service) *Service { return &Service{app: app} }

func (s *Service) SearchObjects(ctx context.Context, _ bearertoken.Token, query string, types *string, perTypeLimit, pageSize *int, pageToken *string) (searchapi.SearchResultPage, error) {
	page, err := s.app.SearchObjects(ctx, query, deref(types), derefInt(perTypeLimit), derefInt(pageSize), deref(pageToken))
	if err != nil {
		var unknown domain.UnknownObjectTypeError
		switch {
		case errors.Is(err, domain.ErrQueryTooShort):
			return searchapi.SearchResultPage{}, searchapi.NewQueryTooShort(domain.MinQueryLength)
		case errors.As(err, &unknown):
			return searchapi.SearchResultPage{}, searchapi.NewUnknownObjectType(unknown.ObjectType)
		case errors.Is(err, domain.ErrInvalidPageToken):
			return searchapi.SearchResultPage{}, searchapi.NewInvalidPageToken()
		case errors.Is(err, domain.ErrNoSubject):
			return searchapi.SearchResultPage{}, authzapi.NewPermissionDenied("search")
		}
		return searchapi.SearchResultPage{}, err
	}
	out := searchapi.SearchResultPage{Hits: make([]searchapi.SearchHit, 0, len(page.Hits))}
	for _, h := range page.Hits {
		hit := searchapi.SearchHit{Rid: h.RID, ObjectType: h.ObjectType, Label: h.Label}
		if h.Snippet != "" {
			snippet := h.Snippet
			hit.Snippet = &snippet
		}
		out.Hits = append(out.Hits, hit)
	}
	if page.NextPageToken != "" {
		token := page.NextPageToken
		out.NextPageToken = &token
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
