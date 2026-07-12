package rid_test

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

// TestOntologyMappingCitesOnlyRealTypes is the markdown-coherence check for R-28 (review-2026-09):
// docs/ontology-mapping.md is the binding Object/Link/Action catalog, and this turns that claim into
// a checked invariant — every numeric RID triple the catalog cites (e.g. "(6,1,14)", "RID 14,1,9/10/11",
// "16,1,1..20") must resolve to a real type in the Go registry (pkg/rid, itself boot-asserted equal to
// the SQL seed). It catches a catalog that lingers a removed/renamed type — e.g. a pre-M41 education
// code like 14,1,1 left behind after M41/D-UnifiedOrgGraph folded institutions onto the tenant graph.
//
// Direction: doc -> Go. We deliberately do NOT assert the reverse (every Go type is documented): the
// catalog describes most core types by name in prose rather than by numeric RID (only ~80 of ~140
// carry a numeric triple; catalogs like taxon_rank / vehicle_brand are grouped under a parent row),
// so a Go->doc numeric check would flag dozens of false positives. doc->Go is the deterministic half.
func TestOntologyMappingCitesOnlyRealTypes(t *testing.T) {
	const docPath = "../../docs/ontology-mapping.md"
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}

	// Real (service, kind, type) triples the Go registry declares (objects + links; no action rows).
	real := map[[3]int]bool{}
	for _, ti := range rid.Types() {
		real[[3]int{ti.Service, ti.Kind, ti.Code}] = true
	}

	// Triples the catalog names only to document their deliberate ABSENCE — not drift.
	allowedAbsent := map[[3]int]bool{
		// M41 / D-UnifiedOrgGraph: a company IS a company-domain tenant organization (RID 4,1,6); it has
		// no own company object RID. ontology-mapping.md cites "no own 15,1,1" to say exactly that.
		{15, 1, 1}: true,
	}

	// Tolerant extractor over prose: "S,K,C" where C is a single code, a "/"-list (9/10/11), or a
	// "..range" (16..20). Restricted to object/link kinds (1,2); action citations (kind 3) are by name.
	tripleRe := regexp.MustCompile(`(\d+)\s*,\s*(\d+)\s*,\s*(\d+(?:\.\.\d+)?(?:/\d+)*)`)
	var offenders []string
	for _, m := range tripleRe.FindAllStringSubmatch(string(data), -1) {
		svc, _ := strconv.Atoi(m[1])
		kind, _ := strconv.Atoi(m[2])
		if kind != int(rid.KindObject) && kind != int(rid.KindLink) {
			continue
		}
		for _, code := range expandCodes(m[3]) {
			key := [3]int{svc, kind, code}
			if real[key] || allowedAbsent[key] {
				continue
			}
			offenders = append(offenders, m[0])
		}
	}

	if len(offenders) > 0 {
		t.Errorf("ontology-mapping.md cites %d RID triple(s) with no matching type in pkg/rid "+
			"(removed/renamed type left in the catalog, or a typo): %s",
			len(offenders), strings.Join(dedupe(offenders), ", "))
	}
}

// expandCodes parses a code-spec: "14" -> [14]; "9/10/11" -> [9,10,11]; "16..20" -> [16..20].
func expandCodes(spec string) []int {
	if strings.Contains(spec, "..") {
		parts := strings.SplitN(spec, "..", 2)
		lo, _ := strconv.Atoi(parts[0])
		hi, _ := strconv.Atoi(parts[1])
		if hi < lo || hi-lo > 100 { // guard against a stray large range from a false match
			return nil
		}
		out := make([]int, 0, hi-lo+1)
		for c := lo; c <= hi; c++ {
			out = append(out, c)
		}
		return out
	}
	var out []int
	for _, p := range strings.Split(spec, "/") {
		if c, err := strconv.Atoi(p); err == nil {
			out = append(out, c)
		}
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
