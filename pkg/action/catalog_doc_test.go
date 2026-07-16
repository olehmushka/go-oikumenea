package action

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

// TestCatalogMatchesDoc is the R-29 coherence gate (same spirit as R-28's ontology-doc test): the
// action-catalog table in docs/ontology-mapping.md §3.1 must equal the pkg/action registry exactly.
// The doc block is regenerated from pkg/action; this test fails if a row drifts, an action is added
// to the registry without the doc (or vice versa), or any field disagrees.
func TestCatalogMatchesDoc(t *testing.T) {
	const docPath = "../../docs/ontology-mapping.md"
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	body := string(raw)
	begin := strings.Index(body, "<!-- BEGIN action-catalog")
	end := strings.Index(body, "<!-- END action-catalog")
	if begin < 0 || end < 0 || end < begin {
		t.Fatalf("action-catalog markers not found in %s", docPath)
	}
	rowRE := regexp.MustCompile(`^\|\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|\s*([^|]+?)\s*\|\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|$`)
	docRows := map[string][4]string{}
	for _, line := range strings.Split(body[begin:end], "\n") {
		m := rowRE.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		docRows[m[1]] = [4]string{m[1], m[2], m[3], m[4]}
	}

	names := rid.Services()
	regRows := map[string][4]string{}
	for _, a := range All() {
		regRows[a.Code] = [4]string{a.Code, names[a.Service], a.TargetType, a.Permission}
	}

	if len(docRows) != len(regRows) {
		t.Fatalf("catalog size mismatch: doc=%d registry=%d (regenerate the §3.1 table from pkg/action)", len(docRows), len(regRows))
	}
	for code, reg := range regRows {
		doc, ok := docRows[code]
		if !ok {
			t.Errorf("action %q in registry but not in §3.1 doc table", code)
			continue
		}
		if doc != reg {
			t.Errorf("action %q disagrees: doc=%v registry=%v", code, doc, reg)
		}
	}
	for code := range docRows {
		if _, ok := regRows[code]; !ok {
			t.Errorf("action %q in §3.1 doc table but not in registry", code)
		}
	}
}
