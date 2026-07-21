package config

import (
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestModuleEnabledDefaults: an absent modules block (or an absent module key) leaves every vertical
// ON, and a non-toggleable core module is ALWAYS on regardless of config — a config typo cannot
// silently disable the core.
func TestModuleEnabledDefaults(t *testing.T) {
	var none Install
	if !none.ModuleEnabled("finance") {
		t.Fatal("finance should default enabled with no modules config")
	}
	if !none.ModuleEnabled("person") {
		t.Fatal("core module person must always be enabled")
	}

	// An explicit `enabled: false` for a core (non-toggleable) module is ignored — it stays on.
	var y Install
	if err := yaml.Unmarshal([]byte("modules:\n  person:\n    enabled: false\n"), &y); err != nil {
		t.Fatal(err)
	}
	if !y.ModuleEnabled("person") {
		t.Fatal("person is not toggleable; enabled:false must be ignored")
	}
}

// TestModuleEnabledExplicitFalse: a toggleable vertical set to enabled:false reads as off; another
// vertical left unset stays on.
func TestModuleEnabledExplicitFalse(t *testing.T) {
	var i Install
	if err := yaml.Unmarshal([]byte("modules:\n  finance:\n    enabled: false\n"), &i); err != nil {
		t.Fatal(err)
	}
	if i.ModuleEnabled("finance") {
		t.Fatal("finance explicitly disabled should read off")
	}
	if !i.ModuleEnabled("religion") {
		t.Fatal("religion unset should stay on")
	}
}

// TestDisabledModulePrefixes: the prefixes cover exactly the disabled verticals, and religion
// contributes BOTH of its code prefixes (religion. and religionorg.) so religionorg.* is also gated.
func TestDisabledModulePrefixes(t *testing.T) {
	var i Install
	if err := yaml.Unmarshal([]byte("modules:\n  finance:\n    enabled: false\n  religion:\n    enabled: false\n"), &i); err != nil {
		t.Fatal(err)
	}
	got := i.DisabledModulePrefixes()
	sort.Strings(got)
	want := []string{"finance.", "religion.", "religionorg."}
	if len(got) != len(want) {
		t.Fatalf("prefixes = %v, want %v", got, want)
	}
	for j := range want {
		if got[j] != want[j] {
			t.Fatalf("prefixes = %v, want %v", got, want)
		}
	}
	// An all-enabled install disables nothing.
	var on Install
	if len(on.DisabledModulePrefixes()) != 0 {
		t.Fatal("no modules disabled should yield no prefixes")
	}
}
