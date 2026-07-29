// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"fmt"
	"strings"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

// personFilter assembles the listPersons query args into the one PersonFilter both list paths take
// (M56 / D-ObjectFacets). It performs only the parsing the wire types force — calendar dates arrive
// as `YYYY-MM-DD` strings, per the contract's date convention (Conjure `datetime` is an instant and
// is deliberately not used for a birthdate). Every VALUE check lives in PersonFilter.Validate, run
// once in the application layer, so the admin and read-scope paths reject identically.
func personFilter(query, sex, status, birthdateFrom, birthdateTo, countryOfBirth, rankID, unitID, graph *string, hasAccount *bool) (domain.PersonFilter, error) {
	from, err := parseFacetDate("birthdateFrom", birthdateFrom)
	if err != nil {
		return domain.PersonFilter{}, err
	}
	to, err := parseFacetDate("birthdateTo", birthdateTo)
	if err != nil {
		return domain.PersonFilter{}, err
	}
	return domain.PersonFilter{
		Query:          derefOr(query, ""),
		Sex:            sex,
		Status:         status,
		BirthdateFrom:  from,
		BirthdateTo:    to,
		CountryOfBirth: countryOfBirth,
		RankID:         rankID,
		UnitID:         unitID,
		Graph:          strings.TrimSpace(derefOr(graph, "")),
		HasAccount:     hasAccount,
	}, nil
}

// parseFacetDate reads an ISO-8601 calendar date bound. A malformed value is ErrInvalid (mapped to
// Person:PersonInvalid) rather than a 500 or a silently-ignored filter — silently dropping an
// unparseable bound would return MORE rows than the caller asked for, which is the dangerous
// direction for a filter to fail in.
func parseFacetDate(arg string, v *string) (*time.Time, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	t, err := time.Parse(domain.ISODate, strings.TrimSpace(*v))
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be an ISO-8601 calendar date (YYYY-MM-DD)", domain.ErrInvalid, arg)
	}
	return &t, nil
}
