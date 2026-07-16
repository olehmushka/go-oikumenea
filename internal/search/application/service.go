// Package application is the unified-search fan-in engine (D-UnifiedSearch, review-2026-09 R-26).
// It owns the composition-time provider registry — each searchable object type registers its
// SearchProvider TOGETHER with its D-VisibilityScope adapter (one registry, two facets today; the
// R-27 link facet extends the same seam) — and executes one federated keyset page per request:
// providers in fixed lexicographic type order, each gated by its read permission (skipped, not
// failed, when the subject lacks it) and trimmed through its registered visibility scope, so the
// endpoint can never serve a row the owning module's own endpoints would withhold (R-30).
package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/olegamysk/go-oikumenea/internal/authorization/scope"
	"github.com/olegamysk/go-oikumenea/internal/search/domain"
)

const (
	defaultPerTypeLimit = 5
	maxPerTypeLimit     = 50
	defaultPageSize     = 25
	maxPageSize         = 100
)

// AuthorityFunc resolves the request subject + instance-admin flag (pep.SubjectAuthority — zero
// queries on the request path). AllowedFunc is the non-erroring permission probe
// (pep.AllowedAnywhere). Injected funcs keep this module free of a pep dependency and the engine
// unit-testable without the authorization stack.
type (
	AuthorityFunc func(ctx context.Context) (subject string, isAdmin bool, err error)
	AllowedFunc   func(ctx context.Context, action string) (bool, error)
)

type registration struct {
	provider   domain.Provider
	visibility scope.Visibility
}

// Service is the fan-in engine. Register every provider at composition time, then MustBeBound
// (main.go's boot seam loop) before serving — a provider set that is empty or missing a
// visibility fails boot, never serves untrimmed (R-30 acceptance).
type Service struct {
	authority AuthorityFunc
	allowed   AllowedFunc
	regs      map[string]registration
	order     []string // registered object types, lexicographic
}

func NewService(authority AuthorityFunc, allowed AllowedFunc) *Service {
	return &Service{authority: authority, allowed: allowed, regs: map[string]registration{}}
}

// Register adds one searchable object type with its visibility scope. Composition-time only; a
// duplicate or structurally incomplete registration is a boot error (returned, not deferred to
// request time).
func (s *Service) Register(p domain.Provider, v scope.Visibility) error {
	switch {
	case p.ObjectType == "":
		return errors.New("search: provider with empty object type")
	case p.ReadPermission == "":
		return fmt.Errorf("search: provider %q has no read permission", p.ObjectType)
	case p.Search == nil:
		return fmt.Errorf("search: provider %q has no search func", p.ObjectType)
	case v == nil:
		return fmt.Errorf("search: provider %q registered without a visibility scope (D-VisibilityScope)", p.ObjectType)
	}
	if _, dup := s.regs[p.ObjectType]; dup {
		return fmt.Errorf("search: duplicate provider for object type %q", p.ObjectType)
	}
	s.regs[p.ObjectType] = registration{provider: p, visibility: v}
	s.order = append(s.order, p.ObjectType)
	sort.Strings(s.order)
	return nil
}

// MustBeBound is the boot-time assertion (review-2026-07 R-11 seam loop): the engine must have at
// least one provider before serving.
func (s *Service) MustBeBound() error {
	if len(s.regs) == 0 {
		return errors.New("search service: no search providers registered (cmd/oikumenea search_providers wiring)")
	}
	return nil
}

// ObjectTypes returns the registered type tokens in engine order (tests, diagnostics).
func (s *Service) ObjectTypes() []string { return append([]string(nil), s.order...) }

