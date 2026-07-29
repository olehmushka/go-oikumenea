// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Facet enrichment (M56 / D-ObjectFacets).
//
// The scale world was built to measure the AUTHORIZATION path, so it only ever populated the columns
// that path reads: persons carried id/display_name/given/surname and units carried
// id/org_id/domain_id/name. Every facet column was therefore at its default — every person
// sex='not_known', status='active', birthdate NULL, no rank, no account; every unit public, active,
// pdp_scoped, with a NULL level and kind.
//
// EXPLAIN against that world is worse than useless for a filter: each predicate is either 100%- or
// 0%-selective, so the planner picks shapes it would never pick on real data, and a plan that looks
// fine here can seq-scan in production. This file gives every facet column a realistic distribution.
//
// It is written as set-based UPDATEs keyed on a stable per-row hash rather than as extra COPY
// columns, for one reason: the SAME code then serves a fresh seed and `-enrich` on an existing scale
// database, so there is exactly one definition of the distribution and no way for the two to drift.

// bucket renders a deterministic 0..99 bucket for a row id, salted so different facets get
// independent (but reproducible) distributions. md5 is used rather than the undocumented hashtext().
func bucket(col, salt string) string {
	return fmt.Sprintf(`((('x' || substr(md5(%s::text || '%s'), 1, 8))::bit(32)::bigint & 2147483647) %% 100)`, col, salt)
}

