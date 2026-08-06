// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	personevents "github.com/olehmushka/go-oikumenea/internal/person/events"
	"github.com/olehmushka/go-oikumenea/pkg/events"
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

// SubscribePersonPurge hard-erases this module's person-link rows on a PersonPurged (D-PersonModuleSplit,
// review-2026-07 R-09): company appointments + UBO beneficiary rows (person_id FK) and the polymorphic
// person-holder founding/shareholding rows (holder_kind='person') are erased when the person is purged.
// Runs in the purge transaction. These deletes previously lived inline in person's repo.Purge — moving
// them here removes person's cross-module writes (the R-08 module-table boundary).
func (s *Service) SubscribePersonPurge(bus *events.Bus) {
	personevents.SubscribeErase(bus,
		`DELETE FROM oikumenea.company_appointments WHERE person_id = $1`,
		`DELETE FROM oikumenea.company_beneficiaries WHERE person_id = $1`,
		`DELETE FROM oikumenea.company_foundings WHERE holder_kind = 'person' AND holder_id = $1`,
		`DELETE FROM oikumenea.company_shareholdings WHERE holder_kind = 'person' AND holder_id = $1`,
	)
}
