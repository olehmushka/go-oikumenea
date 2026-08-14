// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport implements the geo module's generated Conjure GeoService interface: it
// translates the wire contract to/from the application service (overview.md; D-Conjure). Generated
// code in internal/conjure is never hand-edited.
//
// Authorization (M7): reads require `country.read` (held anywhere — the country registry is
// instance-global, not unit-keyed), enforced via the PEP's RequireServiceOrPerson — a service
// principal holding the grant, or a person via the PDP (GH-37: RequireAnywhere's person-shaped
// subject resolution structurally denied every machine caller regardless of grants, the same class
// of gap fixed for religion.read in GH-33). The bearer token carries the acting subject (interim:
// token == person RID; see internal/authorization/pep).
package transport

import (
	"context"

	authzdomain "github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	geoapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/geo"
	"github.com/olehmushka/go-oikumenea/internal/geo/application"
	"github.com/olehmushka/go-oikumenea/internal/geo/domain"
	locapp "github.com/olehmushka/go-oikumenea/internal/localization/application"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// entityCountry is the i18n entity_type country `name` locale-maps are stored under (D-i18n). Keyed by
// the country CODE (the natural key), so the pinax translations preset resolves it without a RID lookup.
const entityCountry = "country"

// Service adapts *application.Service to the generated geoapi.GeoService interface.
type Service struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the geo application service, the localization store (for
// country name locale-maps, D-i18n), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, loc: loc, pep: enforcer}
}

// countryNames returns each country's `name` locale->text map (D-i18n), keyed by country code (the
// entity_id the translations preset stores under). The default-locale slot falls back to the `name`
// column; the i18n store overlays the other locales.
func (s Service) countryNames(ctx context.Context, countries []domain.Country) (map[string]map[string]string, error) {
	defaults := make(map[string]string, len(countries))
	for _, c := range countries {
		defaults[c.Code] = c.Name
	}
	return s.loc.NamesByID(ctx, entityCountry, defaults)
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ geoapi.GeoService = Service{}

// ListCountries implements GET /countries.
func (s Service) ListCountries(ctx context.Context, token bearertoken.Token) (geoapi.CountryList, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, string(authzdomain.PermCountryRead), ""); err != nil {
		return geoapi.CountryList{}, err
	}
	countries, err := s.app.ListCountries(ctx)
	if err != nil {
		return geoapi.CountryList{}, werror.WrapWithContextParams(ctx, err, "list countries failed")
	}
	names, err := s.countryNames(ctx, countries)
	if err != nil {
		return geoapi.CountryList{}, werror.WrapWithContextParams(ctx, err, "resolve country names failed")
	}
	out := make([]geoapi.Country, 0, len(countries))
	for _, c := range countries {
		out = append(out, toAPICountry(c, names[c.Code]))
	}
	return geoapi.CountryList{Countries: out}, nil
}

func toAPICountry(c domain.Country, name map[string]string) geoapi.Country {
	return geoapi.Country{
		Id:     c.ID,
		Code:   c.Code,
		Name:   name,
		Status: c.Status,
	}
}

// ListPlaces implements GET /places — active gazetteer places of a placetype (default region) under a
// country, for region pickers (D-GeoPlaces).
func (s Service) ListPlaces(ctx context.Context, token bearertoken.Token, country string, placetype *string) (geoapi.PlaceList, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, string(authzdomain.PermCountryRead), ""); err != nil {
		return geoapi.PlaceList{}, err
	}
	pt := ""
	if placetype != nil {
		pt = *placetype
	}
	places, err := s.app.ListPlaces(ctx, country, pt)
	if err != nil {
		return geoapi.PlaceList{}, werror.WrapWithContextParams(ctx, err, "list places failed")
	}
	out := make([]geoapi.Place, 0, len(places))
	for _, p := range places {
		out = append(out, toAPIPlace(p))
	}
	return geoapi.PlaceList{Places: out}, nil
}

// ResolveCoordinate implements GET /resolve — reverse-geocode a coordinate to country + nearest place
// for the locations-form prefill (D-GeoPlaces).
func (s Service) ResolveCoordinate(ctx context.Context, token bearertoken.Token, lat, lng float64) (geoapi.CoordinateResolution, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, string(authzdomain.PermCountryRead), ""); err != nil {
		return geoapi.CoordinateResolution{}, err
	}
	res, err := s.app.ResolveCoordinate(ctx, lat, lng)
	if err != nil {
		return geoapi.CoordinateResolution{}, werror.WrapWithContextParams(ctx, err, "resolve coordinate failed")
	}
	var out geoapi.CoordinateResolution
	if res.Country != nil {
		names, err := s.countryNames(ctx, []domain.Country{*res.Country})
		if err != nil {
			return geoapi.CoordinateResolution{}, werror.WrapWithContextParams(ctx, err, "resolve country name failed")
		}
		c := toAPICountry(*res.Country, names[res.Country.Code])
		out.Country = &c
	}
	if res.Place != nil {
		p := toAPINearestPlace(*res.Place)
		out.Place = &p
	}
	return out, nil
}

func toAPINearestPlace(p domain.NearestPlace) geoapi.NearestPlace {
	place := geoapi.NearestPlace{
		Id:             p.ID,
		Placetype:      p.Placetype,
		Name:           p.Name,
		DistanceMeters: p.DistanceMeters,
	}
	if p.CountryID != "" {
		cid := p.CountryID
		place.CountryId = &cid
	}
	return place
}

func toAPIPlace(p domain.Place) geoapi.Place {
	place := geoapi.Place{
		Id:        p.ID,
		Placetype: p.Placetype,
		Name:      p.Name,
		Status:    p.Status,
	}
	if p.CountryID != "" {
		cid := p.CountryID
		place.CountryId = &cid
	}
	return place
}
