// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestAcceptedAudiencesMergesBothSpellings locks the two accepted YAML spellings onto one set. The
// scalar `audience` is the shipped spelling (every existing install.yml uses it) and `audiences` was
// added for public IdPs that serve several clients of the same deployment; silently honouring only
// one of them would either break existing configs or make the multi-client case unconfigurable.
func TestAcceptedAudiencesMergesBothSpellings(t *testing.T) {
	cases := []struct {
		name string
		yml  string
		want []string
	}{
		{
			"scalar only (the shipped spelling)",
			"issuer: https://idp.example\naudience: oikumenea\n",
			[]string{"oikumenea"},
		},
		{
			"list only",
			"issuer: https://idp.example\naudiences: [console, cli]\n",
			[]string{"console", "cli"},
		},
		{
			"both merge, scalar first",
			"issuer: https://idp.example\naudience: console\naudiences: [cli]\n",
			[]string{"console", "cli"},
		},
		{
			"duplicates collapse",
			"issuer: https://idp.example\naudience: console\naudiences: [console, cli, cli]\n",
			[]string{"console", "cli"},
		},
		{
			"neither yields an empty set (GuardIssuerAudience refuses this for oidc at boot)",
			"issuer: https://idp.example\n",
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got Issuer
			if err := yaml.Unmarshal([]byte(tc.yml), &got); err != nil {
				t.Fatalf("unmarshal issuer: %v", err)
			}
			auds := got.AcceptedAudiences()
			if len(auds) != len(tc.want) {
				t.Fatalf("accepted audiences = %v, want %v", auds, tc.want)
			}
			for i, a := range tc.want {
				if auds[i] != a {
					t.Fatalf("accepted audiences = %v, want %v", auds, tc.want)
				}
			}
		})
	}
}

// TestIssuerLabelIsOptional keeps the display label off the identity path: it is cosmetic config for
// binding UIs, so its absence must never change how an issuer is parsed or matched.
func TestIssuerLabelIsOptional(t *testing.T) {
	var got Issuer
	if err := yaml.Unmarshal([]byte("issuer: https://idp.example\naudience: a\n"), &got); err != nil {
		t.Fatalf("unmarshal issuer: %v", err)
	}
	if got.Label != "" {
		t.Fatalf("label defaulted to %q, want empty", got.Label)
	}
	if got.Issuer != "https://idp.example" {
		t.Fatalf("issuer mis-parsed: %q", got.Issuer)
	}
}
