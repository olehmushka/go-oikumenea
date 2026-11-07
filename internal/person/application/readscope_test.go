// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Unit tests for the person read-scope projection seam (D-PersonReadScope). Since review-2026-07
// R-02.1 the intersection logic lives in SQL (membership's SubjectCanReadPerson /
// VisiblePersonIDsForSubject, verified by the reach differential integration test); what remains
// here is the application-layer contract: the input guard (empty subject → nothing readable, never
// an error), faithful delegation, and the boot-time seam assertion (review-2026-07 R-11). The
// instance-admin short-circuit moved to the transport (pep.SubjectAuthority).
package application_test

import (
	"context"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/person/application"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// fakeMembership answers the reach point probe from a canned subject→person allow map.
type fakeMembership struct {
	allow map[string]map[string]bool // subject → person → readable
}

func (f fakeMembership) ActiveUnitIDsForPerson(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (f fakeMembership) VisiblePersonIDsForSubject(_ context.Context, _ string, _ string, _ domain.PersonFilter, _ int) ([]string, error) {
	return nil, nil
}

func (f fakeMembership) VisiblePersonStatsForSubject(_ context.Context, _ string, _ domain.PersonFilter, _ stats.Selection) ([]stats.Group, error) {
	return nil, nil
}

func (f fakeMembership) SubjectCanReadPerson(_ context.Context, subjectPersonID, personID string) (bool, error) {
	return f.allow[subjectPersonID][personID], nil
}

func (f fakeMembership) SubjectReadablePersonsAmong(_ context.Context, subjectPersonID string, personIDs []string) ([]string, error) {
	var out []string
	for _, id := range personIDs {
		if f.allow[subjectPersonID][id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// TestReadablePersonsAmong pins the batch probe the relationship lists trim counterparts with (M60
// ticket 0): it returns exactly the in-reach subset, and an empty subject reads nobody — even when the
// membership seam would have answered for "".
func TestReadablePersonsAmong(t *testing.T) {
	svc := application.NewService(nil, nil, nil, func() int { return 0 })
	svc.SetMembershipReader(fakeMembership{allow: map[string]map[string]bool{
		"reader-A": {"p-1": true, "p-3": true},
		"":         {"p-1": true},
	}})
	got, err := svc.ReadablePersonsAmong(context.Background(), "reader-A", []string{"p-1", "p-2", "p-3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "p-1" || got[1] != "p-3" {
		t.Errorf("reader-A among [p-1 p-2 p-3] = %v, want [p-1 p-3]", got)
	}
	if got, _ := svc.ReadablePersonsAmong(context.Background(), "", []string{"p-1"}); len(got) != 0 {
		t.Errorf("empty subject read %v, want nothing", got)
	}
}

func TestReadablePerson(t *testing.T) {
	svc := application.NewService(nil, nil, nil, func() int { return 0 })
	svc.SetMembershipReader(fakeMembership{allow: map[string]map[string]bool{
		"reader-A": {"p-inA": true}, // reach covers p-inA's unit
	}})

	cases := []struct {
		name    string
		subject string
		person  string
		want    bool
	}{
		{"reader sees a person in a readable unit", "reader-A", "p-inA", true},
		{"reader cannot see a person outside reach", "reader-A", "p-inB", false},
		{"unknown subject sees nobody", "reader-B", "p-inA", false},
		{"empty subject sees nobody", "", "p-inA", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.ReadablePerson(context.Background(), tc.subject, tc.person)
			if err != nil {
				t.Fatalf("ReadablePerson: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ReadablePerson(%s, %s) = %v, want %v", tc.subject, tc.person, got, tc.want)
			}
		})
	}
}

// TestMustBeBound covers the composition-root boot contract (review-2026-07 R-11): the mandatory
// cross-module seams must be wired before serving, so an unbound service fails MustBeBound (which the
// root turns into a boot failure) rather than surfacing a request-time nil deref or a silently-empty
// read-scope page that would read as "no access".
func TestMustBeBound(t *testing.T) {
	svc := application.NewService(nil, nil, nil, func() int { return 0 })
	if err := svc.MustBeBound(); err == nil {
		t.Fatal("MustBeBound must fail before the seams are wired")
	}
	svc.SetMembershipReader(fakeMembership{})
	if err := svc.MustBeBound(); err != nil {
		t.Fatalf("MustBeBound after wiring all seams: %v", err)
	}
}
