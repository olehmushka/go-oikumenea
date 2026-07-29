// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Facet enrichment for the M56 ticket-3 types: memberships (their OWN facets, distinct from the
// person facets they carry for the read-scope path), orders and documents.
//
// The scale world had no orders and no documents at all, and every membership was active with the
// same effective_from. Measuring a filter against that is worse than useless: each predicate is
// either 100%- or 0%-selective, so the planner picks shapes it would never pick on real data and a
// plan that looks fine here can sequential-scan in production. That is exactly the finding ticket 2
// recorded, so the harness grows with the vocabulary rather than after it.
//
// Same construction as facets.go: set-based statements keyed on a stable per-row hash, so one
// definition serves both a fresh seed and `-enrich`, and re-running produces the identical world.

// scaleOrdersPerUnit is how many order headers each unit issues. 100k units x 2 gives a 200k-row
// order table — enough that an unindexed filter is unmistakable in the plan, small enough that the
// seed stays minutes rather than hours.
const scaleOrdersPerUnit = 2

// scaleItemPersonSample bounds the person set order items point at. Items only need a valid FK with
// a plausible spread; joining 200k orders against the full 10^6-person table would dominate the seed
// for no measurement value.
const scaleItemPersonSample = 10000

// enrichTicket3Facets backfills the membership / order / document facet distributions. Idempotent.
func enrichTicket3Facets(ctx context.Context, conn *pgx.Conn, orgID [16]byte) error {
	start := time.Now()

	if err := enrichMembershipFacets(ctx, conn); err != nil {
		return err
	}
	if err := enrichOrderFacets(ctx, conn, orgID); err != nil {
		return err
	}
	if err := enrichDocumentFacets(ctx, conn); err != nil {
		return err
	}

	// Same reasoning as facets.go: the statements above rewrite and bulk-insert heavily, and a
	// harness that silently measures its own bloat is worse than no harness.
	fmt.Println("==> VACUUM FULL + ANALYZE (ticket-3 tables)")
	for _, tbl := range []string{
		"oikumenea.membership_memberships",
		"oikumenea.order_orders", "oikumenea.order_order_items",
		"oikumenea.document_documents",
	} {
		if _, err := conn.Exec(ctx, "VACUUM (FULL, ANALYZE) "+tbl); err != nil {
			return fmt.Errorf("vacuum %s: %w", tbl, err)
		}
	}
	fmt.Printf("    ticket-3 facet enrichment done in %s\n", time.Since(start).Round(time.Second))
	return nil
}

// enrichMembershipFacets gives memberships a status mix and an effective_from spread.
//
// The status mix is the point: every membership the harness seeded was ACTIVE, and every existing
// membership index is PARTIAL on status='active'. A world with no ended rows cannot show whether the
// status-agnostic listing the top-level endpoint ships is index-backed — it would match the partial
// index by accident.
func enrichMembershipFacets(ctx context.Context, conn *pgx.Conn) error {
	fmt.Println("==> enriching membership facet columns (status / effective_from)")

	// ~92% active, ~8% ended. Ended rows are the population that only the top-level list can reach.
	if _, err := conn.Exec(ctx, `
		UPDATE oikumenea.membership_memberships SET
		  status = CASE WHEN `+bucket("id", "mstatus")+` < 92 THEN 'active' ELSE 'ended' END`); err != nil {
		return fmt.Errorf("enrich membership status: %w", err)
	}

	// effective_from spread over ~12 years, so a date-range filter is genuinely selective and M57's
	// joins-per-month histogram has a shape. effective_to follows for the ended rows, keeping the
	// row internally consistent (an ended membership with no end date would be a lie the next reader
	// has to untangle).
	if _, err := conn.Exec(ctx, `
		UPDATE oikumenea.membership_memberships SET
		  effective_from = TIMESTAMPTZ '2014-01-01 00:00:00+00'
		    + (((('x' || substr(md5(id::text || 'mfrom'), 1, 8))::bit(32)::bigint & 2147483647) % 4380) || ' days')::interval,
		  effective_to = NULL`); err != nil {
		return fmt.Errorf("enrich membership effective_from: %w", err)
	}
	if _, err := conn.Exec(ctx, `
		UPDATE oikumenea.membership_memberships SET
		  effective_to = effective_from
		    + (((('x' || substr(md5(id::text || 'mto'), 1, 8))::bit(32)::bigint & 2147483647) % 900 + 30) || ' days')::interval
		WHERE status = 'ended'`); err != nil {
		return fmt.Errorf("enrich membership effective_to: %w", err)
	}
	return nil
}

