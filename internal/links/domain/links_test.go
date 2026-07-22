package domain

import (
	"errors"
	"testing"
)

// The depth-2 token round-trips its full state, and depth-1 (v1) / depth-2 (v2) tokens never decode
// as each other — a depth crossing is rejected, not silently mis-resumed.
func TestDepth2TokenRoundTrip(t *testing.T) {
	in := Depth2Token{
		Origin:  map[string]string{"member_of/a": "cur-1", "kin_parent_of/a": "cur-2"},
		H1Done:  true,
		Front:   "00000000-0000-0000-0000-0000000000ff",
		Node:    "00000000-0000-0000-0000-00000000abcd",
		NodeCur: map[string]string{"kin_parent_of/a": "cur-3"},
	}
	got, err := DecodeDepth2Token(EncodeDepth2Token(in))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.H1Done != in.H1Done || got.Front != in.Front || got.Node != in.Node {
		t.Errorf("scalar mismatch: got %+v want %+v", got, in)
	}
	if len(got.Origin) != len(in.Origin) || got.Origin["member_of/a"] != "cur-1" {
		t.Errorf("origin cursors lost: %v", got.Origin)
	}
	if got.NodeCur["kin_parent_of/a"] != "cur-3" {
		t.Errorf("node cursors lost: %v", got.NodeCur)
	}
}

func TestDepth2TokenEmptyIsFirstPage(t *testing.T) {
	got, err := DecodeDepth2Token("")
	if err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if got.H1Done || got.Front != "" || len(got.Origin) != 0 {
		t.Errorf("empty token should be the zero (first-page) state; got %+v", got)
	}
}

func TestTokenVersionsDoNotCross(t *testing.T) {
	v1 := EncodePageToken(map[string]string{"kin_parent_of/a": "cur"})
	v2 := EncodeDepth2Token(Depth2Token{H1Done: true, Front: "x"})

	if _, err := DecodeDepth2Token(v1); !errors.Is(err, ErrInvalidPageToken) {
		t.Errorf("a v1 token must be rejected by the depth-2 decoder; got err=%v", err)
	}
	if _, err := DecodePageToken(v2); !errors.Is(err, ErrInvalidPageToken) {
		t.Errorf("a v2 token must be rejected by the depth-1 decoder; got err=%v", err)
	}
}
