// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"
)

// newPerson inserts a person (tagged demo) with a name matching `male`, a birthdate in [minAge,maxAge].
func (s *seeder) newPerson(male bool, minAge, maxAge int) (person, error) {
	given, giv2, surname, display := s.makeName(male)
	age := minAge + s.rng.Intn(maxAge-minAge+1)
	bd := time.Now().AddDate(-age, -s.rng.Intn(12), -s.rng.Intn(28))
	id, err := s.ins("person", `
		INSERT INTO oikumenea.person_persons (display_name, given, given2, surname, sex, birthdate, country_of_birth_id, attributes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'{"seed":"demo"}') RETURNING id`,
		display, given, giv2, surname, sexOf(male), bd.Format("2006-01-02"), s.countryUA)
	if err != nil {
		return person{}, err
	}
	return person{id: id, sex: sexOf(male)}, nil
}

func (s *seeder) phaseBPersons() error {
	// Fill the first 100 open positions (military first, so most appointees are ranked). Military
	// positions carry a required_rank; the holder gets that exact rank (D-Rank).
	n := 100
	if len(dir.positions) < n {
		n = len(dir.positions)
	}
	for i := 0; i < n; i++ {
		pos := dir.positions[i]
		male := s.chance(0.85) // mostly men in the ranks; still some women
		p, err := s.newPerson(male, 20, 55)
		if err != nil {
			return err
		}
		s.appointed = append(s.appointed, p)
		// membership on the position's unit, filling the specific billet
		if err := s.exec("membership", `
			INSERT INTO oikumenea.membership_memberships (person_id, unit_id, position_id, status)
			VALUES ($1,$2,$3,'active')`, p.id, pos.unitID, pos.id); err != nil {
			return err
		}
		// rank: give the holder the position's required rank (military only)
		if pos.rankID != "" {
			if err := s.exec("rank", `
				INSERT INTO oikumenea.person_ranks (person_id, system_id, rank_id) VALUES ($1,$2,$3)`,
				p.id, pos.system, pos.rankID); err != nil {
				return err
			}
		}
	}
	return nil
}

// phaseCRelationships builds family clusters around the 100 appointed persons using exactly 200
// relatives: 60 spouses + 120 children + 20 parents, then sprinkles the other tie types.
func (s *seeder) phaseCRelationships() error {
	relCount := 0
	takeRel := func(male bool, minAge, maxAge int) (person, error) {
		p, err := s.newPerson(male, minAge, maxAge)
		if err == nil {
			s.relatives = append(s.relatives, p)
			relCount++
		}
		return p, err
	}

	type couple struct{ a, b person } // a = appointed, b = spouse
	var couples []couple

	// 60 marriages: appointed[0..59] each get an opposite-sex spouse. Male↔female only (seeding rule).
	for i := 0; i < 60 && i < len(s.appointed); i++ {
		ap := s.appointed[i]
		spouse, err := takeRel(ap.sex == "female", 20, 55) // opposite sex
		if err != nil {
			return err
		}
		if err := s.marry(ap, spouse); err != nil {
			return err
		}
		couples = append(couples, couple{ap, spouse})
	}

	// 120 children: 2 per couple, each a kinship child of BOTH parents.
	for _, c := range couples {
		for k := 0; k < 2 && relCount < 200; k++ {
			child, err := takeRel(s.chance(0.5), 1, 18)
			if err != nil {
				return err
			}
			if err := s.kinship(c.a.id, child.id); err != nil {
				return err
			}
			if err := s.kinship(c.b.id, child.id); err != nil {
				return err
			}
		}
	}

	// remaining relatives → parents of appointed persons (relative is the PARENT).
	for i := 0; relCount < 200 && i < len(s.appointed); i++ {
		parent, err := takeRel(s.chance(0.5), 45, 80)
		if err != nil {
			return err
		}
		if err := s.kinship(parent.id, s.appointed[i].id); err != nil {
			return err
		}
	}

	// --- sprinkle the other tie types (reuse existing persons) so every table has rows ---
	// guardianship: a parent guardian over a minor child (first ~15 couples' first child pattern)
	for i := 0; i < 15 && i < len(couples); i++ {
		if len(s.relatives) > i {
			if err := s.exec("guardianship", `
				INSERT INTO oikumenea.person_guardianships (guardian_id, ward_id) VALUES ($1,$2)`,
				couples[i].a.id, s.relatives[60+i].id); err != nil { // 60+ = a child
				return err
			}
		}
	}
	// next_of_kin: spouse nominated as NOK
	for i := 0; i < 40 && i < len(couples); i++ {
		if err := s.exec("next_of_kin", `
			INSERT INTO oikumenea.person_next_of_kin (subject_id, contact_id, priority) VALUES ($1,$2,1)`,
			couples[i].a.id, couples[i].b.id); err != nil {
			return err
		}
	}
	// sponsorship: a senior officer sponsors a junior (relation_code mandatory, category=sponsorship)
	for i := 0; i < 20 && i+1 < len(s.appointed); i += 2 {
		if err := s.exec("sponsorship", `
			INSERT INTO oikumenea.person_sponsorships (sponsor_id, sponsored_id, relation_code) VALUES ($1,$2,'military_mentor')`,
			s.appointed[i].id, s.appointed[i+1].id); err != nil {
			return err
		}
	}
	// association: colleagues among appointed persons (canonical pair)
	for i := 0; i < 30 && i+1 < len(s.appointed); i++ {
		a, b := s.appointed[i].id, s.appointed[i+1].id
		if err := s.exec("association", `
			INSERT INTO oikumenea.person_associations (person_id_a, person_id_b, kind)
			SELECT least($1::uuid,$2::uuid), greatest($1::uuid,$2::uuid), 'associate'
			WHERE least($1::uuid,$2::uuid) <> greatest($1::uuid,$2::uuid)
			ON CONFLICT DO NOTHING`, a, b); err != nil {
			return err
		}
	}
	return nil
}

// marry inserts a married partnership, canonicalizing the pair by uuid order (the *_canonical_pair CHECK).
func (s *seeder) marry(a, b person) error {
	return s.exec("marriage", `
		INSERT INTO oikumenea.person_partnerships (person_id_a, person_id_b, status, effective_from)
		SELECT least($1::uuid,$2::uuid), greatest($1::uuid,$2::uuid), 'married', $3
		WHERE least($1::uuid,$2::uuid) <> greatest($1::uuid,$2::uuid)`,
		a.id, b.id, time.Now().AddDate(-(1+s.rng.Intn(25)), 0, 0).Format("2006-01-02"))
}

func (s *seeder) kinship(parentID, childID string) error {
	if parentID == childID {
		return fmt.Errorf("kinship self-loop")
	}
	return s.exec("kinship", `
		INSERT INTO oikumenea.person_kinships (parent_id, child_id, status) VALUES ($1,$2,'active')`,
		parentID, childID)
}
