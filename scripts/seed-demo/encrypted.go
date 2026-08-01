// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
)

// phaseEEncrypted fills the envelope-encrypted tables. Each secret value is sealed with the app's
// own cipher (built from the dev keys in install.yml) into the table's four envelope columns
// (ciphertext / wrapped_dek / key_ref / blind_index), so the rows decrypt in-app.
func (s *seeder) phaseEEncrypted() error {
	all := s.allPersons()
	const lb = "explicit_consent" // GDPR Art.9 basis for special-category demo data

	// ---- document_personal_codes (national tax number) ----
	for i, p := range all {
		if i%5 != 0 {
			continue
		}
		ct, dek, kr, bi, err := s.seal(fmt.Sprintf("%010d", s.rng.Intn(1_000_000_000)))
		if err != nil {
			return err
		}
		if err := s.exec("personal_code", `INSERT INTO oikumenea.document_personal_codes
			(person_id, scheme_code, value_ciphertext, wrapped_dek, key_ref, value_blind_index)
			VALUES ($1,'ua-rnokpp',$2,$3,$4,$5)`, p.id, ct, dek, kr, bi); err != nil {
			return err
		}
	}

	// ---- finance: accounts (at the demo bank) + holders + cards ----
	//
	// The account-type and card-network catalogs are seeded by pinax but were never REFERENCED here,
	// so every demo account and card carried a NULL account_type_id / network_id. That is invisible
	// until something groups by them: the M58 dashboards then draw a single (unknown) bar, and the
	// bucket→filter click-through those facets exist for is never exercised, because (unknown) is
	// deliberately not a filterable value. Same shape as the vehicle colour gap next door.
	bank := dir.companyOrgID
	accountTypes := s.catCodes("finance_account_types")
	cardNetworks := s.catCodes("finance_card_networks")
	for i, p := range all {
		if i%10 != 0 { // ~30 account holders
			continue
		}
		iban := fmt.Sprintf("UA%02d3052990000026%09d", 10+s.rng.Intn(89), s.rng.Intn(1_000_000_000))
		ct, dek, kr, bi, err := s.seal(iban)
		if err != nil {
			return err
		}
		// Left NULL for a slice of rows on purpose: the (unknown) bucket is a real part of the
		// distribution, and a seed where every row is populated hides the case where it is not.
		var acctType any
		if len(accountTypes) > 0 && s.chance(0.85) {
			acctType = accountTypes[s.rng.Intn(len(accountTypes))]
		}
		acc, err := s.ins("finance_account", `INSERT INTO oikumenea.finance_accounts
			(institution_id, iban_ciphertext, iban_wrapped_dek, key_ref, iban_blind_index, currency, account_type_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`, bank, ct, dek, kr, bi,
			s.pick([]string{"UAH", "UAH", "UAH", "USD", "EUR"}), acctType)
		if err != nil {
			return err
		}
		if err := s.exec("finance_account", `INSERT INTO oikumenea.finance_account_holders (account_id, holder_kind, holder_id) VALUES ($1,'person',$2)`, acc, p.id); err != nil {
			return err
		}
		// a card on the account
		pan := fmt.Sprintf("4149%012d", s.rng.Int63n(1_000_000_000_000))
		pct, pdek, pkr, pbi, err := s.seal(pan)
		if err != nil {
			return err
		}
		var network any
		if len(cardNetworks) > 0 && s.chance(0.9) {
			network = cardNetworks[s.rng.Intn(len(cardNetworks))]
		}
		if err := s.exec("finance_card", `INSERT INTO oikumenea.finance_cards
			(account_id, pan_ciphertext, pan_wrapped_dek, key_ref, pan_blind_index, bin, last_four, card_type, network_id, cardholder_person_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, acc, pct, pdek, pkr, pbi, pan[:6], pan[len(pan)-4:], s.pick([]string{"debit", "credit"}), network, p.id); err != nil {
			return err
		}
	}

	// ---- person_health_records ----
	for i, p := range all {
		if i%20 != 0 {
			continue
		}
		ct, dek, kr, bi, err := s.seal(s.pick([]string{"Field hospital admission, 2023", "Rehabilitation, right leg", "Category B fitness"}))
		if err != nil {
			return err
		}
		if err := s.exec("health", `INSERT INTO oikumenea.person_health_records
			(person_id, kind, detail_ciphertext, detail_wrapped_dek, detail_key_ref, detail_blind_index, legal_basis)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, p.id, s.pick([]string{"hospitalization", "disability", "mental_health"}), ct, dek, kr, bi, lb); err != nil {
			return err
		}
	}

	// ---- person_legal_records ----
	for i, p := range all {
		if i%25 != 0 {
			continue
		}
		ct, dek, kr, bi, err := s.seal(s.pick([]string{"Administrative fine, traffic", "Case dismissed", "Pending investigation"}))
		if err != nil {
			return err
		}
		if err := s.exec("legal", `INSERT INTO oikumenea.person_legal_records
			(person_id, kind, disposition, detail_ciphertext, detail_wrapped_dek, detail_key_ref, detail_blind_index, legal_basis)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, p.id,
			s.pick([]string{"arrest", "court_judgment", "criminal_conviction"}),
			s.pick([]string{"convicted", "acquitted", "dismissed", "pending"}), ct, dek, kr, bi, lb); err != nil {
			return err
		}
	}

	// ---- person_political_leaning (signed decimal in [-1,1]) ----
	for i, p := range all {
		if i%15 != 0 {
			continue
		}
		ct, dek, kr, bi, err := s.seal(fmt.Sprintf("%.2f", s.rng.Float64()*2-1))
		if err != nil {
			return err
		}
		if err := s.exec("political", `INSERT INTO oikumenea.person_political_leaning
			(person_id, leaning_ciphertext, leaning_wrapped_dek, leaning_key_ref, leaning_blind_index, legal_basis)
			VALUES ($1,$2,$3,$4,$5,$6)`, p.id, ct, dek, kr, bi, lb); err != nil {
			return err
		}
	}

	// ---- person_party_memberships ----
	for i, p := range all {
		if i%18 != 0 {
			continue
		}
		ct, dek, kr, bi, err := s.seal(s.pick([]string{"Servant of the People", "European Solidarity", "Batkivshchyna", "Independent"}))
		if err != nil {
			return err
		}
		if err := s.exec("party", `INSERT INTO oikumenea.person_party_memberships
			(person_id, party_ciphertext, party_wrapped_dek, party_key_ref, party_blind_index, legal_basis)
			VALUES ($1,$2,$3,$4,$5,$6)`, p.id, ct, dek, kr, bi, lb); err != nil {
			return err
		}
	}

	// ---- person_ethnicities ----
	for i, p := range all {
		if i%6 != 0 {
			continue
		}
		ct, dek, kr, bi, err := s.seal(s.pick([]string{"Ukrainian", "Russian", "Crimean Tatar", "Romanian", "Hungarian", "Polish"}))
		if err != nil {
			return err
		}
		if err := s.exec("ethnicity", `INSERT INTO oikumenea.person_ethnicities
			(person_id, value_ciphertext, wrapped_dek, key_ref, value_blind_index, legal_basis)
			VALUES ($1,$2,$3,$4,$5,$6)`, p.id, ct, dek, kr, bi, lb); err != nil {
			return err
		}
	}

	// ---- religion_affiliations: enrich a subset of the plaintext rows from Phase D with a sealed value ----
	rows, err := s.tx.Query(s.ctx, `
		SELECT ra.id::text FROM oikumenea.religion_affiliations ra
		JOIN oikumenea.person_persons p ON p.id=ra.person_id
		WHERE p.attributes->>'seed'='demo' AND ra.value_ciphertext IS NULL
		ORDER BY ra.id LIMIT 40`)
	if err != nil {
		return err
	}
	var affIDs []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		affIDs = append(affIDs, id)
	}
	rows.Close()
	for _, id := range affIDs {
		ct, dek, kr, bi, err := s.seal(s.pick([]string{"Orthodox", "Greek Catholic", "Roman Catholic", "Protestant", "Muslim", "Jewish"}))
		if err != nil {
			return err
		}
		if err := s.exec("affiliation_sealed", `UPDATE oikumenea.religion_affiliations
			SET value_ciphertext=$2, wrapped_dek=$3, key_ref=$4, value_blind_index=$5 WHERE id=$1`, id, ct, dek, kr, bi); err != nil {
			return err
		}
	}
	return nil
}
