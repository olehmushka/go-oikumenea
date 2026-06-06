// Package crypto is the envelope-encryption seam for pii:sensitive data (D-CryptoProvider): today the
// national-identifier values held by the document module (docs/modules/platform.md). The model is
// fixed — ciphertext lives in Postgres; a per-record data-encryption key (DEK) encrypts the value and
// is itself wrapped by a key-encryption key (KEK) that never leaves the operator's KMS — while the KMS
// vendor is a pluggable KeyProvider chosen by install config. A keyed-HMAC blind index gives equality
// lookup / uniqueness without decryption, and a short-TTL cache keeps the KMS off the read hot path.
//
// This package is framework-free (standard library + the KeyProvider seam only) and holds no domain
// logic; the document module composes it over a configured backend.
package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// dekLen is the per-record data-encryption key length: AES-256 (D-CryptoProvider).
const dekLen = 32

// KeyProvider is the pluggable KMS seam: it wraps/unwraps the per-record DEK with a KEK that never
// leaves the provider, and reports the active key reference (id + version) recorded alongside each
// ciphertext. Backends (aws-kms, gcp-kms, vault-transit, azure-kv, local-dev) are selected by install
// config; only local-dev ships today. The KMS is on the unwrap (read) path only.
type KeyProvider interface {
	// Wrap encrypts a freshly generated DEK with the KEK, returning the opaque wrapped form persisted
	// as wrapped_dek.
	Wrap(ctx context.Context, dek []byte) ([]byte, error)
	// Unwrap reverses Wrap, recovering the DEK from its wrapped form.
	Unwrap(ctx context.Context, wrapped []byte) ([]byte, error)
	// KeyRef is the active KEK id + version, persisted as key_ref so a later rotation/rewrap knows
	// which KEK produced a given wrapped_dek.
	KeyRef() string
}

// Sealed is the persisted envelope for one protected value: the AES-GCM ciphertext of the value, the
// wrapped DEK, and the KEK reference that produced it. These map 1:1 onto the document_personal_codes
// value_ciphertext / wrapped_dek / key_ref columns. Crypto-erase (person purge) destroys WrappedDEK
// (and may null Ciphertext), rendering the value unrecoverable.
type Sealed struct {
	Ciphertext []byte
	WrappedDEK []byte
	KeyRef     string
}

// Cipher performs envelope seal/open over a KeyProvider plus a keyed-HMAC blind index. It is safe for
// concurrent use.
type Cipher struct {
	provider KeyProvider
	blindKey []byte
	cache    *dekCache
}

// ErrBlindIndexKeyRequired is returned by NewCipher when no blind-index HMAC key is supplied: equality
// lookup / uniqueness over ciphertext depends on a stable keyed index, so it is mandatory.
var ErrBlindIndexKeyRequired = errors.New("crypto: blind-index key is required")

// NewCipher builds an envelope cipher over the given KeyProvider and blind-index HMAC key, caching
// unwrapped DEKs for cacheTTL (a non-positive TTL disables caching — the KMS is consulted on every
// open). The blind-index key must be non-empty.
func NewCipher(provider KeyProvider, blindIndexKey []byte, cacheTTL time.Duration) (*Cipher, error) {
	if provider == nil {
		return nil, errors.New("crypto: key provider is required")
	}
	if len(blindIndexKey) == 0 {
		return nil, ErrBlindIndexKeyRequired
	}
	key := make([]byte, len(blindIndexKey))
	copy(key, blindIndexKey)
	return &Cipher{provider: provider, blindKey: key, cache: newDEKCache(cacheTTL)}, nil
}

// Seal envelope-encrypts plaintext: it mints a random per-record DEK, AES-GCM-encrypts the value under
// it, and wraps the DEK with the provider's KEK. The plaintext is never persisted; only the returned
// Sealed envelope is.
func (c *Cipher) Seal(ctx context.Context, plaintext []byte) (Sealed, error) {
	dek := make([]byte, dekLen)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return Sealed{}, fmt.Errorf("crypto: generate dek: %w", err)
	}
	ciphertext, err := aeadSeal(dek, plaintext)
	if err != nil {
		return Sealed{}, err
	}
	wrapped, err := c.provider.Wrap(ctx, dek)
	if err != nil {
		return Sealed{}, fmt.Errorf("crypto: wrap dek: %w", err)
	}
	c.cache.put(wrapped, dek)
	return Sealed{Ciphertext: ciphertext, WrappedDEK: wrapped, KeyRef: c.provider.KeyRef()}, nil
}

// Open reverses Seal, recovering the plaintext. The DEK is taken from the short-TTL cache when present,
// else unwrapped via the provider's KMS.
func (c *Cipher) Open(ctx context.Context, s Sealed) ([]byte, error) {
	dek, ok := c.cache.get(s.WrappedDEK)
	if !ok {
		var err error
		dek, err = c.provider.Unwrap(ctx, s.WrappedDEK)
		if err != nil {
			return nil, fmt.Errorf("crypto: unwrap dek: %w", err)
		}
		c.cache.put(s.WrappedDEK, dek)
	}
	plaintext, err := aeadOpen(dek, s.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("crypto: open value: %w", err)
	}
	return plaintext, nil
}

// BlindIndex is the keyed HMAC-SHA256 of value, used for equality lookup / cross-person uniqueness
// without decryption (D-CryptoProvider). It is deterministic for a given key + value and not reversible
// to the value. Callers normalize the value (e.g. strip separators, upper-case) before indexing so
// that equal identifiers index identically.
func (c *Cipher) BlindIndex(value []byte) []byte {
	mac := hmac.New(sha256.New, c.blindKey)
	mac.Write(value)
	return mac.Sum(nil)
}

// aeadSeal AES-GCM-encrypts plaintext under key, returning nonce||ciphertext.
func aeadSeal(key, plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// aeadOpen reverses aeadSeal over nonce||ciphertext.
func aeadOpen(key, blob []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("crypto: ciphertext too short")
	}
	return gcm.Open(nil, blob[:ns], blob[ns:], nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

// dekCache is a small TTL cache of unwrapped DEKs keyed by the hex of their wrapped form, keeping the
// KMS off the read hot path (D-CryptoProvider). A non-positive TTL disables it.
type dekCache struct {
	ttl time.Duration
	mu  sync.Mutex
	m   map[string]dekEntry
}

type dekEntry struct {
	dek    []byte
	expiry time.Time
}

func newDEKCache(ttl time.Duration) *dekCache {
	return &dekCache{ttl: ttl, m: make(map[string]dekEntry)}
}

func (c *dekCache) put(wrapped, dek []byte) {
	if c.ttl <= 0 {
		return
	}
	cp := make([]byte, len(dek))
	copy(cp, dek)
	c.mu.Lock()
	c.m[hex.EncodeToString(wrapped)] = dekEntry{dek: cp, expiry: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *dekCache) get(wrapped []byte) ([]byte, bool) {
	if c.ttl <= 0 {
		return nil, false
	}
	key := hex.EncodeToString(wrapped)
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiry) {
		delete(c.m, key)
		return nil, false
	}
	return e.dek, true
}
