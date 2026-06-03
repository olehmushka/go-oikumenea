package application

import (
	"testing"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
)

func TestTokenRoundTrip(t *testing.T) {
	want := domain.Cursor{
		CreatedAt: time.Date(2026, 6, 1, 12, 34, 56, 789, time.UTC),
		ID:        "urn:oikumenea:audit:local:action__test:0192f3a1-0000-7000-8000-000000000000",
	}
	got, err := decodeToken(encodeToken(want))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got == nil || !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestDecodeEmptyTokenIsNilCursor(t *testing.T) {
	got, err := decodeToken("")
	if err != nil || got != nil {
		t.Fatalf("empty token should decode to (nil, nil), got (%+v, %v)", got, err)
	}
}

func TestDecodeGarbageTokenErrors(t *testing.T) {
	for _, tok := range []string{"!!!not-base64!!!", "Zm9v" /* "foo" — no separator */} {
		if _, err := decodeToken(tok); err == nil {
			t.Fatalf("expected error decoding %q", tok)
		}
	}
}
