package application

import (
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): a person's identity documents + personal codes follow them to the canonical person after a
// provisional stub is merged away. Runs in the merge transaction.
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.document_documents SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.document_personal_codes SET person_id = $2 WHERE person_id = $1`,
	)
}
