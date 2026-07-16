// Package application is the generic link-traversal engine (D-LinkTraversal, review-2026-09 R-27).
// It owns the composition-time descriptor registry — each reified link table registers a Descriptor
// together with (per neighbor object type) a D-VisibilityScope adapter (R-30) — and answers "what
// links does object X have?" as a fan-in over the registered tables: for the queried RID's object
// type it selects every incident link arm, runs one keyset query per arm on the arm's existing
// endpoint index, gates each arm by its read permission (skipped, not failed, when the subject
// lacks it) and trims neighbor rows through the neighbor type's visibility scope, so the endpoint
// can never serve a link the owning module's own endpoints would withhold.
//
// The descriptor identifiers (table + columns) come from a COMPILE-TIME registry, never from user
// input; they are still passed through pgx.Identifier.Sanitize before interpolation. This is the
// one place raw dynamic SQL is justified: a union over a runtime-registered set of tables is not
// expressible in sqlc (which needs static table/column names).
package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/authorization/scope"
	"github.com/olegamysk/go-oikumenea/internal/links/domain"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// Querier is the minimal pgx surface the engine needs (satisfied by *pgxpool.Pool).
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// AuthorityFunc resolves the request subject + instance-admin flag (pep.SubjectAuthority).
// AllowedFunc is the non-erroring permission probe (pep.AllowedAnywhere). LabelFunc resolves a
// batch of one object type's RIDs to display names, each a locale→text map (D-i18n: all locales in
// every response); best effort — missing ids simply get no label.
type (
	AuthorityFunc func(ctx context.Context) (subject string, isAdmin bool, err error)
	AllowedFunc   func(ctx context.Context, action string) (bool, error)
	LabelFunc     func(ctx context.Context, ids []string) (map[string]map[string]string, error)
)

// Service is the traversal engine. Register descriptors + visibilities at composition time, then
// MustBeBound (main.go's boot seam loop) before serving.
type Service struct {
	pool       Querier
	authority  AuthorityFunc
	allowed    AllowedFunc
	descs      []domain.Descriptor
	seen       map[[2]int]bool // (service, code) already registered
	visibility map[string]scope.Visibility
	labelers   map[string]LabelFunc
	exempt     map[[2]int]string // (service, code) -> rationale for a deliberately non-traversable link type
}

func NewService(pool Querier, authority AuthorityFunc, allowed AllowedFunc) *Service {
	return &Service{
		pool:       pool,
		authority:  authority,
		allowed:    allowed,
		seen:       map[[2]int]bool{},
		visibility: map[string]scope.Visibility{},
		labelers:   map[string]LabelFunc{},
		exempt:     map[[2]int]string{},
	}
}

// linkTypeNames is the (service, code) -> bare name map of every kind=link RID type, from the
// drift-proof pkg/rid registry (R-28). It is the authority a Descriptor is validated against and the
// completeness set MustBeBound checks.
func linkTypeNames() map[[2]int]string {
	out := map[[2]int]string{}
	for _, t := range rid.Types() {
		if t.Kind == int(rid.KindLink) {
			out[[2]int{t.Service, t.Code}] = t.Name
		}
	}
	return out
}

// Register adds one reified link table. Composition-time only; a descriptor that names no registered
// link type, mismatches its name, is structurally incomplete, or duplicates another is a boot error.
func (s *Service) Register(d domain.Descriptor) error {
	key := [2]int{d.Service, d.Code}
	name, ok := linkTypeNames()[key]
	switch {
	case !ok:
		return fmt.Errorf("links: descriptor (%d,%d) is not a registered link type (pkg/rid)", d.Service, d.Code)
	case d.LinkName != name:
		return fmt.Errorf("links: descriptor (%d,%d) name %q != registry name %q", d.Service, d.Code, d.LinkName, name)
	case d.Table == "":
		return fmt.Errorf("links: descriptor %q has no table", d.LinkName)
	case d.Permission == "":
		return fmt.Errorf("links: descriptor %q has no read permission", d.LinkName)
	}
	if err := validateEndpoint(d.LinkName, "A", d.A); err != nil {
		return err
	}
	if err := validateEndpoint(d.LinkName, "B", d.B); err != nil {
		return err
	}
	if s.seen[key] {
		return fmt.Errorf("links: duplicate descriptor for link type %q", d.LinkName)
	}
	s.seen[key] = true
	s.descs = append(s.descs, d)
	return nil
}

func validateEndpoint(link, side string, e domain.Endpoint) error {
	if e.Column == "" {
		return fmt.Errorf("links: descriptor %q end %s has no column", link, side)
	}
	if len(e.Targets) == 0 {
		return fmt.Errorf("links: descriptor %q end %s has no targets", link, side)
	}
	for _, t := range e.Targets {
		if t.Type == "" {
			return fmt.Errorf("links: descriptor %q end %s has an empty target type", link, side)
		}
		if (e.KindCol == "") != (t.KindValue == "") {
			return fmt.Errorf("links: descriptor %q end %s: KindCol and target KindValue must both be set or both empty", link, side)
		}
	}
	return nil
}