// enrichFacets gives every facet column a realistic distribution. Idempotent: re-running it produces
// the same world, because every value is a pure function of the row's id.
func enrichFacets(ctx context.Context, conn *pgx.Conn, orgID, domainID, graphID [16]byte) error {
	start := time.Now()

	fmt.Println("==> enriching person facet columns (sex / status / birthdate / country of birth)")
	// ~49% male, ~48% female, ~2% not_known, ~1% not_applicable. The not_known slice is deliberately
	// non-empty: it is the data-quality signal M57's sex donut must always show.
	if _, err := conn.Exec(ctx, `
		UPDATE oikumenea.person_persons SET sex = CASE
		  WHEN `+bucket("id", "sex")+` < 49 THEN 'male'
		  WHEN `+bucket("id", "sex")+` < 97 THEN 'female'
		  WHEN `+bucket("id", "sex")+` < 99 THEN 'not_known'
		  ELSE 'not_applicable' END`); err != nil {
		return fmt.Errorf("enrich sex: %w", err)
	}

	// ~96% active, ~3% deactivated, ~1% provisional. `provisional` is the narrow slice that makes a
	// selective-filter plan measurable — the case a missing index turns into a long PK walk.
	if _, err := conn.Exec(ctx, `
		UPDATE oikumenea.person_persons SET status = CASE
		  WHEN `+bucket("id", "status")+` < 96 THEN 'active'
		  WHEN `+bucket("id", "status")+` < 99 THEN 'deactivated'
		  ELSE 'provisional' END`); err != nil {
		return fmt.Errorf("enrich status: %w", err)
	}

	// ~40% unknown birthdate (the mandatory (unknown) bucket must be a real population), the rest
	// spread over a 60-year window.
	if _, err := conn.Exec(ctx, `
		UPDATE oikumenea.person_persons SET birthdate = CASE
		  WHEN `+bucket("id", "bdnull")+` < 40 THEN NULL
		  ELSE DATE '1960-01-01' + (((('x' || substr(md5(id::text || 'bday'), 1, 8))::bit(32)::bigint & 2147483647) % 21900)::int)
		END`); err != nil {
		return fmt.Errorf("enrich birthdate: %w", err)
	}

	// Country of birth over whatever the reference plane holds, with a deliberate long tail: half the
	// directory concentrates on the first country, so a top-N distribution has a realistic shape
	// rather than a uniform one (a uniform ref column makes top-N look artificially cheap).
	var countries int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM oikumenea.geo_countries`).Scan(&countries); err != nil {
		return fmt.Errorf("count countries: %w", err)
	}
	if countries == 0 {
		fmt.Println("    note: no geo_countries rows — countryOfBirth left NULL (seed the pinax reference plane to measure it)")
	} else {
		if _, err := conn.Exec(ctx, `
			WITH ranked AS (
			  SELECT id, (row_number() OVER (ORDER BY code) - 1) AS n FROM oikumenea.geo_countries
			)
			UPDATE oikumenea.person_persons p SET country_of_birth_id = r.id
			FROM ranked r
			WHERE r.n = CASE
			      WHEN `+bucket("p.id", "cob")+` < 50 THEN 0
			      ELSE (('x' || substr(md5(p.id::text || 'cob2'), 1, 8))::bit(32)::bigint & 2147483647) % $1
			    END`, countries); err != nil {
			return fmt.Errorf("enrich country of birth: %w", err)
		}
	}

	fmt.Println("==> enriching person ranks (~60% of the directory)")
	rankIDs, err := ensureScaleRankLadder(ctx, conn)
	if err != nil {
		return err
	}
	if len(rankIDs) > 0 {
		// Delete-then-insert keeps -enrich idempotent (a person holds one rank per system).
		if _, err := conn.Exec(ctx, `
			DELETE FROM oikumenea.person_ranks pr
			USING oikumenea.rank_systems s
			WHERE pr.system_id = s.id AND s.code = 'scale-ranks'`); err != nil {
			return fmt.Errorf("clear scale ranks: %w", err)
		}
		if _, err := conn.Exec(ctx, `
			WITH ladder AS (
			  SELECT r.id, r.system_id, (row_number() OVER (ORDER BY r.sort_order) - 1) AS n
			  FROM oikumenea.rank_ranks r
			  JOIN oikumenea.rank_systems s ON s.id = r.system_id AND s.code = 'scale-ranks'
			)
			INSERT INTO oikumenea.person_ranks (person_id, system_id, rank_id)
			SELECT p.id, l.system_id, l.id
			FROM oikumenea.person_persons p
			JOIN ladder l
			  ON l.n = (('x' || substr(md5(p.id::text || 'rank'), 1, 8))::bit(32)::bigint & 2147483647) % $1
			WHERE `+bucket("p.id", "hasrank")+` < 60`, len(rankIDs)); err != nil {
			return fmt.Errorf("enrich person ranks: %w", err)
		}
	}

	fmt.Println("==> enriching accounts (~35% of the directory — L-AccountOptional)")
	if _, err := conn.Exec(ctx, `
		DELETE FROM oikumenea.account_accounts a
		WHERE a.email LIKE 'scale-%@example.invalid'`); err != nil {
		return fmt.Errorf("clear scale accounts: %w", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.account_accounts (person_id, email)
		SELECT p.id, 'scale-' || p.id::text || '@example.invalid'
		FROM oikumenea.person_persons p
		WHERE `+bucket("p.id", "acct")+` < 35
		  AND NOT EXISTS (SELECT 1 FROM oikumenea.account_accounts a WHERE a.person_id = p.id)`); err != nil {
		return fmt.Errorf("enrich accounts: %w", err)
	}

	fmt.Println("==> enriching unit facet columns (level / visibility / state / pdpScoped / kind)")
	// level = the unit's depth from a graph root, which is what an org chart's width profile means.
	if _, err := conn.Exec(ctx, `
		UPDATE oikumenea.tenant_units u SET level = d.depth
		FROM (
		  SELECT c.descendant_id, min(c.depth)::smallint AS depth
		  FROM oikumenea.tenant_unit_closure c
		  WHERE c.graph_id = $1
		    AND NOT EXISTS (SELECT 1 FROM oikumenea.tenant_unit_edges e
		                    WHERE e.graph_id = $1 AND e.child_id = c.ancestor_id)
		  GROUP BY c.descendant_id
		) d
		WHERE u.id = d.descendant_id AND u.org_id = $2`, graphID, orgID); err != nil {
		return fmt.Errorf("enrich unit level: %w", err)
	}
	if _, err := conn.Exec(ctx, `
		UPDATE oikumenea.tenant_units SET
		  visibility = CASE WHEN `+bucket("id", "vis")+` < 5 THEN 'shadow' ELSE 'public' END,
		  state = CASE
		    WHEN `+bucket("id", "state")+` < 2 THEN 'suspended'
		    WHEN `+bucket("id", "state")+` < 3 THEN 'archived'
		    ELSE 'active' END,
		  pdp_scoped = (`+bucket("id", "pdp")+` >= 2)
		WHERE org_id = $1`, orgID); err != nil {
		return fmt.Errorf("enrich unit visibility/state/pdpScoped: %w", err)
	}
	if err := enrichUnitKinds(ctx, conn, orgID, domainID); err != nil {
		return err
	}

	// The M56 ticket-3 vocabulary: membership's own facets plus the order and document worlds, which
	// did not exist here at all (ticket3.go). It runs its own VACUUM FULL over its own tables.
	if err := enrichTicket3Facets(ctx, conn, orgID); err != nil {
		return err
	}

	// VACUUM FULL, not just ANALYZE. The enrichment above rewrites every person row several times,
	// which leaves the heap badly bloated — measured at 749 MB for 10^6 persons against 150 MB once
	// reclaimed. Measuring a filtered list path on the bloated table exaggerated its cost by roughly
	// 2x and made two query shapes look far apart when they are within noise of each other. A
	// measurement harness that silently measures its own bloat is worse than no harness, so the
	// reclaim is part of the seed rather than something the operator has to know to do.
	fmt.Println("==> VACUUM FULL + ANALYZE (reclaim the enrichment's dead tuples; fresh planner statistics)")
	for _, tbl := range []string{
		"oikumenea.person_persons", "oikumenea.person_ranks", "oikumenea.account_accounts",
		"oikumenea.tenant_units", "oikumenea.membership_memberships",
	} {
		if _, err := conn.Exec(ctx, "VACUUM (FULL, ANALYZE) "+tbl); err != nil {
			return fmt.Errorf("vacuum %s: %w", tbl, err)
		}
	}
	fmt.Printf("    facet enrichment done in %s\n", time.Since(start).Round(time.Second))
	return nil
}

