// Package adapters is the vehicle module's pgx-backed persistence adapter (M26, D-Vehicles). It uses
// raw pgx over a single command surface (the pool for reads, a tx for writes) — the religion/tenant
// raw-SQL style — because of the polymorphic owner, the derived model→brand join, and the cross-module
// label/region lookups. Postgres constraint violations (23505 unique / 23503 FK) map to domain sentinels.
package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/domain"
)

// Repository is the vehicle persistence adapter bound to one command surface (pool or tx).
type Repository struct{ c db.DBTX }

// NewRepository binds a repository to the given command surface.
func NewRepository(conn db.DBTX) *Repository { return &Repository{c: conn} }

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

// ---- small scan/param helpers ----

func textVal(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func intPtr(i pgtype.Int4) *int {
	if i.Valid {
		v := int(i.Int32)
		return &v
	}
	return nil
}

func dateStr(d pgtype.Date) string {
	if d.Valid {
		return d.Time.Format(domain.ISODate)
	}
	return ""
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if t.Valid {
		v := t.Time
		return &v
	}
	return nil
}

func mapPGError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return err // callers translate to their NotFound sentinel
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrConflict
		case "23503":
			return domain.ErrInvalid
		}
	}
	return err
}

// ============================ vehicle types ============================

func (r *Repository) ListVehicleTypes(ctx context.Context) ([]domain.VehicleType, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, parent_id, root_id, status, sort_order
		FROM oikumenea.vehicle_types WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.VehicleType
	for rows.Next() {
		t, err := scanVehicleTypeRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) GetVehicleType(ctx context.Context, id string) (domain.VehicleType, error) {
	row := r.c.QueryRow(ctx, `
		SELECT id, code, name, parent_id, root_id, status, sort_order
		FROM oikumenea.vehicle_types WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanVehicleType(row)
}

// UpsertVehicleType inserts/updates by code. root_id is derived: a node with a parent inherits the
// parent's root_id; a root points root_id at itself (the rank_types denormalized-root pattern).
func (r *Repository) UpsertVehicleType(ctx context.Context, code, name string, parentID *string, sortOrder *int) (domain.VehicleType, error) {
	row := r.c.QueryRow(ctx, `
		WITH p AS (
			SELECT root_id FROM oikumenea.vehicle_types WHERE id = $3::uuid AND deleted_at IS NULL
		)
		INSERT INTO oikumenea.vehicle_types (code, name, parent_id, root_id, sort_order)
		VALUES ($1, $2, $3::uuid, (SELECT root_id FROM p), $4)
		ON CONFLICT (code) WHERE deleted_at IS NULL
		DO UPDATE SET name = EXCLUDED.name, parent_id = EXCLUDED.parent_id,
		              root_id = COALESCE(EXCLUDED.root_id, oikumenea.vehicle_types.root_id),
		              sort_order = EXCLUDED.sort_order, updated_at = now()
		RETURNING id, code, name, parent_id, root_id, status, sort_order`, code, name, parentID, sortOrder)
	t, err := scanVehicleType(row)
	if err != nil {
		return domain.VehicleType{}, err
	}
	// A root (no parent) gets root_id = self in a follow-up (the row's id is unknown at insert time).
	if t.ParentID == "" && t.RootID == "" {
		if _, err := r.c.Exec(ctx, `UPDATE oikumenea.vehicle_types SET root_id = id WHERE id = $1`, t.ID); err != nil {
			return domain.VehicleType{}, err
		}
		t.RootID = t.ID
	}
	return t, nil
}

func scanVehicleType(row pgx.Row) (domain.VehicleType, error) {
	var t domain.VehicleType
	var parent, root pgtype.Text
	var so pgtype.Int4
	if err := row.Scan(&t.ID, &t.Code, &t.Name, &parent, &root, &t.Status, &so); err != nil {
		return domain.VehicleType{}, mapPGError(err)
	}
	t.ParentID, t.RootID, t.SortOrder = textVal(parent), textVal(root), intPtr(so)
	return t, nil
}

func scanVehicleTypeRows(rows pgx.Rows) (domain.VehicleType, error) {
	var t domain.VehicleType
	var parent, root pgtype.Text
	var so pgtype.Int4
	if err := rows.Scan(&t.ID, &t.Code, &t.Name, &parent, &root, &t.Status, &so); err != nil {
		return domain.VehicleType{}, err
	}
	t.ParentID, t.RootID, t.SortOrder = textVal(parent), textVal(root), intPtr(so)
	return t, nil
}

// ============================ brands ============================

func (r *Repository) ListBrands(ctx context.Context, query string) ([]domain.Brand, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, country_id, status, sort_order
		FROM oikumenea.vehicle_brands
		WHERE deleted_at IS NULL
		  AND ($1 = '' OR code ILIKE '%'||$1||'%' OR name ILIKE '%'||$1||'%')
		ORDER BY sort_order NULLS LAST, name`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Brand
	for rows.Next() {
		var b domain.Brand
		var country pgtype.Text
		var so pgtype.Int4
		if err := rows.Scan(&b.ID, &b.Code, &b.Name, &country, &b.Status, &so); err != nil {
			return nil, err
		}
		b.CountryID, b.SortOrder = textVal(country), intPtr(so)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *Repository) GetBrand(ctx context.Context, id string) (domain.Brand, error) {
	row := r.c.QueryRow(ctx, `
		SELECT id, code, name, country_id, status, sort_order
		FROM oikumenea.vehicle_brands WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanBrand(row)
}

func (r *Repository) UpsertBrand(ctx context.Context, code, name string, countryID *string, sortOrder *int) (domain.Brand, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.vehicle_brands (code, name, country_id, sort_order)
		VALUES ($1, $2, $3::uuid, $4)
		ON CONFLICT (code) WHERE deleted_at IS NULL
		DO UPDATE SET name = EXCLUDED.name, country_id = EXCLUDED.country_id,
		              sort_order = EXCLUDED.sort_order, updated_at = now()
		RETURNING id, code, name, country_id, status, sort_order`, code, name, countryID, sortOrder)
	return scanBrand(row)
}

func scanBrand(row pgx.Row) (domain.Brand, error) {
	var b domain.Brand
	var country pgtype.Text
	var so pgtype.Int4
	if err := row.Scan(&b.ID, &b.Code, &b.Name, &country, &b.Status, &so); err != nil {
		return domain.Brand{}, mapPGError(err)
	}
	b.CountryID, b.SortOrder = textVal(country), intPtr(so)
	return b, nil
}

// ============================ models ============================

func (r *Repository) ListModelsByBrand(ctx context.Context, brandID string) ([]domain.Model, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, brand_id, code, name, generation, manufacture_start, manufacture_end, status, sort_order
		FROM oikumenea.vehicle_models WHERE brand_id = $1 AND deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, name`, brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Model
	for rows.Next() {
		m, err := scanModelRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertModel(ctx context.Context, brandID, code, name string, generation, manufactureStart, manufactureEnd *string, sortOrder *int) (domain.Model, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.vehicle_models
			(brand_id, code, name, generation, manufacture_start, manufacture_end, sort_order)
		VALUES ($1, $2, $3, $4, NULLIF($5,'')::date, NULLIF($6,'')::date, $7)
		ON CONFLICT (brand_id, code) WHERE deleted_at IS NULL
		DO UPDATE SET name = EXCLUDED.name, generation = EXCLUDED.generation,
		              manufacture_start = EXCLUDED.manufacture_start, manufacture_end = EXCLUDED.manufacture_end,
		              sort_order = EXCLUDED.sort_order, updated_at = now()
		RETURNING id, brand_id, code, name, generation, manufacture_start, manufacture_end, status, sort_order`,
		brandID, code, name, derefStr(generation), derefStr(manufactureStart), derefStr(manufactureEnd), sortOrder)
	return scanModel(row)
}

func scanModel(row pgx.Row) (domain.Model, error) {
	var m domain.Model
	var gen pgtype.Text
	var ms, me pgtype.Date
	var so pgtype.Int4
	if err := row.Scan(&m.ID, &m.BrandID, &m.Code, &m.Name, &gen, &ms, &me, &m.Status, &so); err != nil {
		return domain.Model{}, mapPGError(err)
	}
	m.Generation, m.ManufactureStart, m.ManufactureEnd, m.SortOrder = textVal(gen), dateStr(ms), dateStr(me), intPtr(so)
	return m, nil
}

func scanModelRows(rows pgx.Rows) (domain.Model, error) {
	var m domain.Model
	var gen pgtype.Text
	var ms, me pgtype.Date
	var so pgtype.Int4
	if err := rows.Scan(&m.ID, &m.BrandID, &m.Code, &m.Name, &gen, &ms, &me, &m.Status, &so); err != nil {
		return domain.Model{}, err
	}
	m.Generation, m.ManufactureStart, m.ManufactureEnd, m.SortOrder = textVal(gen), dateStr(ms), dateStr(me), intPtr(so)
	return m, nil
}

// ============================ registration-number types ============================

func (r *Repository) ListNumberTypes(ctx context.Context) ([]domain.NumberType, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, status, sort_order
		FROM oikumenea.vehicle_registration_number_types WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.NumberType
	for rows.Next() {
		var n domain.NumberType
		var so pgtype.Int4
		if err := rows.Scan(&n.ID, &n.Code, &n.Name, &n.Status, &so); err != nil {
			return nil, err
		}
		n.SortOrder = intPtr(so)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertNumberType(ctx context.Context, code, name string, sortOrder *int) (domain.NumberType, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.vehicle_registration_number_types (code, name, sort_order)
		VALUES ($1, $2, $3)
		ON CONFLICT (code) WHERE deleted_at IS NULL
		DO UPDATE SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order, updated_at = now()
		RETURNING id, code, name, status, sort_order`, code, name, sortOrder)
	var n domain.NumberType
	var so pgtype.Int4
	if err := row.Scan(&n.ID, &n.Code, &n.Name, &n.Status, &so); err != nil {
		return domain.NumberType{}, mapPGError(err)
	}
	n.SortOrder = intPtr(so)
	return n, nil
}

// ============================ vehicles ============================

// vehicleSelect is the shared projection: the vehicle row + the derived brand_id via the model FK.
const vehicleSelect = `
	SELECT v.id, v.type_id, v.model_id, m.brand_id, v.vin, v.color, v.manufacture_date,
	       v.attributes, v.status, v.created_at, v.updated_at
	FROM oikumenea.vehicle_vehicles v
	LEFT JOIN oikumenea.vehicle_models m ON m.id = v.model_id`

func (r *Repository) InsertVehicle(ctx context.Context, in domain.VehicleInput) (domain.Vehicle, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.vehicle_vehicles (type_id, model_id, vin, color, manufacture_date, attributes)
		VALUES ($1, NULLIF($2,'')::uuid, NULLIF($3,''), NULLIF($4,''), NULLIF($5,'')::date,
		        COALESCE(NULLIF($6,'')::jsonb, '{}'::jsonb))
		RETURNING id`,
		in.TypeID, in.ModelID, domain.NormalizeVIN(in.VIN), in.Color, in.ManufactureDate, in.Attributes)
	var id string
	if err := row.Scan(&id); err != nil {
		return domain.Vehicle{}, mapPGError(err)
	}
	return r.GetVehicle(ctx, id)
}

func (r *Repository) GetVehicle(ctx context.Context, id string) (domain.Vehicle, error) {
	row := r.c.QueryRow(ctx, vehicleSelect+` WHERE v.id = $1 AND v.deleted_at IS NULL`, id)
	return scanVehicle(row)
}

func (r *Repository) UpdateVehicle(ctx context.Context, id string, up domain.VehicleUpdate) (domain.Vehicle, error) {
	row := r.c.QueryRow(ctx, `
		UPDATE oikumenea.vehicle_vehicles SET
			type_id          = COALESCE($2::uuid, type_id),
			model_id         = CASE WHEN $3::boolean THEN NULLIF($4,'')::uuid ELSE model_id END,
			vin              = CASE WHEN $5::boolean THEN NULLIF($6,'') ELSE vin END,
			color            = CASE WHEN $7::boolean THEN NULLIF($8,'') ELSE color END,
			manufacture_date = CASE WHEN $9::boolean THEN NULLIF($10,'')::date ELSE manufacture_date END,
			attributes       = CASE WHEN $11::boolean THEN COALESCE(NULLIF($12,'')::jsonb,'{}'::jsonb) ELSE attributes END,
			status           = COALESCE($13, status),
			updated_at       = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id`,
		id,
		ptrUUID(up.TypeID),
		up.ModelID != nil, derefStr(up.ModelID),
		up.VIN != nil, normVINPtr(up.VIN),
		up.Color != nil, derefStr(up.Color),
		up.ManufactureDate != nil, derefStr(up.ManufactureDate),
		up.Attributes != nil, derefStr(up.Attributes),
		up.Status)
	var rid string
	if err := row.Scan(&rid); err != nil {
		return domain.Vehicle{}, mapPGError(err)
	}
	return r.GetVehicle(ctx, rid)
}

func (r *Repository) ListVehicles(ctx context.Context, query, after string, lim int) ([]domain.Vehicle, error) {
	rows, err := r.c.Query(ctx, vehicleSelect+`
		WHERE v.deleted_at IS NULL
		  AND ($1 = '' OR v.vin ILIKE '%'||$1||'%')
		  AND ($2 = '' OR v.id > $2::uuid)
		ORDER BY v.id LIMIT $3`, query, after, lim)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Vehicle
	for rows.Next() {
		v, err := scanVehicleRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) SoftDeleteVehicle(ctx context.Context, id string) (int64, error) {
	tag, err := r.c.Exec(ctx, `UPDATE oikumenea.vehicle_vehicles SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return 0, mapPGError(err)
	}
	return tag.RowsAffected(), nil
}

func scanVehicle(row pgx.Row) (domain.Vehicle, error) {
	var v domain.Vehicle
	var model, brand, vin, color pgtype.Text
	var mdate pgtype.Date
	var attrs []byte
	if err := row.Scan(&v.ID, &v.TypeID, &model, &brand, &vin, &color, &mdate, &attrs, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return domain.Vehicle{}, mapPGError(err)
	}
	v.ModelID, v.BrandID, v.VIN, v.Color = textVal(model), textVal(brand), textVal(vin), textVal(color)
	v.ManufactureDate, v.Attributes = dateStr(mdate), string(attrs)
	return v, nil
}

func scanVehicleRows(rows pgx.Rows) (domain.Vehicle, error) {
	var v domain.Vehicle
	var model, brand, vin, color pgtype.Text
	var mdate pgtype.Date
	var attrs []byte
	if err := rows.Scan(&v.ID, &v.TypeID, &model, &brand, &vin, &color, &mdate, &attrs, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return domain.Vehicle{}, err
	}
	v.ModelID, v.BrandID, v.VIN, v.Color = textVal(model), textVal(brand), textVal(vin), textVal(color)
	v.ManufactureDate, v.Attributes = dateStr(mdate), string(attrs)
	return v, nil
}

// ============================ registrations ============================

const registrationCols = `id, vehicle_id, owner_kind, owner_id, country_id, subdivision_id,
	registration_number, number_type_id, status, effective_from, effective_to, created_at, updated_at`

func (r *Repository) InsertRegistration(ctx context.Context, vehicleID string, in domain.RegistrationInput) (domain.Registration, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.vehicle_registrations
			(vehicle_id, owner_kind, owner_id, country_id, subdivision_id, registration_number,
			 number_type_id, effective_from)
		VALUES ($1, $2, $3, $4, NULLIF($5,'')::uuid, $6, NULLIF($7,'')::uuid, COALESCE($8, now()))
		RETURNING `+registrationCols,
		vehicleID, in.OwnerKind, in.OwnerID, in.CountryID, in.SubdivisionID,
		strings.TrimSpace(in.RegistrationNumber), in.NumberTypeID, in.EffectiveFrom)
	return scanRegistration(row)
}

func (r *Repository) GetRegistration(ctx context.Context, id string) (domain.Registration, error) {
	row := r.c.QueryRow(ctx, `SELECT `+registrationCols+`
		FROM oikumenea.vehicle_registrations WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanRegistration(row)
}

// CloseActiveRegistrationsForVehicle ends every active registration of a vehicle (re-registration =
// the prior one closed). Idempotent — no active rows is a no-op.
func (r *Repository) CloseActiveRegistrationsForVehicle(ctx context.Context, vehicleID string) error {
	_, err := r.c.Exec(ctx, `
		UPDATE oikumenea.vehicle_registrations
		SET status = 'closed', effective_to = COALESCE(effective_to, now()), updated_at = now()
		WHERE vehicle_id = $1 AND status = 'active' AND deleted_at IS NULL`, vehicleID)
	return mapPGError(err)
}

func (r *Repository) CloseRegistration(ctx context.Context, id string) (domain.Registration, error) {
	row := r.c.QueryRow(ctx, `
		UPDATE oikumenea.vehicle_registrations
		SET status = 'closed', effective_to = COALESCE(effective_to, now()), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+registrationCols, id)
	return scanRegistration(row)
}

func (r *Repository) ListRegistrationsByVehicle(ctx context.Context, vehicleID string) ([]domain.Registration, error) {
	rows, err := r.c.Query(ctx, `SELECT `+registrationCols+`
		FROM oikumenea.vehicle_registrations WHERE vehicle_id = $1 AND deleted_at IS NULL
		ORDER BY effective_from DESC, created_at DESC`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Registration
	for rows.Next() {
		reg, err := scanRegistrationRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, reg)
	}
	return out, rows.Err()
}

func (r *Repository) ListRegistrationsByPersonOwner(ctx context.Context, personID string) ([]domain.PersonRegistration, error) {
	rows, err := r.c.Query(ctx, `
		SELECT reg.id, reg.vehicle_id, v.vin, v.type_id, v.model_id, m.brand_id,
		       reg.registration_number, reg.country_id, reg.subdivision_id, reg.status,
		       reg.effective_from, reg.effective_to
		FROM oikumenea.vehicle_registrations reg
		JOIN oikumenea.vehicle_vehicles v ON v.id = reg.vehicle_id
		LEFT JOIN oikumenea.vehicle_models m ON m.id = v.model_id
		WHERE reg.owner_kind = 'person' AND reg.owner_id = $1 AND reg.deleted_at IS NULL
		ORDER BY reg.effective_from DESC, reg.created_at DESC`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PersonRegistration
	for rows.Next() {
		var p domain.PersonRegistration
		var vin, model, brand, sub pgtype.Text
		var to pgtype.Timestamptz
		if err := rows.Scan(&p.ID, &p.VehicleID, &vin, &p.TypeID, &model, &brand,
			&p.RegistrationNumber, &p.CountryID, &sub, &p.Status, &p.EffectiveFrom, &to); err != nil {
			return nil, err
		}
		p.VIN, p.ModelID, p.BrandID, p.SubdivisionID = textVal(vin), textVal(model), textVal(brand), textVal(sub)
		p.EffectiveTo = timePtr(to)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) ErasePersonRegistrations(ctx context.Context, personID string) (int64, error) {
	tag, err := r.c.Exec(ctx, `
		UPDATE oikumenea.vehicle_registrations SET deleted_at = now()
		WHERE owner_kind = 'person' AND owner_id = $1 AND deleted_at IS NULL`, personID)
	if err != nil {
		return 0, mapPGError(err)
	}
	return tag.RowsAffected(), nil
}

func scanRegistration(row pgx.Row) (domain.Registration, error) {
	var reg domain.Registration
	var sub, numType pgtype.Text
	var to pgtype.Timestamptz
	if err := row.Scan(&reg.ID, &reg.VehicleID, &reg.OwnerKind, &reg.OwnerID, &reg.CountryID, &sub,
		&reg.RegistrationNumber, &numType, &reg.Status, &reg.EffectiveFrom, &to, &reg.CreatedAt, &reg.UpdatedAt); err != nil {
		return domain.Registration{}, mapPGError(err)
	}
	reg.SubdivisionID, reg.NumberTypeID, reg.EffectiveTo = textVal(sub), textVal(numType), timePtr(to)
	return reg, nil
}

func scanRegistrationRows(rows pgx.Rows) (domain.Registration, error) {
	var reg domain.Registration
	var sub, numType pgtype.Text
	var to pgtype.Timestamptz
	if err := rows.Scan(&reg.ID, &reg.VehicleID, &reg.OwnerKind, &reg.OwnerID, &reg.CountryID, &sub,
		&reg.RegistrationNumber, &numType, &reg.Status, &reg.EffectiveFrom, &to, &reg.CreatedAt, &reg.UpdatedAt); err != nil {
		return domain.Registration{}, err
	}
	reg.SubdivisionID, reg.NumberTypeID, reg.EffectiveTo = textVal(sub), textVal(numType), timePtr(to)
	return reg, nil
}

// ============================ brand manufacturers ============================

func (r *Repository) InsertManufacturer(ctx context.Context, brandID string, in domain.ManufacturerInput) (domain.Manufacturer, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.vehicle_brand_manufacturers (brand_id, company_id, effective_from, effective_to)
		VALUES ($1, $2, NULLIF($3,'')::date, NULLIF($4,'')::date)
		RETURNING id, brand_id, company_id, effective_from, effective_to, created_at, updated_at`,
		brandID, in.CompanyID, in.EffectiveFrom, in.EffectiveTo)
	return scanManufacturer(row)
}

func (r *Repository) GetManufacturer(ctx context.Context, id string) (domain.Manufacturer, error) {
	row := r.c.QueryRow(ctx, `
		SELECT id, brand_id, company_id, effective_from, effective_to, created_at, updated_at
		FROM oikumenea.vehicle_brand_manufacturers WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanManufacturer(row)
}

func (r *Repository) SoftDeleteManufacturer(ctx context.Context, id string) (int64, error) {
	tag, err := r.c.Exec(ctx, `UPDATE oikumenea.vehicle_brand_manufacturers SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return 0, mapPGError(err)
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) ListManufacturersByBrand(ctx context.Context, brandID string) ([]domain.Manufacturer, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, brand_id, company_id, effective_from, effective_to, created_at, updated_at
		FROM oikumenea.vehicle_brand_manufacturers WHERE brand_id = $1 AND deleted_at IS NULL
		ORDER BY effective_from DESC NULLS LAST, created_at DESC`, brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Manufacturer
	for rows.Next() {
		var m domain.Manufacturer
		var from, to pgtype.Date
		if err := rows.Scan(&m.ID, &m.BrandID, &m.CompanyID, &from, &to, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.EffectiveFrom, m.EffectiveTo = dateStr(from), dateStr(to)
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanManufacturer(row pgx.Row) (domain.Manufacturer, error) {
	var m domain.Manufacturer
	var from, to pgtype.Date
	if err := row.Scan(&m.ID, &m.BrandID, &m.CompanyID, &from, &to, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return domain.Manufacturer{}, mapPGError(err)
	}
	m.EffectiveFrom, m.EffectiveTo = dateStr(from), dateStr(to)
	return m, nil
}

// ============================ cross-reference helpers ============================

func (r *Repository) IsRegion(ctx context.Context, placeID string) (bool, error) {
	var ok bool
	err := r.c.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM oikumenea.geo_places
			WHERE id = $1::uuid AND placetype = 'region' AND status = 'active')`, placeID).Scan(&ok)
	return ok, err
}

func (r *Repository) TypeNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.vehicle_types WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, ids)
}

func (r *Repository) BrandNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.vehicle_brands WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, ids)
}

func (r *Repository) ModelNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.vehicle_models WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, ids)
}

func (r *Repository) CompanyNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	// M41 / D-UnifiedOrgGraph: a company manufacturer is a `company`-domain tenant organization.
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.tenant_organizations WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, ids)
}

func (r *Repository) PlaceNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.geo_places WHERE id = ANY($1::uuid[])`, ids)
}

func (r *Repository) namesByIDs(ctx context.Context, sql string, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.c.Query(ctx, sql, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// ---- param helpers ----

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func normVINPtr(s *string) string {
	if s == nil {
		return ""
	}
	return domain.NormalizeVIN(*s)
}

// ptrUUID returns the *string for a nullable uuid update param ($n::uuid COALESCE); nil leaves unchanged.
func ptrUUID(s *string) *string { return s }
