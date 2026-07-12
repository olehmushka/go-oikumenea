package adapters

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/geo/adapters/geosql"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
)

// Location adapter methods (D-Location, M19). They translate between the domain Location shape and the
// sqlc rows; the geometry never appears here (it is built from lat/lng in SQL and projected back via
// ST_Y/ST_X). pgx maps no-rows to ErrLocationNotFound at this boundary.

func (r *Repository) InsertLocation(ctx context.Context, w domain.LocationWrite) (domain.Location, error) {
	row, err := r.q.InsertLocation(ctx, geosql.InsertLocationParams{
		Longitude:        w.Longitude,
		Latitude:         w.Latitude,
		Mgrs:             text(w.MGRS),
		SourceCoordinate: sourceJSON(w.SourceCoordinate),
		CountryID:        w.CountryID,
		AdminArea1:       text(w.AdminArea1),
		AdminArea2:       text(w.AdminArea2),
		Locality:         text(w.Locality),
		Street:           text(w.Street),
		HouseNumber:      text(w.HouseNumber),
		PostalCode:       text(w.PostalCode),
		RawAddress:       text(w.RawAddress),
		TypeID:           text(w.TypeID),
	})
	if err != nil {
		return domain.Location{}, err
	}
	return locationFromInsert(row), nil
}

func (r *Repository) GetLocation(ctx context.Context, id string) (domain.Location, error) {
	row, err := r.q.GetLocation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Location{}, domain.ErrLocationNotFound
	}
	if err != nil {
		return domain.Location{}, err
	}
	return locationFromGet(row), nil
}

func (r *Repository) UpdateLocation(ctx context.Context, id string, w domain.LocationWrite) (domain.Location, error) {
	row, err := r.q.UpdateLocation(ctx, geosql.UpdateLocationParams{
		Longitude:        w.Longitude,
		Latitude:         w.Latitude,
		Mgrs:             text(w.MGRS),
		SourceCoordinate: sourceJSON(w.SourceCoordinate),
		CountryID:        w.CountryID,
		AdminArea1:       text(w.AdminArea1),
		AdminArea2:       text(w.AdminArea2),
		Locality:         text(w.Locality),
		Street:           text(w.Street),
		HouseNumber:      text(w.HouseNumber),
		PostalCode:       text(w.PostalCode),
		RawAddress:       text(w.RawAddress),
		TypeID:           text(w.TypeID),
		ID:               id,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Location{}, domain.ErrLocationNotFound
	}
	if err != nil {
		return domain.Location{}, err
	}
	return locationFromUpdate(row), nil
}

func (r *Repository) SoftDeleteLocation(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteLocation(ctx, id)
}

func (r *Repository) ListLocationsNear(ctx context.Context, lat, lng, radiusM, afterDist float64, afterID string, limit int) ([]domain.Location, error) {
	rows, err := r.q.ListLocationsNear(ctx, geosql.ListLocationsNearParams{
		Lng: lng, Lat: lat, RadiusM: radiusM, AfterDist: afterDist, AfterID: afterID, Lim: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Location, 0, len(rows))
	for _, row := range rows {
		out = append(out, locationFromNear(row))
	}
	return out, nil
}

func (r *Repository) ListLocationsInBbox(ctx context.Context, minLat, minLng, maxLat, maxLng float64, after string, limit int) ([]domain.Location, error) {
	rows, err := r.q.ListLocationsInBbox(ctx, geosql.ListLocationsInBboxParams{
		MinLng: minLng, MinLat: minLat, MaxLng: maxLng, MaxLat: maxLat, After: after, Lim: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Location, 0, len(rows))
	for _, row := range rows {
		out = append(out, locationFromBbox(row))
	}
	return out, nil
}

func (r *Repository) SearchLocationsByText(ctx context.Context, query, after string, limit int) ([]domain.Location, error) {
	rows, err := r.q.SearchLocationsByText(ctx, geosql.SearchLocationsByTextParams{
		Query: query, After: after, Lim: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Location, 0, len(rows))
	for _, row := range rows {
		out = append(out, locationFromSearch(row))
	}
	return out, nil
}

func (r *Repository) ListLocationTypes(ctx context.Context) ([]domain.LocationType, error) {
	rows, err := r.q.ListLocationTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LocationType, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.LocationType{ID: row.ID, Code: row.Code, Name: row.Name, Status: row.Status})
	}
	return out, nil
}

// The sqlc Insert/Get/Update/Near/Bbox rows are structurally identical; mappers per row type keep the
// translation explicit without reflection.
func locationFromInsert(row geosql.InsertLocationRow) domain.Location {
	return domain.Location{
		ID: row.ID, Latitude: row.Latitude, Longitude: row.Longitude,
		MGRS: strp(row.Mgrs), SourceCoordinate: srcFromDB(row.SourceCoordinate), CountryID: row.CountryID,
		AdminArea1: strp(row.AdminArea1), AdminArea2: strp(row.AdminArea2), Locality: strp(row.Locality),
		Street: strp(row.Street), HouseNumber: strp(row.HouseNumber), PostalCode: strp(row.PostalCode),
		RawAddress: strp(row.RawAddress), TypeID: strp(row.TypeID),
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}

func locationFromGet(row geosql.GetLocationRow) domain.Location {
	return locationFromInsert(geosql.InsertLocationRow(row))
}

func locationFromUpdate(row geosql.UpdateLocationRow) domain.Location {
	return locationFromInsert(geosql.InsertLocationRow(row))
}

func locationFromNear(row geosql.ListLocationsNearRow) domain.Location {
	// The Near row carries the projection plus the computed distance_m (the second half of its keyset
	// cursor); map the shared fields, then attach the distance for the transport's page token (review R-21).
	loc := locationFromInsert(geosql.InsertLocationRow{
		ID: row.ID, Latitude: row.Latitude, Longitude: row.Longitude, Mgrs: row.Mgrs,
		SourceCoordinate: row.SourceCoordinate, CountryID: row.CountryID,
		AdminArea1: row.AdminArea1, AdminArea2: row.AdminArea2, Locality: row.Locality,
		Street: row.Street, HouseNumber: row.HouseNumber, PostalCode: row.PostalCode,
		RawAddress: row.RawAddress, TypeID: row.TypeID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	loc.DistanceM = row.DistanceM
	return loc
}

func locationFromBbox(row geosql.ListLocationsInBboxRow) domain.Location {
	return locationFromInsert(geosql.InsertLocationRow(row))
}

func locationFromSearch(row geosql.SearchLocationsByTextRow) domain.Location {
	return locationFromInsert(geosql.InsertLocationRow(row))
}

// sourceJSON / srcFromDB bridge the source_coordinate jsonb column (sqlc maps jsonb to []byte). A nil
// write defaults to '{}' so the NOT NULL column always holds valid JSON; an empty read surfaces as nil.
func sourceJSON(m json.RawMessage) []byte {
	if len(m) == 0 {
		return []byte("{}")
	}
	return m
}

func srcFromDB(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

// text / strp bridge *string <-> pgtype.Text at the sqlc boundary.
func text(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func strp(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	v := t.String
	return &v
}
