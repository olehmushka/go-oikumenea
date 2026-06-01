// Package id holds UUIDv7 helpers that mirror the SQL oikumenea.uuid_v7() generator
// (docs/modules/platform.md). UUIDv7 is the time-ordered crypto component inside every RID
// (D-ResourceIdentifiers); having a Go-side generator lets application code mint ids without a DB
// round-trip when needed.
package id

import (
	"github.com/google/uuid"
)

// NewV7 returns a time-ordered UUIDv7 in canonical lowercase form.
func NewV7() (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
