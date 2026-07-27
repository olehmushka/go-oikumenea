// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Discovery domain (D-Religion discovery surface, M25): the per-tradition site/service-type catalogs,
// the reified site Link (worship-community Unit ↔ a shared Location), per-site service schedules, and
// search-only aliases — the substrate that makes religious organizations findable (where/when/under
// what names) with privacy-preserving spatial search.
//
// The publish-precision projection is APP-SIDE here (Coarsen, below): D-Location dropped H3, so a public
// coordinate is coarsened by rounding rather than projected to an H3 cell. The full coordinate stays in
// the shared location_locations row; coarsening is a read-time projection on the site link.
package domain

import (
	"errors"
	"math"
	"strings"
	"time"
)

// SiteType is a per-tradition place/site type (church/cathedral/mosque/synagogue/temple/…).
type SiteType struct {
	ID               string
	TraditionTaxonID string // "" = generic
	Code             string
	Name             string
	Status           string
	SortOrder        *int
}

// ServiceType is a per-tradition service/observance type (main/youth/prayer/jumua/shabbat/…).
type ServiceType struct {
	ID               string
	TraditionTaxonID string // "" = generic
	Code             string
	Name             string
	Status           string
	SortOrder        *int
}

// Site is a worship-community unit's place: the reified link__site_of (Unit ↔ Location). Latitude /
// Longitude carry the EXACT shared-location coordinate (returned to authorized owners); the discovery
// search coarsens them per PublicPrecision before returning.
type Site struct {
	ID              string
	OrgUnitID       string
	LocationID      string
	SiteTypeID      string
	SiteTypeCode    string
	SiteTypeName    string // default-locale; translated via the i18n store
	Visibility      string // public | unlisted | private
	PublicPrecision string // exact | street | neighborhood | city | hidden
	IsPrimary       bool
	Latitude        float64
	Longitude       float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SiteInput is the create payload for a site.
type SiteInput struct {
	OrgUnitID       string
	LocationID      string
	SiteTypeID      string
	Visibility      string
	PublicPrecision string
	IsPrimary       bool
}

// Validate checks a site create input.
func (in SiteInput) Validate() error {
	if strings.TrimSpace(in.OrgUnitID) == "" || strings.TrimSpace(in.LocationID) == "" || strings.TrimSpace(in.SiteTypeID) == "" {
		return ErrInvalid
	}
	if in.Visibility != "" && !ValidSiteVisibility(in.Visibility) {
		return ErrInvalid
	}
	if in.PublicPrecision != "" && !ValidPublicPrecision(in.PublicPrecision) {
		return ErrInvalid
	}
	return nil
}

// SiteUpdate patches a site (type / visibility / precision / primary flag).
type SiteUpdate struct {
	SiteTypeID      *string
	Visibility      *string
	PublicPrecision *string
	IsPrimary       *bool
}

// ServiceSchedule is a per-site recurring service time (weekly day or an RRULE subset).
type ServiceSchedule struct {
	ID              string
	SiteID          string
	ServiceTypeID   string
	ServiceTypeCode string
	ServiceTypeName string // default-locale; translated via the i18n store
	DayOfWeek       *int   // 0=Sunday … 6=Saturday; nil when rrule-driven
	RRule           string
	StartTime       string // "HH:MM" or ""
	EndTime         string // "HH:MM" or ""
	Timezone        string // IANA zone
	Language        string // ISO 639-3
	Mode            string // in_person | online | hybrid
	MeetingURL      string
	Description     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ScheduleInput is the create payload for a service schedule.
type ScheduleInput struct {
	SiteID        string
	ServiceTypeID string
	DayOfWeek     *int
	RRule         string
	StartTime     string
	EndTime       string
	Timezone      string
	Language      string
	Mode          string
	MeetingURL    string
	Description   string
}

// Validate checks a schedule create input: a recurrence (day or rrule), a timezone, a valid mode, and a
// meeting URL when the mode is online/hybrid.
func (in ScheduleInput) Validate() error {
	if strings.TrimSpace(in.SiteID) == "" || strings.TrimSpace(in.ServiceTypeID) == "" {
		return ErrInvalid
	}
	if in.DayOfWeek == nil && strings.TrimSpace(in.RRule) == "" {
		return ErrInvalid
	}
	if in.DayOfWeek != nil && (*in.DayOfWeek < 0 || *in.DayOfWeek > 6) {
		return ErrInvalid
	}
	if strings.TrimSpace(in.Timezone) == "" {
		return ErrInvalid
	}
	mode := in.Mode
	if mode == "" {
		mode = "in_person"
	}
	if !ValidServiceMode(mode) {
		return ErrInvalid
	}
	if (mode == "online" || mode == "hybrid") && strings.TrimSpace(in.MeetingURL) == "" {
		return ErrInvalid
	}
	return nil
}

// Alias is a search-only alternative name for an org unit (never displayed).
type Alias struct {
	ID        string
	UnitID    string
	AliasText string
	AliasType string // nickname | abbreviation | historical | misspelling | transliteration
	Locale    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AliasInput is the create payload for an alias.
type AliasInput struct {
	UnitID    string
	AliasText string
	AliasType string
	Locale    string
}

// Validate checks an alias create input.
func (in AliasInput) Validate() error {
	if strings.TrimSpace(in.UnitID) == "" || strings.TrimSpace(in.AliasText) == "" || !ValidAliasType(in.AliasType) {
		return ErrInvalid
	}
	return nil
}

// DiscoveryQuery is the parameter set for the public discovery search.
type DiscoveryQuery struct {
	// Spatial window: a radius (Lat/Lng/RadiusM) OR a bounding box (Min/Max Lat/Lng). Both empty = no
	// spatial filter.
	Lat, Lng, RadiusM *float64
	MinLat, MinLng    *float64
	MaxLat, MaxLng    *float64
	Religion          string // restrict to org units classified under this taxon (closure)
	Language          string // a service in this language (ISO 639-3)
	DayOfWeek         *int   // a service on this weekday
	OnlineOnly        bool   // a service offered online or hybrid
	Query             string // fuzzy match on unit code/name or an alias
	Limit             int
}

// DiscoverySite is a public discovery hit: the site with its coordinate coarsened per PublicPrecision
// (Latitude/Longitude are nil when the precision is `hidden`).
type DiscoverySite struct {
	ID              string
	OrgUnitID       string
	SiteTypeID      string
	SiteTypeCode    string
	SiteTypeName    string
	PublicPrecision string
	IsPrimary       bool
	Latitude        *float64
	Longitude       *float64
}

// precisionDecimals maps a publish precision to the number of decimal places a coordinate is rounded to.
// (exact has no rounding; hidden omits the coordinate entirely.) ~11 m / ~110 m / ~1.1 km at the
// equator for street / neighborhood / city.
var precisionDecimals = map[string]int{"street": 4, "neighborhood": 3, "city": 2}

// Coarsen projects an exact (lat,lng) to the publish precision (D-Location dropped H3, so this is an
// app-side rounding): `exact` returns the full coordinate, `street`/`neighborhood`/`city` round to
// decreasing decimal places, and `hidden` returns ok=false (the coordinate must be omitted).
func Coarsen(lat, lng float64, precision string) (rlat, rlng float64, ok bool) {
	switch precision {
	case "", "exact":
		return lat, lng, true
	case "hidden":
		return 0, 0, false
	default:
		d, known := precisionDecimals[precision]
		if !known {
			return lat, lng, true
		}
		f := math.Pow(10, float64(d))
		return math.Round(lat*f) / f, math.Round(lng*f) / f, true
	}
}

var (
	siteVisibilities = map[string]struct{}{"public": {}, "unlisted": {}, "private": {}}
	publicPrecisions = map[string]struct{}{"exact": {}, "street": {}, "neighborhood": {}, "city": {}, "hidden": {}}
	serviceModes     = map[string]struct{}{"in_person": {}, "online": {}, "hybrid": {}}
	aliasTypes       = map[string]struct{}{"nickname": {}, "abbreviation": {}, "historical": {}, "misspelling": {}, "transliteration": {}}
)

// ValidSiteVisibility reports whether v is a known site visibility.
func ValidSiteVisibility(v string) bool { _, ok := siteVisibilities[v]; return ok }

// ValidPublicPrecision reports whether p is a known publish precision.
func ValidPublicPrecision(p string) bool { _, ok := publicPrecisions[p]; return ok }

// ValidServiceMode reports whether m is a known service mode.
func ValidServiceMode(m string) bool { _, ok := serviceModes[m]; return ok }

// ValidAliasType reports whether t is a known alias type.
func ValidAliasType(t string) bool { _, ok := aliasTypes[t]; return ok }

var (
	ErrSiteTypeNotFound    = errors.New("religion: site type not found")
	ErrServiceTypeNotFound = errors.New("religion: service type not found")
	ErrSiteNotFound        = errors.New("religion: site not found")
	ErrScheduleNotFound    = errors.New("religion: service schedule not found")
	ErrAliasNotFound       = errors.New("religion: alias not found")
)
