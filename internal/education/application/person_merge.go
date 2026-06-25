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
