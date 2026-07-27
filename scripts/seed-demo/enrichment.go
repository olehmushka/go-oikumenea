// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"
)

// allPersons returns appointed + relatives.
func (s *seeder) allPersons() []person {
	return append(append([]person{}, s.appointed...), s.relatives...)
}

// catID resolves a catalog row id by code (first match).
func (s *seeder) catID(table, code string) (string, error) {
	var id string
	err := s.scalar(&id, fmt.Sprintf(`SELECT id::text FROM oikumenea.%s WHERE code=$1 ORDER BY id LIMIT 1`, table), code)
	if err != nil {
		return "", fmt.Errorf("%s code=%s: %w", table, code, err)
	}
	return id, nil
}

func (s *seeder) phaseDEnrichment() error {
	all := s.allPersons()
	// resolve catalog ids we reference
	passportType, err := s.catID("document_document_types", "passport")
	if err != nil {
		return err
	}
	militaryIDType, _ := s.catID("document_document_types", "military-id")
	carType, err := s.catID("vehicle_types", "car")
	if err != nil {
		return err
	}
	eyeBrown, _ := s.catID("platform_colors", "brown")
	hairBlack, _ := s.catID("platform_colors", "black")
	affMember, err := s.catID("religion_affiliation_types", "member")
	if err != nil {
		return err
	}
	clergyGrade, _ := s.catID("religion_clergy_grades", "presbyter")
	regScheme, _ := s.catID("company_registration_schemes", "ua-edrpou")

	// ---- D1 contacts ----
	var phoneIDs []string
	for _, p := range all {
		if s.chance(0.85) {
			id, err := s.ins("email", `INSERT INTO oikumenea.person_emails (person_id, type_code, address) VALUES ($1,$2,$3::citext) RETURNING id`,
				p.id, s.pick([]string{"personal", "work"}), fmt.Sprintf("user%d@example.ua", s.rng.Intn(1_000_000)))
			_ = id
			if err != nil {
				return err
			}
		}
		if s.chance(0.9) {
			id, err := s.ins("phone", `INSERT INTO oikumenea.person_phones (person_id, type_code, number) VALUES ($1,$2,$3) RETURNING id`,
				p.id, s.pick([]string{"mobile", "home", "work"}), fmt.Sprintf("+3806%d", 10_000_000+s.rng.Intn(89_999_999)))
			if err != nil {
				return err
			}
			if len(phoneIDs) < 60 {
				phoneIDs = append(phoneIDs, id)
			}
		}
		if s.chance(0.5) {
			if _, err := s.ins("social", `INSERT INTO oikumenea.person_social_accounts (person_id, platform_code, handle, source) VALUES ($1,$2,$3,'self_declared') RETURNING id`,
				p.id, s.pick([]string{"telegram", "facebook", "x", "instagram"}), fmt.Sprintf("@user%d", s.rng.Intn(1_000_000))); err != nil {
				return err
			}
		}
	}
	// call signs for military appointees
	for i, p := range s.appointed {
		if i < 85 && s.chance(0.5) {
			if err := s.exec("call_sign", `INSERT INTO oikumenea.person_call_signs (person_id, call_sign) VALUES ($1,$2)`,
				p.id, s.pick([]string{"Sokil", "Vovk", "Berkut", "Tygr", "Hrim", "Vlad", "Skif", "Kobra"})+fmt.Sprintf("-%d", s.rng.Intn(99))); err != nil {
				return err
			}
		}
	}
	// messenger links hang off a phone (no person_id column)
	for _, ph := range phoneIDs {
		if s.chance(0.4) {
			if err := s.exec("messenger", `INSERT INTO oikumenea.person_messenger_links (phone_id, platform_code) VALUES ($1,$2)`,
				ph, s.pick([]string{"telegram", "viber", "whatsapp", "signal"})); err != nil {
				return err
			}
		}
	}

	// ---- D2 documents ----
	for i, p := range all {
		if s.chance(0.7) {
			if err := s.exec("document", `INSERT INTO oikumenea.document_documents (person_id, type_id, number, issuing_country_id) VALUES ($1,$2,$3,$4)`,
				p.id, passportType, fmt.Sprintf("FC%06d", s.rng.Intn(1_000_000)), s.countryUA); err != nil {
				return err
			}
		}
		if militaryIDType != "" && i < 85 && s.chance(0.6) {
			if err := s.exec("document", `INSERT INTO oikumenea.document_documents (person_id, type_id, number, issuing_country_id) VALUES ($1,$2,$3,$4)`,
				p.id, militaryIDType, fmt.Sprintf("MI%06d", s.rng.Intn(1_000_000)), s.countryUA); err != nil {
				return err
			}
		}
	}

	// ---- D3 person basics ----
	for _, p := range all {
		if s.chance(0.6) {
			// eng transliteration variant
			if err := s.exec("name_variant", `INSERT INTO oikumenea.person_name_variants (person_id, locale, display_name)
				SELECT $1,'eng', display_name FROM oikumenea.person_persons WHERE id=$1 ON CONFLICT DO NOTHING`, p.id); err != nil {
				return err
			}
		}
		if s.chance(0.95) {
			if err := s.exec("citizenship", `INSERT INTO oikumenea.person_citizenships (person_id, country_id, is_primary) VALUES ($1,$2,true) ON CONFLICT DO NOTHING`, p.id, s.countryUA); err != nil {
				return err
			}
		}
		if s.chance(0.85) {
			if err := s.exec("residence", `INSERT INTO oikumenea.person_residences (person_id, country_id, valid_from) VALUES ($1,$2,$3)`,
				p.id, s.countryUA, time.Now().AddDate(-s.rng.Intn(10), 0, 0).Format("2006-01-02")); err != nil {
				return err
			}
		}
		if s.chance(0.9) {
			if err := s.exec("language", `INSERT INTO oikumenea.person_languages (person_id, language_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, p.id, s.langUA); err != nil {
				return err
			}
		}
	}

	// ---- D4 addresses (locations first) ----
	var locIDs []string
	for i := 0; i < 40; i++ {
		lon := 22.0 + s.rng.Float64()*18.0 // roughly Ukraine bbox
		lat := 44.0 + s.rng.Float64()*8.0
		id, err := s.ins("location", `INSERT INTO oikumenea.location_locations (geom, country_id, locality, street, house_number, raw_address)
			VALUES (ST_SetSRID(ST_MakePoint($1,$2),4326), $3, $4, $5, $6, 'DEMO') RETURNING id`,
			lon, lat, s.countryUA, s.pick([]string{"Kyiv", "Kharkiv", "Lviv", "Odesa", "Dnipro"}),
			s.pick([]string{"Soborna", "Shevchenka", "Franka", "Lesi Ukrainky"})+" St", fmt.Sprintf("%d", 1+s.rng.Intn(120)))
		if err != nil {
			return err
		}
		locIDs = append(locIDs, id)
	}
	for _, p := range all {
		if s.chance(0.55) {
			if err := s.exec("address", `INSERT INTO oikumenea.person_addresses (person_id, location_id, role) VALUES ($1,$2,$3)`,
				p.id, locIDs[s.rng.Intn(len(locIDs))], s.pick([]string{"home", "mailing"})); err != nil {
				return err
			}
		}
	}

	// ---- D5 vehicles ----
	brands := []struct{ code, name string }{{"demo-toyota", "Toyota"}, {"demo-vw", "Volkswagen"}, {"demo-skoda", "Skoda"}, {"demo-bmw", "BMW"}, {"demo-zaz", "ZAZ"}}
	var modelIDs []string
	for _, b := range brands {
		bid, err := s.ins("vehicle_brand", `INSERT INTO oikumenea.vehicle_brands (code, name) VALUES ($1,$2) RETURNING id`, b.code, b.name)
		if err != nil {
			return err
		}
		for m := 1; m <= 2; m++ {
			mid, err := s.ins("vehicle_model", `INSERT INTO oikumenea.vehicle_models (brand_id, code, name) VALUES ($1,$2,$3) RETURNING id`,
				bid, fmt.Sprintf("demo-%s-m%d", b.code, m), b.name+fmt.Sprintf(" Model %d", m))
			if err != nil {
				return err
			}
			modelIDs = append(modelIDs, mid)
		}
	}
	for _, p := range all {
		if s.chance(0.3) {
			vid, err := s.ins("vehicle", `INSERT INTO oikumenea.vehicle_vehicles (type_id, model_id, vin, attributes) VALUES ($1,$2,$3,'{"seed":"demo"}') RETURNING id`,
				carType, modelIDs[s.rng.Intn(len(modelIDs))], fmt.Sprintf("DEMOVIN%08d", s.rng.Intn(100_000_000)))
			if err != nil {
				return err
			}
			if err := s.exec("registration", `INSERT INTO oikumenea.vehicle_registrations (vehicle_id, owner_kind, owner_id, country_id, registration_number)
				VALUES ($1,'person',$2,$3,$4)`, vid, p.id, s.countryUA, fmt.Sprintf("AA%04d%s", s.rng.Intn(10000), s.pick([]string{"BX", "CE", "KA", "HH"}))); err != nil {
				return err
			}
		}
	}

	// ---- D6 education (university) ----
	uni := dir.universityOrgID
	prog, err := s.ins("edu_program", `INSERT INTO oikumenea.education_programs (institution_id, code, name) VALUES ($1,'demo-cs','Computer Science') RETURNING id`, uni)
	if err != nil {
		return err
	}
	_ = prog
	qual, err := s.ins("edu_qualification", `INSERT INTO oikumenea.education_qualifications (institution_id, code, name) VALUES ($1,'demo-bsc','BSc Computer Science') RETURNING id`, uni)
	if err != nil {
		return err
	}
	for _, u := range dir.studentUnitIDs {
		if _, err := s.ins("edu_group", `INSERT INTO oikumenea.education_groups (unit_id, code, name) VALUES ($1,$2,$3) RETURNING id`,
			u, fmt.Sprintf("demo-grp-%d", s.rng.Intn(1000)), "Group "+fmt.Sprintf("%d", 1+s.rng.Intn(5))); err != nil {
			return err
		}
	}
	epos, err := s.ins("edu_position", `INSERT INTO oikumenea.education_positions (institution_id, code, title) VALUES ($1,'demo-prof','Professor') RETURNING id`, uni)
	if err != nil {
		return err
	}
	// enroll ~40 persons as students; qualify some; appoint a couple staff
	for i, p := range all {
		if i%7 == 0 {
			if err := s.exec("enrollment", `INSERT INTO oikumenea.person_education_enrollments (person_id, institution_id, program_id, status) VALUES ($1,$2,$3,'enrolled')`,
				p.id, uni, prog); err != nil {
				return err
			}
			if s.chance(0.4) {
				if err := s.exec("qualification", `INSERT INTO oikumenea.person_education_qualifications (person_id, qualification_id) VALUES ($1,$2)`, p.id, qual); err != nil {
					return err
				}
			}
		}
	}
	for i := 0; i < 3 && i < len(all); i++ {
		if err := s.exec("edu_appointment", `INSERT INTO oikumenea.education_appointments (person_id, position_id, status) VALUES ($1,$2,'active') ON CONFLICT DO NOTHING`, all[i].id, epos); err != nil {
			return err
		}
	}

	// ---- D7 company ----
	co := dir.companyOrgID
	var coPositions []string
	for _, t := range []struct{ code, title string }{{"ceo", "Chief Executive Officer"}, {"cfo", "Chief Financial Officer"}, {"teller", "Bank Teller"}} {
		id, err := s.ins("company_position", `INSERT INTO oikumenea.company_positions (company_id, code, title) VALUES ($1,$2,$3) RETURNING id`, co, t.code, t.title)
		if err != nil {
			return err
		}
		coPositions = append(coPositions, id)
	}
	if regScheme != "" {
		if err := s.exec("company_reg", `INSERT INTO oikumenea.company_registrations (company_id, scheme_id, identifier) VALUES ($1,$2,$3)`, co, regScheme, "14360570"); err != nil {
			return err
		}
	}
	for i, cp := range coPositions { // one distinct person per company billet
		if i < len(all) {
			if err := s.exec("company_appointment", `INSERT INTO oikumenea.company_appointments (person_id, position_id, status) VALUES ($1,$2,'active')`, all[i].id, cp); err != nil {
				return err
			}
		}
	}
	for i := 2; i < 6 && i < len(all); i++ {
		if err := s.exec("beneficiary", `INSERT INTO oikumenea.company_beneficiaries (company_id, person_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, co, all[i].id); err != nil {
			return err
		}
		if err := s.exec("shareholding", `INSERT INTO oikumenea.company_shareholdings (company_id, holder_kind, holder_id) VALUES ($1,'person',$2)`, co, all[i].id); err != nil {
			return err
		}
	}

	// ---- D8 religion ----
	for _, p := range all {
		if s.chance(0.45) {
			if err := s.exec("affiliation", `INSERT INTO oikumenea.religion_affiliations (person_id, affiliation_type_id) VALUES ($1,$2)`, p.id, affMember); err != nil {
				return err
			}
		}
	}
	if clergyGrade != "" {
		for i := 0; i < 2 && i < len(s.appointed); i++ {
			if err := s.exec("clergy", `INSERT INTO oikumenea.religion_clergy_credentials (person_id, clergy_grade_id, org_unit_id) VALUES ($1,$2,$3)`,
				s.relatives[i].id, clergyGrade, dir.churchUnitID); err != nil {
				return err
			}
		}
	}

	// ---- D9 external orgs ----
	var extKind string
	_ = s.scalar(&extKind, `SELECT id::text FROM oikumenea.external_org_kinds WHERE code='ngo' LIMIT 1`)
	if extKind != "" {
		for _, n := range []string{"DEMO Veterans Foundation", "DEMO Red Cross Chapter", "DEMO Civic Watch"} {
			if err := s.exec("external_org", `INSERT INTO oikumenea.external_organizations (kind_id, name) VALUES ($1,$2)`, extKind, n); err != nil {
				return err
			}
		}
	}

	// ---- D10 overlays (plaintext) ----
	for i, p := range all {
		if s.chance(0.5) {
			if err := s.exec("physical", `INSERT INTO oikumenea.person_physical_descriptions (person_id, height_cm, weight_kg, build, eye_color_id, hair_color_id)
				VALUES ($1,$2,$3,$4,$5,$6)`, p.id, 160+s.rng.Intn(35), 55+s.rng.Intn(45), s.pick([]string{"slim", "average", "athletic", "heavy"}), nullable(eyeBrown), nullable(hairBlack)); err != nil {
				return err
			}
		}
		if s.chance(0.3) {
			if err := s.exec("mark", `INSERT INTO oikumenea.person_distinguishing_marks (person_id, kind) VALUES ($1,$2)`, p.id, s.pick([]string{"tattoo", "scar", "piercing", "birthmark"})); err != nil {
				return err
			}
		}
		if s.chance(0.3) {
			if err := s.exec("personality", `INSERT INTO oikumenea.person_personality (person_id, framework, result) VALUES ($1,'mbti',$2)`, p.id,
				s.pick([]string{"INTJ", "ENFP", "ISTP", "ESFJ", "INFP", "ESTP"})); err != nil {
				return err
			}
		}
		if i < 10 && s.chance(0.5) {
			if err := s.exec("gov_position", `INSERT INTO oikumenea.person_government_positions (person_id, title, body) VALUES ($1,$2,$3)`, p.id,
				"Deputy", s.pick([]string{"City Council", "Regional Council"})); err != nil {
				return err
			}
		}
		if s.chance(0.4) {
			if err := s.exec("insurance", `INSERT INTO oikumenea.person_insurance (person_id, type, provider) VALUES ($1,$2,$3)`, p.id,
				s.pick([]string{"health", "life", "disability"}), s.pick([]string{"Oranta", "ARX", "UNIQA"})); err != nil {
				return err
			}
		}
		if s.chance(0.1) {
			if err := s.exec("sanction", `INSERT INTO oikumenea.person_regulatory_sanctions (person_id, regulator) VALUES ($1,$2)`, p.id, s.pick([]string{"NBU", "SEC", "OFAC"})); err != nil {
				return err
			}
		}
		if s.chance(0.2) {
			if err := s.exec("wallet", `INSERT INTO oikumenea.person_crypto_wallets (person_id, address, chain) VALUES ($1,$2,$3)`, p.id,
				fmt.Sprintf("0x%040x", s.rng.Uint64()), s.pick([]string{"ethereum", "bitcoin", "tron"})); err != nil {
				return err
			}
		}
		if s.chance(0.1) {
			if err := s.exec("watchlist", `INSERT INTO oikumenea.person_watchlist_matches (person_id, on_list, lists, pep) VALUES ($1,true,$2,$3)`, p.id,
				[]string{s.pick([]string{"OFAC-SDN", "EU-CFSP", "INTERPOL"})}, s.chance(0.3)); err != nil {
				return err
			}
		}
	}
	return nil
}
