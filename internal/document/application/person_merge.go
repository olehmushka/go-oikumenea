// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/jackc/pgx/v5"
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

// SubscribePersonPurge erases this module's person records on a PersonPurged (D-PersonModuleSplit,
// review-2026-07 R-09): a purged person's documents are NULLed and their personal codes crypto-erased in
// the purge transaction. Distinct from the raw-SQL SubscribeErase used by education/company because the
// erase crypto-erases + writes a correlated audit row, so it subscribes to TypePersonPurged directly.
func (s *Service) SubscribePersonPurge(bus *events.Bus) {
	bus.Subscribe(personevents.TypePersonPurged, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(personevents.PersonPurged)
		if !ok {
			return nil
		}
		_, _, err := s.erasePersonRecordsTx(ctx, tx, e.ID)
		return err
	})
}
