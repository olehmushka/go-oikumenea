// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package domain holds the generic link-traversal model (D-LinkTraversal, review-2026-09 R-27): the
// Descriptor each module registers for one reified link table at composition time, the type-erased
// link row, and the composite keyset page token that federates the per-arm cursors. Like the search
// module this owns no tables and mints no RIDs — every link and every neighbor is another module's
// object, identified by its self-describing RID (D-ResourceIdentifiers). The descriptor set is the
// Go counterpart of the web console's hand-authored links[] arrays, but derived from and validated
// against the pkg/rid link-type registry so the two cannot drift (pairs with R-28).
package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// Target is one object type an endpoint column can hold. Plain (uuid FK) endpoints have exactly one
// Target with an empty KindValue; polymorphic (kind+id text) endpoints have one per discriminator
// value (e.g. finance holder: {person→person}, {company→organization}).
type Target struct {
	KindValue string // "" for a plain endpoint; the *_kind column value for a polymorphic one
	Type      string // the ontology registry token for RIDs stored in this endpoint
}

// Endpoint is one of a link table's two ends: its id column, an optional discriminator column for
// polymorphic ends, and the object type(s) it points at.
type Endpoint struct {
	Column  string // sql identifier of the id column (uuid, or text for a polymorphic end)
	KindCol string // "" for a plain end; the discriminator column for a polymorphic end
	Targets []Target
}

// Descriptor registers one reified link table for generic traversal. Service+Code identify the link
// type in the pkg/rid registry (kind is always link); LinkName is its bare registry name. A and B
// are the two ends; a query filters on whichever end matches the queried object and returns the
// other as the neighbor. Permission gates the whole arm (pep.AllowedAnywhere). AttrCols are extra
// display columns surfaced verbatim as string attrs (status, role, effective dates, …).
type Descriptor struct {
	Service      int
	Code         int
	LinkName     string
	Table        string
	A            Endpoint
	B            Endpoint
	Permission   string
	AttrCols     []string
	NoSoftDelete bool // true for the rare link table without a deleted_at column
	// FilterCol/FilterVal, when set, add `<FilterCol> = <FilterVal>` (bound param) to every arm query.
	// Its purpose is twofold: keep the traversed graph to current edges (e.g. status='active'), and —
	// critically at scale — MATCH the owning module's PARTIAL index predicate so generic traversal
	// stays index-backed rather than seq-scanning (e.g. membership's status='active' partial indexes).
	FilterCol string
	FilterVal string
}

// Direction of a link row relative to the queried object.
const (
	DirOut  = "out"  // queried object is the source end (A); neighbor is the B end
	DirIn   = "in"   // queried object is the target end (B); neighbor is the A end
	DirPeer = "peer" // symmetric same-type link (both ends the queried object's type)
)

// RawLink is one traversed link. Labels is the best-effort neighbor display name as a locale→text
// map (D-i18n: all locales in every response, no negotiation), filled by the engine's registered
// labelers; empty ⇒ the client falls back to the RID tail. Hop is the neighbor's distance from the
// queried object (1 = direct, 2 = second-hop reached via ViaRID); depth-1 rows leave both zero-valued
// so their wire shape is unchanged.
type RawLink struct {
	LinkRID    string
	TargetRID  string
	TargetType string
	Direction  string
	Labels     map[string]string
	Attrs      map[string]string
	Hop        int    // 0 (==1, direct) or 2; only depth-2 responses ever set it
	ViaRID     string // the hop-1 neighbor a hop-2 row was reached through ("" for direct rows)
}

// Group is all links of one (link type, direction) incident to the queried object.
type Group struct {
	LinkType   string
	TargetType string
	Direction  string
	Rows       []RawLink
}

// ObjectLinks is the grouped result; Neighbors is the same rows flattened (search-around).
type (
	ObjectLinks struct {
		RID           string
		Groups        []Group
		NextPageToken string
	}
	Neighborhood struct {
		RID           string
		Neighbors     []RawLink
		NextPageToken string
	}
)

