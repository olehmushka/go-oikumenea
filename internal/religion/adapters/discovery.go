// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Discovery persistence (D-Religion discovery surface, M25): the site/service-type catalogs, the reified
// site Link (joined to the shared location_locations for coordinates), per-site service schedules, the
// search-only aliases, and the closure-aware PostGIS discovery search. Raw pgx, same style as the rest
// of the module. The publish-precision projection is applied in the application layer, not here — this
// returns the EXACT coordinate; the search application coarsens it.
package adapters

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/religion/domain"
)

// ---- site types ----

func (r *Repository) ListSiteTypes(ctx context.Context) ([]domain.SiteType, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, tradition_taxon_id, code, name, status, sort_order
		FROM oikumenea.religion_site_types WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SiteType
	for rows.Next() {
		var s domain.SiteType
		var tradition pgtype.Text
		var so pgtype.Int4
		if err := rows.Scan(&s.ID, &tradition, &s.Code, &s.Name, &s.Status, &so); err != nil {
			return nil, err
		}
		s.TraditionTaxonID, s.SortOrder = textVal(tradition), intPtr(so)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertSiteType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.SiteType, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_site_types (tradition_taxon_id, code, name, sort_order)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (tradition_taxon_id, code) WHERE deleted_at IS NULL
		DO UPDATE SET name=EXCLUDED.name, sort_order=EXCLUDED.sort_order
		RETURNING id, tradition_taxon_id, code, name, status, sort_order`, traditionTaxonID, code, name, sortOrder)
	var s domain.SiteType
	var tradition pgtype.Text
	var so pgtype.Int4
	if err := row.Scan(&s.ID, &tradition, &s.Code, &s.Name, &s.Status, &so); err != nil {
		return domain.SiteType{}, mapPGError(err)
	}
	s.TraditionTaxonID, s.SortOrder = textVal(tradition), intPtr(so)
	return s, nil
}

// ---- service types ----

func (r *Repository) ListServiceTypes(ctx context.Context) ([]domain.ServiceType, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, tradition_taxon_id, code, name, status, sort_order
		FROM oikumenea.religion_service_types WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ServiceType
	for rows.Next() {
		var s domain.ServiceType
		var tradition pgtype.Text
		var so pgtype.Int4
		if err := rows.Scan(&s.ID, &tradition, &s.Code, &s.Name, &s.Status, &so); err != nil {
			return nil, err
		}
		s.TraditionTaxonID, s.SortOrder = textVal(tradition), intPtr(so)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertServiceType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.ServiceType, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_service_types (tradition_taxon_id, code, name, sort_order)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (tradition_taxon_id, code) WHERE deleted_at IS NULL
		DO UPDATE SET name=EXCLUDED.name, sort_order=EXCLUDED.sort_order
		RETURNING id, tradition_taxon_id, code, name, status, sort_order`, traditionTaxonID, code, name, sortOrder)
	var s domain.ServiceType
	var tradition pgtype.Text
	var so pgtype.Int4
	if err := row.Scan(&s.ID, &tradition, &s.Code, &s.Name, &s.Status, &so); err != nil {
		return domain.ServiceType{}, mapPGError(err)
	}
	s.TraditionTaxonID, s.SortOrder = textVal(tradition), intPtr(so)
	return s, nil
}

// ---- sites ----

const siteCols = `s.id, s.org_unit_id, s.location_id, s.site_type_id, st.code, st.name,
	s.visibility, s.public_precision, s.is_primary,
	ST_Y(l.geom::geometry)::double precision, ST_X(l.geom::geometry)::double precision,
	s.created_at, s.updated_at`

const siteFrom = `FROM oikumenea.religion_sites s
	JOIN oikumenea.religion_site_types st ON st.id = s.site_type_id
	JOIN oikumenea.location_locations l ON l.id = s.location_id`

func scanSite(row pgx.Row) (domain.Site, error) {
	var s domain.Site
	if err := row.Scan(&s.ID, &s.OrgUnitID, &s.LocationID, &s.SiteTypeID, &s.SiteTypeCode, &s.SiteTypeName,
		&s.Visibility, &s.PublicPrecision, &s.IsPrimary, &s.Latitude, &s.Longitude,
		&s.CreatedAt, &s.UpdatedAt); err != nil {
		return domain.Site{}, mapPGError(err)
	}
	return s, nil
}

