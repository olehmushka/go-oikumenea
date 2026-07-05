package application

import (
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): an account-holder edge owned by the person (polymorphic holder_id text, holder_kind='person')
// follows them to the canonical record after a provisional stub is merged away; a named cardholder does
// likewise. Runs in the merge transaction.
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.finance_account_holders SET holder_id = $2 WHERE holder_id = $1 AND holder_kind = 'person'`,
		`UPDATE oikumenea.finance_cards SET cardholder_person_id = $2::uuid WHERE cardholder_person_id = $1::uuid`,
	)
}
