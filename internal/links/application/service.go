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
	// frontierBatchSize is how many hop-1 frontier nodes depth-2 enumerates per round of arm queries
	// (one distinct-neighbor keyset query per origin arm). Larger ⇒ fewer enumeration rounds per page.
	frontierBatchSize = 256
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

// SearchAroundDepth is the depth-parameterised search-around: depth<=1 is the flat depth-1
// neighborhood (v1 token, unchanged); depth>=2 is the exhaustive two-hop walk (D-LinkTraversal
// depth-2, "full keyset frontier"). depth>2 is clamped to 2.
func (s *Service) SearchAroundDepth(ctx context.Context, ridStr, linkTypesCSV string, depth, pageSize int, pageToken string) (domain.Neighborhood, error) {
	if depth <= 1 {
		return s.SearchAround(ctx, ridStr, linkTypesCSV, pageSize, pageToken)
	}
	return s.searchAround2(ctx, ridStr, linkTypesCSV, pageSize, pageToken)
}

// searchAround2 runs one page of the exhaustive depth-2 walk. Two sequential keyset phases share the
// page's row budget: (1) drain the origin's hop-1 arms (reusing collect, tagging rows hop=1); (2)
// enumerate the trimmed+gated hop-1 neighbors as a frontier in neighbor-RID order and expand each
// with an inner hop-2 collect (rows tagged hop=2, viaRid=<frontier node>), excluding the trivial
// backtrack edge to the origin. Progress is serialized into a small fixed-size Depth2Token.
func (s *Service) searchAround2(ctx context.Context, ridStr, linkTypesCSV string, pageSize int, pageToken string) (domain.Neighborhood, error) {
	subject, isAdmin, r, err := s.resolve(ctx, ridStr)
	if err != nil {
		return domain.Neighborhood{}, err
	}
	tok, err := domain.DecodeDepth2Token(pageToken)
	if err != nil {
		return domain.Neighborhood{}, err
	}
	budget := clamp(pageSize, defaultPageSize, maxPageSize)
	originUUID := r.UUID()
	rows := make([]domain.RawLink, 0, budget)

	// PHASE 1 — hop-1: reuse collect (its own per-arm gate + trim + v1 keyset).
	if !tok.H1Done {
		_, _, groups, next, err := s.collect(ctx, ridStr, linkTypesCSV, budget, domain.EncodePageToken(tok.Origin))
		if err != nil {
			return domain.Neighborhood{}, err
		}
		rows = appendHop(rows, groups, 1, "", originUUID)
		if next != "" { // hop-1 has more — stay in phase 1
			cursors, _ := domain.DecodePageToken(next)
			return domain.Neighborhood{RID: ridStr, Neighbors: rows,
				NextPageToken: domain.EncodeDepth2Token(domain.Depth2Token{Origin: cursors})}, nil
		}
		tok.H1Done = true // hop-1 exhausted; fall through with the remaining budget
	}

	// PHASE 2 — frontier expansion. Need the gated+filtered origin arms for enumeration.
	arms, err := s.gatedArms(ctx, r.TypeName(), linkTypesCSV)
	if err != nil {
		return domain.Neighborhood{}, err
	}

	// Resume a frontier node left mid-expansion by the previous page.
	if tok.Node != "" {
		if len(rows) >= budget {
			return domain.Neighborhood{RID: ridStr, Neighbors: rows, NextPageToken: domain.EncodeDepth2Token(tok)}, nil
		}
		_, _, groups, next, err := s.collect(ctx, tok.Node, linkTypesCSV, budget-len(rows), domain.EncodePageToken(tok.NodeCur))
		if err != nil {
			return domain.Neighborhood{}, err
		}
		rows = appendHop(rows, groups, 2, tok.Node, originUUID)
		if next != "" { // node still not exhausted
			cursors, _ := domain.DecodePageToken(next)
			tok.NodeCur = cursors
			return domain.Neighborhood{RID: ridStr, Neighbors: rows, NextPageToken: domain.EncodeDepth2Token(tok)}, nil
		}
		tok.Front = tok.Node // node fully expanded
		tok.Node, tok.NodeCur = "", nil
	}

	// Walk the remaining frontier in neighbor-RID order, enumerating it a BATCH at a time (one round
	// of arm queries per batch, not per node) so a wide frontier does not re-scan the origin's arms
	// once per neighbor.
	for {
		if len(rows) >= budget {
			return domain.Neighborhood{RID: ridStr, Neighbors: rows,
				NextPageToken: domain.EncodeDepth2Token(domain.Depth2Token{H1Done: true, Front: tok.Front})}, nil
		}
		batch, err := s.frontierNodes(ctx, subject, isAdmin, arms, originUUID, tok.Front, frontierBatchSize)
		if err != nil {
			return domain.Neighborhood{}, err
		}
		if len(batch) == 0 {
			break // frontier exhausted — the walk is complete
		}
		for _, fn := range batch {
			fnode := fn.TargetRID
			if len(rows) >= budget {
				return domain.Neighborhood{RID: ridStr, Neighbors: rows,
					NextPageToken: domain.EncodeDepth2Token(domain.Depth2Token{H1Done: true, Front: tok.Front})}, nil
			}
			_, _, groups, next, err := s.collect(ctx, fnode, linkTypesCSV, budget-len(rows), "")
			if err != nil {
				return domain.Neighborhood{}, err
			}
			rows = appendHop(rows, groups, 2, fnode, originUUID)
			if next != "" { // this node did not fit the remaining budget
				cursors, _ := domain.DecodePageToken(next)
				return domain.Neighborhood{RID: ridStr, Neighbors: rows,
					NextPageToken: domain.EncodeDepth2Token(domain.Depth2Token{H1Done: true, Front: tok.Front, Node: fnode, NodeCur: cursors})}, nil
			}
			tok.Front = fnode // node done; advance the frontier high-water mark
		}
	}
	return domain.Neighborhood{RID: ridStr, Neighbors: rows, NextPageToken: ""}, nil
}

