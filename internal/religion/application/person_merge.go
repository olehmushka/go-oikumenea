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
// M29): clergy credentials (subject + conferring person), the lay-affiliation link (the encrypted belief
// value is unchanged — only the person_id moves), and an org-policy decider follow the person to the
// canonical record after a provisional stub is merged away. Runs in the merge transaction.
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.religion_clergy_credentials SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.religion_clergy_credentials SET conferred_by_person_id = $2 WHERE conferred_by_person_id = $1`,
		`UPDATE oikumenea.religion_affiliations SET person_id = $2 WHERE person_id = $1`,
		`UPDATE oikumenea.religion_org_policies SET decided_by_person_id = $2 WHERE decided_by_person_id = $1`,
	)
}

// SubscribePersonPurge crypto-erases this module's person affiliations on a PersonPurged
// (D-PersonModuleSplit, review-2026-07 R-09): a purged person's encrypted lay-affiliation belief values
// are crypto-erased (rows kept as tombstones) in the purge transaction. The clergy-credential subject and
// the conferrer/decider references are NOT erased (retained like an audit tombstone — the person id stays
// resolvable-or-redacted), matching the module's designed purge scope. It crypto-erases + writes an audit
// row, so it subscribes to TypePersonPurged directly rather than via SubscribeErase.
func (s *Service) SubscribePersonPurge(bus *events.Bus) {
	bus.Subscribe(personevents.TypePersonPurged, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(personevents.PersonPurged)
		if !ok {
			return nil
		}
		_, err := s.erasePersonAffiliationsTx(ctx, tx, e.ID)
		return err
	})
}
