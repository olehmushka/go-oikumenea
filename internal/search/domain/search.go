// Package domain holds the search module's core model (D-UnifiedSearch, review-2026-09 R-26): the
// SearchProvider contract each searchable module registers at composition time, the type-erased hit,
// and the composite keyset page token that federates the per-provider cursors. The module owns no
// tables and mints no RIDs — every hit is another module's object, identified by its self-describing
// RID (D-ResourceIdentifiers).
package domain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// RawHit is one provider-local match before the engine stamps the object type: the object's RID,
// its primary display line, and an optional secondary line (code, MGRS, …; "" = none).
type RawHit struct {
	ID      string
	Label   string
	Snippet string
}

// SearchFunc runs one keyset page of a provider's trigram search. `after` is the provider's own
// cursor ("" = first page; opaque to the engine — raw keyset key or an encoded module token, the
// provider chooses). It returns up to `limit` hits plus the next cursor ("" = exhausted). The
// subject/isAdmin pair lets a provider whose search is visibility-trimmed in SQL (person,
// D-PersonReadScope) pick the right query shape; catalog providers ignore it.
type SearchFunc func(ctx context.Context, subject string, isAdmin bool, query, after string, limit int) ([]RawHit, string, error)

// Provider is one searchable object type's registration (D-UnifiedSearch): the ontology registry
// token, the read permission gating the whole provider (the engine SKIPS a provider the subject
// cannot read), and the search func. PreTrimmed marks a provider whose SearchFunc already trims
// rows to the subject's visibility in SQL, so the engine's post-trim through the registered
// D-VisibilityScope adapter is skipped (person); the Visibility must be registered regardless.
type Provider struct {
	ObjectType     string
	ReadPermission string
	PreTrimmed     bool
	Search         SearchFunc
}

// Hit is the type-erased search result: RawHit + the ontology registry token.
type Hit struct {
	RID        string
	ObjectType string
	Label      string
	Snippet    string
}

// Page is one engine result page: hits grouped by object type in fixed lexicographic type order,
// plus the composite next-page token ("" = every selected provider exhausted).
type Page struct {
	Hits          []Hit
	NextPageToken string
}

// MinQueryLength is the shortest accepted query (mirrors the console palette's gate; a 1-char
// trigram probe is unselective enough to be a different workload).
const MinQueryLength = 2

var (
	// ErrQueryTooShort rejects queries under MinQueryLength.
	ErrQueryTooShort = errors.New("search: query too short")
	// ErrInvalidPageToken rejects tokens this engine did not issue.
	ErrInvalidPageToken = errors.New("search: invalid page token")
	// ErrNoSubject rejects requests without an authenticated subject (fail closed; the per-provider
	// permission gate and visibility trim are meaningless without one).
	ErrNoSubject = errors.New("search: no authenticated subject")
)

// UnknownObjectTypeError names a `types` filter entry with no registered provider.
type UnknownObjectTypeError struct{ ObjectType string }

func (e UnknownObjectTypeError) Error() string {
	return fmt.Sprintf("search: unknown object type %q", e.ObjectType)
}

// pageToken is the wire shape of the composite keyset token: one cursor per NON-EXHAUSTED provider,
// keyed by object type. A provider absent from the map is done; an empty map is never issued (the
// engine returns no token instead). Versioned for forward compatibility.
type pageToken struct {
	V       int               `json:"v"`
	Cursors map[string]string `json:"c"`
}

// EncodePageToken serializes the per-provider cursors ("" when no provider has more).
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
