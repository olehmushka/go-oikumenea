// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"math"
	"strings"
	"testing"
)

// MGRS forward fixtures. Kyiv is pinned to the exact string the previous DB plpgsql location_mgrs()
// produced (the one verified M19 fixture); for the others we pin the zone+band+square prefix (full
// 1m-precision correctness is covered by the round-trip test below).
func TestDeriveMGRS(t *testing.T) {
	cases := []struct {
		name       string
		lat, lon   float64
		want       string // exact when full length, else a prefix
		exactMatch bool
	}{
		{"kyiv", 50.4501, 30.5234, "36UUA2418291607", true},
		{"sydney", -33.8688, 151.2093, "56HLH", false},
		{"london", 51.5074, -0.1278, "30UXC", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DeriveMGRS(c.lat, c.lon)
			if got == nil {
				t.Fatalf("got nil MGRS")
			}
			if c.exactMatch && *got != c.want {
				t.Errorf("DeriveMGRS(%v,%v) = %q, want %q", c.lat, c.lon, *got, c.want)
			}
			if !c.exactMatch && !strings.HasPrefix(*got, c.want) {
				t.Errorf("DeriveMGRS(%v,%v) = %q, want prefix %q", c.lat, c.lon, *got, c.want)
			}
		})
	}
}

func TestDeriveMGRSPolarIsNil(t *testing.T) {
	if got := DeriveMGRS(85.0, 10.0); got != nil {
		t.Errorf("expected nil MGRS for polar latitude, got %q", *got)
	}
}

// Round-trip: parsing an MGRS string back to lat/lon should land within ~1m (the MGRS precision).
func TestMGRSRoundTrip(t *testing.T) {
	pts := [][2]float64{{50.4501, 30.5234}, {-33.8688, 151.2093}, {51.5074, -0.1278}}
	for _, p := range pts {
		mgrs := DeriveMGRS(p[0], p[1])
		if mgrs == nil {
			t.Fatalf("nil MGRS for %v", p)
		}
		in := CoordinateInput{Format: FormatMGRS, MGRS: mgrs}
		lat, lon, err := in.ToWGS84()
		if err != nil {
			t.Fatalf("parse %q: %v", *mgrs, err)
		}
		if math.Abs(lat-p[0]) > 0.0001 || math.Abs(lon-p[1]) > 0.0001 {
			t.Errorf("round-trip %q = (%v,%v), want (%v,%v)", *mgrs, lat, lon, p[0], p[1])
		}
	}
}

func TestLatLonPassthrough(t *testing.T) {
	lat0, lon0 := 50.4501, 30.5234
	in := CoordinateInput{Format: FormatLatLon, Latitude: &lat0, Longitude: &lon0}
	lat, lon, err := in.ToWGS84()
	if err != nil || lat != lat0 || lon != lon0 {
		t.Fatalf("latlon passthrough = (%v,%v,%v)", lat, lon, err)
	}
}

func TestUTMToWGS84(t *testing.T) {
	// Kyiv UTM: zone 36N, easting ~324182, northing ~5591607 (from the MGRS derivation).
	zone := 36
	e, n := 324182.0, 5591607.0
	hemi := "N"
	in := CoordinateInput{Format: FormatUTM, Zone: &zone, Easting: &e, Northing: &n, Hemisphere: &hemi}
	lat, lon, err := in.ToWGS84()
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(lat-50.4501) > 0.001 || math.Abs(lon-30.5234) > 0.001 {
		t.Errorf("UTM->WGS84 = (%v,%v), want ~(50.4501,30.5234)", lat, lon)
	}
}

func TestSK42ToWGS84(t *testing.T) {
	// Kyiv in СК-42, zone 6 (central meridian 33°E), approximate easting/northing. We only assert the
	// result is in the right neighbourhood (the Pulkovo→WGS84 Helmert is metre-accurate, not exact).
	zone := 6
	e, n := 388000.0, 5590000.0
	in := CoordinateInput{Format: FormatSK42, Zone: &zone, Easting: &e, Northing: &n}
	lat, lon, err := in.ToWGS84()
	if err != nil {
		t.Fatal(err)
	}
	if lat < 49 || lat > 52 || lon < 29 || lon > 32 {
		t.Errorf("СК-42->WGS84 = (%v,%v), want near Kyiv", lat, lon)
	}
}

func TestInvalidCoordinates(t *testing.T) {
	bad := []CoordinateInput{
		{Format: "nonsense"},
		{Format: FormatMGRS, MGRS: strptr("not-mgrs")},
		{Format: FormatLatLon}, // missing fields
		{Format: FormatUTM, Zone: intptr(36)},
	}
	for i, in := range bad {
		if _, _, err := in.ToWGS84(); err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}

func strptr(s string) *string { return &s }
func intptr(i int) *int       { return &i }
