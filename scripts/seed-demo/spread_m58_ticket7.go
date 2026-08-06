// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

// The M58 ticket 7 spread: the enrollment population.
//
// Runs on the create-if-absent REFRESH path as well as a full seed, for the reason ticket 4 gave when
// it made that path exist. Deterministic given the same row set (it orders by id and buckets by
// row_number), so re-running changes nothing.
//
// Why it is needed. phaseDEnrichment enrolls every seventh person, and every one of those rows was
// written the same way:
//
//	INSERT INTO person_education_enrollments (person_id, institution_id, program_id, status)
//	VALUES ($1, $2, $3, 'enrolled')
//
// — one institution, one programme, status always `enrolled`, and unit_id / group_id /
// degree_level_id / effective_from all NULL. Of the seven facets this ticket declares, that leaves
// FOUR that cannot be exercised at all (three ref charts entirely `(unknown)`, one enum chart a
// single bar) and two more with exactly one bucket. It is ticket 3's finding for the third time: the
// facet code is right, the guards are green, and the world is empty — so the click-through the whole
// vocabulary exists for has nothing to click.
//
// The degree-level spread is the one that matters most, because `degreeLevelId` is the first
// catalog-ordered facet (facet.StrategyCatalog) and its defining property is that levels with NO
// rows still get a bar. A seed where every level is populated would render correctly and prove
// nothing about the property; a seed where every level is empty proves nothing either. So the spread
// deliberately populates SIX of the nine ISCED levels and leaves three empty — and leaves a slice of
// rows with no level at all, because `(unknown)` is a real bucket here (a recorded enrollment whose
// level nobody captured) rather than an artefact.

// ensureDemoProgrammes tops the university up to four programmes, create-if-absent by code.
//
// phaseDEnrichment seeds exactly one (`demo-cs`), so the programme chart would be a single bar
// however well the rows were spread — the same shape ticket 5 hit when every demo org carried the
// column defaults. Create-if-absent by the stable `code` (D-Code) is what makes this safe from the
// refresh path, the arrangement upsertProfileOrg established.
func (s *seeder) ensureDemoProgrammes() error {
	var uni string
	if err := s.scalar(&uni, `SELECT id::text FROM oikumenea.tenant_organizations WHERE code='khnu' LIMIT 1`); err != nil || uni == "" {
		return nil // no demo university (a database seeded by something else) — nothing to top up
	}
	for _, p := range []struct{ code, name string }{
		{"demo-math", "Applied Mathematics"},
		{"demo-hist", "History"},
		{"demo-med", "General Medicine"},
	} {
		if err := s.exec("edu_program", `
			INSERT INTO oikumenea.education_programs (institution_id, code, name)
			SELECT $1, $2, $3
			WHERE NOT EXISTS (
			  SELECT 1 FROM oikumenea.education_programs WHERE code = $2 AND deleted_at IS NULL)`,
			uni, p.code, p.name); err != nil {
			return err
		}
	}
	return nil
}

