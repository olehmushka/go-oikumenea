// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/audit/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/olehmushka/go-oikumenea/pkg/action"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

type stubRepo struct{ inserted bool }

func (r *stubRepo) Insert(context.Context, domain.Entry) error                   { r.inserted = true; return nil }
func (r *stubRepo) Get(context.Context, string) (domain.Entry, error)            { return domain.Entry{}, nil }
func (r *stubRepo) Query(context.Context, domain.Filter) ([]domain.Entry, error) { return nil, nil }
func (r *stubRepo) EnsureCurrentPartitions(context.Context) error                { return nil }
func (r *stubRepo) Stats(context.Context, domain.Filter, stats.Selection) ([]stats.Group, error) {
	return nil, nil
}

// a well-formed action RID (kind nibble = 3) so domain.Validate passes and the action-type gate runs.
const actionRID = "00000000-0000-8300-8000-000000000000"

func newEntry(actionCode string) domain.Entry {
	return domain.Entry{
		ID:         actionRID,
		ActorType:  domain.ActorSystem,
		Subsystem:  "test",
		Action:     actionCode,
		TargetType: "unit",
		RequestID:  "req-1",
		Outcome:    domain.OutcomeSuccess,
	}
}

// TestRecordRejectsUnregisteredAction is R-29's acceptance: an audited write whose action is not in
// the pkg/action catalog fails (before touching the repo), while a catalogued action reaches insert.
func TestRecordRejectsUnregisteredAction(t *testing.T) {
	var repo stubRepo
	s := NewService(nil, func(db.DBTX) domain.Repository { return &repo }, func() int { return 50 })

	err := s.Record(context.Background(), nil, newEntry("bogus.action.nope"))
	if !errors.Is(err, action.ErrUnregistered) {
		t.Fatalf("unregistered action must fail with ErrUnregistered, got %v", err)
	}
	if repo.inserted {
		t.Fatalf("insert must not be reached for an unregistered action")
	}

	if err := s.Record(context.Background(), nil, newEntry("unit.create")); err != nil {
		t.Fatalf("registered action unit.create should pass: %v", err)
	}
	if !repo.inserted {
		t.Fatalf("registered action should reach insert")
	}
}
