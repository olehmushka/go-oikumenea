// Discovery transport (D-Religion discovery surface, M25): site/service-type catalog reads on
// religion.read and writes on religion.catalog.manage (instance); per-unit site/alias writes on
// site.manage and schedule writes on schedule.manage (both checked against the organization unit over
// the canonical graph); the public discovery search on religion.read (results coarsened per precision).
package transport

import (
	"context"

	religionapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/religion"
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

// ---- site types ----

func (s ReligionService) ListSiteTypes(ctx context.Context, token bearertoken.Token) (religionapi.SiteTypeList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.SiteTypeList{}, err
	}
	types, err := s.app.ListSiteTypes(ctx)
	if err != nil {
		return religionapi.SiteTypeList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.ID] = t.Name
	}
	names, err := s.names(ctx, entSiteType, defaults)
	if err != nil {
		return religionapi.SiteTypeList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.SiteType, 0, len(types))
	for _, t := range types {
		out = append(out, religionapi.SiteType{Id: t.ID, TraditionTaxonId: emptyToNil(t.TraditionTaxonID), Code: t.Code, Name: names[t.ID], Status: t.Status, SortOrder: t.SortOrder})
	}
	return religionapi.SiteTypeList{SiteTypes: out}, nil
}

func (s ReligionService) UpsertSiteType(ctx context.Context, token bearertoken.Token, req religionapi.UpsertSiteTypeRequest) (religionapi.SiteType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.SiteType{}, err
	}
	t, err := s.app.UpsertSiteType(ctx, req.TraditionTaxonId, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return religionapi.SiteType{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entSiteType, t.ID, t.Name)
	if err != nil {
		return religionapi.SiteType{}, s.mapError(ctx, err)
	}
	return religionapi.SiteType{Id: t.ID, TraditionTaxonId: emptyToNil(t.TraditionTaxonID), Code: t.Code, Name: name, Status: t.Status, SortOrder: t.SortOrder}, nil
}

// ---- service types ----

func (s ReligionService) ListServiceTypes(ctx context.Context, token bearertoken.Token) (religionapi.ServiceTypeList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.ServiceTypeList{}, err
	}
	types, err := s.app.ListServiceTypes(ctx)
	if err != nil {
		return religionapi.ServiceTypeList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.ID] = t.Name
	}
	names, err := s.names(ctx, entServiceType, defaults)
	if err != nil {
		return religionapi.ServiceTypeList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.ServiceType, 0, len(types))
	for _, t := range types {
		out = append(out, religionapi.ServiceType{Id: t.ID, TraditionTaxonId: emptyToNil(t.TraditionTaxonID), Code: t.Code, Name: names[t.ID], Status: t.Status, SortOrder: t.SortOrder})
	}
	return religionapi.ServiceTypeList{ServiceTypes: out}, nil
}

func (s ReligionService) UpsertServiceType(ctx context.Context, token bearertoken.Token, req religionapi.UpsertServiceTypeRequest) (religionapi.ServiceType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.ServiceType{}, err
	}
	t, err := s.app.UpsertServiceType(ctx, req.TraditionTaxonId, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return religionapi.ServiceType{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entServiceType, t.ID, t.Name)
	if err != nil {
		return religionapi.ServiceType{}, s.mapError(ctx, err)
	}
	return religionapi.ServiceType{Id: t.ID, TraditionTaxonId: emptyToNil(t.TraditionTaxonID), Code: t.Code, Name: name, Status: t.Status, SortOrder: t.SortOrder}, nil
}

// ---- sites ----

func (s ReligionService) ListUnitSites(ctx context.Context, token bearertoken.Token, unitID string) (religionapi.SiteList, error) {
	if err := s.pep.Require(ctx, token, readPerm, unitID); err != nil {
		return religionapi.SiteList{}, err
	}
	sites, err := s.app.ListUnitSites(ctx, unitID)
	if err != nil {
		return religionapi.SiteList{}, s.mapError(ctx, err)
	}
	return s.siteList(ctx, sites)
}