func (r *Repository) ListSitesByUnit(ctx context.Context, unitID string) ([]domain.Site, error) {
	rows, err := r.c.Query(ctx, `SELECT `+siteCols+` `+siteFrom+`
		WHERE s.org_unit_id = $1 AND s.deleted_at IS NULL
		ORDER BY s.is_primary DESC, s.id`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Site
	for rows.Next() {
		s, err := scanSite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) GetSite(ctx context.Context, id string) (domain.Site, error) {
	row := r.c.QueryRow(ctx, `SELECT `+siteCols+` `+siteFrom+` WHERE s.id = $1 AND s.deleted_at IS NULL`, id)
	s, err := scanSite(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Site{}, domain.ErrSiteNotFound
	}
	return s, err
}

// ClearPrimarySite unsets the active primary flag for a unit (so a new primary can be set atomically).
func (r *Repository) ClearPrimarySite(ctx context.Context, unitID string) error {
	_, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_sites SET is_primary = false
		WHERE org_unit_id = $1 AND is_primary AND deleted_at IS NULL`, unitID)
	return mapPGError(err)
}

func (r *Repository) InsertSite(ctx context.Context, in domain.SiteInput) (domain.Site, error) {
	visibility := in.Visibility
	if visibility == "" {
		visibility = "public"
	}
	precision := in.PublicPrecision
	if precision == "" {
		precision = "exact"
	}
	var id string
	err := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_sites (org_unit_id, location_id, site_type_id, visibility, public_precision, is_primary)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		in.OrgUnitID, in.LocationID, in.SiteTypeID, visibility, precision, in.IsPrimary).Scan(&id)
	if err != nil {
		return domain.Site{}, mapPGError(err)
	}
	return r.GetSite(ctx, id)
}

func (r *Repository) UpdateSite(ctx context.Context, id string, up domain.SiteUpdate) (domain.Site, error) {
	tag, err := r.c.Exec(ctx, `
		UPDATE oikumenea.religion_sites SET
			site_type_id = COALESCE($2, site_type_id),
			visibility = COALESCE($3, visibility),
			public_precision = COALESCE($4, public_precision),
			is_primary = COALESCE($5, is_primary)
		WHERE id = $1 AND deleted_at IS NULL`,
		id, up.SiteTypeID, up.Visibility, up.PublicPrecision, up.IsPrimary)
	if err != nil {
		return domain.Site{}, mapPGError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Site{}, domain.ErrSiteNotFound
	}
	return r.GetSite(ctx, id)
}

func (r *Repository) SoftDeleteSite(ctx context.Context, id string) error {
	ct, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_sites SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrSiteNotFound
	}
	return nil
}

// ---- service schedules ----

const scheduleCols = `sc.id, sc.site_id, sc.service_type_id, vt.code, vt.name,
	sc.day_of_week, COALESCE(sc.rrule,''), COALESCE(to_char(sc.start_time,'HH24:MI'),''),
	COALESCE(to_char(sc.end_time,'HH24:MI'),''), sc.timezone, COALESCE(sc.language,''),
	sc.mode, COALESCE(sc.meeting_url,''), COALESCE(sc.description,''), sc.created_at, sc.updated_at`

const scheduleFrom = `FROM oikumenea.religion_service_schedules sc
	JOIN oikumenea.religion_service_types vt ON vt.id = sc.service_type_id`

func scanSchedule(row pgx.Row) (domain.ServiceSchedule, error) {
	var s domain.ServiceSchedule
	var dow pgtype.Int2
	if err := row.Scan(&s.ID, &s.SiteID, &s.ServiceTypeID, &s.ServiceTypeCode, &s.ServiceTypeName,
		&dow, &s.RRule, &s.StartTime, &s.EndTime, &s.Timezone, &s.Language,
		&s.Mode, &s.MeetingURL, &s.Description, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return domain.ServiceSchedule{}, mapPGError(err)
	}
	if dow.Valid {
		v := int(dow.Int16)
		s.DayOfWeek = &v
	}
	return s, nil
}

func (r *Repository) ListSchedulesBySite(ctx context.Context, siteID string) ([]domain.ServiceSchedule, error) {
	rows, err := r.c.Query(ctx, `SELECT `+scheduleCols+` `+scheduleFrom+`
		WHERE sc.site_id = $1 AND sc.deleted_at IS NULL
		ORDER BY sc.day_of_week NULLS LAST, sc.start_time NULLS LAST, sc.id`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ServiceSchedule
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) GetSchedule(ctx context.Context, id string) (domain.ServiceSchedule, error) {
	row := r.c.QueryRow(ctx, `SELECT `+scheduleCols+` `+scheduleFrom+` WHERE sc.id = $1 AND sc.deleted_at IS NULL`, id)
	s, err := scanSchedule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ServiceSchedule{}, domain.ErrScheduleNotFound
	}
	return s, err
}

func (r *Repository) InsertSchedule(ctx context.Context, in domain.ScheduleInput) (domain.ServiceSchedule, error) {
	mode := in.Mode
	if mode == "" {
		mode = "in_person"
	}
	var id string
	err := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_service_schedules
			(site_id, service_type_id, day_of_week, rrule, start_time, end_time, timezone, language, mode, meeting_url, description)
		VALUES ($1,$2,$3,$4,$5::time,$6::time,$7,$8,$9,$10,$11) RETURNING id`,
		in.SiteID, in.ServiceTypeID, in.DayOfWeek, nilIfEmpty(in.RRule),
		nilIfEmpty(in.StartTime), nilIfEmpty(in.EndTime), in.Timezone, nilIfEmpty(in.Language),
		mode, nilIfEmpty(in.MeetingURL), nilIfEmpty(in.Description)).Scan(&id)
	if err != nil {
		return domain.ServiceSchedule{}, mapPGError(err)
	}
	return r.GetSchedule(ctx, id)
}

func (r *Repository) SoftDeleteSchedule(ctx context.Context, id string) error {
	ct, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_service_schedules SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrScheduleNotFound
	}
	return nil
}

