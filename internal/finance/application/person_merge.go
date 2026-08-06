// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	personevents "github.com/olehmushka/go-oikumenea/internal/person/events"
	"github.com/olehmushka/go-oikumenea/pkg/events"
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

// SubscribePersonPurge erases this module's person holdings on a PersonPurged (D-PersonModuleSplit,
// review-2026-07 R-09): the accounts (+ cards) the purged person SOLELY holds are crypto-erased and their
// holder edges soft-deleted in the purge transaction. Company-held/joint accounts survive. It crypto-erases
// + writes an audit row, so it subscribes to TypePersonPurged directly rather than via SubscribeErase.
func (s *Service) SubscribePersonPurge(bus *events.Bus) {
	bus.Subscribe(personevents.TypePersonPurged, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(personevents.PersonPurged)
		if !ok {
			return nil
		}
		_, err := s.erasePersonAccountsTx(ctx, tx, e.ID)
		return err
	})
}
