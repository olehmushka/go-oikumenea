package main

import "fmt"

// openPosition is a billet ready to be filled; rankID is the UA rank a holder should carry ("" for
// civilian positions, e.g. university).
type openPosition struct {
	id     string
	unitID string
	rankID string
	system string // rank system id or ""
	title  string
}

// echelon → UA-armed-forces rank code (land force). Resolved to rank_ranks ids on demand.
var echelonRank = map[string]string{
	"brigade":   "polkovnyk",     // OF-5
	"battalion": "pidpolkovnyk",  // OF-4
	"company":   "kapitan",       // OF-2
	"platoon":   "leitenant",     // OF-1
}

var rifleRanks = []string{"soldat", "starshyi-soldat", "molodshyi-serzhant", "serzhant"}

func (s *seeder) rankID(code string) (string, error) {
	var id string
	err := s.scalar(&id,
		`SELECT id::text FROM oikumenea.rank_ranks WHERE system_id=$1 AND code=$2 AND deleted_at IS NULL ORDER BY id LIMIT 1`,
		s.uaSystemID, code)
	if err != nil {
		return "", fmt.Errorf("resolve rank %q: %w", code, err)
	}
	return id, nil
}

// state carried between phases
type dirState struct {
	militaryDomain, universityDomain, companyDomain, churchDomain string
	kindID                                                        map[string]string // military kind code -> id
	positions                                                     []openPosition
	universityOrgID, companyOrgID                                 string
	churchUnitID                                                  string
	studentUnitIDs                                                []string // faculties/departments for enrollment
	militaryUnitIDs                                               []string
}

var dir = &dirState{kindID: map[string]string{}}

func (s *seeder) domainID(code string) (string, error) {
	var id string
	err := s.scalar(&id, `SELECT id::text FROM oikumenea.tenant_domains WHERE code=$1`, code)
	return id, err
}

func (s *seeder) militaryKind(code string) (string, error) {
	if id, ok := dir.kindID[code]; ok {
		return id, nil
	}
	var id string
	if err := s.scalar(&id, `SELECT id::text FROM oikumenea.tenant_unit_kinds WHERE domain_id=$1 AND code=$2`, dir.militaryDomain, code); err != nil {
		return "", fmt.Errorf("military unit_kind %q: %w", code, err)
	}
	dir.kindID[code] = id
	return id, nil
}

// createOrg inserts an organization tagged demo, plus its command+operational graphs; returns
// (orgID, commandGraphID).
func (s *seeder) createOrg(code, name, domainID string) (string, string, error) {
	orgID, err := s.ins("org", `
		INSERT INTO oikumenea.tenant_organizations (code, name, domain_id, metadata)
		VALUES ($1,$2,$3,'{"seed":"demo"}') RETURNING id`, code, name, domainID)
	if err != nil {
		return "", "", err
	}
	cmd, err := s.ins("graph", `
		INSERT INTO oikumenea.tenant_graphs (org_id, code, name, is_default, is_authority_bearing)
		VALUES ($1,'command','Command',true,true) RETURNING id`, orgID)
	if err != nil {
		return "", "", err
	}
	if _, err := s.ins("graph", `
		INSERT INTO oikumenea.tenant_graphs (org_id, code, name, is_default, is_authority_bearing)
		VALUES ($1,'operational','Operational',false,true) RETURNING id`, orgID); err != nil {
		return "", "", err
	}
	return orgID, cmd, nil
}

func (s *seeder) createUnit(orgID, domainID, kindID, name string) (string, error) {
	if kindID == "" {
		return s.ins("unit", `INSERT INTO oikumenea.tenant_units (org_id, domain_id, name) VALUES ($1,$2,$3) RETURNING id`, orgID, domainID, name)
	}
	return s.ins("unit", `INSERT INTO oikumenea.tenant_units (org_id, domain_id, kind_id, name) VALUES ($1,$2,$3,$4) RETURNING id`, orgID, domainID, kindID, name)
}

func (s *seeder) addEdge(graphID, parent, child string) error {
	return s.exec("edge", `INSERT INTO oikumenea.tenant_unit_edges (graph_id, parent_id, child_id) VALUES ($1,$2,$3)`, graphID, parent, child)
}