// ---- aliases ----

func (r *Repository) ListAliasesByUnit(ctx context.Context, unitID string) ([]domain.Alias, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, unit_id, alias_text, alias_type, COALESCE(locale,''), created_at, updated_at
		FROM oikumenea.religion_aliases WHERE unit_id = $1 AND deleted_at IS NULL
		ORDER BY alias_text, id`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Alias
	for rows.Next() {
		var a domain.Alias
		if err := rows.Scan(&a.ID, &a.UnitID, &a.AliasText, &a.AliasType, &a.Locale, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) InsertAlias(ctx context.Context, in domain.AliasInput) (domain.Alias, error) {
	var a domain.Alias
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_aliases (unit_id, alias_text, alias_type, locale)
		VALUES ($1,$2,$3,$4)
		RETURNING id, unit_id, alias_text, alias_type, COALESCE(locale,''), created_at, updated_at`,
		in.UnitID, in.AliasText, in.AliasType, nilIfEmpty(in.Locale))
	if err := row.Scan(&a.ID, &a.UnitID, &a.AliasText, &a.AliasType, &a.Locale, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return domain.Alias{}, mapPGError(err)
	}
	return a, nil
}

func (r *Repository) SoftDeleteAlias(ctx context.Context, id string) error {
	ct, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_aliases SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrAliasNotFound
	}
	return nil
}

// ---- discovery search ----

// SearchSites runs the closure-aware PostGIS discovery search over PUBLIC sites: a spatial window
// (radius via ST_DWithin or a bbox via ST_Intersects), an optional religion-taxon filter (org units
// classified under the taxon via the taxonomy closure), service language/day/online filters (a single
// matching schedule), and a fuzzy match on the unit code/name or an alias. Returns the EXACT coordinate
// (the application coarsens per public_precision).
func (r *Repository) SearchSites(ctx context.Context, q domain.DiscoveryQuery) ([]domain.Site, error) {
	conds := []string{"s.deleted_at IS NULL", "s.visibility = 'public'"}
	args := []any{}
	add := func(a any) string { args = append(args, a); return "$" + strconv.Itoa(len(args)) }

	orderBy := "s.id"
	if q.Lat != nil && q.Lng != nil && q.RadiusM != nil {
		pt := "ST_SetSRID(ST_MakePoint(" + add(*q.Lng) + "::double precision," + add(*q.Lat) + "::double precision),4326)::geography"
		conds = append(conds, "ST_DWithin(l.geom, "+pt+", "+add(*q.RadiusM)+"::double precision)")
		orderBy = "l.geom <-> " + pt + ", s.id"
	} else if q.MinLat != nil && q.MinLng != nil && q.MaxLat != nil && q.MaxLng != nil {
		env := "ST_MakeEnvelope(" + add(*q.MinLng) + "::double precision," + add(*q.MinLat) + "::double precision," +
			add(*q.MaxLng) + "::double precision," + add(*q.MaxLat) + "::double precision,4326)::geography"
		conds = append(conds, "ST_Intersects(l.geom, "+env+")")
	}

	if q.Religion != "" {
		conds = append(conds, `s.org_unit_id IN (
			SELECT oc.unit_id FROM oikumenea.religion_org_classifications oc
			JOIN oikumenea.religion_taxa_closure tc ON tc.descendant_id = oc.taxon_id
			WHERE tc.ancestor_id = `+add(q.Religion)+` AND oc.deleted_at IS NULL)`)
	}

	// service filters collapse into one EXISTS so they hit the SAME schedule row.
	schedConds := []string{"sc.site_id = s.id", "sc.deleted_at IS NULL"}
	if q.Language != "" {
		schedConds = append(schedConds, "sc.language = "+add(q.Language))
	}
	if q.DayOfWeek != nil {
		schedConds = append(schedConds, "sc.day_of_week = "+add(*q.DayOfWeek))
	}
	if q.OnlineOnly {
		schedConds = append(schedConds, "sc.mode IN ('online','hybrid')")
	}
	if len(schedConds) > 2 {
		conds = append(conds, "EXISTS (SELECT 1 FROM oikumenea.religion_service_schedules sc WHERE "+strings.Join(schedConds, " AND ")+")")
	}

	if q.Query != "" {
		like := add("%" + strings.ToLower(q.Query) + "%")
		conds = append(conds, `(EXISTS (SELECT 1 FROM oikumenea.religion_aliases a
				WHERE a.unit_id = s.org_unit_id AND a.deleted_at IS NULL AND lower(a.alias_text) LIKE `+like+`)
			OR EXISTS (SELECT 1 FROM oikumenea.tenant_units u
				WHERE u.id = s.org_unit_id AND (lower(u.code) LIKE `+like+` OR lower(u.name) LIKE `+like+`)))`)
	}

	sql := `SELECT ` + siteCols + ` ` + siteFrom + `
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY ` + orderBy + `
		LIMIT ` + add(q.Limit)
	rows, err := r.c.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Site
	for rows.Next() {
		s, err := scanSite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
