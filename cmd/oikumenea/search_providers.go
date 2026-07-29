// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Search-provider wiring (review-2026-09 R-26 / D-UnifiedSearch): composition glue registering each
// searchable object type's SearchProvider TOGETHER with its D-VisibilityScope adapter (R-30) on the
// unified-search engine. Lives beside main.go because it is pure wiring — closures over the module
// application services, the same late-bound-seam posture as SetLocationLookup/SetWatchlistLookup.
//
// Every provider fans in an EXISTING per-module trigram search query (D-PersonSearch + the R-21
// generalization); object-type tokens are the ontology registry vocabulary (pkg/rid / the generated
// web mirror). Person is PreTrimmed: its search runs the D-PersonReadScope visibility semi-join in
// SQL (VisiblePersonIDsForSubjectSearch), so the engine skips the post-trim; the person-scope
// Visibility is registered regardless (the R-27 link facet will consume it).
package main

import (
	"context"
	"fmt"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/scope"
	companyapp "github.com/olegamysk/go-oikumenea/internal/company/application"
	educationapp "github.com/olegamysk/go-oikumenea/internal/education/application"
	geoapp "github.com/olegamysk/go-oikumenea/internal/geo/application"
	geodomain "github.com/olegamysk/go-oikumenea/internal/geo/domain"
	languageapp "github.com/olegamysk/go-oikumenea/internal/language/application"
	langdomain "github.com/olegamysk/go-oikumenea/internal/language/domain"
	membershipapp "github.com/olegamysk/go-oikumenea/internal/membership/application"
	personapp "github.com/olegamysk/go-oikumenea/internal/person/application"
	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	searchapp "github.com/olegamysk/go-oikumenea/internal/search/application"
	searchdomain "github.com/olegamysk/go-oikumenea/internal/search/domain"
)

