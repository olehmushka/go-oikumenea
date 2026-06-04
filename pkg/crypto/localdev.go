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
// interface without touching the document module.
type LocalDevProvider struct {
	kek    []byte
	keyRef string
}

// NewLocalDevProvider builds the local-dev backend over a 32-byte KEK. The KeyRef embeds a short,
// non-reversible fingerprint of the KEK so a key change is visible in persisted key_ref values
// ("local-dev:<fp8>"), supporting later rotation/rewrap detection.
func NewLocalDevProvider(kek []byte) (*LocalDevProvider, error) {
	if len(kek) != kekLen {
		return nil, fmt.Errorf("crypto: local-dev KEK must be %d bytes, got %d", kekLen, len(kek))
	}
	cp := make([]byte, len(kek))
	copy(cp, kek)
	sum := sha256.Sum256(cp)
	return &LocalDevProvider{kek: cp, keyRef: "local-dev:" + hex.EncodeToString(sum[:4])}, nil
}

var _ KeyProvider = (*LocalDevProvider)(nil)

// Wrap AES-GCM-encrypts the DEK under the KEK.
func (p *LocalDevProvider) Wrap(_ context.Context, dek []byte) ([]byte, error) {
	if len(dek) == 0 {
		return nil, errors.New("crypto: empty dek")
	}
	return aeadSeal(p.kek, dek)
}

// Unwrap reverses Wrap.
func (p *LocalDevProvider) Unwrap(_ context.Context, wrapped []byte) ([]byte, error) {
	return aeadOpen(p.kek, wrapped)
}

// KeyRef returns the active KEK reference (id + fingerprint).
func (p *LocalDevProvider) KeyRef() string { return p.keyRef }
