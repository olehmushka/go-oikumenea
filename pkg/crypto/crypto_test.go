package crypto

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func testKEK() []byte {
	k := make([]byte, kekLen)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}

func newTestCipher(t *testing.T, ttl time.Duration) *Cipher {
	t.Helper()
	p, err := NewLocalDevProvider(testKEK())
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	c, err := NewCipher(p, []byte("blind-index-key-0123456789"), ttl)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return c
}

func TestSealOpenRoundTrip(t *testing.T) {
	c := newTestCipher(t, time.Minute)
	ctx := context.Background()
	plaintext := []byte("1234567899")

	sealed, err := c.Seal(ctx, plaintext)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if bytes.Contains(sealed.Ciphertext, plaintext) {
		t.Fatal("ciphertext must not contain the plaintext")
	}
	if len(sealed.WrappedDEK) == 0 || sealed.KeyRef == "" {
		t.Fatal("sealed envelope is missing wrapped DEK or key ref")
	}

	got, err := c.Open(ctx, sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plaintext)
	}
}

func TestSealIsNondeterministic(t *testing.T) {
	c := newTestCipher(t, 0) // cache disabled
	ctx := context.Background()
	a, _ := c.Seal(ctx, []byte("same"))
	b, _ := c.Seal(ctx, []byte("same"))
	if bytes.Equal(a.Ciphertext, b.Ciphertext) {
		t.Fatal("two seals of the same value must differ (random DEK + nonce)")
	}
	if bytes.Equal(a.WrappedDEK, b.WrappedDEK) {
		t.Fatal("each record gets its own wrapped DEK")
	}
}

func TestBlindIndexDeterministicAndKeyed(t *testing.T) {
	c := newTestCipher(t, time.Minute)
	v := []byte("1234567899")
	if !bytes.Equal(c.BlindIndex(v), c.BlindIndex(v)) {
		t.Fatal("blind index must be deterministic for equal values")
	}
	if bytes.Equal(c.BlindIndex(v), c.BlindIndex([]byte("other"))) {
		t.Fatal("blind index must differ for different values")
	}

	p, _ := NewLocalDevProvider(testKEK())
	other, _ := NewCipher(p, []byte("a-different-blind-index-key"), time.Minute)
	if bytes.Equal(c.BlindIndex(v), other.BlindIndex(v)) {
		t.Fatal("blind index must depend on the key")
	}
}

func TestOpenWithDisabledCacheStillWorks(t *testing.T) {
	c := newTestCipher(t, 0)
	ctx := context.Background()
	sealed, _ := c.Seal(ctx, []byte("payload"))
	got, err := c.Open(ctx, sealed)
	if err != nil || string(got) != "payload" {
		t.Fatalf("open without cache: got %q err %v", got, err)
	}
}

func TestNewCipherRequiresBlindKey(t *testing.T) {
	p, _ := NewLocalDevProvider(testKEK())
	if _, err := NewCipher(p, nil, time.Minute); err == nil {
		t.Fatal("expected error for missing blind-index key")
	}
}

func TestLocalDevProviderRejectsShortKEK(t *testing.T) {
	if _, err := NewLocalDevProvider([]byte("too-short")); err == nil {
		t.Fatal("expected error for short KEK")
	}
}

func TestKeyRefReflectsKEK(t *testing.T) {
	p1, _ := NewLocalDevProvider(testKEK())
	other := testKEK()
	other[0] ^= 0xff
	p2, _ := NewLocalDevProvider(other)
	if p1.KeyRef() == p2.KeyRef() {
		t.Fatal("KeyRef must differ for different KEKs")
	}
}