// ensureScaleRankLadder seeds a small ordered rank scheme (the facet is a ref over an ORDERED
// vocabulary, so a flat set would not exercise the ordering M57 depends on). Idempotent.
func ensureScaleRankLadder(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	var systemID string
	err := conn.QueryRow(ctx, `SELECT id FROM oikumenea.rank_systems WHERE code = 'scale-ranks'`).Scan(&systemID)
	if err != nil {
		if err := conn.QueryRow(ctx, `
			INSERT INTO oikumenea.rank_systems (code, name, sort_order) VALUES ('scale-ranks', 'Scale harness ranks', 1)
			RETURNING id`).Scan(&systemID); err != nil {
			return nil, fmt.Errorf("seed rank system: %w", err)
		}
		var catID, typeID string
		if err := conn.QueryRow(ctx, `
			INSERT INTO oikumenea.rank_categories (system_id, code, name, sort_order)
			VALUES ($1, 'scale-enlisted', 'Enlisted', 1) RETURNING id`, systemID).Scan(&catID); err != nil {
			return nil, fmt.Errorf("seed rank category: %w", err)
		}
		if err := conn.QueryRow(ctx, `
			INSERT INTO oikumenea.rank_types (system_id, category_id, code, name, sort_order)
			VALUES ($1, $2, 'scale-type', 'Scale type', 1) RETURNING id`, systemID, catID).Scan(&typeID); err != nil {
			return nil, fmt.Errorf("seed rank type: %w", err)
		}
		for i := 1; i <= 12; i++ {
			if _, err := conn.Exec(ctx, `
				INSERT INTO oikumenea.rank_ranks (type_id, system_id, code, name, sort_order)
				VALUES ($1, $2, $3, $4, $5)`,
				typeID, systemID, fmt.Sprintf("scale-rank-%02d", i), fmt.Sprintf("Scale rank %d", i), i); err != nil {
				return nil, fmt.Errorf("seed rank %d: %w", i, err)
			}
		}
	}
	rows, err := conn.Query(ctx, `
		SELECT r.id FROM oikumenea.rank_ranks r
		JOIN oikumenea.rank_systems s ON s.id = r.system_id AND s.code = 'scale-ranks'
		ORDER BY r.sort_order`)
	if err != nil {
		return nil, fmt.Errorf("read rank ladder: %w", err)
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

// enrichUnitKinds seeds a handful of domain-scoped unit kinds and spreads the org's units across
// them, so the unitKind ref facet has a real distribution instead of a single NULL bucket.
func enrichUnitKinds(ctx context.Context, conn *pgx.Conn, orgID, domainID [16]byte) error {
	kinds := []struct{ code, name string }{
		{"scale-command", "Command"}, {"scale-brigade", "Brigade"}, {"scale-battalion", "Battalion"},
		{"scale-company", "Company"}, {"scale-platoon", "Platoon"}, {"scale-squad", "Squad"},
	}
	for i, k := range kinds {
		if _, err := conn.Exec(ctx, `
			INSERT INTO oikumenea.tenant_unit_kinds (domain_id, code, name, sort_order)
			VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, domainID, k.code, k.name, i+1); err != nil {
			return fmt.Errorf("seed unit kind %s: %w", k.code, err)
		}
	}
	if _, err := conn.Exec(ctx, `
		WITH ranked AS (
		  SELECT k.id, (row_number() OVER (ORDER BY k.sort_order) - 1) AS n
		  FROM oikumenea.tenant_unit_kinds k
		  WHERE k.domain_id = $1 AND k.code LIKE 'scale-%'
		)
		UPDATE oikumenea.tenant_units u SET kind_id = r.id
		FROM ranked r
		WHERE u.org_id = $2
		  AND r.n = (('x' || substr(md5(u.id::text || 'kind'), 1, 8))::bit(32)::bigint & 2147483647) % $3`,
		domainID, orgID, len(kinds)); err != nil {
		return fmt.Errorf("assign unit kinds: %w", err)
	}
	return nil
}

// loadScaleWorldIDs resolves the seeded world's org / domain / graph ids, for the -enrich path that
// runs against an already-populated database.
func loadScaleWorldIDs(ctx context.Context, conn *pgx.Conn) (org, dom, graph [16]byte, err error) {
	if err = conn.QueryRow(ctx, `
		SELECT o.id, o.domain_id, g.id
		FROM oikumenea.tenant_organizations o
		JOIN oikumenea.tenant_graphs g ON g.org_id = o.id AND g.code = $2
		WHERE o.code = $1`, orgCode, graphCode).Scan(&org, &dom, &graph); err != nil {
		return org, dom, graph, fmt.Errorf("locate the seeded scale world (run without -enrich first): %w", err)
	}
	return org, dom, graph, nil
}
