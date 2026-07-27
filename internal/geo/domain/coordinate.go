// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

// Coordinate input conversion (D-Location, M19). A location's coordinate can be supplied in several
// formats; the application converts each to a canonical WGS84 lat/lon (the geom spine) and derives the
// MGRS in pure Go — no DB extension, no cgo. The original input is preserved verbatim in the location's
// source_coordinate JSONB. The converter set is a small registry keyed by `format`, so adding a format
// is a local change here, not a schema change.
//
// Projection/datum maths use github.com/wroge/wgs84 (zero-dependency, pure Go): UTM and СК-42 are both
// transverse-Mercator projections, differing only in ellipsoid (WGS84 vs Krassowsky-1940), scale factor
// (0.9996 vs 1.0), and the datum shift to WGS84 (none vs a Pulkovo-1942 Helmert). MGRS forward/inverse
// is implemented here (the forward is a 1:1 port of the previous plpgsql location_mgrs()).

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/wroge/wgs84"
)

// Supported coordinate input formats (the CoordinateInput.Format discriminator).
const (
	FormatLatLon   = "latlon"
	FormatMGRS     = "mgrs"
	FormatUTM      = "utm"
	FormatSK42     = "sk42"
	FormatSK42Grid = "sk42grid"
)

// CoordinateInput is a coordinate supplied in one of the supported formats. Format selects which fields
// are read; the rest are ignored. It mirrors the Conjure CoordinateInput (same JSON field names) so the
// stored source_coordinate round-trips back into the API response unchanged.
type CoordinateInput struct {
	Format     string   `json:"format"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
	MGRS       *string  `json:"mgrs,omitempty"`
	Zone       *int     `json:"zone,omitempty"`
	Hemisphere *string  `json:"hemisphere,omitempty"`
	Easting    *float64 `json:"easting,omitempty"`
	Northing   *float64 `json:"northing,omitempty"`
	Grid       *string  `json:"grid,omitempty"`
}

// ToWGS84 converts the input to canonical WGS84 (lat, lon), or ErrCoordinateInvalid when the fields the
// chosen format needs are missing/unparseable, or ErrCoordinateOutOfRange when the result is off-Earth.
func (in CoordinateInput) ToWGS84() (lat, lon float64, err error) {
	switch strings.ToLower(strings.TrimSpace(in.Format)) {
	case FormatLatLon:
		if in.Latitude == nil || in.Longitude == nil {
			return 0, 0, ErrCoordinateInvalid
		}
		lat, lon = *in.Latitude, *in.Longitude
	case FormatMGRS:
		if in.MGRS == nil {
			return 0, 0, ErrCoordinateInvalid
		}
		lat, lon, err = parseMGRS(*in.MGRS)
	case FormatUTM:
		if in.Zone == nil || in.Easting == nil || in.Northing == nil {
			return 0, 0, ErrCoordinateInvalid
		}
		north := !(in.Hemisphere != nil && strings.EqualFold(strings.TrimSpace(*in.Hemisphere), "S"))
		lat, lon, err = utmToWGS84(*in.Zone, *in.Easting, *in.Northing, north)
	case FormatSK42:
		if in.Zone == nil || in.Easting == nil || in.Northing == nil {
			return 0, 0, ErrCoordinateInvalid
		}
		lat, lon, err = sk42ToWGS84(*in.Zone, *in.Easting, *in.Northing)
	case FormatSK42Grid:
		if in.Grid == nil {
			return 0, 0, ErrCoordinateInvalid
		}
		lat, lon, err = sk42GridToWGS84(*in.Grid)
	default:
		return 0, 0, ErrCoordinateInvalid
	}
	if err != nil {
		return 0, 0, err
	}
	if math.IsNaN(lat) || math.IsNaN(lon) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, ErrCoordinateOutOfRange
	}
	return lat, lon, nil
}

// Raw serialises the input verbatim for the source_coordinate column (never errors in practice; an
// empty object on the impossible marshal failure).
func (in CoordinateInput) Raw() json.RawMessage {
	b, err := json.Marshal(in)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}

// ---------------------------------------------------------------- UTM / СК-42 (transverse Mercator)

// utmProj is the WGS84 UTM projection for a zone (central meridian 6*zone-183, scale 0.9996, 500km false
// easting; southern hemisphere carries the 10,000,000m false northing so northings stay positive).
func utmProj(zone int, north bool) wgs84.ProjectedReferenceSystem {
	northf := 0.0
	if !north {
		northf = 10000000
	}
	return wgs84.WGS84().TransverseMercator(float64(6*zone-183), 0, 0.9996, 500000, northf)
}

func utmToWGS84(zone int, easting, northing float64, north bool) (lat, lon float64, err error) {
	if zone < 1 || zone > 60 {
		return 0, 0, ErrCoordinateInvalid
	}
	lon, lat, _ = utmProj(zone, north).To(wgs84.LonLat())(easting, northing, 0)
	return lat, lon, nil
}

// sk42Datum is СК-42 (Krassowsky-1940 ellipsoid) with the Pulkovo-1942→WGS84 7-parameter Helmert shift
// (EPSG:1267, position-vector convention; accurate to a few metres over the former-USSR area).
func sk42Datum() wgs84.Datum {
	return wgs84.Helmert(6378245, 298.3, 23.92, -141.27, -80.9, 0, 0.35, 0.82, -0.12)
}

func sk42ToWGS84(zone int, easting, northing float64) (lat, lon float64, err error) {
	if zone < 1 || zone > 60 {
		return 0, 0, ErrCoordinateInvalid
	}
	// СК-42 writes the y-coordinate with the zone number prepended (zone*1e6 + 500000 + Δ); strip it if
	// the full value was supplied so only the in-zone easting (incl. the 500km false easting) remains.
	e := easting
	if e >= 1000000 {
		e = math.Mod(e, 1000000)
	}
	proj := sk42Datum().TransverseMercator(float64(6*zone-3), 0, 1.0, 500000, 0)
	lon, lat, _ = proj.To(wgs84.LonLat())(e, northing, 0)
	return lat, lon, nil
}

// sk42GridToWGS84 accepts a full numeric СК-42 reference written as one string: "<zone> <northing_m>
// <easting_m>" (whitespace/comma/semicolon separated). Truncated map-sheet square nomenclature (e.g.
// L-37-012) needs the sheet index to resolve and is out of scope.
func sk42GridToWGS84(grid string) (lat, lon float64, err error) {
	fields := strings.FieldsFunc(grid, func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '\t' || r == '/'
	})
	if len(fields) != 3 {
		return 0, 0, ErrCoordinateInvalid
	}
	zone, e1 := strconv.Atoi(fields[0])
	northing, e2 := strconv.ParseFloat(fields[1], 64)
	easting, e3 := strconv.ParseFloat(fields[2], 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return 0, 0, ErrCoordinateInvalid
	}
	return sk42ToWGS84(zone, easting, northing)
}

// ---------------------------------------------------------------- MGRS

const (
	mgrsBandLetters = "CDEFGHJKLMNPQRSTUVWX" // 20 latitude bands, 8° each, from 80°S (skip I, O)
	mgrsRowLetters  = "ABCDEFGHJKLMNPQRSTUV" // 100km row letters (skip I, O)
)

// mgrsColSet is the 8-letter 100km-column alphabet for the zone (one of three sets by zone%3).
func mgrsColSet(zone int) string {
	switch zone % 3 {
	case 1:
		return "ABCDEFGH"
	case 2:
		return "JKLMNPQR"
	default:
		return "STUVWXYZ"
	}
}

// DeriveMGRS returns the 1m-precision MGRS grid reference for a WGS84 coordinate, or nil for the polar
// UPS regions (lat outside [-80,84]) which use a different grid and are out of scope. A 1:1 port of the
// previous DB plpgsql location_mgrs(): zone + latitude band + 100km square + 5-digit easting/northing.
func DeriveMGRS(lat, lon float64) *string {
	if lat < -80 || lat > 84 {
		return nil
	}
	zone := min(int(math.Floor((lon+180)/6))+1, 60) // lon == 180.0 edge folds into zone 60
	north := lat >= 0
	easting, northing, _ := wgs84.LonLat().To(utmProj(zone, north))(lon, lat, 0)

	bandIdx := min(int(math.Floor((lat+80)/8)), 19) // band X spans 72..84 (12°)
	colIdx := int(math.Floor(easting/100000)) - 1
	if colIdx < 0 || colIdx > 7 {
		return nil
	}
	rowOffset := 0
	if zone%2 == 0 {
		rowOffset = 5
	}
	rowIdx := (int(math.Floor(northing/100000)) + rowOffset) % 20

	var sb strings.Builder
	sb.WriteString(strconv.Itoa(zone))
	sb.WriteByte(mgrsBandLetters[bandIdx])
	sb.WriteByte(mgrsColSet(zone)[colIdx])
	sb.WriteByte(mgrsRowLetters[rowIdx])
	sb.WriteString(pad5(int64(math.Floor(easting)) % 100000))
	sb.WriteString(pad5(int64(math.Floor(northing)) % 100000))
	s := sb.String()
	return &s
}

// parseMGRS is the inverse of DeriveMGRS: an MGRS string → WGS84 lat/lon. It reconstructs the UTM
// easting/northing (resolving the 2,000,000m row-letter ambiguity from the latitude band) then inverts
// the UTM projection.
func parseMGRS(raw string) (lat, lon float64, err error) {
	s := strings.ToUpper(strings.Join(strings.Fields(raw), ""))
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i > 2 {
		return 0, 0, ErrCoordinateInvalid
	}
	zone, e := strconv.Atoi(s[:i])
	if e != nil || zone < 1 || zone > 60 {
		return 0, 0, ErrCoordinateInvalid
	}
	rest := s[i:]
	if len(rest) < 3 {
		return 0, 0, ErrCoordinateInvalid
	}
	band, colC, rowC := rest[0], rest[1], rest[2]
	digits := rest[3:]
	if len(digits)%2 != 0 || len(digits) > 10 {
		return 0, 0, ErrCoordinateInvalid
	}
	bandIdx := strings.IndexByte(mgrsBandLetters, band)
	colIdx := strings.IndexByte(mgrsColSet(zone), colC)
	rowIdxRaw := strings.IndexByte(mgrsRowLetters, rowC)
	if bandIdx < 0 || colIdx < 0 || rowIdxRaw < 0 {
		return 0, 0, ErrCoordinateInvalid
	}
	north := band >= 'N'

	var eDig, nDig float64
	if half := len(digits) / 2; half > 0 {
		ev, e1 := strconv.Atoi(digits[:half])
		nv, e2 := strconv.Atoi(digits[half:])
		if e1 != nil || e2 != nil {
			return 0, 0, ErrCoordinateInvalid
		}
		mult := math.Pow(10, float64(5-half))
		eDig, nDig = float64(ev)*mult, float64(nv)*mult
	}
	easting := float64((colIdx+1)*100000) + eDig

	rowOffset := 0
	if zone%2 == 0 {
		rowOffset = 5
	}
	rowIdx := ((rowIdxRaw-rowOffset)%20 + 20) % 20
	base := float64(rowIdx*100000) + nDig // northing modulo the 2,000,000m row cycle

	proj := utmProj(zone, north)
	// The band's lower-latitude boundary projects to the smallest northing the band can hold; a band is
	// 8° (<2,000,000m) tall, so exactly one 2,000,000m multiple of `base` lands inside it.
	cm := float64(6*zone - 183)
	_, minN, _ := wgs84.LonLat().To(proj)(cm, float64(-80+8*bandIdx), 0)
	northing := base + math.Ceil((minN-base)/2000000)*2000000

	lon, lat, _ = proj.To(wgs84.LonLat())(easting, northing, 0)
	return lat, lon, nil
}

func pad5(v int64) string {
	if v < 0 {
		v = 0
	}
	s := strconv.FormatInt(v, 10)
	if len(s) >= 5 {
		return s
	}
	return strings.Repeat("0", 5-len(s)) + s
}