// RegisterVisibility maps a neighbor object type to its D-VisibilityScope adapter. Every target type
// any descriptor points at MUST have one (checked by MustBeBound) — a cross-type surface never
// serves an untrimmed neighbor type (R-30).
func (s *Service) RegisterVisibility(objectType string, v scope.Visibility) {
	s.visibility[objectType] = v
}

// RegisterLabeler attaches an optional batch label resolver for a neighbor object type. Absent ⇒
// rows of that type carry no targetLabel and the client falls back to the RID tail.
func (s *Service) RegisterLabeler(objectType string, fn LabelFunc) { s.labelers[objectType] = fn }

// Exempt marks a kind=link RID type as deliberately non-traversable (encrypted/free-text/untyped
// neighbor, or no neighbor object at all) with a rationale, so MustBeBound's completeness check is
// honest rather than silently incomplete.
func (s *Service) Exempt(service, code int, why string) { s.exempt[[2]int{service, code}] = why }

// MustBeBound is the boot-time coverage assertion (the R-27 drift guard, pairing R-28): every
// kind=link type in the pkg/rid registry is either registered or explicitly exempt, and every
// neighbor object type a descriptor points at has a registered visibility scope. A link type added
// by a future migration therefore fails boot until it is registered or exempted here.
func (s *Service) MustBeBound() error {
	if len(s.descs) == 0 {
		return errors.New("links service: no descriptors registered (cmd/oikumenea link_descriptors wiring)")
	}
	var missing []string
	for key, name := range linkTypeNames() {
		if s.seen[key] {
			continue
		}
		if _, ok := s.exempt[key]; ok {
			continue
		}
		missing = append(missing, fmt.Sprintf("%s (%d,%d)", name, key[0], key[1]))
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("links service: link types neither registered nor exempt: %s", strings.Join(missing, ", "))
	}
	for _, d := range s.descs {
		for _, e := range []domain.Endpoint{d.A, d.B} {
			for _, t := range e.Targets {
				if _, ok := s.visibility[t.Type]; !ok {
					return fmt.Errorf("links service: descriptor %q neighbor type %q has no registered visibility scope (D-VisibilityScope)", d.LinkName, t.Type)
				}
			}
		}
	}
	return nil
}

// arm is one (descriptor, queried side) traversal: the queried object sits on `self`, the neighbor
// is `other`, and (for a polymorphic self end) selfKind filters the discriminator column.
type arm struct {
	desc      domain.Descriptor
	self      domain.Endpoint
	other     domain.Endpoint
	selfKind  string // "" unless the queried end is polymorphic
	direction string
	key       string // stable cursor key: linkName + side
}

// armsFor selects every link arm incident to a queried object of the given type, in stable order.
func (s *Service) armsFor(objectType string) []arm {
	var arms []arm
	for _, d := range s.descs {
		aHolds := targetKind(d.A, objectType)
		bHolds := targetKind(d.B, objectType)
		symmetric := aHolds != nil && bHolds != nil
		if aHolds != nil {
			dir := domain.DirOut
			if symmetric {
				dir = domain.DirPeer
			}
			arms = append(arms, arm{desc: d, self: d.A, other: d.B, selfKind: *aHolds, direction: dir, key: d.LinkName + "/a"})
		}
		if bHolds != nil {
			dir := domain.DirIn
			if symmetric {
				dir = domain.DirPeer
			}
			arms = append(arms, arm{desc: d, self: d.B, other: d.A, selfKind: *bHolds, direction: dir, key: d.LinkName + "/b"})
		}
	}
	sort.Slice(arms, func(i, j int) bool { return arms[i].key < arms[j].key })
	return arms
}

// targetKind reports whether an endpoint can hold objectType, returning the discriminator value to
// filter on ("" for a plain end) or nil if it cannot.
func targetKind(e domain.Endpoint, objectType string) *string {
	for _, t := range e.Targets {
		if t.Type == objectType {
			kv := t.KindValue
			return &kv
		}
	}
	return nil
}

// GetObjectLinks returns every registered link incident to ridStr, grouped by (link type, direction,
// neighbor type). linkTypesCSV restricts to named bare link types ("" = all). A per-arm cursor
// advances over RAW (pre-trim) rows so a visibility-trimmed page may run short but never skips.
func (s *Service) GetObjectLinks(ctx context.Context, ridStr, linkTypesCSV string, pageSize int, pageToken string) (domain.ObjectLinks, error) {
	subject, isAdmin, groups, next, err := s.collect(ctx, ridStr, linkTypesCSV, pageSize, pageToken)
	if err != nil {
		return domain.ObjectLinks{}, err
	}
	_ = subject
	_ = isAdmin
	return domain.ObjectLinks{RID: ridStr, Groups: groups, NextPageToken: next}, nil
}

