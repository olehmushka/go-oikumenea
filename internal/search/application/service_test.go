// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/authorization/scope"
	"github.com/olegamysk/go-oikumenea/internal/search/domain"
)

// fakeProvider serves `ids` in order, `limit` per call, keyset by numeric index encoded in the
// cursor — the same contract the real providers implement over their trigram queries.
func fakeProvider(objectType, perm string, ids []string) domain.Provider {
	return domain.Provider{
		ObjectType:     objectType,
		ReadPermission: perm,
		Search: func(_ context.Context, _ string, _ bool, _ string, after string, limit int) ([]domain.RawHit, string, error) {
			start := 0
			if after != "" {
				var err error
				if start, err = strconv.Atoi(after); err != nil {
					return nil, "", err
				}
			}
			var hits []domain.RawHit
			i := start
			for ; i < len(ids) && len(hits) < limit; i++ {
				hits = append(hits, domain.RawHit{ID: ids[i], Label: "L:" + ids[i]})
			}
			next := ""
			if i < len(ids) {
				next = strconv.Itoa(i)
			}
			return hits, next, nil
		},
	}
}

func allowAll(context.Context, string) (bool, error) { return true, nil }

func subjectAuthority(subject string, isAdmin bool) AuthorityFunc {
	return func(context.Context) (string, bool, error) { return subject, isAdmin, nil }
}

func newEngine(t *testing.T, allowed AllowedFunc, provs ...domain.Provider) *Service {
	t.Helper()
	s := NewService(subjectAuthority("subj", false), allowed)
	for _, p := range provs {
		if err := s.Register(p, scope.NewCatalogScope()); err != nil {
			t.Fatalf("register %s: %v", p.ObjectType, err)
		}
	}
	return s
}

func ids(hits []domain.Hit, objectType string) []string {
	var out []string
	for _, h := range hits {
		if objectType == "" || h.ObjectType == objectType {
			out = append(out, h.RID)
		}
	}
	return out
}

func TestRegisterValidation(t *testing.T) {
	s := NewService(subjectAuthority("subj", false), allowAll)
	if err := s.MustBeBound(); err == nil {
		t.Fatal("empty registry must fail MustBeBound (R-30 acceptance)")
	}
	if err := s.Register(fakeProvider("a", "a.read", nil), nil); err == nil {
		t.Fatal("provider without a visibility scope must fail composition (R-30 acceptance)")
	}
	if err := s.Register(fakeProvider("a", "a.read", nil), scope.NewCatalogScope()); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.Register(fakeProvider("a", "a.read", nil), scope.NewCatalogScope()); err == nil {
		t.Fatal("duplicate object type must fail composition")
	}
	if err := s.Register(domain.Provider{ObjectType: "b", ReadPermission: "b.read"}, scope.NewCatalogScope()); err == nil {
		t.Fatal("provider without a search func must fail composition")
	}
	if err := s.MustBeBound(); err != nil {
		t.Fatalf("MustBeBound after registration: %v", err)
	}
}

func TestQueryAndSubjectGates(t *testing.T) {
	s := newEngine(t, allowAll, fakeProvider("a", "a.read", []string{"x"}))
	if _, err := s.SearchObjects(context.Background(), " q ", "", 0, 0, ""); !errors.Is(err, domain.ErrQueryTooShort) {
		t.Fatalf("1-rune query: %v", err)
	}
	anon := NewService(subjectAuthority("", false), allowAll)
	_ = anon.Register(fakeProvider("a", "a.read", nil), scope.NewCatalogScope())
	if _, err := anon.SearchObjects(context.Background(), "qq", "", 0, 0, ""); !errors.Is(err, domain.ErrNoSubject) {
		t.Fatalf("anonymous subject: %v", err)
	}
}

