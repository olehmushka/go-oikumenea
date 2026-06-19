// Location transport (D-Location, M19): implements the generated locationapi.LocationService — audited
// CRUD + spatial queries over the shared place entity. It PEP-gates each op (a location has no unit
// scope, so reads/writes are satisfied anywhere), assembles the place-type `name` as a locale->text map
// via the localization service, and maps domain errors to the Conjure Location:* SerializableErrors.
package transport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	locationapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/location"
	"github.com/olegamysk/go-oikumenea/internal/geo/application"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// entityLocationType is the i18n entity_type the place-type `name` locale-maps are stored under.
const entityLocationType = "location_type"

// LocationService adapts *application.Service to the generated locationapi.LocationService interface.
type LocationService struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewLocationService builds the transport adapter over the geo/location application service, the
// localization service (place-type name maps), and the PEP enforcer.
func NewLocationService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) LocationService {
	return LocationService{app: app, loc: loc, pep: enforcer}
}

const (
	locReadPerm    = string(authzdomain.PermLocationRead)
	locCreatePerm  = string(authzdomain.PermLocationCreate)
	locUpdatePerm  = string(authzdomain.PermLocationUpdate)
	locTypesManage = string(authzdomain.PermLocationTypesManage)
)

// compile-time assertion that the transport satisfies the generated server interface.
var _ locationapi.LocationService = LocationService{}

// ---------------------------------------------------------------- writes

func (s LocationService) CreateLocation(ctx context.Context, token bearertoken.Token, req locationapi.LocationWrite) (locationapi.Location, error) {
	if err := s.pep.RequireAnywhere(ctx, token, locCreatePerm); err != nil {
		return locationapi.Location{}, err
	}
	in, err := toInput(req)
	if err != nil {
		return locationapi.Location{}, err
	}
	loc, err := s.app.CreateLocation(ctx, in)
	if err != nil {
		return locationapi.Location{}, s.mapError(ctx, err, "")
	}
	return s.toAPI(ctx, loc)
}

func (s LocationService) UpdateLocation(ctx context.Context, token bearertoken.Token, locationID string, req locationapi.LocationWrite) (locationapi.Location, error) {
	if err := s.pep.RequireAnywhere(ctx, token, locUpdatePerm); err != nil {
		return locationapi.Location{}, err
	}
	in, err := toInput(req)
	if err != nil {
		return locationapi.Location{}, err
	}
	loc, err := s.app.UpdateLocation(ctx, locationID, in)
	if err != nil {
		return locationapi.Location{}, s.mapError(ctx, err, locationID)
	}
	return s.toAPI(ctx, loc)
}

func (s LocationService) DeleteLocation(ctx context.Context, token bearertoken.Token, locationID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, locUpdatePerm); err != nil {
		return err
	}
	if err := s.app.DeleteLocation(ctx, locationID); err != nil {
		return s.mapError(ctx, err, locationID)
	}
	return nil
}

// ---------------------------------------------------------------- reads

func (s LocationService) GetLocation(ctx context.Context, token bearertoken.Token, locationID string) (locationapi.Location, error) {
	if err := s.pep.RequireAnywhere(ctx, token, locReadPerm); err != nil {
		return locationapi.Location{}, err
	}
	loc, err := s.app.GetLocation(ctx, locationID)
	if err != nil {
		return locationapi.Location{}, s.mapError(ctx, err, locationID)
	}
	return s.toAPI(ctx, loc)
}

func (s LocationService) ListLocations(ctx context.Context, token bearertoken.Token, lat, lng, radiusM, minLat, minLng, maxLat, maxLng *float64, pageSize *int, pageToken, query *string) (locationapi.LocationPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, locReadPerm); err != nil {
		return locationapi.LocationPage{}, err
	}
	offset := decodeOffset(pageToken)
	size := 0
	if pageSize != nil {
		size = *pageSize
	}

	var (
		locs    []domain.Location
		hasMore bool
		err     error
	)
	switch {
	case query != nil && strings.TrimSpace(*query) != "":
		// Text search over address fields — no spatial window required (backs the typeahead picker).
		locs, hasMore, err = s.app.SearchLocations(ctx, strings.TrimSpace(*query), size, offset)
	case lat != nil && lng != nil && radiusM != nil:
		locs, hasMore, err = s.app.ListLocationsNear(ctx, *lat, *lng, *radiusM, size, offset)
	case minLat != nil && minLng != nil && maxLat != nil && maxLng != nil:
		locs, hasMore, err = s.app.ListLocationsInBbox(ctx, *minLat, *minLng, *maxLat, *maxLng, size, offset)
	default:
		return locationapi.LocationPage{}, locationapi.NewQueryWindowRequired()
	}
	if err != nil {
		return locationapi.LocationPage{}, s.mapError(ctx, err, "")
	}

	names, err := s.typeNames(ctx)
	if err != nil {
		return locationapi.LocationPage{}, s.mapError(ctx, err, "")
	}
	out := make([]locationapi.Location, 0, len(locs))
	for _, l := range locs {
		out = append(out, toAPILocation(l, names))
	}
	page := locationapi.LocationPage{Locations: out}
	if hasMore {
		next := encodeOffset(offset + len(out))
		page.NextPageToken = &next
	}
	return page, nil
}