// spreadEnrollmentFacets gives the enrollment dashboard something to describe.
//
// Every assignment is by `row_number() % k` over an id ordering, so the result depends on the row SET
// and not on when it ran. The moduli are coprime-ish rather than round (11, 7, 5, 13) so the facets
// do not correlate: with 4 and 8, say, every doctoral student would also be in one faculty and the
// cross-filtering a dashboard exists for would return the same rows twice.
func (s *seeder) spreadEnrollmentFacets() error {
	if err := s.ensureDemoProgrammes(); err != nil {
		return err
	}
	// Six of the nine ISCED levels, chosen as the tertiary end of the scale (3..8) — the levels a
	// university register would actually hold. 0/1/2 stay EMPTY on purpose: they are what proves the
	// catalog strategy emits a zero-count bar rather than dropping the level, which is the whole
	// difference between this strategy and topN, and it cannot be demonstrated on a fully-populated
	// scale.
	if err := s.exec("enrollment_facets", `
		WITH numbered AS (
		  SELECT id, row_number() OVER (ORDER BY id) - 1 AS n
		  FROM oikumenea.person_education_enrollments
		  WHERE deleted_at IS NULL
		),
		levels AS (
		  SELECT id, row_number() OVER (ORDER BY isced_level) - 1 AS k
		  FROM oikumenea.education_degree_levels
		  WHERE deleted_at IS NULL AND isced_level BETWEEN 3 AND 8
		),
		programmes AS (
		  SELECT id, row_number() OVER (ORDER BY code) - 1 AS k
		  FROM oikumenea.education_programs
		  WHERE deleted_at IS NULL AND code LIKE 'demo-%'
		),
		-- A group belongs to a unit, so the two are picked TOGETHER: assigning them independently
		-- would produce students in a faculty whose study group belongs to another one. The facets are
		-- independent; the world they describe is not.
		cohorts AS (
		  SELECT g.id AS group_id, g.unit_id, row_number() OVER (ORDER BY g.id) - 1 AS k
		  FROM oikumenea.education_groups g
		  WHERE g.deleted_at IS NULL
		),
		statuses(k, status) AS (VALUES
		  (0, 'enrolled'), (1, 'enrolled'), (2, 'enrolled'), (3, 'enrolled'),
		  (4, 'enrolled'), (5, 'enrolled'), (6, 'graduated'), (7, 'graduated'),
		  (8, 'graduated'), (9, 'on_leave'), (10, 'withdrawn'), (11, 'expelled')
		),
		picked AS (
		  SELECT nu.id,
		         nu.n,
		         -- every 11th row keeps a NULL level: (unknown) is a real bucket for this facet
		         CASE WHEN nu.n % 11 = 0 THEN NULL
		              ELSE (SELECT l.id FROM levels l WHERE l.k = nu.n % (SELECT count(*) FROM levels))
		         END AS degree_level_id,
		         (SELECT p.id FROM programmes p WHERE p.k = nu.n % (SELECT count(*) FROM programmes)) AS program_id,
		         (SELECT c.unit_id FROM cohorts c WHERE c.k = nu.n % (SELECT count(*) FROM cohorts)) AS unit_id,
		         -- every 7th row sits in a faculty with no study group recorded
		         CASE WHEN nu.n % 7 = 0 THEN NULL
		              ELSE (SELECT c.group_id FROM cohorts c WHERE c.k = nu.n % (SELECT count(*) FROM cohorts))
		         END AS group_id,
		         (SELECT st.status FROM statuses st WHERE st.k = nu.n % 12) AS status,
		         -- Five intake years, September and February — a real register's two intakes per year,
		         -- which is what makes the month histogram read as a cycle rather than as noise. Every
		         -- 13th row keeps a NULL intake, again because (unknown) is real here.
		         CASE WHEN nu.n % 13 = 0 THEN NULL
		              ELSE make_date((2019 + (nu.n % 5))::int, CASE WHEN nu.n % 2 = 0 THEN 9 ELSE 2 END, 1)
		         END AS effective_from
		  FROM numbered nu
		)
		UPDATE oikumenea.person_education_enrollments e
		SET degree_level_id = p.degree_level_id,
		    program_id      = COALESCE(p.program_id, e.program_id),
		    unit_id         = p.unit_id,
		    group_id        = p.group_id,
		    status          = COALESCE(p.status, e.status),
		    effective_from  = p.effective_from,
		    -- A terminal status with no end date is a row that contradicts itself; the facet does not
		    -- read this column, but the drawer and the list both show it.
		    effective_to    = CASE WHEN p.status IN ('graduated','withdrawn','expelled') AND p.effective_from IS NOT NULL
		                           THEN p.effective_from + INTERVAL '4 years' ELSE NULL END
		FROM picked p
		WHERE e.id = p.id`); err != nil {
		return err
	}
	return nil
}
