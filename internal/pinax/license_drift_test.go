// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package pinax

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The pinax presets are go:embed-ed into the binary, so shipping oikumenea ships THEIR data under
// THEIR terms — two of which (ODbL-1.0, CC-BY-SA-4.0) are share-alike and reach further than the
// repository's Apache-2.0 LICENSE. docs/reference/data-licenses.md is what tells a redistributor
// that; these guards keep it honest when a preset is added, retired, or relicensed upstream.

const (
	dataLicensesDoc = "../../docs/reference/data-licenses.md"
	noticeFile      = "../../NOTICE"
)

var licenseField = regexp.MustCompile(`(?m)^license:\s*(.+?)\s*$`)

// Every embedded preset must declare a license and be named in the data-licenses doc — otherwise a
// redistributor cannot discover an obligation that binds them.
func TestPresetLicensesAreDocumented(t *testing.T) {
	doc, err := os.ReadFile(dataLicensesDoc)
	if err != nil {
		t.Fatalf("reading %s: %v", dataLicensesDoc, err)
	}
	text := string(doc)

	entries, err := presetFS.ReadDir("presets")
	if err != nil {
		t.Fatalf("reading embedded presets: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded presets found — the guard would pass vacuously")
	}

	for _, e := range entries {
		name := e.Name()
		raw, err := presetFS.ReadFile("presets/" + name)
		if err != nil {
			t.Fatalf("reading preset %s: %v", name, err)
		}
		m := licenseField.FindSubmatch(raw)
		if m == nil {
			t.Errorf("preset %s declares no `license:` field — every bundled dataset must state its terms", name)
			continue
		}
		license := strings.TrimSpace(string(m[1]))

		if !strings.Contains(text, name) {
			t.Errorf("preset %s is not listed in %s — add its row before shipping it", name, dataLicensesDoc)
		}
		// The SPDX id itself must appear too, so relicensing upstream cannot slip through with the
		// filename row left stale.
		if !strings.Contains(text, license) {
			t.Errorf("preset %s declares license %q, which does not appear in %s — the doc is stale",
				name, license, dataLicensesDoc)
		}
	}
}

// Share-alike terms bind a redistributor beyond attribution, so they must be spelled out in prose,
// not left to be inferred from an SPDX string in a table cell.
func TestShareAlikeLicensesAreCalledOut(t *testing.T) {
	doc, err := os.ReadFile(dataLicensesDoc)
	if err != nil {
		t.Fatalf("reading %s: %v", dataLicensesDoc, err)
	}
	text := string(doc)

	shareAlike := []string{"ODbL-1.0", "CC-BY-SA-4.0"}
	inUse := map[string]bool{}
	entries, _ := presetFS.ReadDir("presets")
	for _, e := range entries {
		raw, _ := presetFS.ReadFile("presets/" + e.Name())
		if m := licenseField.FindSubmatch(raw); m != nil {
			inUse[strings.TrimSpace(string(m[1]))] = true
		}
	}
	for _, l := range shareAlike {
		if !inUse[l] {
			continue // not bundled (any more) — nothing to explain
		}
		if !strings.Contains(text, "share alike") && !strings.Contains(text, "share-alike") {
			t.Errorf("%s is bundled but %s never explains the share-alike obligation in prose", l, dataLicensesDoc)
		}
	}
}

// CC-BY and CC-BY-SA both require attribution, and Apache-2.0 propagates NOTICE to redistributors —
// so NOTICE is where the required credit has to live.
func TestAttributionRequiringSourcesAreInNotice(t *testing.T) {
	notice, err := os.ReadFile(noticeFile)
	if err != nil {
		t.Fatalf("reading %s: %v", noticeFile, err)
	}
	text := string(notice)
	for _, must := range []string{"Glottolog", "CLDR", "mledoze", "Natural Earth", "Factbook"} {
		if !strings.Contains(text, must) {
			t.Errorf("NOTICE is missing required attribution for %q", must)
		}
	}
}