// enrichOrderFacets seeds order headers + one item each. Orders did not exist in the scale world at
// all, so this is a create rather than an update; `scale-` numbering makes the re-seed a delete of
// exactly the harness's own rows.
func enrichOrderFacets(ctx context.Context, conn *pgx.Conn, orgID [16]byte) error {
	fmt.Println("==> seeding orders (status / issued_on / items with a type mix)")

	typeIDs, err := ensureScaleOrderTypes(ctx, conn)
	if err != nil {
		return err
	}

	// Items first: order_order_items FKs order_id ON DELETE CASCADE, but being explicit keeps the
	// delete order obvious to the next reader.
	if _, err := conn.Exec(ctx, `
		DELETE FROM oikumenea.order_orders WHERE number LIKE 'scale-ord-%'`); err != nil {
		return fmt.Errorf("clear scale orders: %w", err)
	}

	// ~10% draft, ~85% issued, ~5% revoked. A draft has NULL issued_on, which is what makes the
	// (unknown) bucket a real population and what a date-bounded filter must exclude.
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.order_orders (number, issuing_unit_id, status, issued_on)
		SELECT
		  'scale-ord-' || u.id::text || '-' || g.i::text,
		  u.id,
		  CASE
		    WHEN `+bucket("u.id::text || g.i::text", "ostatus")+` < 10 THEN 'draft'
		    WHEN `+bucket("u.id::text || g.i::text", "ostatus")+` < 95 THEN 'issued'
		    ELSE 'revoked' END,
		  CASE
		    WHEN `+bucket("u.id::text || g.i::text", "ostatus")+` < 10 THEN NULL
		    ELSE DATE '2016-01-01' + ((('x' || substr(md5(u.id::text || g.i::text || 'oday'), 1, 8))::bit(32)::bigint & 2147483647) % 3650)::int
		  END
		FROM oikumenea.tenant_units u
		CROSS JOIN generate_series(1, $2) AS g(i)
		WHERE u.org_id = $1 AND u.deleted_at IS NULL`, orgID, scaleOrdersPerUnit); err != nil {
		return fmt.Errorf("seed scale orders: %w", err)
	}

	// One item per order, its type drawn from the ladder with a long tail (half the items land on the
	// first type), so the orderTypeId top-N distribution has a realistic shape rather than a uniform
	// one — a uniform ref column makes top-N look artificially cheap.
	if _, err := conn.Exec(ctx, `
		WITH types AS (
		  SELECT id, (row_number() OVER (ORDER BY sort_order) - 1) AS n
		  FROM oikumenea.order_order_types WHERE code LIKE 'scale-otype-%'
		), people AS (
		  SELECT id, (row_number() OVER (ORDER BY id) - 1) AS n
		  FROM (SELECT id FROM oikumenea.person_persons ORDER BY id LIMIT $2) s
		), n_people AS (SELECT count(*)::bigint AS c FROM people)
		INSERT INTO oikumenea.order_order_items (order_id, type_id, person_id)
		SELECT o.id, t.id, pp.id
		FROM oikumenea.order_orders o
		CROSS JOIN n_people
		JOIN types t ON t.n = CASE
		      WHEN `+bucket("o.id", "otype")+` < 50 THEN 0
		      ELSE (('x' || substr(md5(o.id::text || 'otype2'), 1, 8))::bit(32)::bigint & 2147483647) % $1
		    END
		JOIN people pp
		  ON pp.n = (('x' || substr(md5(o.id::text || 'operson'), 1, 8))::bit(32)::bigint & 2147483647) % n_people.c
		WHERE o.number LIKE 'scale-ord-%'`, len(typeIDs), scaleItemPersonSample); err != nil {
		return fmt.Errorf("seed scale order items: %w", err)
	}
	return nil
}

// enrichDocumentFacets seeds documents for ~60% of the directory. Documents did not exist in the
// scale world either, so this is a create; the harness's own rows are identified by their `issuer`.
func enrichDocumentFacets(ctx context.Context, conn *pgx.Conn) error {
	fmt.Println("==> seeding documents (type / status / issuing country / issued+expires dates)")

	typeIDs, err := ensureScaleDocumentTypes(ctx, conn)
	if err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, `
		DELETE FROM oikumenea.document_documents WHERE issuer = 'scale-harness'`); err != nil {
		return fmt.Errorf("clear scale documents: %w", err)
	}

	var countries int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM oikumenea.geo_countries`).Scan(&countries); err != nil {
		return fmt.Errorf("count countries: %w", err)
	}

	// ~85% active, ~10% superseded, ~5% revoked; ~25% have NO expiry (the meaningful (no expiry)
	// bucket — a permanent document, not missing data); ~15% have no issue date.
	//
	// The issuing country is left NULL when the reference plane is unseeded rather than faked: a
	// fabricated FK target would make the top-N distribution measure a shape that does not exist.
	countryExpr := "NULL"
	if countries > 0 {
		countryExpr = `(SELECT c.id FROM (
			  SELECT id, (row_number() OVER (ORDER BY code) - 1) AS n FROM oikumenea.geo_countries
			) c WHERE c.n = CASE
			      WHEN ` + bucket("p.id", "dcty") + ` < 50 THEN 0
			      ELSE (('x' || substr(md5(p.id::text || 'dcty2'), 1, 8))::bit(32)::bigint & 2147483647) % ` + fmt.Sprint(countries) + `
			    END)`
	}

	if _, err := conn.Exec(ctx, `
		WITH types AS (
		  SELECT id, (row_number() OVER (ORDER BY sort_order) - 1) AS n
		  FROM oikumenea.document_document_types WHERE code LIKE 'scale-dtype-%'
		)
		INSERT INTO oikumenea.document_documents
		  (person_id, type_id, number, issuer, status, issued_on, expires_on, issuing_country_id)
		SELECT
		  p.id,
		  t.id,
		  'SC' || substr(md5(p.id::text || 'dnum'), 1, 10),
		  'scale-harness',
		  CASE
		    WHEN `+bucket("p.id", "dstatus")+` < 85 THEN 'active'
		    WHEN `+bucket("p.id", "dstatus")+` < 95 THEN 'superseded'
		    ELSE 'revoked' END,
		  CASE
		    WHEN `+bucket("p.id", "dissued")+` < 15 THEN NULL
		    ELSE DATE '2010-01-01' + ((('x' || substr(md5(p.id::text || 'dday'), 1, 8))::bit(32)::bigint & 2147483647) % 5475)::int
		  END,
		  CASE
		    WHEN `+bucket("p.id", "dexp")+` < 25 THEN NULL
		    ELSE DATE '2024-01-01' + ((('x' || substr(md5(p.id::text || 'dexpday'), 1, 8))::bit(32)::bigint & 2147483647) % 3650)::int
		  END,
		  `+countryExpr+`
		FROM oikumenea.person_persons p
		JOIN types t
		  ON t.n = (('x' || substr(md5(p.id::text || 'dtype'), 1, 8))::bit(32)::bigint & 2147483647) % $1
		WHERE p.deleted_at IS NULL AND `+bucket("p.id", "hasdoc")+` < 60`, len(typeIDs)); err != nil {
		return fmt.Errorf("seed scale documents: %w", err)
	}
	return nil
}

