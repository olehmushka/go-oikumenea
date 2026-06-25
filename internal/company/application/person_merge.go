package application

import (
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): company appointments + UBO beneficiary rows (real person_id FK) and the polymorphic person-holder
// founding/shareholding rows (holder_id text, holder_kind='person') follow the person to the canonical
// record after a provisional stub is merged away. Runs in the merge transaction.
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.company_appointments SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.company_beneficiaries SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.company_foundings SET holder_id = $2 WHERE holder_id = $1 AND holder_kind = 'person'`,
		`UPDATE oikumenea.company_shareholdings SET holder_id = $2 WHERE holder_id = $1 AND holder_kind = 'person'`,
	)
}