func (s ReligionService) CreateSite(ctx context.Context, token bearertoken.Token, unitID string, req religionapi.CreateSiteRequest) (religionapi.Site, error) {
	if err := s.pep.Require(ctx, token, sitePerm, unitID); err != nil {
		return religionapi.Site{}, err
	}
	site, err := s.app.AddSite(ctx, domain.SiteInput{
		OrgUnitID: unitID, LocationID: req.LocationId, SiteTypeID: req.SiteTypeId,
		Visibility: strOr(req.Visibility), PublicPrecision: strOr(req.PublicPrecision), IsPrimary: boolOr(req.IsPrimary),
	})
	if err != nil {
		return religionapi.Site{}, s.mapError(ctx, err)
	}
	return s.siteAPI(ctx, site)
}

func (s ReligionService) UpdateSite(ctx context.Context, token bearertoken.Token, siteID string, req religionapi.UpdateSiteRequest) (religionapi.Site, error) {
	unitID, err := s.app.SiteUnitID(ctx, siteID)
	if err != nil {
		return religionapi.Site{}, s.mapError(ctx, err)
	}
	if err := s.pep.Require(ctx, token, sitePerm, unitID); err != nil {
		return religionapi.Site{}, err
	}
	site, err := s.app.UpdateSite(ctx, siteID, domain.SiteUpdate{
		SiteTypeID: req.SiteTypeId, Visibility: req.Visibility, PublicPrecision: req.PublicPrecision, IsPrimary: req.IsPrimary,
	})
	if err != nil {
		return religionapi.Site{}, s.mapError(ctx, err)
	}
	return s.siteAPI(ctx, site)
}

func (s ReligionService) DeleteSite(ctx context.Context, token bearertoken.Token, siteID string) error {
	unitID, err := s.app.SiteUnitID(ctx, siteID)
	if err != nil {
		return s.mapError(ctx, err)
	}
	if err := s.pep.Require(ctx, token, sitePerm, unitID); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteSite(ctx, siteID))
}

// ---- service schedules ----

func (s ReligionService) ListSiteSchedules(ctx context.Context, token bearertoken.Token, siteID string) (religionapi.ServiceScheduleList, error) {
	unitID, err := s.app.SiteUnitID(ctx, siteID)
	if err != nil {
		return religionapi.ServiceScheduleList{}, s.mapError(ctx, err)
	}
	if err := s.pep.Require(ctx, token, readPerm, unitID); err != nil {
		return religionapi.ServiceScheduleList{}, err
	}
	rows, err := s.app.ListSiteSchedules(ctx, siteID)
	if err != nil {
		return religionapi.ServiceScheduleList{}, s.mapError(ctx, err)
	}
	return s.scheduleList(ctx, rows)
}

func (s ReligionService) CreateSchedule(ctx context.Context, token bearertoken.Token, siteID string, req religionapi.CreateScheduleRequest) (religionapi.ServiceSchedule, error) {
	unitID, err := s.app.SiteUnitID(ctx, siteID)
	if err != nil {
		return religionapi.ServiceSchedule{}, s.mapError(ctx, err)
	}
	if err := s.pep.Require(ctx, token, schedulePerm, unitID); err != nil {
		return religionapi.ServiceSchedule{}, err
	}
	sched, err := s.app.AddSchedule(ctx, domain.ScheduleInput{
		SiteID: siteID, ServiceTypeID: req.ServiceTypeId, DayOfWeek: req.DayOfWeek, RRule: strOr(req.Rrule),
		StartTime: strOr(req.StartTime), EndTime: strOr(req.EndTime), Timezone: req.Timezone, Language: strOr(req.Language),
		Mode: strOr(req.Mode), MeetingURL: strOr(req.MeetingUrl), Description: strOr(req.Description),
	})
	if err != nil {
		return religionapi.ServiceSchedule{}, s.mapError(ctx, err)
	}
	return s.scheduleAPI(ctx, sched)
}