// SearchAround is GetObjectLinks flattened to a neighbor list (the graph-explorer shape).
func (s *Service) SearchAround(ctx context.Context, ridStr, linkTypesCSV string, pageSize int, pageToken string) (domain.Neighborhood, error) {
	_, _, groups, next, err := s.collect(ctx, ridStr, linkTypesCSV, pageSize, pageToken)
	if err != nil {
		return domain.Neighborhood{}, err
	}
	var flat []domain.RawLink
	for _, g := range groups {
		flat = append(flat, g.Rows...)
	}
	return domain.Neighborhood{RID: ridStr, Neighbors: flat, NextPageToken: next}, nil
}

func (s *Service) collect(ctx context.Context, ridStr, linkTypesCSV string, pageSize int, pageToken string) (string, bool, []domain.Group, string, error) {
	r, err := rid.Parse(ridStr)
	if err != nil {
		return "", false, nil, "", domain.UnknownObjectTypeError{RID: ridStr}
	}
	objectType := r.TypeName()
	if objectType == "" || r.Kind() != rid.KindObject {
		return "", false, nil, "", domain.UnknownObjectTypeError{RID: ridStr}
	}
	subject, isAdmin, err := s.authority(ctx)
	if err != nil {
		return "", false, nil, "", err
	}
	if subject == "" {
		return "", false, nil, "", domain.ErrNoSubject
	}
	filter, err := s.linkTypeFilter(linkTypesCSV)
	if err != nil {
		return "", false, nil, "", err
	}
	cursors, err := domain.DecodePageToken(pageToken)
	if err != nil {
		return "", false, nil, "", err
	}

	budget := clamp(pageSize, defaultPageSize, maxPageSize)
	arms := s.armsFor(objectType)
	// groups accumulate keyed by (linkName, direction, neighborType) preserving arm order.
	type gkey struct{ link, dir, typ string }
	order := []gkey{}
	byKey := map[gkey]*domain.Group{}
	next := map[string]string{}
	count := 0

	for _, a := range arms {
		if filter != nil {
			if _, ok := filter[a.desc.LinkName]; !ok {
				continue
			}
		}
		after := ""
		if cursors != nil {
			c, pending := cursors[a.key]
			if !pending {
				continue // arm exhausted on an earlier page
			}
			after = c
		}
		if count >= budget {
			next[a.key] = after // page full before this arm ran; carry cursor unchanged
			continue
		}
		ok, err := s.allowed(ctx, a.desc.Permission)
		if err != nil {
			return "", false, nil, "", err
		}
		if !ok {
			continue // per-arm read-permission gate
		}
		limit := budget - count
		raws, cursor, err := s.runArm(ctx, a, r.UUID(), after, limit)
		if err != nil {
			return "", false, nil, "", fmt.Errorf("links arm %q: %w", a.key, err)
		}
		kept, err := s.trim(ctx, subject, isAdmin, raws)
		if err != nil {
			return "", false, nil, "", fmt.Errorf("links visibility %q: %w", a.key, err)
		}
		for _, rl := range kept {
			k := gkey{a.desc.LinkName, rl.Direction, rl.TargetType}
			g, ok := byKey[k]
			if !ok {
				g = &domain.Group{LinkType: a.desc.LinkName, TargetType: rl.TargetType, Direction: rl.Direction}
				byKey[k] = g
				order = append(order, k)
			}
			g.Rows = append(g.Rows, rl)
		}
		count += len(kept)
		if cursor != "" {
			next[a.key] = cursor
		}
	}

	groups := make([]domain.Group, 0, len(order))
	for _, k := range order {
		groups = append(groups, *byKey[k])
	}
	if err := s.attachLabels(ctx, groups); err != nil {
		return "", false, nil, "", err
	}
	return subject, isAdmin, groups, domain.EncodePageToken(next), nil
}