func (s *seeder) rebuildClosure(graphID string) error {
	return s.exec("closure_rebuild", `
		WITH RECURSIVE
		  nodes AS (
		    SELECT parent_id AS u FROM oikumenea.tenant_unit_edges WHERE graph_id=$1
		    UNION SELECT child_id FROM oikumenea.tenant_unit_edges WHERE graph_id=$1),
		  reach AS (
		    SELECT u AS ancestor_id, u AS descendant_id, 0 AS depth FROM nodes
		    UNION ALL
		    SELECT r.ancestor_id, e.child_id, r.depth+1
		    FROM reach r JOIN oikumenea.tenant_unit_edges e
		      ON e.graph_id=$1 AND e.parent_id=r.descendant_id)
		INSERT INTO oikumenea.tenant_unit_closure (graph_id, ancestor_id, descendant_id, depth)
		SELECT $1::uuid, ancestor_id, descendant_id, min(depth)::int
		FROM reach GROUP BY ancestor_id, descendant_id`, graphID)
}

// addPosition creates a billet under a unit and remembers it as fillable.
func (s *seeder) addPosition(unitID, code, title, rankID, system string) error {
	id, err := s.ins("position", `
		INSERT INTO oikumenea.membership_positions (unit_id, code, title, required_rank_id)
		VALUES ($1,$2,$3,$4) RETURNING id`, unitID, code, title, nullable(rankID))
	if err != nil {
		return err
	}
	dir.positions = append(dir.positions, openPosition{id: id, unitID: unitID, rankID: rankID, system: system, title: title})
	return nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *seeder) phaseADirectory() error {
	var err error
	if dir.militaryDomain, err = s.domainID("military"); err != nil {
		return err
	}
	if dir.universityDomain, err = s.domainID("university"); err != nil {
		return err
	}
	if dir.companyDomain, err = s.domainID("company"); err != nil {
		return err
	}
	if dir.churchDomain, err = s.domainID("church"); err != nil {
		return err
	}

	// --- military brigades ---
	brigades := []struct{ code, name string }{
		{"79-oadbr", "79th Air Assault Brigade"},
		{"30-ombr", "30th Mechanized Brigade"},
	}
	for _, bg := range brigades {
		orgID, cmd, err := s.createOrg(bg.code, bg.name, dir.militaryDomain)
		if err != nil {
			return err
		}
		if err := s.buildBrigade(orgID, cmd, bg.name); err != nil {
			return err
		}
		if err := s.rebuildClosure(cmd); err != nil {
			return err
		}
	}

	// --- university (civilian tree) ---
	uniOrg, uniCmd, err := s.createOrg("khnu", "Kharkiv National University", dir.universityDomain)
	if err != nil {
		return err
	}
	dir.universityOrgID = uniOrg
	if err := s.buildUniversity(uniOrg, uniCmd); err != nil {
		return err
	}
	if err := s.rebuildClosure(uniCmd); err != nil {
		return err
	}
	// university org-profile
	var uniKind string
	_ = s.scalar(&uniKind, `SELECT id::text FROM oikumenea.education_institution_kinds WHERE code='university' LIMIT 1`)
	if uniKind != "" {
		if err := s.exec("edu_profile", `INSERT INTO oikumenea.education_org_profiles (institution_id, kind_id) VALUES ($1,$2)`, uniOrg, uniKind); err != nil {
			return err
		}
	}

	// --- company satellite (a bank, for finance) ---
	coOrg, coCmd, err := s.createOrg("privatbank", "PrivatBank JSC", dir.companyDomain)
	if err != nil {
		return err
	}
	dir.companyOrgID = coOrg
	_ = coCmd // single-unit company: no edges/closure needed (no subtree grants over it)
	if _, err := s.createUnit(coOrg, dir.companyDomain, "", "Head Office"); err != nil {
		return err
	}
	var legalForm string
	_ = s.scalar(&legalForm, `SELECT id::text FROM oikumenea.company_legal_forms WHERE code='jsc' LIMIT 1`)
	if legalForm != "" {
		if err := s.exec("co_profile", `INSERT INTO oikumenea.company_org_profiles (company_id, legal_form_id) VALUES ($1,$2)`, coOrg, legalForm); err != nil {
			return err
		}
	}

	// --- church satellite (for religion org profile + clergy) ---
	chOrg, _, err := s.createOrg("upc-parish", "St. Michael Parish", dir.churchDomain)
	if err != nil {
		return err
	}
	chUnit, err := s.createUnit(chOrg, dir.churchDomain, "", "St. Michael Parish")
	if err != nil {
		return err
	}
	dir.churchUnitID = chUnit
	if err := s.exec("rel_profile", `INSERT INTO oikumenea.religion_org_profiles (unit_id) VALUES ($1)`, chUnit); err != nil {
		return err
	}
	return nil
}

