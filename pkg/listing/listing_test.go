// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package listing

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

const sampleRID = "0192f3a1-4b2c-8def-9012-3456789abcde"

func TestEncodeDecodeCursorRoundTrip(t *testing.T) {
	for _, id := range []string{sampleRID, "a", strings.Repeat("x", 512)} {
		tok := EncodeCursor(id)
		got, err := DecodeCursor(tok)
		if err != nil {
			t.Fatalf("DecodeCursor(%q) error: %v", tok, err)
		}
		if got != id {
			t.Fatalf("round-trip: got %q want %q", got, id)
		}
	}
}

// A token travels in a query parameter, so it must never contain '+', '/' or '='. This is the bug
// the six pre-M56 StdEncoding transports carried: '+' decodes to a space in a query string.
func TestEncodeCursorIsURLSafe(t *testing.T) {
	// 0xFB 0xFF encodes to "+/" under the standard alphabet — the exact byte pair that breaks.
	for _, id := range []string{sampleRID, string([]byte{0xfb, 0xff}), strings.Repeat("\xfb\xff", 8)} {
		tok := EncodeCursor(id)
		if strings.ContainsAny(tok, "+/=") {
			t.Fatalf("EncodeCursor(%q) = %q contains a non-URL-safe character", id, tok)
		}
	}
}

func TestEmptyCursorIsFirstPageNotAnError(t *testing.T) {
	if got := EncodeCursor(""); got != "" {
		t.Fatalf("EncodeCursor(\"\") = %q, want empty", got)
	}
	got, err := DecodeCursor("")
	if err != nil || got != "" {
		t.Fatalf("DecodeCursor(\"\") = (%q, %v), want (\"\", nil)", got, err)
	}
	got, err = DecodeCursorPtr(nil)
	if err != nil || got != "" {
		t.Fatalf("DecodeCursorPtr(nil) = (%q, %v), want (\"\", nil)", got, err)
	}
}

// Tokens minted by the pre-M56 StdEncoding transports (company, education, externalorg, finance,
// religion, vehicle) must survive the upgrade — a client holding one mid-listing keeps paging.
func TestDecodeCursorAcceptsEveryBase64Alphabet(t *testing.T) {
	raw := []byte(sampleRID)
	for name, enc := range map[string]*base64.Encoding{
		"RawURL": base64.RawURLEncoding,
		"Std":    base64.StdEncoding,
		"URL":    base64.URLEncoding,
		"RawStd": base64.RawStdEncoding,
	} {
		got, err := DecodeCursor(enc.EncodeToString(raw))
		if err != nil {
			t.Fatalf("%s: unexpected error %v", name, err)
		}
		if got != sampleRID {
			t.Fatalf("%s: got %q want %q", name, got, sampleRID)
		}
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, tok := range []string{"!!!not base64!!!", "a b c", "%%%%"} {
		if _, err := DecodeCursor(tok); !errors.Is(err, ErrInvalidPageToken) {
			t.Fatalf("DecodeCursor(%q) error = %v, want ErrInvalidPageToken", tok, err)
		}
	}
}

func TestTupleRoundTrip(t *testing.T) {
	parts := []string{"2026-07-27T10:11:12.13579Z", sampleRID}
	got, err := DecodeTuple(EncodeTuple(parts...), 2)
	if err != nil {
		t.Fatalf("DecodeTuple error: %v", err)
	}
	if len(got) != 2 || got[0] != parts[0] || got[1] != parts[1] {
		t.Fatalf("got %q want %q", got, parts)
	}
}

// A tuple token of the wrong arity is one this endpoint did not issue (or issued under a different
// ORDER BY) — reject it rather than silently truncate to a wrong keyset position.
func TestDecodeTupleRejectsWrongArityAndEmptyParts(t *testing.T) {
	if _, err := DecodeTuple(EncodeTuple("a", "b", "c"), 2); !errors.Is(err, ErrInvalidPageToken) {
		t.Fatalf("arity 3 into 2: err = %v, want ErrInvalidPageToken", err)
	}
	if _, err := DecodeTuple(EncodeTuple("a"), 2); !errors.Is(err, ErrInvalidPageToken) {
		t.Fatalf("arity 1 into 2: err = %v, want ErrInvalidPageToken", err)
	}
	if _, err := DecodeTuple(EncodeTuple("a", ""), 2); !errors.Is(err, ErrInvalidPageToken) {
		t.Fatalf("empty part: err = %v, want ErrInvalidPageToken", err)
	}
	if got, err := DecodeTuple("", 2); err != nil || got != nil {
		t.Fatalf("DecodeTuple(\"\", 2) = (%v, %v), want (nil, nil)", got, err)
	}
}

// A single-column cursor and a 1-tuple are the same bytes, so the two codecs interoperate.
func TestCursorAndOneTupleAgree(t *testing.T) {
	if EncodeCursor(sampleRID) != EncodeTuple(sampleRID) {
		t.Fatal("EncodeCursor and EncodeTuple disagree on a single part")
	}
}

func TestPageSizeResolve(t *testing.T) {
	p := PageSize{Default: 50, Max: 500}
	for _, tc := range []struct{ in, want int }{
		{0, 50}, {-1, 50}, {1, 1}, {50, 50}, {499, 499}, {500, 500}, {501, 500}, {1 << 20, 500},
	} {
		if got := p.Resolve(tc.in); got != tc.want {
			t.Fatalf("Resolve(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
	if got := p.ResolvePtr(nil); got != 50 {
		t.Fatalf("ResolvePtr(nil) = %d, want 50", got)
	}
	n := 900
	if got := p.ResolvePtr(&n); got != 500 {
		t.Fatalf("ResolvePtr(900) = %d, want 500", got)
	}
}

// Max == 0 means "no ceiling"; a module that clamps only the lower bound must not be capped to zero.
func TestPageSizeUncapped(t *testing.T) {
	p := PageSize{Default: 25}
	if got := p.Resolve(10_000); got != 10_000 {
		t.Fatalf("uncapped Resolve(10000) = %d, want 10000", got)
	}
	if got := p.Resolve(0); got != 25 {
		t.Fatalf("uncapped Resolve(0) = %d, want 25", got)
	}
}
