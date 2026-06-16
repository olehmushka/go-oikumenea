// Package transport implements the geo module's generated Conjure GeoService interface: it
// translates the wire contract to/from the application service (overview.md; D-Conjure). Generated
// code in internal/conjure is never hand-edited.
//
// Authorization (M7): reads require `country.read` (held anywhere — the country registry is
// instance-global, not unit-keyed), enforced via the PEP. The bearer token carries the acting
// subject (interim: token == person RID; see internal/authorization/pep).
package transport

import (
	"context"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	geoapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/geo"
	"github.com/olegamysk/go-oikumenea/internal/geo/application"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// Service adapts *application.Service to the generated geoapi.GeoService interface.
type Service struct {
	app *application.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the geo application service + PEP enforcer.
func NewService(app *application.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, pep: enforcer}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ geoapi.GeoService = Service{}

// ListCountries implements GET /countries.
func (s Service) ListCountries(ctx context.Context, token bearertoken.Token) (geoapi.CountryList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermCountryRead)); err != nil {
		return geoapi.CountryList{}, err
	}
	countries, err := s.app.ListCountries(ctx)
	if err != nil {
		return geoapi.CountryList{}, werror.WrapWithContextParams(ctx, err, "list countries failed")
	}
	out := make([]geoapi.Country, 0, len(countries))
	for _, c := range countries {
		out = append(out, toAPICountry(c))
	}
	return geoapi.CountryList{Countries: out}, nil
}

func toAPICountry(c domain.Country) geoapi.Country {
	return geoapi.Country{
		Id:     c.ID,
		Code:   c.Code,
		Name:   c.Name,
		Status: c.Status,
	}
}
