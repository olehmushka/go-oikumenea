// Discovery orchestration (D-Religion discovery surface, M25): audited writes for the site/service-type
// catalogs, the sites (with the one-primary-per-unit invariant), per-site schedules and search-only
// aliases, plus the closure-aware PostGIS discovery search whose results are coarsened per the site's
// publish precision (D-Location dropped H3, so coarsening is app-side rounding in domain.Coarsen).
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
)

// ---- site types ----

func (s *Service) ListSiteTypes(ctx context.Context) ([]domain.SiteType, error) {
	return s.newRepo(s.querier(ctx)).ListSiteTypes(ctx)
}

func (s *Service) UpsertSiteType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.SiteType, error) {
	var out domain.SiteType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertSiteType(ctx, traditionTaxonID, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.site-type.upsert", v.ID, "", v)
	})
	return out, err
}

// ---- service types ----

func (s *Service) ListServiceTypes(ctx context.Context) ([]domain.ServiceType, error) {
	return s.newRepo(s.querier(ctx)).ListServiceTypes(ctx)
}

func (s *Service) UpsertServiceType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.ServiceType, error) {
	var out domain.ServiceType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertServiceType(ctx, traditionTaxonID, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.service-type.upsert", v.ID, "", v)
	})
	return out, err
}

// ---- sites ----

func (s *Service) ListUnitSites(ctx context.Context, unitID string) ([]domain.Site, error) {
	return s.newRepo(s.querier(ctx)).ListSitesByUnit(ctx, unitID)
}

// AddSite attaches a site to a unit. A new primary site clears any existing primary first (the
// one-primary-per-unit invariant is enforced by a partial-unique index as defence in depth).
func (s *Service) AddSite(ctx context.Context, in domain.SiteInput) (domain.Site, error) {
	if err := in.Validate(); err != nil {
		return domain.Site{}, err
	}
	var out domain.Site
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if in.IsPrimary {
			if err := repo.ClearPrimarySite(ctx, in.OrgUnitID); err != nil {
				return err
			}
		}
		site, err := repo.InsertSite(ctx, in)
		if err != nil {
			return err
		}
		out = site
		return s.record(ctx, tx, "religion.site.add", site.ID, site.OrgUnitID, site)
	})
	return out, err
}

// UpdateSite patches a site; promoting it to primary clears the unit's existing primary first.
func (s *Service) UpdateSite(ctx context.Context, id string, up domain.SiteUpdate) (domain.Site, error) {
	if up.Visibility != nil && !domain.ValidSiteVisibility(*up.Visibility) {
		return domain.Site{}, domain.ErrInvalid
	}
	if up.PublicPrecision != nil && !domain.ValidPublicPrecision(*up.PublicPrecision) {
		return domain.Site{}, domain.ErrInvalid
	}
	var out domain.Site
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		existing, err := repo.GetSite(ctx, id)
		if err != nil {
			return err
		}
		if up.IsPrimary != nil && *up.IsPrimary {
			if err := repo.ClearPrimarySite(ctx, existing.OrgUnitID); err != nil {
				return err
			}
		}
		site, err := repo.UpdateSite(ctx, id, up)
		if err != nil {
			return err
		}
		out = site
		return s.record(ctx, tx, "religion.site.update", site.ID, site.OrgUnitID, site)
	})
	return out, err
}

func (s *Service) DeleteSite(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		existing, err := repo.GetSite(ctx, id)
		if err != nil {
			return err
		}
		if err := repo.SoftDeleteSite(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "religion.site.delete", id, existing.OrgUnitID, map[string]any{"id": id, "deleted": true})
	})
}

// SiteUnitID resolves the org unit a site belongs to (used by the transport layer to PEP-gate writes
// on the owning unit over the canonical graph).
func (s *Service) SiteUnitID(ctx context.Context, siteID string) (string, error) {
	site, err := s.newRepo(s.querier(ctx)).GetSite(ctx, siteID)
	if err != nil {
		return "", err
	}
	return site.OrgUnitID, nil
}

