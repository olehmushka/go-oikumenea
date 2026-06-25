package application

import (
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): a vehicle registration owned by the person (polymorphic owner_id text, owner_kind='person')
// follows them to the canonical record after a provisional stub is merged away. Runs in the merge
// transaction.
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.vehicle_registrations SET owner_id = $2 WHERE owner_id = $1 AND owner_kind = 'person'`,
	)
}
