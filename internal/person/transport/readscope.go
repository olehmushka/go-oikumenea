// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"

	personapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/person"
)

// The person read scope (D-PersonReadScope), owned by these two helpers and nothing else in this
// package (M60 ticket 0).
//
// Until M60 ticket 0 only GetPerson applied the projection. Every other `GET /persons/{personId}/…`
// handler — emails, phones, addresses, health, legal, ethnicity, watchlist match, political leaning,
// the six family/social relationship lists, … — gated its permission code ANYWHERE and returned the
// rows, and no person/personprofile/personsensitive table carries an RLS policy to backstop it. So one
// grant anywhere read any person's sub-resources instance-wide. The drift guard in readscope_test.go
// holds every such handler, enumerated from the Conjure contract, to requireReadablePerson.

// requireReadablePerson admits the caller to one named person's data: an instance admin always, any
// other subject only when the person falls in their readable reach (the person service's SQL point
// probe, R-02.1). An unreadable person is reported as PersonNotFound — exactly GetPerson's answer — so
// a sub-resource endpoint cannot confirm the existence of a person the caller cannot see.
func (s Service) requireReadablePerson(ctx context.Context, personID string) error {
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return s.mapError(ctx, err, personID)
	}
	if isAdmin {
		return nil
	}
	ok, err := s.app.ReadablePerson(ctx, subject, personID)
	if err != nil {
		return s.mapError(ctx, err, personID)
	}
	if !ok {
		return personapi.NewPersonNotFound(personID)
	}
	return nil
}

// trimByCounterpart drops the relationship rows whose counterpart — the endpoint that is not the path
// person — the caller cannot read (the D-VisibilityScope trim the links engine applies to the same
// links). endpoints returns a row's two person ids; one batch probe covers the whole list.
func trimByCounterpart[T any](ctx context.Context, s Service, personID string, rows []T, endpoints func(T) (string, string)) ([]T, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return nil, err
	}
	if isAdmin {
		return rows, nil
	}
	counterpart := func(r T) string {
		a, b := endpoints(r)
		if a == personID {
			return b
		}
		return a
	}
	ids := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		id := counterpart(r)
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	readable, err := s.app.ReadablePersonsAmong(ctx, subject, ids)
	if err != nil {
		return nil, err
	}
	ok := make(map[string]struct{}, len(readable))
	for _, id := range readable {
		ok[id] = struct{}{}
	}
	out := rows[:0:0]
	for _, r := range rows {
		if _, keep := ok[counterpart(r)]; keep {
			out = append(out, r)
		}
	}
	return out, nil
}