// ---- service schedules ----

func (s *Service) ListSiteSchedules(ctx context.Context, siteID string) ([]domain.ServiceSchedule, error) {
	return s.newRepo(s.querier(ctx)).ListSchedulesBySite(ctx, siteID)
}

func (s *Service) AddSchedule(ctx context.Context, in domain.ScheduleInput) (domain.ServiceSchedule, error) {
	if err := in.Validate(); err != nil {
		return domain.ServiceSchedule{}, err
	}
	var out domain.ServiceSchedule
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		site, err := repo.GetSite(ctx, in.SiteID)
		if err != nil {
			return err
		}
		sched, err := repo.InsertSchedule(ctx, in)
		if err != nil {
			return err
		}
		out = sched
		return s.record(ctx, tx, "religion.schedule.add", sched.ID, site.OrgUnitID, sched)
	})
	return out, err
}

func (s *Service) DeleteSchedule(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		sched, err := repo.GetSchedule(ctx, id)
		if err != nil {
			return err
		}
		site, err := repo.GetSite(ctx, sched.SiteID)
		if err != nil {
			return err
		}
		if err := repo.SoftDeleteSchedule(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "religion.schedule.delete", id, site.OrgUnitID, map[string]any{"id": id, "deleted": true})
	})
}

// ScheduleUnitID resolves the org unit a schedule's site belongs to (transport PEP gate).
func (s *Service) ScheduleUnitID(ctx context.Context, scheduleID string) (string, error) {
	repo := s.newRepo(s.querier(ctx))
	sched, err := repo.GetSchedule(ctx, scheduleID)
	if err != nil {
		return "", err
	}
	site, err := repo.GetSite(ctx, sched.SiteID)
	if err != nil {
		return "", err
	}
	return site.OrgUnitID, nil
}

// ---- aliases ----

func (s *Service) ListUnitAliases(ctx context.Context, unitID string) ([]domain.Alias, error) {
	return s.newRepo(s.querier(ctx)).ListAliasesByUnit(ctx, unitID)
}

func (s *Service) AddAlias(ctx context.Context, in domain.AliasInput) (domain.Alias, error) {
	if err := in.Validate(); err != nil {
		return domain.Alias{}, err
	}
	var out domain.Alias
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		a, err := s.newRepo(tx).InsertAlias(ctx, in)
		if err != nil {
			return err
		}
		out = a
		return s.record(ctx, tx, "religion.alias.add", a.ID, a.UnitID, a)
	})
	return out, err
}

func (s *Service) DeleteAlias(ctx context.Context, unitID, aliasID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).SoftDeleteAlias(ctx, aliasID); err != nil {
			return err
		}
		return s.record(ctx, tx, "religion.alias.delete", aliasID, unitID, map[string]any{"id": aliasID, "deleted": true})
	})
}

// ---- discovery search ----

// SearchSites runs the public discovery search and coarsens each hit's coordinate per its publish
// precision (domain.Coarsen — app-side rounding; `hidden` omits the coordinate).
func (s *Service) SearchSites(ctx context.Context, q domain.DiscoveryQuery) ([]domain.DiscoverySite, error) {
	if q.Limit <= 0 {
		q.Limit = defaultPageSize
	}
	if q.Limit > maxPageSize {
		q.Limit = maxPageSize
	}
	sites, err := s.newRepo(s.querier(ctx)).SearchSites(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DiscoverySite, 0, len(sites))
	for _, site := range sites {
		hit := domain.DiscoverySite{
			ID: site.ID, OrgUnitID: site.OrgUnitID, SiteTypeID: site.SiteTypeID,
			SiteTypeCode: site.SiteTypeCode, SiteTypeName: site.SiteTypeName,
			PublicPrecision: site.PublicPrecision, IsPrimary: site.IsPrimary,
		}
		if lat, lng, ok := domain.Coarsen(site.Latitude, site.Longitude, site.PublicPrecision); ok {
			hit.Latitude, hit.Longitude = &lat, &lng
		}
		out = append(out, hit)
	}
	return out, nil
}
