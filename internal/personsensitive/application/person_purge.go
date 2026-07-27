// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonPurge erases this module's sensitive person-owned rows on a PersonPurged
// (D-PersonModuleSplit, review-2026-07 R-09): when the person core purges a person it publishes
// PersonPurged, and personsensitive erases (hard-delete or crypto-erase) that person's physical identity,
// ethnicity, party membership, watchlist/sanctions and M35 overlays in the SAME purge transaction. It
// runs the erases through the module's repository over the publisher's tx (no per-row audit — the
// person.purge action already records the purge).
func (s *Service) SubscribePersonPurge(bus *events.Bus) {
	bus.Subscribe(personevents.TypePersonPurged, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(personevents.PersonPurged)
		if !ok {
			return nil
		}
		return s.newRepo(tx).ErasePerson(ctx, e.ID)
	})
}