func (s LocationService) ListLocationTypes(ctx context.Context, token bearertoken.Token) (locationapi.LocationTypeList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, locReadPerm); err != nil {
		return locationapi.LocationTypeList{}, err
	}
	types, err := s.app.ListLocationTypes(ctx)
	if err != nil {
		return locationapi.LocationTypeList{}, s.mapError(ctx, err, "")
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.ID] = t.Name
	}
	names, err := s.loc.NamesByID(ctx, entityLocationType, defaults)
	if err != nil {
		return locationapi.LocationTypeList{}, s.mapError(ctx, err, "")
	}
	out := make([]locationapi.LocationType, 0, len(types))
	for _, t := range types {
		out = append(out, locationapi.LocationType{Id: t.ID, Code: t.Code, Name: names[t.ID], Status: t.Status})
	}
	return locationapi.LocationTypeList{LocationTypes: out}, nil
}

// ---------------------------------------------------------------- mapping

// toInput maps the wire payload to the domain input, enforcing the coordinate-required rule (the
// coordinate is optional on the wire so a missing coordinate is a domain error, not a deserialization
// failure). The application resolves the coordinate to WGS84 and derives the MGRS.
func toInput(req locationapi.LocationWrite) (domain.LocationInput, error) {
	if req.Coordinate == nil {
		return domain.LocationInput{}, locationapi.NewCoordinateRequired()
	}
	return domain.LocationInput{
		Coordinate:  toDomainCoord(*req.Coordinate),
		CountryID:   req.CountryId,
		AdminArea1:  req.AdminArea1,
		AdminArea2:  req.AdminArea2,
		Locality:    req.Locality,
		Street:      req.Street,
		HouseNumber: req.HouseNumber,
		PostalCode:  req.PostalCode,
		RawAddress:  req.RawAddress,
		TypeID:      req.TypeId,
	}, nil
}

// toDomainCoord / fromDomainCoord bridge the Conjure CoordinateInput and the domain CoordinateInput
// (identical field shapes; the conversion logic lives in the domain).
func toDomainCoord(c locationapi.CoordinateInput) domain.CoordinateInput {
	return domain.CoordinateInput{
		Format: c.Format, Latitude: c.Latitude, Longitude: c.Longitude, MGRS: c.Mgrs,
		Zone: c.Zone, Hemisphere: c.Hemisphere, Easting: c.Easting, Northing: c.Northing, Grid: c.Grid,
	}
}

// fromDomainCoord unmarshals the stored source coordinate (json.RawMessage) back into the wire type.
func fromDomainCoord(raw []byte) *locationapi.CoordinateInput {
	if len(raw) == 0 {
		return nil
	}
	var c locationapi.CoordinateInput
	if err := json.Unmarshal(raw, &c); err != nil || c.Format == "" {
		return nil
	}
	return &c
}

// toAPI converts one domain Location, loading the place-type name maps so a typed location carries its
// localized type name.
func (s LocationService) toAPI(ctx context.Context, l domain.Location) (locationapi.Location, error) {
	names, err := s.typeNames(ctx)
	if err != nil {
		return locationapi.Location{}, s.mapError(ctx, err, "")
	}
	return toAPILocation(l, names), nil
}

// typeNames returns id -> (locale -> text) for the active place-type catalog (D-i18n).
func (s LocationService) typeNames(ctx context.Context) (map[string]map[string]string, error) {
	types, err := s.app.ListLocationTypes(ctx)
	if err != nil {
		return nil, err
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.ID] = t.Name
	}
	return s.loc.NamesByID(ctx, entityLocationType, defaults)
}

func toAPILocation(l domain.Location, typeNames map[string]map[string]string) locationapi.Location {
	out := locationapi.Location{
		Id:               l.ID,
		Latitude:         l.Latitude,
		Longitude:        l.Longitude,
		Mgrs:             l.MGRS,
		SourceCoordinate: fromDomainCoord(l.SourceCoordinate),
		CountryId:        l.CountryID,
		AdminArea1:       l.AdminArea1,
		AdminArea2:       l.AdminArea2,
		Locality:         l.Locality,
		Street:           l.Street,
		HouseNumber:      l.HouseNumber,
		PostalCode:       l.PostalCode,
		RawAddress:       l.RawAddress,
		TypeId:           l.TypeID,
		CreatedAt:        datetime.DateTime(l.CreatedAt),
		UpdatedAt:        datetime.DateTime(l.UpdatedAt),
	}
	if l.TypeID != nil {
		if nm, ok := typeNames[*l.TypeID]; ok {
			out.TypeName = &nm
		}
	}
	return out
}

func (s LocationService) mapError(ctx context.Context, err error, locationID string) error {
	switch {
	case errors.Is(err, domain.ErrLocationNotFound):
		return locationapi.NewLocationNotFound(locationID)
	case errors.Is(err, domain.ErrLocationInUse):
		return locationapi.NewLocationInUse(locationID)
	case errors.Is(err, domain.ErrCoordinateInvalid), errors.Is(err, domain.ErrCoordinateOutOfRange):
		return locationapi.NewCoordinateInvalid()
	case errors.Is(err, domain.ErrInvalidLocation):
		return locationapi.NewCoordinateRequired()
	}
	return werror.WrapWithContextParams(ctx, err, "location operation failed")
}

// ---------------------------------------------------------------- page token (opaque base64 offset)

func encodeOffset(off int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(off)))
}

// decodeOffset is lenient: an absent or unparseable token starts from offset 0.
func decodeOffset(token *string) int {
	if token == nil || *token == "" {
		return 0
	}
	raw, err := base64.RawURLEncoding.DecodeString(*token)
	if err != nil {
		return 0
	}
	off, err := strconv.Atoi(string(raw))
	if err != nil || off < 0 {
		return 0
	}
	return off
}
