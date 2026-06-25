package application

import (
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): clergy credentials (subject + conferring person), the lay-affiliation link (the encrypted belief
// value is unchanged — only the person_id moves), and an org-policy decider follow the person to the
// canonical record after a provisional stub is merged away. Runs in the merge transaction.
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.religion_clergy_credentials SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.religion_clergy_credentials SET conferred_by_person_id = $2 WHERE conferred_by_person_id = $1`,
		`UPDATE oikumenea.religion_affiliations SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.religion_org_policies SET decided_by_person_id = $2 WHERE decided_by_person_id = $1`,
	)
}
