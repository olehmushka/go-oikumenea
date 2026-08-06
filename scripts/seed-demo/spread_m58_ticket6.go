// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

// The M58 ticket 6 spreads: location facets and the grant population.
//
// Both run on the create-if-absent REFRESH path as well as a full seed, for the reason ticket 4 gave
// when it made that path exist — the seeder was all-or-nothing, and `-reset` is blocked on any dev
// database carrying hand-made rows it does not own. A ticket that needs an existing column to VARY
// could otherwise only be applied by destroying the database it wanted to demonstrate on.
//
// Why they are needed at all: without them neither of this ticket's dashboards can be exercised.
//
//   - Every demo location carried country_id = UA and type_id = NULL, so the country chart would be a
//     single bar and the place-type chart 100% `(unknown)` — and a bucket you cannot click is a
//     click-through you cannot verify. That is ticket 3's worst defect exactly (four seeded catalogs
//     that nothing referenced), and it is the one the guards cannot catch, because the code was right
//     and the world was empty.
//   - seed-demo created NO role assignments at all. The grant table is the one the whole PDP is built
//     on and the seeder never wrote a row into it.
//
// Both are deterministic given the same row set (they order by id and bucket by row_number), so
// re-running changes nothing.

// spreadLocationFacets gives the demo locations a country and a place-type distribution.
//
// The COORDINATE moves with the country rather than the country alone. Spreading country_id over a
// set of rows whose points all sit inside Ukraine would produce a registry where a German address is
// at 50°N 30°E, and the radius window — the thing this ticket taught the dashboard to honour — would
// then return "Germany" for a search around Kyiv. The country column is a directory attribute here
// (nothing derives it from geom), which is precisely why nothing would have complained.
//
// Type is left NULL on a quarter of the rows on purpose. `(unknown)` is a real bucket for this facet,
// not an artefact: classifying a place is optional, and a chart that never showed the unclassified
// share would misrepresent the registry.
func (s *seeder) spreadLocationFacets() error {
	// lat/lng ranges are deliberately coarse — a rough national box, not a border polygon. They exist
	// so that a radius or bbox window selects a recognisable subset, not to be geographically exact.
	if err := s.exec("location_country", `
		WITH numbered AS (
		  SELECT id, row_number() OVER (ORDER BY id) - 1 AS n
		  FROM oikumenea.location_locations
		  WHERE raw_address = 'DEMO' AND deleted_at IS NULL
		),
		boxes(bucket, iso, lat_lo, lat_hi, lng_lo, lng_hi) AS (VALUES
		  (0, 'UA', 44.4, 52.3,  22.2,  40.1),
		  (1, 'UA', 44.4, 52.3,  22.2,  40.1),
		  (2, 'UA', 44.4, 52.3,  22.2,  40.1),
		  (3, 'UA', 44.4, 52.3,  22.2,  40.1),
		  (4, 'UA', 44.4, 52.3,  22.2,  40.1),
		  (5, 'UA', 44.4, 52.3,  22.2,  40.1),
		  (6, 'PL', 49.1, 54.7,  14.2,  24.0),
		  (7, 'DE', 47.4, 54.9,   6.0,  14.9),
		  (8, 'TR', 36.1, 42.0,  26.1,  44.7),
		  (9, 'US', 25.9, 48.9,-124.6, -67.0)
		),
		picked AS (
		  SELECT nu.id,
		         c.id AS country_id,
		         b.lat_lo + (b.lat_hi - b.lat_lo) * ((nu.n * 37 % 100)::double precision / 100) AS lat,
		         b.lng_lo + (b.lng_hi - b.lng_lo) * ((nu.n * 53 % 100)::double precision / 100) AS lng
		  FROM numbered nu
		  JOIN boxes b ON b.bucket = nu.n % 10
		  JOIN oikumenea.geo_countries c ON c.code = b.iso
		)
		UPDATE oikumenea.location_locations l
		SET country_id = p.country_id,
		    geom = ST_SetSRID(ST_MakePoint(p.lng, p.lat), 4326)::geography,
		    mgrs = NULL
		FROM picked p
		WHERE l.id = p.id`); err != nil {
		return err
	}
	// Place type over the three catalog entries plus NULL, ordered by code so the mapping is stable
	// whatever order the catalog was seeded in.
	return s.exec("location_type", `
		WITH numbered AS (
		  SELECT id, row_number() OVER (ORDER BY id) - 1 AS n
		  FROM oikumenea.location_locations
		  WHERE raw_address = 'DEMO' AND deleted_at IS NULL
		),
		types AS (
		  SELECT id, row_number() OVER (ORDER BY code) - 1 AS t
		  FROM oikumenea.location_location_types
		  WHERE deleted_at IS NULL AND status = 'active'
		)
		UPDATE oikumenea.location_locations l
		SET type_id = t.id
		FROM numbered nu LEFT JOIN types t ON t.t = nu.n % 4
		WHERE l.id = nu.id`)
}

