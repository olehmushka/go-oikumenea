package application

import (
	"errors"
	"fmt"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
)

// The page token is the opaque encoding of the keyset cursor (created_at, id) of the last entry on
// the previous page (API conventions: token-based pagination, no offset) — a COMPOSITE cursor, so it
// rides pkg/listing's tuple codec rather than the single-column one. It is purely positional,
// carrying no privileged data.

// ErrInvalidPageToken is returned when a supplied pageToken cannot be decoded (mapped to
// INVALID_ARGUMENT by transport). It wraps listing.ErrInvalidPageToken so callers may match either.
var ErrInvalidPageToken = fmt.Errorf("audit: %w", listing.ErrInvalidPageToken)

func encodeToken(c domain.Cursor) string {
	return listing.EncodeTuple(c.CreatedAt.UTC().Format(time.RFC3339Nano), c.ID)
}

func decodeToken(token string) (*domain.Cursor, error) {
	parts, err := listing.DecodeTuple(token, 2)
	if err != nil {
		return nil, errors.Join(ErrInvalidPageToken, err)
	}
	if parts == nil { // empty token = first page
		return nil, nil
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, errors.Join(ErrInvalidPageToken, err)
	}
	return &domain.Cursor{CreatedAt: createdAt, ID: parts[1]}, nil
}
