package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestPinaxAutoseedDefault covers the D-Pinax opt-out gate: an absent pinax.autoseed defaults to true
// (a fresh install seeds the world), an explicit false skips boot autoseed, and an explicit true keeps
// it on. The pointer field is what makes "absent" distinguishable from "false".
func TestPinaxAutoseedDefault(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want bool
	}{
		{"absent section defaults on", ``, true},
		{"empty section defaults on", "pinax: {}", true},
		{"explicit true", "pinax:\n  autoseed: true", true},
		{"explicit false opts out", "pinax:\n  autoseed: false", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var install Install
			if err := yaml.Unmarshal([]byte(tc.yaml), &install); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := install.Pinax.AutoseedEnabled(); got != tc.want {
				t.Fatalf("AutoseedEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}
