// Unit tests for the person read-scope projection (D-PersonReadScope). These exercise the pure
// decision logic of ReadablePerson with a fake membership reader and a constructed effective reach —
// no database — so the leak-closing rule (F-001) is verified directly: a reader sees a person only
// when on the instance plane or when the person's active-membership units intersect the reader's
// effective readable units.
package application_test

import (
	"context"
	"testing"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/person/application"
)

// fakeMembership maps a person id to its active-membership unit ids.
type fakeMembership struct {
	units map[string][]string
}

func (f fakeMembership) ActiveUnitIDsForPerson(_ context.Context, personID string) ([]string, error) {
	return f.units[personID], nil
}

func (f fakeMembership) PersonIDsWithActiveMembershipInUnits(_ context.Context, _ []string, _ string, _ int) ([]string, error) {
	return nil, nil
}

// reach builds an effective reach: instance-admin or a set of readable unit ids.
func reach(admin bool, readable ...string) authzdomain.Reach {
	r := authzdomain.Reach{InstanceAdmin: admin, Readable: map[string]struct{}{}}
	for _, u := range readable {
		r.Readable[u] = struct{}{}
	}
	return r
}

func TestReadablePerson(t *testing.T) {
	svc := application.NewService(nil, nil, nil, func() int { return 0 })
	svc.SetMembershipReader(fakeMembership{units: map[string][]string{
		"p-inA":  {"unit-A"},
		"p-inB":  {"unit-B"},
		"p-none": nil, // membership-less
	}})

	cases := []struct {
		name   string
		reach  authzdomain.Reach
		person string
		want   bool
	}{
		{"instance admin sees anyone", reach(true), "p-inB", true},
		{"instance admin sees a membership-less person", reach(true), "p-none", true},
		{"reader sees a person in a readable unit", reach(false, "unit-A"), "p-inA", true},
		{"reader cannot see a person outside reach", reach(false, "unit-A"), "p-inB", false},
		{"reader cannot see a membership-less person", reach(false, "unit-A"), "p-none", false},
		{"empty reach sees nobody", reach(false), "p-inA", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.ReadablePerson(context.Background(), tc.reach, tc.person)
			if err != nil {
				t.Fatalf("ReadablePerson: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ReadablePerson(%s) = %v, want %v", tc.person, got, tc.want)
			}
		})
	}
}

// TestReadablePerson_NoMembershipReader covers the pre-binding / safety case: without a membership
// reader, only an instance admin may read.
func TestReadablePerson_NoMembershipReader(t *testing.T) {
	svc := application.NewService(nil, nil, nil, func() int { return 0 })
	if ok, _ := svc.ReadablePerson(context.Background(), reach(true), "p"); !ok {
		t.Fatal("instance admin must be readable even without a membership reader")
	}
	if ok, _ := svc.ReadablePerson(context.Background(), reach(false, "unit-A"), "p"); ok {
		t.Fatal("non-admin must not be readable when no membership reader is bound")
	}
}
