package application

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
)

// The page token is the opaque encoding of the keyset cursor (created_at, id) of the last entry on
// the previous page (API conventions: token-based pagination, no offset). It is base64url over
// "<RFC3339Nano>\x1f<actionRID>" — purely positional, carrying no privileged data.
const tokenSep = "\x1f"

// ErrInvalidPageToken is returned when a supplied pageToken cannot be decoded (mapped to
// INVALID_ARGUMENT by transport).
var ErrInvalidPageToken = errors.New("invalid page token")

func encodeToken(c domain.Cursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + tokenSep + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeToken(token string) (*domain.Cursor, error) {
	if token == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, errors.Join(ErrInvalidPageToken, err)
	}
	ts, id, ok := strings.Cut(string(raw), tokenSep)
	if !ok || id == "" {
		return nil, ErrInvalidPageToken
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, errors.Join(ErrInvalidPageToken, err)
	}
	return &domain.Cursor{CreatedAt: createdAt, ID: id}, nil
}