// ensureScaleOrderTypes seeds a small order-type catalog. Idempotent.
func ensureScaleOrderTypes(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	kinds := []struct{ category, effect string }{
		{"personnel-list", "record-only"},
		{"appointment", "membership-start"},
		{"appointment", "membership-end"},
		{"leave-travel", "record-only"},
		{"discipline-incentive", "record-only"},
		{"duty-roster", "record-only"},
		{"personnel-list", "rank-change"},
		{"appointment", "rank-change"},
	}
	for i, k := range kinds {
		if _, err := conn.Exec(ctx, `
			INSERT INTO oikumenea.order_order_types (code, name, category, effect, sort_order)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (code) DO NOTHING`,
			fmt.Sprintf("scale-otype-%02d", i+1), fmt.Sprintf("Scale order type %d", i+1),
			k.category, k.effect, i+1); err != nil {
			return nil, fmt.Errorf("seed order type %d: %w", i+1, err)
		}
	}
	return scaleCatalogIDs(ctx, conn,
		`SELECT id FROM oikumenea.order_order_types WHERE code LIKE 'scale-otype-%' ORDER BY sort_order`)
}

// ensureScaleDocumentTypes seeds a small document-type catalog. Idempotent.
func ensureScaleDocumentTypes(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	for i := 1; i <= 10; i++ {
		if _, err := conn.Exec(ctx, `
			INSERT INTO oikumenea.document_document_types (code, name, sort_order)
			VALUES ($1, $2, $3) ON CONFLICT (code) DO NOTHING`,
			fmt.Sprintf("scale-dtype-%02d", i), fmt.Sprintf("Scale document type %d", i), i); err != nil {
			return nil, fmt.Errorf("seed document type %d: %w", i, err)
		}
	}
	return scaleCatalogIDs(ctx, conn,
		`SELECT id FROM oikumenea.document_document_types WHERE code LIKE 'scale-dtype-%' ORDER BY sort_order`)
}

func scaleCatalogIDs(ctx context.Context, conn *pgx.Conn, query string) ([]string, error) {
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read scale catalog: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
