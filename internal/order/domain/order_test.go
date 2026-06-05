package domain

import (
	"errors"
	"testing"
)

func TestValidCategoryAndEffect(t *testing.T) {
	for _, c := range []string{"personnel-list", "appointment", "leave-travel", "discipline-incentive", "duty-roster"} {
		if !ValidCategory(c) {
			t.Errorf("ValidCategory(%q) = false, want true", c)
		}
	}
	if ValidCategory("nonsense") {
		t.Error("ValidCategory(nonsense) = true, want false")
	}
	for _, e := range []string{"membership-start", "membership-end", "rank-change", "record-only"} {
		if !ValidEffect(e) {
			t.Errorf("ValidEffect(%q) = false, want true", e)
		}
	}
	if ValidEffect("nonsense") {
		t.Error("ValidEffect(nonsense) = true, want false")
	}
}

func TestOrderTypeValidate(t *testing.T) {
	ok := OrderType{Code: "appoint", Name: "Appointment", Category: CategoryAppointment, Effect: EffectMembershipStart}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid type rejected: %v", err)
	}
	for _, bad := range []OrderType{
		{Name: "x", Category: CategoryAppointment, Effect: EffectMembershipStart}, // no code
		{Code: "x", Category: CategoryAppointment, Effect: EffectMembershipStart}, // no name
		{Code: "x", Name: "x", Category: "bogus", Effect: EffectMembershipStart},  // bad category
		{Code: "x", Name: "x", Category: CategoryAppointment, Effect: "bogus"},    // bad effect
	} {
		if err := bad.Validate(); !errors.Is(err, ErrOrderInvalid) {
			t.Errorf("Validate(%+v) = %v, want ErrOrderInvalid", bad, err)
		}
	}
}

func TestRequiredTargetsPresent(t *testing.T) {
	cases := []struct {
		name   string
		effect OrderEffect
		item   OrderItem
		want   bool
	}{
		{"start with unit", EffectMembershipStart, OrderItem{UnitID: "u"}, true},
		{"start with position", EffectMembershipStart, OrderItem{PositionID: "p"}, true},
		{"start with neither", EffectMembershipStart, OrderItem{}, false},
		{"end with unit", EffectMembershipEnd, OrderItem{UnitID: "u"}, true},
		{"end with neither", EffectMembershipEnd, OrderItem{}, false},
		{"rank with rank", EffectRankChange, OrderItem{RankID: "r"}, true},
		{"rank without rank", EffectRankChange, OrderItem{}, false},
		{"record-only needs nothing", EffectRecordOnly, OrderItem{}, true},
	}
	for _, c := range cases {
		if got := RequiredTargetsPresent(c.effect, c.item); got != c.want {
			t.Errorf("%s: RequiredTargetsPresent = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOrderTypePatchValidate(t *testing.T) {
	bad := "bogus"
	if err := (OrderTypePatch{Status: &bad}).Validate(); !errors.Is(err, ErrOrderInvalid) {
		t.Errorf("patch with bad status = %v, want ErrOrderInvalid", err)
	}
	ok := "retired"
	if err := (OrderTypePatch{Status: &ok}).Validate(); err != nil {
		t.Errorf("patch with valid status rejected: %v", err)
	}
}
