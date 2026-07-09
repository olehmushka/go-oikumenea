package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): a vehicle registration owned by the person (polymorphic owner_id text, owner_kind='person')
// follows them to the canonical record after a provisional stub is merged away. Runs in the merge
// transaction.
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.vehicle_registrations SET owner_id = $2 WHERE owner_id = $1 AND owner_kind = 'person'`,
	)
}

// SubscribePersonPurge erases this module's person registrations on a PersonPurged (D-PersonModuleSplit,
// review-2026-07 R-09): a purged person's owned registrations are soft-deleted in the purge transaction.
// It writes an audit row, so it subscribes to TypePersonPurged directly rather than via SubscribeErase.
func (s *Service) SubscribePersonPurge(bus *events.Bus) {
	bus.Subscribe(personevents.TypePersonPurged, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(personevents.PersonPurged)
		if !ok {
			return nil
		}
		_, err := s.erasePersonRegistrationsTx(ctx, tx, e.ID)
		return err
	})
}