func registerSearchProviders(
	searchSvc *searchapp.Service,
	personSvc *personapp.Service,
	membershipSvc *membershipapp.Service,
	languageSvc *languageapp.Service,
	geoSvc *geoapp.Service,
	educationSvc *educationapp.Service,
	companySvc *companyapp.Service,
) error {
	catalog := scope.NewCatalogScope()
	personScope := scope.NewPersonScope(membershipSvc.SubjectReadablePersonsAmong)

	type reg struct {
		p   searchdomain.Provider
		vis scope.Visibility
	}
	regs := []reg{
		{p: searchdomain.Provider{
			ObjectType:     "person",
			ReadPermission: string(authzdomain.PermPersonRead),
			PreTrimmed:     true, // visibility folded into the SQL below
			Search: func(ctx context.Context, subject string, isAdmin bool, q, after string, limit int) ([]searchdomain.RawHit, string, error) {
				var page personapp.Page
				var err error
				// Unified search passes only the text query: a search hit is a name match, not a
				// faceted listing (the facets belong to the list endpoint, D-ObjectFacets).
				filter := persondomain.PersonFilter{Query: q}
				if isAdmin {
					page, err = personSvc.ListPersons(ctx, filter, limit, after)
				} else {
					page, err = personSvc.ListVisiblePersons(ctx, subject, filter, limit, after)
				}
				if err != nil {
					return nil, "", err
				}
				hits := make([]searchdomain.RawHit, 0, len(page.Persons))
				for _, p := range page.Persons {
					hits = append(hits, searchdomain.RawHit{ID: p.ID, Label: p.DisplayName, Snippet: p.Code})
				}
				return hits, page.NextPageToken, nil
			},
		}, vis: personScope},

		{p: searchdomain.Provider{
			ObjectType:     "languoid",
			ReadPermission: string(authzdomain.PermLanguageRead),
			Search: func(ctx context.Context, _ string, _ bool, q, after string, limit int) ([]searchdomain.RawHit, string, error) {
				rows, next, err := languageSvc.ListLanguoidsPage(ctx, langdomain.Filter{Query: q, After: after, Limit: limit})
				if err != nil {
					return nil, "", err
				}
				hits := make([]searchdomain.RawHit, 0, len(rows))
				for _, r := range rows {
					hits = append(hits, searchdomain.RawHit{ID: r.ID, Label: r.Name, Snippet: r.Code})
				}
				return hits, next, nil
			},
		}, vis: catalog},

		{p: searchdomain.Provider{
			ObjectType:     "location",
			ReadPermission: string(authzdomain.PermLocationRead),
			Search: func(ctx context.Context, _ string, _ bool, q, after string, limit int) ([]searchdomain.RawHit, string, error) {
				rows, more, err := geoSvc.SearchLocations(ctx, q, after, limit)
				if err != nil {
					return nil, "", err
				}
				hits := make([]searchdomain.RawHit, 0, len(rows))
				for _, r := range rows {
					hits = append(hits, searchdomain.RawHit{ID: r.ID, Label: locationLabel(r), Snippet: deref(r.MGRS)})
				}
				next := ""
				if more && len(rows) > 0 {
					next = rows[len(rows)-1].ID
				}
				return hits, next, nil
			},
		}, vis: catalog},

		{p: searchdomain.Provider{
			ObjectType:     "institution",
			ReadPermission: string(authzdomain.PermEducationRead),
			Search: func(ctx context.Context, _ string, _ bool, q, after string, limit int) ([]searchdomain.RawHit, string, error) {
				rows, err := educationSvc.ListInstitutions(ctx, q, after, limit)
				if err != nil {
					return nil, "", err
				}
				rows, next := trimOverfetchTo(rows, limit, func(i int) string { return rows[i].ID })
				hits := make([]searchdomain.RawHit, 0, len(rows))
				for _, r := range rows {
					hits = append(hits, searchdomain.RawHit{ID: r.ID, Label: r.Name, Snippet: r.Code})
				}
				return hits, next, nil
			},
		}, vis: catalog},

		{p: searchdomain.Provider{
			ObjectType:     "company",
			ReadPermission: string(authzdomain.PermCompanyRead),
			Search: func(ctx context.Context, _ string, _ bool, q, after string, limit int) ([]searchdomain.RawHit, string, error) {
				rows, err := companySvc.ListCompanies(ctx, q, after, limit)
				if err != nil {
					return nil, "", err
				}
				rows, next := trimOverfetchTo(rows, limit, func(i int) string { return rows[i].ID })
				hits := make([]searchdomain.RawHit, 0, len(rows))
				for _, r := range rows {
					snippet := r.ShortName
					if snippet == "" {
						snippet = r.Code
					}
					hits = append(hits, searchdomain.RawHit{ID: r.ID, Label: r.LegalName, Snippet: snippet})
				}
				return hits, next, nil
			},
		}, vis: catalog},

		{p: searchdomain.Provider{
			ObjectType:     "publication",
			ReadPermission: string(authzdomain.PermEducationRead),
			Search: func(ctx context.Context, _ string, _ bool, q, after string, limit int) ([]searchdomain.RawHit, string, error) {
				rows, err := educationSvc.ListPublications(ctx, q, after, limit)
				if err != nil {
					return nil, "", err
				}
				rows, next := trimOverfetchTo(rows, limit, func(i int) string { return rows[i].ID })
				hits := make([]searchdomain.RawHit, 0, len(rows))
				for _, r := range rows {
					hits = append(hits, searchdomain.RawHit{ID: r.ID, Label: r.Title, Snippet: r.Code})
				}
				return hits, next, nil
			},
		}, vis: catalog},

		{p: searchdomain.Provider{
			ObjectType:     "scholarship",
			ReadPermission: string(authzdomain.PermEducationRead),
			Search: func(ctx context.Context, _ string, _ bool, q, after string, limit int) ([]searchdomain.RawHit, string, error) {
				rows, err := educationSvc.ListScholarships(ctx, q, after, limit)
				if err != nil {
					return nil, "", err
				}
				rows, next := trimOverfetchTo(rows, limit, func(i int) string { return rows[i].ID })
				hits := make([]searchdomain.RawHit, 0, len(rows))
				for _, r := range rows {
					hits = append(hits, searchdomain.RawHit{ID: r.ID, Label: r.Name, Snippet: r.Code})
				}
				return hits, next, nil
			},
		}, vis: catalog},
	}

	// A disabled vertical (D-DataPacks, M54) passes a nil service; drop its providers so the module falls
	// out of unified search rather than fanning in against a nil closure. education owns institution /
	// publication / scholarship; company owns company.
	skip := map[string]bool{}
	if educationSvc == nil {
		skip["institution"], skip["publication"], skip["scholarship"] = true, true, true
	}
	if companySvc == nil {
		skip["company"] = true
	}
	for _, r := range regs {
		if skip[r.p.ObjectType] {
			continue
		}
		if err := searchSvc.Register(r.p, r.vis); err != nil {
			return err
		}
	}
	return nil
}

// trimOverfetchTo resolves the module list methods' `limit+1` overfetch sentinel into (page, next
// cursor): more than `limit` rows means another page exists, keyed by the trimmed page's last id.
func trimOverfetchTo[T any](rows []T, limit int, idAt func(i int) string) ([]T, string) {
	if len(rows) <= limit {
		return rows, ""
	}
	rows = rows[:limit]
	return rows, idAt(limit - 1)
}

// locationLabel composes a location's display line: the raw address when captured, else the
// locality/admin composition, else the bare coordinates.
func locationLabel(l geodomain.Location) string {
	if s := deref(l.RawAddress); s != "" {
		return s
	}
	parts := make([]string, 0, 3)
	for _, p := range []*string{l.Locality, l.AdminArea2, l.AdminArea1} {
		if s := deref(p); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) > 0 {
		out := parts[0]
		for _, p := range parts[1:] {
			out += ", " + p
		}
		return out
	}
	return fmt.Sprintf("%.5f, %.5f", l.Latitude, l.Longitude)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
