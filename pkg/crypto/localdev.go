package crypto

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// kekLen is the local-dev KEK length: AES-256 (mirrors the DEK width).
const kekLen = 32

// LocalDevProvider is the self-hosted, no-external-KMS KeyProvider backend (D-CryptoProvider): it
// wraps DEKs with a symmetric KEK supplied in install config (var/conf/install.yml). It exists so dev,
// tests, and air-gapped deployments run the SAME envelope code path as a real KMS; it is NOT a
// substitute for a managed KMS in production (the KEK then sits in the operator's config rather than a
// hardware/cloud key store). Backends aws-kms/gcp-kms/vault-transit/azure-kv slot in behind the same
// interface without touching the encrypted modules.
//
// It supports **key rotation** (review R-22): besides the single active KEK (used for Wrap + KeyRef),
// it may retain any number of PREVIOUS KEKs used for Unwrap only. Rotation is: promote a new active
// KEK, keep the old one as previous, run `oikumenea rewrap` (re-wraps every DEK under the new active
// KEK, flipping key_ref), then drop the old KEK from config. Unwrap tries the active KEK first, then
// each previous one — AES-GCM's authentication tag makes a wrong-key attempt fail cleanly, so the
// wrapped DEK's own key_ref is not needed to pick the KEK.
type LocalDevProvider struct {
	active []byte   // the KEK used for Wrap + reported by KeyRef
	keks   [][]byte // active first, then previous KEKs (Unwrap fallback order)
	keyRef string
}

// NewLocalDevProvider builds the local-dev backend over a single 32-byte active KEK (no previous keys).
func NewLocalDevProvider(kek []byte) (*LocalDevProvider, error) {
	return NewLocalDevProviderWithPrevious(kek, nil)
}

// NewLocalDevProviderWithPrevious builds the local-dev backend over an active KEK plus zero or more
// previous KEKs retained for Unwrap during a rotation (review R-22). The KeyRef embeds a short,
// non-reversible fingerprint of the ACTIVE KEK ("local-dev:<fp8>") so a key change is visible in
// persisted key_ref values; previous KEKs never appear in KeyRef and are never used to Wrap.
func NewLocalDevProviderWithPrevious(active []byte, previous [][]byte) (*LocalDevProvider, error) {
	if len(active) != kekLen {
		return nil, fmt.Errorf("crypto: local-dev active KEK must be %d bytes, got %d", kekLen, len(active))
	}
	keks := make([][]byte, 0, 1+len(previous))
	keks = append(keks, cloneKEK(active))
	for i, p := range previous {
		if len(p) != kekLen {
			return nil, fmt.Errorf("crypto: local-dev previous KEK #%d must be %d bytes, got %d", i, kekLen, len(p))
		}
		keks = append(keks, cloneKEK(p))
	}
	return &LocalDevProvider{active: keks[0], keks: keks, keyRef: keyRefFor(keks[0])}, nil
}

func cloneKEK(k []byte) []byte {
	cp := make([]byte, len(k))
	copy(cp, k)
	return cp
}

// keyRefFor derives the persisted key reference for a KEK: "local-dev:" + the first 4 bytes of its
// SHA-256 (a stable, non-reversible fingerprint). Used to detect which rows a rotation still has to
// rewrap (rows whose key_ref differs from the active KEK's).
func keyRefFor(kek []byte) string {
	sum := sha256.Sum256(kek)
	return "local-dev:" + hex.EncodeToString(sum[:4])
}

var _ KeyProvider = (*LocalDevProvider)(nil)

// Wrap AES-GCM-encrypts the DEK under the active KEK.
func (p *LocalDevProvider) Wrap(_ context.Context, dek []byte) ([]byte, error) {
	if len(dek) == 0 {
		return nil, errors.New("crypto: empty dek")
	}
	return aeadSeal(p.active, dek)
}

// Unwrap reverses Wrap, trying the active KEK first and then each previous KEK. AES-GCM authentication
// rejects the wrong key, so the first KEK that opens the wrapped DEK produced it.
func (p *LocalDevProvider) Unwrap(_ context.Context, wrapped []byte) ([]byte, error) {
	var lastErr error
	for _, kek := range p.keks {
		dek, err := aeadOpen(kek, wrapped)
		if err == nil {
			return dek, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("crypto: no KEK configured")
	}
	return nil, fmt.Errorf("crypto: unwrap dek with %d local-dev KEK(s): %w", len(p.keks), lastErr)
}

// KeyRef returns the active KEK reference (id + fingerprint).
func (p *LocalDevProvider) KeyRef() string { return p.keyRef }
