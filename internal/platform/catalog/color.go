// The per-domain color catalog (D-Color): platform's first RID-bearing reference Object, sitting next
// to the lawful-basis catalog. A thin raw-pgx repository + an audited write path (the platform module
// owns infrastructure + cross-cutting reference data, not a domain aggregate — overview.md). Colors are
// referenced by HARD FK from person physical descriptions (eye/hair) + vehicles; the `domain`
// discriminator (eye | hair | vehicle) keeps the vocabularies independent.
package catalog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
)

// ErrInvalidColor is returned for a malformed color upsert (bad domain/code/name/hex).
var ErrInvalidColor = errors.New("invalid color request")

// ErrUnknownColor is returned when a color id does not resolve to an active catalog row.
var ErrUnknownColor = errors.New("unknown color")

// validColorDomains is the closed set of palette domains (mirrors the platform_colors CHECK).
var validColorDomains = map[string]struct{}{"eye": {}, "hair": {}, "vehicle": {}}

// Color is one entry in a per-domain color palette (D-Color). Name is the default-locale label; the
// transport overlays the full locale->text map (D-i18n). Hex is a nullable representative swatch.
type Color struct {
	ID        string
	Domain    string // eye | hair | vehicle
	Code      string
	Name      string
	Hex       *string
	Status    string // active | retired
	SortOrder *int
}

// ColorService reads + (instance-admin) upserts the color catalog, auditing writes (D-Audit). It also
// answers ColorDomain for the consuming modules' hard-FK domain check (person/vehicle).
type ColorService struct {
	pool  *pgxpool.Pool
	audit *auditapp.Service
}

// NewColorService builds the color catalog service over the platform pool + the audit service.
func NewColorService(pool *pgxpool.Pool, audit *auditapp.Service) *ColorService {
	return &ColorService{pool: pool, audit: audit}
}

// List returns the catalog for one domain, ordered by (sort_order, code). An empty domain lists all.
func (s *ColorService) List(ctx context.Context, domain string) ([]Color, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, domain, code, name, hex, status, sort_order
		FROM oikumenea.platform_colors
		WHERE deleted_at IS NULL AND ($1 = '' OR domain = $1)
		ORDER BY domain, sort_order NULLS LAST, code`, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Color, 0, 32)
	for rows.Next() {
		var c Color
		if err := rows.Scan(&c.ID, &c.Domain, &c.Code, &c.Name, &c.Hex, &c.Status, &c.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ColorDomain returns the domain of an active color, or ErrUnknownColor. The consuming modules use it
// to enforce the hard FK's palette (e.g. eye_color_id must point at a domain='eye' color).
func (s *ColorService) ColorDomain(ctx context.Context, id string) (string, error) {
	var domain string
	err := s.pool.QueryRow(ctx,
		`SELECT domain FROM oikumenea.platform_colors WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&domain)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnknownColor
	}
	if err != nil {
		return "", err
	}
	return domain, nil
}

// Upsert adds or updates a color (instance-admin), recording an audit row in the same transaction
// (D-Audit). domain must be eye | hair | vehicle; code + name are required.
func (s *ColorService) Upsert(ctx context.Context, c Color) (Color, error) {
	if _, ok := validColorDomains[c.Domain]; !ok || c.Code == "" || c.Name == "" {
		return Color{}, ErrInvalidColor
	}
	var out Color
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO oikumenea.platform_colors (domain, code, name, hex, sort_order)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (domain, code) WHERE deleted_at IS NULL DO UPDATE SET
				name = EXCLUDED.name, hex = EXCLUDED.hex, sort_order = EXCLUDED.sort_order,
				status = 'active'
			RETURNING id, domain, code, name, hex, status, sort_order`,
			c.Domain, c.Code, c.Name, c.Hex, c.SortOrder)
		if err := row.Scan(&out.ID, &out.Domain, &out.Code, &out.Name, &out.Hex, &out.Status, &out.SortOrder); err != nil {
			return err
		}
		return s.record(ctx, tx, "color.upsert", out.ID, map[string]any{"id": out.ID, "domain": out.Domain, "code": out.Code})
	})
	return out, err
}

// inTx runs fn in a transaction, committing on success and rolling back on error.
func (s *ColorService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record mints a platform Action RID (1,3,0) and writes the audit row in the caller's transaction.
func (s *ColorService) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(1, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	raw, _ := json.Marshal(after)
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  "platform-catalog",
		Action:     action,
		TargetType: "color",
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      raw,
		Outcome:    auditdomain.OutcomeSuccess,
	})
}
