// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

func sptr(s string) *string { return &s }
func dptr(s string) *time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return &d
}

// testRID is a well-formed RID literal (UUIDv8 carrying our app code) for the shape checks; any
// valid RID works, so this reuses one of pkg/rid's own canonical test literals.
const testRID = "00000000-0000-8101-860a-000000000000"

// TestTestRIDIsValid keeps the fixture honest: if the RID shape ever changes, every "rejects a
// non-RID" case below would still pass while the "accepts a RID" cases quietly stopped proving
// anything. This fails first and says why.
func TestTestRIDIsValid(t *testing.T) {
	if !rid.IsRID(testRID) {
		t.Fatalf("fixture %q is no longer a valid RID — update testRID", testRID)
	}
}

func TestPersonFilterValidateAccepts(t *testing.T) {
	r := testRID
	f := PersonFilter{
		Query:          "ivan",
		Sex:            sptr("female"),
		Status:         sptr("active"),
		BirthdateFrom:  dptr("1980-01-01"),
		BirthdateTo:    dptr("2000-12-31"),
		CountryOfBirth: sptr(r),
		RankID:         sptr(r),
		UnitID:         sptr(r),
		Graph:          "command",
		HasAccount:     func() *bool { b := true; return &b }(),
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("a fully-populated valid filter was rejected: %v", err)
	}
	if f.IsZero() {
		t.Error("a populated filter reports IsZero")
	}
	if !(PersonFilter{}).IsZero() {
		t.Error("the empty filter does not report IsZero")
	}
	if err := (PersonFilter{}).Validate(); err != nil {
		t.Errorf("the empty filter was rejected: %v", err)
	}
	// Equal bounds are a single-day window, not an inversion.
	if err := (PersonFilter{BirthdateFrom: dptr("1990-05-05"), BirthdateTo: dptr("1990-05-05")}).Validate(); err != nil {
		t.Errorf("equal birthdate bounds rejected: %v", err)
	}
}

func TestPersonFilterValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		f    PersonFilter
		want string
	}{
		{"unknown sex", PersonFilter{Sex: sptr("f")}, "sex must be one of"},
		{"unknown status", PersonFilter{Status: sptr("retired")}, "status must be one of"},
		{"country not a rid", PersonFilter{CountryOfBirth: sptr("UA")}, "countryOfBirth must be a RID"},
		{"rank not a rid", PersonFilter{RankID: sptr("sergeant")}, "rankId must be a RID"},
		{"unit not a rid", PersonFilter{UnitID: sptr("1st-brigade")}, "unitId must be a RID"},
		{"inverted birthdate window", PersonFilter{BirthdateFrom: dptr("2000-01-01"), BirthdateTo: dptr("1990-01-01")}, "birthdateTo precedes"},
		{"graph without unit", PersonFilter{Graph: "command"}, "meaningless without unitId"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.f.Validate()
			if err == nil {
				t.Fatal("accepted an invalid filter")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("error does not wrap ErrInvalid: %v", err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not mention %q", err, c.want)
			}
		})
	}
	// graph WITH unitId is fine — the pairing rule is one-directional.
	if err := (PersonFilter{UnitID: sptr(testRID), Graph: "operational"}).Validate(); err != nil {
		t.Errorf("graph alongside unitId rejected: %v", err)
	}
}

// TestEnumValuesComeFromTheCatalog pins the single-source contract: Validate accepts exactly the
// facet catalog's declared Values, so adding a status to the DDL without adding it to the catalog
// cannot silently leave the filter rejecting a legal row.
func TestEnumValuesComeFromTheCatalog(t *testing.T) {
	o, ok := facet.Default.Get("person")
	if !ok {
		t.Fatal("person facets are not registered")
	}
	checked := 0
	for _, f := range o.Facets {
		if f.Kind != facet.KindEnum {
			continue
		}
		for _, v := range f.Values {
			val := v
			var pf PersonFilter
			switch f.Key {
			case "sex":
				pf.Sex = &val
			case "status":
				pf.Status = &val
			default:
				t.Fatalf("unhandled enum facet %q — extend this test alongside the catalog", f.Key)
			}
			if err := pf.Validate(); err != nil {
				t.Errorf("catalog value %s=%q rejected by Validate: %v", f.Key, val, err)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no enum facet values were checked — the catalog lookup is broken")
	}
}