func (s *seeder) buildBrigade(orgID, cmd, bgName string) error {
	brKind, _ := s.militaryKind("brigade")
	btKind, _ := s.militaryKind("battalion")
	coKind, _ := s.militaryKind("company")
	plKind, _ := s.militaryKind("platoon")

	brig, err := s.createUnit(orgID, dir.militaryDomain, brKind, bgName+" HQ")
	if err != nil {
		return err
	}
	dir.militaryUnitIDs = append(dir.militaryUnitIDs, brig)
	if err := s.commanderPosition(brig, "brigade"); err != nil {
		return err
	}
	for b := 1; b <= 3; b++ {
		bat, err := s.createUnit(orgID, dir.militaryDomain, btKind, fmt.Sprintf("%d Battalion", b))
		if err != nil {
			return err
		}
		dir.militaryUnitIDs = append(dir.militaryUnitIDs, bat)
		if err := s.addEdge(cmd, brig, bat); err != nil {
			return err
		}
		if err := s.commanderPosition(bat, "battalion"); err != nil {
			return err
		}
		for c := 1; c <= 3; c++ {
			comp, err := s.createUnit(orgID, dir.militaryDomain, coKind, fmt.Sprintf("%d/%d Company", b, c))
			if err != nil {
				return err
			}
			dir.militaryUnitIDs = append(dir.militaryUnitIDs, comp)
			if err := s.addEdge(cmd, bat, comp); err != nil {
				return err
			}
			if err := s.commanderPosition(comp, "company"); err != nil {
				return err
			}
			for p := 1; p <= 3; p++ {
				plat, err := s.createUnit(orgID, dir.militaryDomain, plKind, fmt.Sprintf("%d/%d/%d Platoon", b, c, p))
				if err != nil {
					return err
				}
				dir.militaryUnitIDs = append(dir.militaryUnitIDs, plat)
				if err := s.addEdge(cmd, comp, plat); err != nil {
					return err
				}
				if err := s.commanderPosition(plat, "platoon"); err != nil {
					return err
				}
				// two rifleman billets per platoon
				for r := 1; r <= 2; r++ {
					rc := s.pick(rifleRanks)
					rid, err := s.rankID(rc)
					if err != nil {
						return err
					}
					if err := s.addPosition(plat, fmt.Sprintf("rifle-%d", r), "Rifleman", rid, s.uaSystemID); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (s *seeder) commanderPosition(unitID, echelon string) error {
	rid, err := s.rankID(echelonRank[echelon])
	if err != nil {
		return err
	}
	title := map[string]string{"brigade": "Brigade Commander", "battalion": "Battalion Commander",
		"company": "Company Commander", "platoon": "Platoon Leader"}[echelon]
	return s.addPosition(unitID, echelon+"-cmd", title, rid, s.uaSystemID)
}

func (s *seeder) buildUniversity(orgID, cmd string) error {
	facKind, _ := s.militaryKind2(dir.universityDomain, "faculty")
	depKind, _ := s.militaryKind2(dir.universityDomain, "department")
	uni, err := s.createUnit(orgID, dir.universityDomain, "", "Kharkiv National University")
	if err != nil {
		return err
	}
	if err := s.addPosition(uni, "rector", "Rector", "", ""); err != nil {
		return err
	}
	for f := 1; f <= 3; f++ {
		fac, err := s.createUnit(orgID, dir.universityDomain, facKind, fmt.Sprintf("Faculty %d", f))
		if err != nil {
			return err
		}
		if err := s.addEdge(cmd, uni, fac); err != nil {
			return err
		}
		if err := s.addPosition(fac, fmt.Sprintf("dean-%d", f), "Dean", "", ""); err != nil {
			return err
		}
		for d := 1; d <= 2; d++ {
			dep, err := s.createUnit(orgID, dir.universityDomain, depKind, fmt.Sprintf("Department %d.%d", f, d))
			if err != nil {
				return err
			}
			if err := s.addEdge(cmd, fac, dep); err != nil {
				return err
			}
			dir.studentUnitIDs = append(dir.studentUnitIDs, dep)
			if err := s.addPosition(dep, fmt.Sprintf("head-%d-%d", f, d), "Head of Department", "", ""); err != nil {
				return err
			}
			if err := s.addPosition(dep, fmt.Sprintf("lect-%d-%d", f, d), "Lecturer", "", ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// militaryKind2 resolves a unit_kind for an arbitrary domain (reused for university kinds).
func (s *seeder) militaryKind2(domainID, code string) (string, error) {
	var id string
	err := s.scalar(&id, `SELECT id::text FROM oikumenea.tenant_unit_kinds WHERE domain_id=$1 AND code=$2`, domainID, code)
	return id, err
}
