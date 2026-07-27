// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonPurge erases this module's person-owned directory rows on a PersonPurged
// (D-PersonModuleSplit, review-2026-07 R-09): when the person core purges a person (PII scrubbed to a
// status=purged tombstone) it publishes PersonPurged, and personprofile hard-deletes all of that
// person's citizenships/residences/addresses/contact-channels/languages/relationships/institutional ties
// in the SAME purge transaction. It runs raw erases (no per-row audit — the person.purge action already
// records the purge), so it subscribes to the erase via the module's repository over the publisher's tx.
func (s *Service) SubscribePersonPurge(bus *events.Bus) {
	bus.Subscribe(personevents.TypePersonPurged, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(personevents.PersonPurged)
		if !ok {
			return nil
		}
		return s.newRepo(tx).ErasePerson(ctx, e.ID)
	})
}
