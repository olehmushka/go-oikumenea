// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package personevents holds the person module's domain events (D-OverlayFoundation, M29). Today the
// only event is PersonMerged: when a provisional stub person is merged into a canonical person, the
// person module re-homes its OWN edges and then publishes PersonMerged so every other module that
// references a person by RID can re-home its rows IN THE SAME TRANSACTION (the architecture's
// cross-module-mutation-via-events rule — keeps the monolith extraction-ready).
//
// This package is a leaf: it imports only pkg/events (for the Event interface) and the standard
// library, so the producer (person/application) and the consumers (every person-referencing module's
// application) can depend on it without an import cycle — those modules never import person, and the
// event references the two persons by their string RIDs only.
package personevents

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olehmushka/go-oikumenea/pkg/events"
)

// TypePersonMerged is the events.Event.Type() dispatch key for a provisional→canonical merge.
const TypePersonMerged = "person.merged"

// PersonMerged is emitted in the MergePerson transaction (D-OverlayFoundation): the provisional stub
// FromID has been merged into the canonical IntoID, so every subscriber must re-point its
// person-referencing rows FromID → IntoID. Confidence carries the operator's certainty about the
// identity equation (confirmed|probable|possible) for the audit trail; subscribers may ignore it.
//
// Collision note: a provisional person is a minimal-PII stub, so unique-constraint collisions on
// re-point (e.g. a duplicate membership in one unit) are not expected. Subscribers do a plain
// re-point UPDATE and do NOT de-duplicate — surfacing/merging colliding rows is a parked seam
// (matches the decision's "fuzzy dedup is parked").
type PersonMerged struct {
	FromID     string // the provisional stub being merged away (tombstoned 'purged' after this event)
	IntoID     string // the surviving canonical person
	Confidence string // confirmed | probable | possible
}

// Type implements events.Event.
func (PersonMerged) Type() string { return TypePersonMerged }

// compile-time assertion that PersonMerged satisfies the bus event contract.
var _ events.Event = PersonMerged{}

// SubscribeRepoint registers a PersonMerged handler that re-homes a module's person-referencing rows in
// the publisher's transaction: it runs every stmt with ($1 = FromID, $2 = IntoID). Each stmt is a plain
// re-point UPDATE of one person column (e.g. `UPDATE … SET person_id = $2 WHERE person_id = $1`); for a
// polymorphic holder add `AND <kind> = 'person'`. The convenience the cross-module subscribers use so
// each module owns its own statements while sharing the dispatch + transaction plumbing.
func SubscribeRepoint(bus *events.Bus, stmts ...string) {
	bus.Subscribe(TypePersonMerged, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(PersonMerged)
		if !ok {
			return nil
		}
		for _, q := range stmts {
			if _, err := tx.Exec(ctx, q, e.FromID, e.IntoID); err != nil {
				return err
			}
		}
		return nil
	})
}

// TypePersonPurged is the events.Event.Type() dispatch key for a person hard-erase.
const TypePersonPurged = "person.purged"

// PersonPurged is emitted in the PurgePerson transaction (D-PersonModuleSplit, review-2026-07 R-09):
// the person ID has been hard-erased (PII NULLed, id kept as a tombstone), so every module that holds
// the person's rows erases its OWN rows in the SAME transaction — the counterpart to PersonMerged's
// re-point. This removes person's cross-module purge writes: instead of person deleting other modules'
// tables inline, each owning module subscribes and erases what it owns (satisfies the R-08 module-table
// boundary). It is published only on a real purge, NOT on the merge-tombstone Purge of a stub (whose
// rows were already re-pointed away by PersonMerged).
type PersonPurged struct {
	ID string // the person being hard-erased
}

// Type implements events.Event.
func (PersonPurged) Type() string { return TypePersonPurged }

// compile-time assertion that PersonPurged satisfies the bus event contract.
var _ events.Event = PersonPurged{}

// SubscribeErase registers a PersonPurged handler that hard-erases a module's person-owned rows in the
// publisher's transaction: it runs every stmt with ($1 = ID). Each stmt is a plain
// `DELETE FROM … WHERE person_id = $1` (for a polymorphic holder, `WHERE holder_kind = 'person' AND
// holder_id = $1`). The counterpart to SubscribeRepoint: each module owns its own erase statements while
// sharing the dispatch + transaction plumbing. Modules whose erase is more than a raw DELETE (crypto-erase,
// audit) subscribe to TypePersonPurged directly instead.
func SubscribeErase(bus *events.Bus, stmts ...string) {
	bus.Subscribe(TypePersonPurged, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(PersonPurged)
		if !ok {
			return nil
		}
		for _, q := range stmts {
			if _, err := tx.Exec(ctx, q, e.ID); err != nil {
				return err
			}
		}
		return nil
	})
}