// seedGrantSpread writes the demo role assignments, create-if-absent.
//
// The population is chosen to make every chart on the assignment dashboard readable and every bucket
// clickable: several roles, both scopes, many target units, a handful of people holding more than one
// grant, and some REVOKED rows so the active-only default is visibly a default rather than an
// accident of an empty table.
//
// Idempotent by construction: authz_role_assignments_active_idx is UNIQUE on
// (subject, role, unit, scope, graph) WHERE revoked_at IS NULL, so ON CONFLICT DO NOTHING is the
// natural shape and a re-run adds nothing.
//
// Scope and graph move together because the schema says so — authz_role_assignments_graph_scope
// CHECKs that graph_id IS NOT NULL exactly when scope = 'subtree' — which is also why the assignment
// dashboard's `(unknown)` graph bucket is the unit-scope count under another name.
func (s *seeder) seedGrantSpread() error {
	if err := s.exec("grant", `
		WITH people AS (
		  SELECT id, row_number() OVER (ORDER BY id) - 1 AS n
		  FROM oikumenea.person_persons
		  WHERE attributes->>'seed' = 'demo' AND deleted_at IS NULL
		  LIMIT 24
		),
		units AS (
		  SELECT u.id, u.org_id, row_number() OVER (ORDER BY u.id) - 1 AS n
		  FROM oikumenea.tenant_units u
		  JOIN oikumenea.tenant_organizations o ON o.id = u.org_id
		  WHERE o.metadata->>'seed' = 'demo' AND u.deleted_at IS NULL AND o.deleted_at IS NULL
		  LIMIT 30
		),
		roles AS (
		  SELECT id, row_number() OVER (ORDER BY code) - 1 AS n, count(*) OVER () AS total
		  FROM oikumenea.authz_roles WHERE deleted_at IS NULL
		),
		-- One authority-bearing graph per organization, chosen by code so the pick does not depend on
		-- insertion order. A subtree grant MUST name one; a unit grant must not.
		graphs AS (
		  SELECT DISTINCT ON (org_id) org_id, id
		  FROM oikumenea.tenant_graphs
		  WHERE is_authority_bearing AND deleted_at IS NULL AND org_id IS NOT NULL
		  ORDER BY org_id, code
		),
		pairs AS (
		  SELECT p.id AS person_id, u.id AS unit_id, g.id AS graph_id, r.id AS role_id,
		         -- Two grants for every third person, so the "most-granted people" bar has a shape.
		         CASE WHEN p.n % 3 = 0 THEN 'subtree' ELSE 'unit' END AS scope,
		         p.n
		  FROM people p
		  JOIN units u ON u.n = (p.n * 7) % 30
		  JOIN roles r ON r.n = p.n % r.total
		  JOIN graphs g ON g.org_id = u.org_id
		)
		INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope, graph_id)
		SELECT person_id, role_id, unit_id, scope,
		       CASE WHEN scope = 'subtree' THEN graph_id ELSE NULL END
		FROM pairs
		ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	// A second grant for a third of the people, on a different unit and role, so the subject and role
	// distributions are not a flat line of ones.
	if err := s.exec("grant", `
		WITH people AS (
		  SELECT id, row_number() OVER (ORDER BY id) - 1 AS n
		  FROM oikumenea.person_persons
		  WHERE attributes->>'seed' = 'demo' AND deleted_at IS NULL
		  LIMIT 24
		),
		units AS (
		  SELECT u.id, u.org_id, row_number() OVER (ORDER BY u.id) - 1 AS n
		  FROM oikumenea.tenant_units u
		  JOIN oikumenea.tenant_organizations o ON o.id = u.org_id
		  WHERE o.metadata->>'seed' = 'demo' AND u.deleted_at IS NULL AND o.deleted_at IS NULL
		  LIMIT 30
		),
		roles AS (
		  SELECT id, row_number() OVER (ORDER BY code) - 1 AS n, count(*) OVER () AS total
		  FROM oikumenea.authz_roles WHERE deleted_at IS NULL
		)
		INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope, graph_id)
		SELECT p.id, r.id, u.id, 'unit', NULL
		FROM people p
		JOIN units u ON u.n = (p.n * 11 + 3) % 30
		JOIN roles r ON r.n = (p.n + 4) % r.total
		WHERE p.n % 3 = 1
		ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	// Revoke a few. They stay in the table and out of every listing — which is the point: it is what
	// makes "totalCount counts ACTIVE grants" a demonstrable claim rather than a sentence in a doc.
	if err := s.exec("grant_revoked", `
		UPDATE oikumenea.authz_role_assignments a
		SET revoked_at = now() - interval '30 days'
		WHERE a.revoked_at IS NULL
		  AND a.id IN (
		    SELECT id FROM oikumenea.authz_role_assignments
		    WHERE revoked_at IS NULL
		    ORDER BY id
		    OFFSET 2 LIMIT 5)`); err != nil {
		return err
	}
	// Bump the revocation epoch, or the per-process grant cache (D-AuthzGrantCache) will serve
	// pre-seed authority until its TTL expires — a running server would ignore every grant just
	// written, and the dashboard would disagree with the database for no visible reason.
	return s.exec("authz_epoch", `UPDATE oikumenea.authz_epoch SET epoch = epoch + 1 WHERE singleton`)
}