func (s ReligionService) DeleteSchedule(ctx context.Context, token bearertoken.Token, scheduleID string) error {
	unitID, err := s.app.ScheduleUnitID(ctx, scheduleID)
	if err != nil {
		return s.mapError(ctx, err)
	}
	if err := s.pep.Require(ctx, token, schedulePerm, unitID); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteSchedule(ctx, scheduleID))
}

// ---- aliases ----

func (s ReligionService) ListUnitAliases(ctx context.Context, token bearertoken.Token, unitID string) (religionapi.AliasList, error) {
	if err := s.pep.Require(ctx, token, readPerm, unitID); err != nil {
		return religionapi.AliasList{}, err
	}
	rows, err := s.app.ListUnitAliases(ctx, unitID)
	if err != nil {
		return religionapi.AliasList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.Alias, 0, len(rows))
	for _, a := range rows {
		out = append(out, aliasAPI(a))
	}
	return religionapi.AliasList{Aliases: out}, nil
}

func (s ReligionService) CreateAlias(ctx context.Context, token bearertoken.Token, unitID string, req religionapi.CreateAliasRequest) (religionapi.Alias, error) {
	if err := s.pep.Require(ctx, token, sitePerm, unitID); err != nil {
		return religionapi.Alias{}, err
	}
	a, err := s.app.AddAlias(ctx, domain.AliasInput{UnitID: unitID, AliasText: req.AliasText, AliasType: req.AliasType, Locale: strOr(req.Locale)})
	if err != nil {
		return religionapi.Alias{}, s.mapError(ctx, err)
	}
	return aliasAPI(a), nil
}

func (s ReligionService) DeleteAlias(ctx context.Context, token bearertoken.Token, unitID, aliasID string) error {
	if err := s.pep.Require(ctx, token, sitePerm, unitID); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteAlias(ctx, unitID, aliasID))
}

// ---- discovery search ----