var (
	// ErrInvalidPageToken rejects tokens this engine did not issue.
	ErrInvalidPageToken = errors.New("links: invalid page token")
	// ErrNoSubject rejects requests without an authenticated subject (fail closed).
	ErrNoSubject = errors.New("links: no authenticated subject")
)

// UnknownObjectTypeError names a RID that does not decode to a registered object type.
type UnknownObjectTypeError struct{ RID string }

func (e UnknownObjectTypeError) Error() string {
	return fmt.Sprintf("links: %q does not decode to a known object type", e.RID)
}

// pageToken is the wire shape of the composite keyset token: one cursor per NON-EXHAUSTED link arm,
// keyed by the arm's stable key. An arm absent from the map is done; an empty map is never issued.
type pageToken struct {
	V       int               `json:"v"`
	Cursors map[string]string `json:"c"`
}

// EncodePageToken serializes the per-arm cursors ("" when no arm has more).
func EncodePageToken(cursors map[string]string) string {
	if len(cursors) == 0 {
		return ""
	}
	b, _ := json.Marshal(pageToken{V: 1, Cursors: cursors})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodePageToken parses a previously issued token. "" means first page (nil map).
func DecodePageToken(s string) (map[string]string, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalidPageToken
	}
	var t pageToken
	if err := json.Unmarshal(raw, &t); err != nil || t.V != 1 || len(t.Cursors) == 0 {
		return nil, ErrInvalidPageToken
	}
	return t.Cursors, nil
}

// Depth2Token is the resumable state of an exhaustive depth-2 search-around walk (D-LinkTraversal
// depth-2, "full keyset frontier"). The walk is two sequential keyset phases: (1) drain the origin's
// hop-1 arms — Origin carries one cursor per non-exhausted origin arm, exactly like the v1 token;
// (2) once H1Done, enumerate the trimmed/gated hop-1 neighbors as a frontier in NEIGHBOR-RID order
// (Front is the last neighbor RID already expanded) and expand each with an inner hop-2 collect (Node
// is the frontier node currently mid-walk, NodeCur its per-arm cursors). A small, fixed-size token:
// two scalar cursors plus two per-arm maps, never the frontier set itself.
type Depth2Token struct {
	Origin  map[string]string // hop-1 origin arm cursors; empty once hop-1 drained
	H1Done  bool              // hop-1 fully drained — the walk is in the frontier phase
	Front   string            // last frontier neighbor RID fully expanded (keyset high-water mark)
	Node    string            // frontier node currently mid-expansion ("" between nodes)
	NodeCur map[string]string // that node's inner hop-2 arm cursors
}

// depth2Wire is the on-the-wire shape (short JSON keys; omitempty keeps depth-1-drained tokens small).
type depth2Wire struct {
	V       int               `json:"v"`
	Origin  map[string]string `json:"o,omitempty"`
	H1Done  bool              `json:"h1d,omitempty"`
	Front   string            `json:"ff,omitempty"`
	Node    string            `json:"fn,omitempty"`
	NodeCur map[string]string `json:"fc,omitempty"`
}

// EncodeDepth2Token serializes a depth-2 walk position. The engine calls it only when the walk has
// more to yield, so unlike the v1 encoder it always produces a token.
func EncodeDepth2Token(t Depth2Token) string {
	b, _ := json.Marshal(depth2Wire{
		V:       2,
		Origin:  t.Origin,
		H1Done:  t.H1Done,
		Front:   t.Front,
		Node:    t.Node,
		NodeCur: t.NodeCur,
	})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeDepth2Token parses a depth-2 token. "" means first page (a zero Depth2Token: hop-1 from the
// start). A v1 token (or any non-v2 payload) is rejected — depth crossings never share a token.
func DecodeDepth2Token(s string) (Depth2Token, error) {
	if s == "" {
		return Depth2Token{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Depth2Token{}, ErrInvalidPageToken
	}
	var w depth2Wire
	if err := json.Unmarshal(raw, &w); err != nil || w.V != 2 {
		return Depth2Token{}, ErrInvalidPageToken
	}
	return Depth2Token{
		Origin:  w.Origin,
		H1Done:  w.H1Done,
		Front:   w.Front,
		Node:    w.Node,
		NodeCur: w.NodeCur,
	}, nil
}