// runArm executes one arm's keyset query and returns up to `limit` raw links plus the next cursor
// ("" = exhausted). The cursor is the last RAW row's link RID (trimming happens after).
func (s *Service) runArm(ctx context.Context, a arm, srcUUID, after string, limit int) ([]domain.RawLink, string, error) {
	sql, args := buildArmQuery(a, srcUUID, after, limit+1)
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	polyNeighbor := a.other.KindCol != ""
	var out []domain.RawLink
	lastRID := ""
	for rows.Next() {
		// scan targets: link_rid, neighbor_rid, [neighbor_kind], attr1..attrN
		dest := make([]any, 0, 3+len(a.desc.AttrCols))
		var linkRID, neighborRID string
		var neighborKind *string
		dest = append(dest, &linkRID, &neighborRID)
		if polyNeighbor {
			dest = append(dest, &neighborKind)
		}
		attrPtrs := make([]*string, len(a.desc.AttrCols))
		for i := range a.desc.AttrCols {
			dest = append(dest, &attrPtrs[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, "", err
		}
		lastRID = linkRID
		neighborType := a.other.Targets[0].Type
		if polyNeighbor {
			nt, ok := neighborTypeFor(a.other, neighborKind)
			if !ok {
				continue // discriminator not in the descriptor (guarded by the DB CHECK) — skip defensively
			}
			neighborType = nt
		}
		attrs := map[string]string{}
		for i, name := range a.desc.AttrCols {
			if attrPtrs[i] != nil {
				attrs[name] = *attrPtrs[i]
			}
		}
		if len(attrs) == 0 {
			attrs = nil
		}
		out = append(out, domain.RawLink{
			LinkRID:    linkRID,
			TargetRID:  neighborRID,
			TargetType: neighborType,
			Direction:  a.direction,
			Attrs:      attrs,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	cursor := ""
	if len(out) > limit {
		out = out[:limit]
		cursor = lastKeptRID(out)
	}
	_ = lastRID
	return out, cursor, nil
}

func lastKeptRID(rows []domain.RawLink) string {
	if len(rows) == 0 {
		return ""
	}
	return rows[len(rows)-1].LinkRID
}

func neighborTypeFor(e domain.Endpoint, kind *string) (string, bool) {
	if kind == nil {
		return "", false
	}
	for _, t := range e.Targets {
		if t.KindValue == *kind {
			return t.Type, true
		}
	}
	return "", false
}

// trim filters raw links through the per-neighbor-type visibility scope, preserving order. Rows of a
// type with no registered scope are dropped (fail closed — but MustBeBound guarantees one exists).
func (s *Service) trim(ctx context.Context, subject string, isAdmin bool, raws []domain.RawLink) ([]domain.RawLink, error) {
	if len(raws) == 0 {
		return raws, nil
	}
	// bucket ids by neighbor type
	idsByType := map[string][]string{}
	for _, rl := range raws {
		idsByType[rl.TargetType] = append(idsByType[rl.TargetType], rl.TargetRID)
	}
	readable := map[string]struct{}{}
	for typ, ids := range idsByType {
		v, ok := s.visibility[typ]
		if !ok {
			continue // fail closed: no scope ⇒ none readable
		}
		ok2, err := v.ReadableIDs(ctx, subject, isAdmin, ids)
		if err != nil {
			return nil, err
		}
		for _, id := range ok2 {
			readable[typ+"\x00"+id] = struct{}{}
		}
	}
	out := make([]domain.RawLink, 0, len(raws))
	for _, rl := range raws {
		if _, ok := readable[rl.TargetType+"\x00"+rl.TargetRID]; ok {
			out = append(out, rl)
		}
	}
	return out, nil
}

// attachLabels fills targetLabel per neighbor type via the registered labelers (best effort).
func (s *Service) attachLabels(ctx context.Context, groups []domain.Group) error {
	idsByType := map[string][]string{}
	for _, g := range groups {
		for _, rl := range g.Rows {
			idsByType[rl.TargetType] = append(idsByType[rl.TargetType], rl.TargetRID)
		}
	}
	labels := map[string]map[string]string{}
	for typ, ids := range idsByType {
		fn, ok := s.labelers[typ]
		if !ok {
			continue
		}
		m, err := fn(ctx, ids)
		if err != nil {
			return err
		}
		for id, l := range m {
			labels[typ+"\x00"+id] = l
		}
	}
	if len(labels) == 0 {
		return nil
	}
	for gi := range groups {
		for ri := range groups[gi].Rows {
			rl := &groups[gi].Rows[ri]
			if l, ok := labels[rl.TargetType+"\x00"+rl.TargetRID]; ok && len(l) > 0 {
				rl.Labels = l
			}
		}
	}
	return nil
}

func (s *Service) linkTypeFilter(csv string) (map[string]struct{}, error) {
	if strings.TrimSpace(csv) == "" {
		return nil, nil
	}
	known := map[string]struct{}{}
	for _, d := range s.descs {
		known[d.LinkName] = struct{}{}
	}
	set := map[string]struct{}{}
	for _, part := range strings.Split(csv, ",") {
		t := strings.TrimSpace(part)
		if t == "" {
			continue
		}
		if _, ok := known[t]; !ok {
			// An unknown/exempt link-type filter simply matches nothing rather than erroring — the
			// caller may pass a superset of types across many objects.
			continue
		}
		set[t] = struct{}{}
	}
	return set, nil
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
