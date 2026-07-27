// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): the education appointments + person bindings (enrollments, dorm stays) and the M20 reference-layer
// person links (authorships, research/governance memberships, grant holdings, qualifications, scholarship
// awards) follow the person to the canonical record after a provisional stub is merged away. Runs in the
// merge transaction.
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.education_appointments SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.person_education_enrollments SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.person_dormitory_stays SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.person_publication_authorships SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.person_research_memberships SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.person_grant_holdings SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.person_governance_memberships SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.person_education_qualifications SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.person_scholarship_awards SET person_id = $2 WHERE person_id = $1`,
	)
}

// SubscribePersonPurge hard-erases this module's person-owned rows on a PersonPurged (D-PersonModuleSplit,
// review-2026-07 R-09): the education person bindings (enrollments, dorm stays) and the M20 reference-layer
// person links (authorships, research/governance memberships, grant holdings, qualifications, scholarship
// awards) are erased when the person is purged. Runs in the purge transaction. These deletes previously
// lived inline in person's repo.Purge — moving them here removes person's cross-module writes (R-08).
// (education_appointments is intentionally NOT erased here — it matches the prior purge behavior, which
// re-homed appointments on merge but did not delete them on purge.)
func (s *Service) SubscribePersonPurge(bus *events.Bus) {
	personevents.SubscribeErase(bus,
		`DELETE FROM oikumenea.person_education_enrollments WHERE person_id = $1`,
		`DELETE FROM oikumenea.person_dormitory_stays WHERE person_id = $1`,
		`DELETE FROM oikumenea.person_publication_authorships WHERE person_id = $1`,
		`DELETE FROM oikumenea.person_research_memberships WHERE person_id = $1`,
		`DELETE FROM oikumenea.person_grant_holdings WHERE person_id = $1`,
		`DELETE FROM oikumenea.person_governance_memberships WHERE person_id = $1`,
		`DELETE FROM oikumenea.person_education_qualifications WHERE person_id = $1`,
		`DELETE FROM oikumenea.person_scholarship_awards WHERE person_id = $1`,
	)
}
