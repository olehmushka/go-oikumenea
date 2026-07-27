// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// SubscribePersonEvents re-homes this module's person references on a PersonMerged (D-OverlayFoundation,
// M29): a subject's role assignments + instance-admin grants follow them to the canonical person after a
// provisional stub is merged away. Runs in the merge transaction. (A provisional stub holds no grants;
// this exists so the substrate is complete.)
func (s *Service) SubscribePersonEvents(bus *events.Bus) {
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.authz_role_assignments SET subject_person_id = $2 WHERE subject_person_id = $1`,
		`UPDATE oikumenea.authz_instance_admins SET person_id = $2 WHERE person_id = $1`,
		// Repointing subjects mutates authority — bump the revocation epoch in the merge tx
		// (D-AuthzGrantCache). SubscribeRepoint binds ($1=old,$2=new) to every statement; the
		// always-true-on-a-real-merge predicate just consumes them.
		`UPDATE oikumenea.authz_epoch SET epoch = epoch + 1 WHERE singleton AND $1::uuid IS DISTINCT FROM $2::uuid`,
	)
}