func TestTypesFilterAndUnknownType(t *testing.T) {
	s := newEngine(t, allowAll,
		fakeProvider("beta", "b.read", []string{"b1"}),
		fakeProvider("alpha", "a.read", []string{"a1"}))
	page, err := s.SearchObjects(context.Background(), "qq", "alpha", 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(page.Hits, ""); len(got) != 1 || got[0] != "a1" {
		t.Fatalf("types filter leaked: %v", got)
	}
	if _, err := s.SearchObjects(context.Background(), "qq", "alpha,ghost", 0, 0, ""); err == nil {
		t.Fatal("unknown type in filter must error")
	} else {
		var unknown domain.UnknownObjectTypeError
		if !errors.As(err, &unknown) || unknown.ObjectType != "ghost" {
			t.Fatalf("wrong error: %v", err)
		}
	}
}

func TestPermissionGateSkipsProvider(t *testing.T) {
	allowed := func(_ context.Context, action string) (bool, error) { return action == "a.read", nil }
	s := newEngine(t, allowed,
		fakeProvider("alpha", "a.read", []string{"a1"}),
		fakeProvider("beta", "b.read", []string{"b1", "b2"}))
	page, err := s.SearchObjects(context.Background(), "qq", "", 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(page.Hits, "beta"); len(got) != 0 {
		t.Fatalf("permission-lacking provider leaked rows: %v", got)
	}
	if got := ids(page.Hits, "alpha"); len(got) != 1 {
		t.Fatalf("allowed provider missing: %v", got)
	}
	if page.NextPageToken != "" {
		t.Fatalf("skipped provider must not hold the token open: %q", page.NextPageToken)
	}
}

func TestVisibilityTrimAndPreTrimmed(t *testing.T) {
	// Visibility keeps only ids ending in "1"; the provider cursor still advances over RAW rows.
	odd := visFunc(func(_ context.Context, _ string, _ bool, cand []string) ([]string, error) {
		var out []string
		for _, id := range cand {
			if id[len(id)-1] == '1' {
				out = append(out, id)
			}
		}
		return out, nil
	})
	s := NewService(subjectAuthority("subj", false), allowAll)
	if err := s.Register(fakeProvider("alpha", "a.read", []string{"a1", "a2", "a3"}), odd); err != nil {
		t.Fatal(err)
	}
	pre := fakeProvider("beta", "b.read", []string{"b2"})
	pre.PreTrimmed = true
	if err := s.Register(pre, odd); err != nil {
		t.Fatal(err)
	}
	page, err := s.SearchObjects(context.Background(), "qq", "", 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(page.Hits, "alpha"); len(got) != 1 || got[0] != "a1" {
		t.Fatalf("trim failed: %v", got)
	}
	if got := ids(page.Hits, "beta"); len(got) != 1 || got[0] != "b2" {
		t.Fatalf("PreTrimmed provider must bypass the post-trim: %v", got)
	}
}

func TestPaginationRoundTripNoSkipNoDup(t *testing.T) {
	// Two providers × 7 ids each, perTypeLimit 3: the multi-page walk must return every id exactly
	// once and end with no token.
	var alphaIDs, betaIDs []string
	for i := range 7 {
		alphaIDs = append(alphaIDs, fmt.Sprintf("a%d", i))
		betaIDs = append(betaIDs, fmt.Sprintf("b%d", i))
	}
	s := newEngine(t, allowAll,
		fakeProvider("alpha", "a.read", alphaIDs),
		fakeProvider("beta", "b.read", betaIDs))
	seen := map[string]int{}
	token := ""
	for range 10 {
		page, err := s.SearchObjects(context.Background(), "qq", "", 3, 100, token)
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range page.Hits {
			seen[h.RID]++
		}
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	if len(seen) != 14 {
		t.Fatalf("walk saw %d ids, want 14: %v", len(seen), seen)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("id %s returned %d times", id, n)
		}
	}
}

func TestPageSizeCarriesUnvisitedCursor(t *testing.T) {
	// pageSize 2 with perTypeLimit 5: alpha fills the page; beta must survive in the token with its
	// cursor untouched and be served on the continuation.
	s := newEngine(t, allowAll,
		fakeProvider("alpha", "a.read", []string{"a0", "a1"}),
		fakeProvider("beta", "b.read", []string{"b0"}))
	page, err := s.SearchObjects(context.Background(), "qq", "", 5, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(page.Hits, ""); len(got) != 2 || page.NextPageToken == "" {
		t.Fatalf("first page wrong: hits=%v token=%q", got, page.NextPageToken)
	}
	page2, err := s.SearchObjects(context.Background(), "qq", "", 5, 100, page.NextPageToken)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(page2.Hits, "beta"); len(got) != 1 || got[0] != "b0" {
		t.Fatalf("carried provider not served on continuation: %v", page2.Hits)
	}
	if page2.NextPageToken != "" {
		t.Fatalf("walk must terminate, token=%q", page2.NextPageToken)
	}
}

func TestInvalidPageTokens(t *testing.T) {
	s := newEngine(t, allowAll, fakeProvider("alpha", "a.read", []string{"a0"}))
	for _, tok := range []string{"!!!", "eyJ2IjoyfQ", domain.EncodePageToken(map[string]string{"ghost": ""})} {
		if _, err := s.SearchObjects(context.Background(), "qq", "", 0, 0, tok); !errors.Is(err, domain.ErrInvalidPageToken) {
			t.Fatalf("token %q: %v", tok, err)
		}
	}
}

// visFunc adapts a func to scope.Visibility for test doubles.
type visFunc func(ctx context.Context, subject string, isAdmin bool, candidateIDs []string) ([]string, error)

func (f visFunc) ReadableIDs(ctx context.Context, subject string, isAdmin bool, candidateIDs []string) ([]string, error) {
	return f(ctx, subject, isAdmin, candidateIDs)
}