// SearchObjects runs one federated page. typesCSV restricts the provider set (comma-separated
// object-type tokens; "" = all registered). perTypeLimit caps each provider's contribution per
// page; pageSize caps the page total. pageToken continues every non-exhausted provider's keyset;
// hits are grouped by type in lexicographic type order, and a provider's cursor advances over its
// RAW (pre-trim) rows so a visibility-trimmed page may run short but never skips a row.
func (s *Service) SearchObjects(ctx context.Context, query, typesCSV string, perTypeLimit, pageSize int, pageToken string) (domain.Page, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < domain.MinQueryLength {
		return domain.Page{}, domain.ErrQueryTooShort
	}
	subject, isAdmin, err := s.authority(ctx)
	if err != nil {
		return domain.Page{}, err
	}
	if subject == "" {
		return domain.Page{}, domain.ErrNoSubject
	}

	selected, err := s.selectTypes(typesCSV)
	if err != nil {
		return domain.Page{}, err
	}
	cursors, err := domain.DecodePageToken(pageToken)
	if err != nil {
		return domain.Page{}, err
	}
	// Continuation: only providers the token still carries; a token key that is not a
	// registered type is not a token we issued.
	for t := range cursors {
		if _, ok := s.regs[t]; !ok {
			return domain.Page{}, domain.ErrInvalidPageToken
		}
	}

	perType := clamp(perTypeLimit, defaultPerTypeLimit, maxPerTypeLimit)
	total := clamp(pageSize, defaultPageSize, maxPageSize)

	page := domain.Page{}
	next := map[string]string{}
	for _, objectType := range selected {
		after := ""
		if cursors != nil {
			c, pending := cursors[objectType]
			if !pending {
				continue // exhausted on an earlier page
			}
			after = c
		}
		remaining := total - len(page.Hits)
		if remaining <= 0 {
			// Page full before this provider ran: carry its cursor unchanged.
			next[objectType] = after
			continue
		}
		reg := s.regs[objectType]
		ok, err := s.allowed(ctx, reg.provider.ReadPermission)
		if err != nil {
			return domain.Page{}, err
		}
		if !ok {
			continue // gate: the provider contributes nothing, now or on later pages
		}
		limit := min(perType, remaining)
		raw, cursor, err := reg.provider.Search(ctx, subject, isAdmin, query, after, limit)
		if err != nil {
			return domain.Page{}, fmt.Errorf("search provider %q: %w", objectType, err)
		}
		kept := raw
		if !reg.provider.PreTrimmed {
			if kept, err = s.trim(ctx, reg.visibility, subject, isAdmin, raw); err != nil {
				return domain.Page{}, fmt.Errorf("search visibility %q: %w", objectType, err)
			}
		}
		for _, h := range kept {
			page.Hits = append(page.Hits, domain.Hit{RID: h.ID, ObjectType: objectType, Label: h.Label, Snippet: h.Snippet})
		}
		if cursor != "" {
			next[objectType] = cursor
		}
	}
	page.NextPageToken = domain.EncodePageToken(next)
	return page, nil
}

func (s *Service) selectTypes(typesCSV string) ([]string, error) {
	if strings.TrimSpace(typesCSV) == "" {
		return s.order, nil
	}
	set := map[string]struct{}{}
	for _, part := range strings.Split(typesCSV, ",") {
		t := strings.TrimSpace(part)
		if t == "" {
			continue
		}
		if _, ok := s.regs[t]; !ok {
			return nil, domain.UnknownObjectTypeError{ObjectType: t}
		}
		set[t] = struct{}{}
	}
	if len(set) == 0 {
		return s.order, nil
	}
	out := make([]string, 0, len(set))
	for _, t := range s.order {
		if _, ok := set[t]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// trim filters raw hits through the visibility scope, preserving order (the scope contract).
func (s *Service) trim(ctx context.Context, v scope.Visibility, subject string, isAdmin bool, raw []domain.RawHit) ([]domain.RawHit, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	ids := make([]string, len(raw))
	for i, h := range raw {
		ids[i] = h.ID
	}
	readable, err := v.ReadableIDs(ctx, subject, isAdmin, ids)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(readable))
	for _, id := range readable {
		set[id] = struct{}{}
	}
	out := make([]domain.RawHit, 0, len(readable))
	for _, h := range raw {
		if _, ok := set[h.ID]; ok {
			out = append(out, h)
		}
	}
	return out, nil
}

func clamp(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}
