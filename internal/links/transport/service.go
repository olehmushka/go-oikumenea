// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport adapts the generated LinkService Conjure interface onto the traversal engine
// (D-LinkTraversal). Like the search module there is no endpoint-level Require BY DESIGN:
// authorization is entirely per-link-arm (read-permission gate) + per-row (neighbor visibility
// trim) inside the engine — an authenticated subject with no read grants gets an empty result, not
// a 403. Reads are not audited (matches every other read path).
package transport

import (
	"context"
	"errors"

	authzapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/authorization"
	linksapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/links"
	"github.com/olegamysk/go-oikumenea/internal/links/application"
	"github.com/olegamysk/go-oikumenea/internal/links/domain"
	"github.com/palantir/pkg/bearertoken"
)

type Service struct {
	app *application.Service
}

func NewService(app *application.Service) *Service { return &Service{app: app} }

func (s *Service) GetObjectLinks(ctx context.Context, _ bearertoken.Token, ridArg string, linkTypes *string, pageSize *int, pageToken *string) (linksapi.ObjectLinks, error) {
	res, err := s.app.GetObjectLinks(ctx, ridArg, deref(linkTypes), derefInt(pageSize), deref(pageToken))
	if err != nil {
		return mapErr(linksapi.ObjectLinks{}, err)
	}
	out := linksapi.ObjectLinks{Rid: res.RID, Groups: make([]linksapi.LinkGroup, 0, len(res.Groups))}
	for _, g := range res.Groups {
		grp := linksapi.LinkGroup{
			LinkType:   g.LinkType,
			TargetType: g.TargetType,
			Direction:  g.Direction,
			Rows:       make([]linksapi.LinkRow, 0, len(g.Rows)),
		}
		for _, r := range g.Rows {
			grp.Rows = append(grp.Rows, toRow(r))
		}
		out.Groups = append(out.Groups, grp)
	}
	if res.NextPageToken != "" {
		t := res.NextPageToken
		out.NextPageToken = &t
	}
	return out, nil
}

func (s *Service) SearchAround(ctx context.Context, _ bearertoken.Token, ridArg string, depth *int, linkTypes *string, pageSize *int, pageToken *string) (linksapi.Neighborhood, error) {
	res, err := s.app.SearchAroundDepth(ctx, ridArg, deref(linkTypes), derefIntDefault(depth, 1), derefInt(pageSize), deref(pageToken))
	if err != nil {
		return mapErr(linksapi.Neighborhood{}, err)
	}
	out := linksapi.Neighborhood{Rid: res.RID, Neighbors: make([]linksapi.LinkRow, 0, len(res.Neighbors))}
	for _, r := range res.Neighbors {
		out.Neighbors = append(out.Neighbors, toRow(r))
	}
	if res.NextPageToken != "" {
		t := res.NextPageToken
		out.NextPageToken = &t
	}
	return out, nil
}

func toRow(r domain.RawLink) linksapi.LinkRow {
	row := linksapi.LinkRow{
		LinkRid:    r.LinkRID,
		TargetRid:  r.TargetRID,
		TargetType: r.TargetType,
		Direction:  r.Direction,
	}
	if len(r.Labels) > 0 {
		l := r.Labels
		row.TargetLabel = &l
	}
	if len(r.Attrs) > 0 {
		a := r.Attrs
		row.Attrs = &a
	}
	if r.Hop != 0 {
		h := r.Hop
		row.Hop = &h
	}
	if r.ViaRID != "" {
		v := r.ViaRID
		row.ViaRid = &v
	}
	return row
}

func mapErr[T any](zero T, err error) (T, error) {
	var unknown domain.UnknownObjectTypeError
	switch {
	case errors.As(err, &unknown):
		return zero, linksapi.NewUnknownObjectType(unknown.RID)
	case errors.Is(err, domain.ErrInvalidPageToken):
		return zero, linksapi.NewInvalidPageToken()
	case errors.Is(err, domain.ErrNoSubject):
		return zero, authzapi.NewPermissionDenied("links")
	}
	return zero, err
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

func derefIntDefault(i *int, def int) int {
	if i == nil {
		return def
	}
	return *i
}
