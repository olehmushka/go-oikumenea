package transport

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	rankapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/rank"
	"github.com/olegamysk/go-oikumenea/internal/rank/domain"
)

// mapErrorContract pins the HTTP classification of every rank domain error `mapError` translates. Like
// the person guard, this catches the class of bug where a catalog/FK sentinel is returned but missing
// from the switch and falls through to a generic 500 — the case here was ErrUnknownCountry on system
// upsert (resolved from a bad ISO code by CountryIDByCode), which used to 500 instead of 400.
var mapErrorContract = []struct {
	name string
	err  error
	want func(error) bool
}{
	{"ErrSystemNotFound", domain.ErrSystemNotFound, rankapi.IsRankSystemNotFound},
	{"ErrCategoryNotFound", domain.ErrCategoryNotFound, rankapi.IsRankCategoryNotFound},
	{"ErrTypeNotFound", domain.ErrTypeNotFound, rankapi.IsRankTypeNotFound},
	{"ErrRankNotFound", domain.ErrRankNotFound, rankapi.IsRankNotFound},
	{"ErrGradeNotFound", domain.ErrGradeNotFound, rankapi.IsRankInvalid}, // an unknown standardized grade is client input, not a 404
	{"ErrUnknownCountry", domain.ErrUnknownCountry, rankapi.IsRankInvalid},
	{"ErrInvalid", domain.ErrInvalid, rankapi.IsRankInvalid},
	{"ErrCodeConflict", domain.ErrCodeConflict, rankapi.IsRankCodeConflict},
	{"ErrInUse", domain.ErrInUse, rankapi.IsRankInUse},
}

// TestMapErrorClassifiesEverySentinel proves each contracted domain error maps to its typed Conjure
// error and NOT the `default:` 500 wrap.
func TestMapErrorClassifiesEverySentinel(t *testing.T) {
	ctx := context.Background()
	var svc Service // mapError is a value receiver and reads no Service fields
	for _, tc := range mapErrorContract {
		got := svc.mapError(ctx, tc.err, errCtx{})
		if !tc.want(got) {
			t.Errorf("mapError(%s) = %T (%v); want a typed 4xx Conjure error, not the 500 default", tc.name, got, got)
		}
	}
}

// TestMapErrorContractCoversAllCatalogAndNotFoundSentinels is the drift guard: every catalog/FK
// (`ErrUnknown*`) and lookup (`*NotFound`) sentinel defined in the rank domain package must appear in
// mapErrorContract (and is therefore verified above to map to a typed non-500 error).
func TestMapErrorContractCoversAllCatalogAndNotFoundSentinels(t *testing.T) {
	covered := make(map[string]bool, len(mapErrorContract))
	for _, tc := range mapErrorContract {
		covered[tc.name] = true
	}
	re := regexp.MustCompile(`(Err[A-Za-z]+)\s*=\s*errors\.New`)
	files, err := filepath.Glob("../domain/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no rank domain source files found (glob err=%v)", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			name := m[1]
			if !strings.HasSuffix(name, "NotFound") && !strings.HasPrefix(name, "ErrUnknown") {
				continue
			}
			if !covered[name] {
				t.Errorf("domain.%s is a catalog/FK/not-found sentinel but is absent from mapErrorContract — "+
					"add a mapError case for it and list it here, else it returns HTTP 500", name)
			}
		}
	}
}
