package crypto

import (
	"bytes"
	"context"
	"testing"
)

// TestKeyRotationRewrap proves the R-22 rotation mechanic end to end at the crypto layer: a value
// sealed under KEK A can be opened by a rotated provider that keeps A as a *previous* key, the DEK can
// be re-wrapped onto the active KEK B (payload ciphertext untouched), and afterwards a provider holding
// ONLY B opens the re-wrapped envelope but NOT the original — i.e. the rewrap actually moved the row off
// A, which is what lets the operator retire A.
func TestKeyRotationRewrap(t *testing.T) {
	ctx := context.Background()
	blind := []byte("blind-index-key-0123456789")
	keyA := bytes.Repeat([]byte{0xA1}, kekLen)
	keyB := bytes.Repeat([]byte{0xB2}, kekLen)
	plaintext := []byte("4111111111111111")

	// Seal under A.
	provA, err := NewLocalDevProvider(keyA)
	if err != nil {
		t.Fatal(err)
	}
	cipherA, err := NewCipher(provA, blind, 0)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := cipherA.Seal(ctx, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// Rotate: active B, previous [A].
	provB, err := NewLocalDevProviderWithPrevious(keyB, [][]byte{keyA})
	if err != nil {
		t.Fatal(err)
	}
	cipherB, err := NewCipher(provB, blind, 0)
	if err != nil {
		t.Fatal(err)
	}

	// The rotated provider opens the A-sealed value via the previous key, and its active key_ref differs.
	if got, err := cipherB.Open(ctx, sealed); err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("rotated provider must open A-sealed value: got %q err %v", got, err)
	}
	if cipherB.ActiveKeyRef() == sealed.KeyRef {
		t.Fatalf("active key_ref must change after rotation (was %s)", sealed.KeyRef)
	}

	// Rewrap the DEK onto B; the payload ciphertext is reused verbatim.
	newWrapped, newKeyRef, err := cipherB.Rewrap(ctx, sealed.WrappedDEK)
	if err != nil {
		t.Fatal(err)
	}
	if newKeyRef != cipherB.ActiveKeyRef() {
		t.Fatalf("rewrap must stamp the active key_ref, got %s want %s", newKeyRef, cipherB.ActiveKeyRef())
	}
	rewrapped := Sealed{Ciphertext: sealed.Ciphertext, WrappedDEK: newWrapped, KeyRef: newKeyRef}
	if got, err := cipherB.Open(ctx, rewrapped); err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("re-wrapped value must still open: got %q err %v", got, err)
	}

	// A provider holding ONLY B proves the row moved: it opens the re-wrapped envelope but not the original.
	provBonly, err := NewLocalDevProvider(keyB)
	if err != nil {
		t.Fatal(err)
	}
	cipherBonly, err := NewCipher(provBonly, blind, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := cipherBonly.Open(ctx, rewrapped); err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("B-only provider must open the re-wrapped value: got %q err %v", got, err)
	}
	if _, err := cipherBonly.Open(ctx, sealed); err == nil {
		t.Fatal("B-only provider must NOT open the original A-sealed value (rotation would be a no-op otherwise)")
	}
}

// TestBlindIndexReindexAfterKeyChange proves the blind-index rotation core: after the blind-index key
// changes, decrypting the value and recomputing the index yields the new-key index (deterministic), so
// the CLI's decrypt→BlindIndex reindex pass is correct and idempotent.
func TestBlindIndexReindexAfterKeyChange(t *testing.T) {
	ctx := context.Background()
	kek := bytes.Repeat([]byte{0xC3}, kekLen)
	prov, err := NewLocalDevProvider(kek)
	if err != nil {
		t.Fatal(err)
	}
	value := []byte("DE89370400440532013000")

	oldCipher, err := NewCipher(prov, []byte("old-blind-index-key"), 0)
	if err != nil {
		t.Fatal(err)
	}
	newCipher, err := NewCipher(prov, []byte("new-blind-index-key"), 0)
	if err != nil {
		t.Fatal(err)
	}

	sealed, err := oldCipher.Seal(ctx, value)
	if err != nil {
		t.Fatal(err)
	}
	oldIndex := oldCipher.BlindIndex(value)

	// Reindex the way the CLI does: open with the (rotated) cipher, recompute under the new key.
	pt, err := newCipher.Open(ctx, sealed)
	if err != nil {
		t.Fatal(err)
	}
	got := newCipher.BlindIndex(pt)
	if bytes.Equal(got, oldIndex) {
		t.Fatal("reindex must produce a different blind index under a new key")
	}
	if !bytes.Equal(got, newCipher.BlindIndex(value)) {
		t.Fatal("reindex must be deterministic for the new key + value")
	}
}
