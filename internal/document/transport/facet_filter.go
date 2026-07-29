// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"fmt"
	"strings"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/document/domain"
)

// documentFilter assembles the listDocuments query args into the one DocumentFilter both list paths
// take (M56 ticket 3 / D-ObjectFacets). It performs only the parsing the wire types force —
// calendar bounds arrive as `YYYY-MM-DD` strings. Every VALUE check lives in DocumentFilter.Validate,
// run once in the application layer, so the admin and holder-scoped paths reject identically.
func documentFilter(typeID, status, issuingCountryID, issuedOnFrom, issuedOnTo, expiresOnFrom, expiresOnTo *string) (domain.DocumentFilter, error) {
	var (
		f   domain.DocumentFilter
		err error
	)
	for _, b := range []struct {
		arg string
		in  *string
		out **time.Time
	}{
		{"issuedOnFrom", issuedOnFrom, &f.IssuedOnFrom},
		{"issuedOnTo", issuedOnTo, &f.IssuedOnTo},
		{"expiresOnFrom", expiresOnFrom, &f.ExpiresOnFrom},
		{"expiresOnTo", expiresOnTo, &f.ExpiresOnTo},
	} {
		if *b.out, err = parseFacetDate(b.arg, b.in); err != nil {
			return domain.DocumentFilter{}, err
		}
	}
	f.TypeID = trimmedPtr(typeID)
	f.Status = trimmedPtr(status)
	f.IssuingCountryID = trimmedPtr(issuingCountryID)
	return f, nil
}

// parseFacetDate reads an ISO-8601 calendar date bound. A malformed value is ErrDocumentInvalid
// (mapped to Document:DocumentInvalid) rather than a 500 or a silently-ignored filter — silently
// dropping an unparseable bound would return MORE rows than the caller asked for, which is the
// dangerous direction for a filter to fail in.
func parseFacetDate(arg string, v *string) (*time.Time, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	t, err := time.Parse(domain.ISODate, strings.TrimSpace(*v))
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be an ISO-8601 calendar date (YYYY-MM-DD)", domain.ErrDocumentInvalid, arg)
	}
	return &t, nil
}

// trimmedPtr normalizes an optional string arg: absent, or blank after trimming, both mean "no
// filter". A blank arg must NOT become a filter for the empty string, which would match nothing and
// silently return an empty page.
func trimmedPtr(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	if t == "" {
		return nil
	}
	return &t
}
