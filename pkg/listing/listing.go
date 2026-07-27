// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package listing is the shared kernel for token-paginated list endpoints (M56 / D-ObjectFacets):
// one keyset-cursor codec and one page-size clamp, replacing the per-module copies that had drifted
// into three incompatible shapes.
//
// It is a leaf: stdlib only, no imports from internal/ or any other pkg/, so every module's
// application AND transport layer can use it without a dependency cycle.
//
// # The cursor
//
// A page token is the opaque, URL-safe base64 of the keyset position — the last row's RID for a
// single-column keyset, or an ASCII-unit-separator-joined tuple for a composite one (audit's
// (created_at, id); geo's (distance, id)). It is purely positional and carries no privileged data,
// but it is still OPAQUE by contract: callers must round-trip it verbatim, never construct one.
//
// Encoding is always base64 **RawURL** (unpadded, `-`/`_` alphabet), because a token travels in a
// query parameter. Six transport packages previously emitted StdEncoding, whose `+` and `/` and `=`
// are not URL-safe — a `+` in a query string decodes to a space, corrupting the cursor. Decode is
// therefore deliberately TOLERANT: it accepts all four base64 alphabets, so tokens issued by the
// pre-M56 StdEncoding endpoints keep working across the upgrade while every newly issued token is
// URL-safe.
//
// # Page size
//
// PageSize carries a module's own Default and Max rather than imposing one pair, because the
// existing modules legitimately differ (the M0-M11 core clamps at 500, the M18+ verticals at 200).
// A module declares its policy once as a package-level var and clamps through it.
package listing

import (
	"encoding/base64"
	"errors"
	"strings"
)

// ErrInvalidPageToken reports a page token this service did not issue (or that was corrupted in
// transit). Transports map it to INVALID_ARGUMENT.
var ErrInvalidPageToken = errors.New("listing: invalid page token")

// sep joins the parts of a composite keyset cursor. ASCII US (unit separator) cannot occur in a RID,
// an RFC 3339 timestamp or a formatted float, so it needs no escaping.
const sep = "\x1f"

// EncodeCursor makes a single-column keyset position (the last row's id) into an opaque, URL-safe
// page token. An empty id yields an empty token, which callers treat as "no next page".
func EncodeCursor(id string) string {
	if id == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

// DecodeCursor parses a token previously issued by EncodeCursor. An empty token means "first page"
// and returns ("", nil) — the absence of a cursor is not an error. A token that is not valid base64
// in any alphabet returns ErrInvalidPageToken.
func DecodeCursor(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	raw, err := decodeBase64(token)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// DecodeCursorPtr is DecodeCursor over an optional Conjure query arg; a nil pointer is the first page.
func DecodeCursorPtr(token *string) (string, error) {
	if token == nil {
		return "", nil
	}
	return DecodeCursor(*token)
}

// EncodeTuple makes a COMPOSITE keyset position into one opaque token — for a keyset ordered by more
// than one column, e.g. audit's (created_at, id) or a nearest-first (distance, id). Part order is
// the ORDER BY order. Encoding no parts yields an empty token.
func EncodeTuple(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, sep)))
}

// DecodeTuple parses a token issued by EncodeTuple and requires exactly n parts — a token with the
// wrong arity is one this endpoint did not issue (or issued under a different ordering), so it is
// rejected rather than silently truncated. An empty token means "first page" and returns (nil, nil).
func DecodeTuple(token string, n int) ([]string, error) {
	if token == "" {
		return nil, nil
	}
	raw, err := decodeBase64(token)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(raw), sep)
	if len(parts) != n {
		return nil, ErrInvalidPageToken
	}
	for _, p := range parts {
		if p == "" {
			return nil, ErrInvalidPageToken
		}
	}
	return parts, nil
}

// decodeBase64 accepts any of the four base64 alphabets, preferring RawURL (what this package
// emits). The fallbacks keep tokens issued by the pre-M56 StdEncoding transports decodable; without
// them an upgrade would invalidate every page token a client was holding mid-listing.
func decodeBase64(token string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding,
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
	} {
		if raw, err := enc.DecodeString(token); err == nil {
			return raw, nil
		}
	}
	return nil, ErrInvalidPageToken
}

// PageSize is one endpoint family's page-size policy: the size used when the caller asks for none
// (or a non-positive one), and the ceiling a caller cannot exceed.
type PageSize struct {
	Default int
	Max     int
}

// Resolve clamps a caller-supplied page size into the policy. A non-positive request means "no
// preference" and gets Default; anything above Max is capped rather than rejected, so a client
// asking for more simply gets the maximum page.
func (p PageSize) Resolve(requested int) int {
	if requested <= 0 {
		return p.Default
	}
	if p.Max > 0 && requested > p.Max {
		return p.Max
	}
	return requested
}

// ResolvePtr is Resolve over an optional Conjure query arg; a nil pointer means "no preference".
func (p PageSize) ResolvePtr(requested *int) int {
	if requested == nil {
		return p.Default
	}
	return p.Resolve(*requested)
}