// resolve validates the RID as a known object and resolves the request subject once (shared by the
// depth-2 phases). Mirrors collect's front matter.
func (s *Service) resolve(ctx context.Context, ridStr string) (string, bool, rid.RID, error) {
	r, err := rid.Parse(ridStr)
	if err != nil {
		return "", false, rid.RID{}, domain.UnknownObjectTypeError{RID: ridStr}
	}
	if r.TypeName() == "" || r.Kind() != rid.KindObject {
		return "", false, rid.RID{}, domain.UnknownObjectTypeError{RID: ridStr}
	}
	subject, isAdmin, err := s.authority(ctx)
	if err != nil {
		return "", false, rid.RID{}, err
	}
	if subject == "" {
		return "", false, rid.RID{}, domain.ErrNoSubject
	}
	return subject, isAdmin, r, nil
}

// gatedArms is the origin's incident arms restricted by the linkTypes filter and the per-arm read
// gate — the arms frontier enumeration draws hop-1 neighbors from (matching collect's own gating).
func (s *Service) gatedArms(ctx context.Context, objectType, linkTypesCSV string) ([]arm, error) {
	filter, err := s.linkTypeFilter(linkTypesCSV)
	if err != nil {
		return nil, err
	}
	var out []arm
	for _, a := range s.armsFor(objectType) {
		if filter != nil {
			if _, ok := filter[a.desc.LinkName]; !ok {
				continue
			}
		}
		ok, err := s.allowed(ctx, a.desc.Permission)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, a)
		}
	}
	return out, nil
}

// frontierNodes returns up to `size` trimmed (readable) hop-1 neighbor RIDs strictly greater than
// `after`, in neighbor-RID order, skipping raw batches that trim away entirely; nil ⇒ the frontier is
// exhausted. One round of arm queries yields a whole batch of frontier nodes, so a wide frontier is
// enumerated per-batch rather than per-node.
func (s *Service) frontierNodes(ctx context.Context, subject string, isAdmin bool, arms []arm, srcUUID, after string, size int) ([]domain.RawLink, error) {
	cursor := after
	for {
		raws, rawNext, err := s.frontierBatch(ctx, arms, srcUUID, cursor, size)
		if err != nil {
			return nil, err
		}
		if len(raws) == 0 {
			return nil, nil
		}
		kept, err := s.trim(ctx, subject, isAdmin, raws)
		if err != nil {
			return nil, err
		}
		if len(kept) > 0 {
			return kept, nil
		}
		if rawNext == "" {
			return nil, nil
		}
		cursor = rawNext
	}
}

// frontierBatch fetches up to `limit` distinct hop-1 neighbor RIDs > `after` across the given arms,
// in neighbor-RID order. Rows carry only TargetRID + TargetType (for trim); rawNext is the keyset
// high-water mark ("" ⇒ the arms hold no more). Global-top-`limit` correctness: the smallest `limit`
// distinct neighbors are a subset of the union of each arm's smallest `limit`, so a per-arm LIMIT is
// sufficient and any un-returned neighbor is strictly greater than rawNext (caught next round).
func (s *Service) frontierBatch(ctx context.Context, arms []arm, srcUUID, after string, limit int) ([]domain.RawLink, string, error) {
	seen := map[string]string{} // neighbor rid -> type (dedup across arms)
	var ids []string
	for _, a := range arms {
		sql, args := buildFrontierQuery(a, srcUUID, after, limit)
		rows, err := s.pool.Query(ctx, sql, args...)
		if err != nil {
			return nil, "", err
		}
		polyNeighbor := a.other.KindCol != ""
		for rows.Next() {
			var nrid string
			var nkind *string
			dest := []any{&nrid}
			if polyNeighbor {
				dest = append(dest, &nkind)
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return nil, "", err
			}
			typ := a.other.Targets[0].Type
			if polyNeighbor {
				t, ok := neighborTypeFor(a.other, nkind)
				if !ok {
					continue
				}
				typ = t
			}
			if _, dup := seen[nrid]; !dup {
				seen[nrid] = typ
				ids = append(ids, nrid)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, "", err
		}
		rows.Close()
	}
	sort.Strings(ids)
	rawNext := ""
	if len(ids) > limit {
		ids = ids[:limit]
		rawNext = ids[len(ids)-1]
	}
	out := make([]domain.RawLink, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.RawLink{TargetRID: id, TargetType: seen[id]})
	}
	return out, rawNext, nil
}

// appendHop flattens a collect result's groups onto rows, tagging each with hop (+ viaRid for hop 2)
// and dropping the trivial backtrack edge to the origin at hop 2.
func appendHop(rows []domain.RawLink, groups []domain.Group, hop int, via, originUUID string) []domain.RawLink {
	for _, g := range groups {
		for _, rl := range g.Rows {
			if hop == 2 && strings.EqualFold(rl.TargetRID, originUUID) {
				continue
			}
			rl.Hop = hop
			rl.ViaRID = via
			rows = append(rows, rl)
		}
	}
	return rows
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
