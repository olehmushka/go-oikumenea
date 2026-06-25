package application

import (
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): an order item's subject becomes the canonical person after a provisional stub is merged away.
// Runs in the merge transaction.
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.order_order_items SET person_id = $2 WHERE person_id = $1`,
	)
}