func (s ReligionService) SearchSites(ctx context.Context, token bearertoken.Token, lat, lng, radiusM, minLat, minLng, maxLat, maxLng *float64, religion, language *string, dayOfWeek *int, onlineOnly *bool, query *string, pageSize *int) (religionapi.DiscoverySitePage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.DiscoverySitePage{}, err
	}
	q := domain.DiscoveryQuery{
		Lat: lat, Lng: lng, RadiusM: radiusM, MinLat: minLat, MinLng: minLng, MaxLat: maxLat, MaxLng: maxLng,
		Religion: strOr(religion), Language: strOr(language), DayOfWeek: dayOfWeek, OnlineOnly: boolOr(onlineOnly),
		Query: strOr(query), Limit: pageSizeOr(pageSize),
	}
	hits, err := s.app.SearchSites(ctx, q)
	if err != nil {
		return religionapi.DiscoverySitePage{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(hits))
	for _, h := range hits {
		defaults[h.SiteTypeID] = h.SiteTypeName
	}
	names, err := s.names(ctx, entSiteType, defaults)
	if err != nil {
		return religionapi.DiscoverySitePage{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.DiscoverySite, 0, len(hits))
	for _, h := range hits {
		out = append(out, religionapi.DiscoverySite{
			Id: h.ID, OrgUnitId: h.OrgUnitID, SiteTypeId: h.SiteTypeID, SiteTypeCode: h.SiteTypeCode,
			SiteTypeName: names[h.SiteTypeID], PublicPrecision: h.PublicPrecision, IsPrimary: h.IsPrimary,
			Latitude: h.Latitude, Longitude: h.Longitude,
		})
	}
	return religionapi.DiscoverySitePage{Sites: out}, nil
}

// ---- mappers ----

func (s ReligionService) siteList(ctx context.Context, sites []domain.Site) (religionapi.SiteList, error) {
	defaults := make(map[string]string, len(sites))
	for _, st := range sites {
		defaults[st.SiteTypeID] = st.SiteTypeName
	}
	names, err := s.names(ctx, entSiteType, defaults)
	if err != nil {
		return religionapi.SiteList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.Site, 0, len(sites))
	for _, st := range sites {
		out = append(out, siteToAPI(st, names[st.SiteTypeID]))
	}
	return religionapi.SiteList{Sites: out}, nil
}

func (s ReligionService) siteAPI(ctx context.Context, site domain.Site) (religionapi.Site, error) {
	name, err := s.nameMap(ctx, entSiteType, site.SiteTypeID, site.SiteTypeName)
	if err != nil {
		return religionapi.Site{}, s.mapError(ctx, err)
	}
	return siteToAPI(site, name), nil
}

func siteToAPI(site domain.Site, typeName map[string]string) religionapi.Site {
	return religionapi.Site{
		Id: site.ID, OrgUnitId: site.OrgUnitID, LocationId: site.LocationID,
		SiteTypeId: site.SiteTypeID, SiteTypeCode: site.SiteTypeCode, SiteTypeName: typeName,
		Visibility: site.Visibility, PublicPrecision: site.PublicPrecision, IsPrimary: site.IsPrimary,
		Latitude: site.Latitude, Longitude: site.Longitude,
		CreatedAt: datetime.DateTime(site.CreatedAt), UpdatedAt: datetime.DateTime(site.UpdatedAt),
	}
}

func (s ReligionService) scheduleList(ctx context.Context, rows []domain.ServiceSchedule) (religionapi.ServiceScheduleList, error) {
	defaults := make(map[string]string, len(rows))
	for _, sc := range rows {
		defaults[sc.ServiceTypeID] = sc.ServiceTypeName
	}
	names, err := s.names(ctx, entServiceType, defaults)
	if err != nil {
		return religionapi.ServiceScheduleList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.ServiceSchedule, 0, len(rows))
	for _, sc := range rows {
		out = append(out, scheduleToAPI(sc, names[sc.ServiceTypeID]))
	}
	return religionapi.ServiceScheduleList{Schedules: out}, nil
}

func (s ReligionService) scheduleAPI(ctx context.Context, sc domain.ServiceSchedule) (religionapi.ServiceSchedule, error) {
	name, err := s.nameMap(ctx, entServiceType, sc.ServiceTypeID, sc.ServiceTypeName)
	if err != nil {
		return religionapi.ServiceSchedule{}, s.mapError(ctx, err)
	}
	return scheduleToAPI(sc, name), nil
}

func scheduleToAPI(sc domain.ServiceSchedule, typeName map[string]string) religionapi.ServiceSchedule {
	return religionapi.ServiceSchedule{
		Id: sc.ID, SiteId: sc.SiteID, ServiceTypeId: sc.ServiceTypeID, ServiceTypeCode: sc.ServiceTypeCode, ServiceTypeName: typeName,
		DayOfWeek: sc.DayOfWeek, Rrule: emptyToNil(sc.RRule), StartTime: emptyToNil(sc.StartTime), EndTime: emptyToNil(sc.EndTime),
		Timezone: sc.Timezone, Language: emptyToNil(sc.Language), Mode: sc.Mode, MeetingUrl: emptyToNil(sc.MeetingURL),
		Description: emptyToNil(sc.Description), CreatedAt: datetime.DateTime(sc.CreatedAt), UpdatedAt: datetime.DateTime(sc.UpdatedAt),
	}
}

func aliasAPI(a domain.Alias) religionapi.Alias {
	return religionapi.Alias{
		Id: a.ID, UnitId: a.UnitID, AliasText: a.AliasText, AliasType: a.AliasType, Locale: emptyToNil(a.Locale),
		CreatedAt: datetime.DateTime(a.CreatedAt), UpdatedAt: datetime.DateTime(a.UpdatedAt),
	}
}
