// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	personevents "github.com/olehmushka/go-oikumenea/internal/person/events"
	"github.com/olehmushka/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): a membership belongs to the canonical person after a provisional stub is merged away. Runs in
// the merge transaction. Plain re-point; a (person, unit) collision is not de-duplicated (parked seam —
// a provisional stub holds no membership).
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.membership_memberships SET person_id = $2 WHERE person_id = $1`,
	)
}
