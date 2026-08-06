// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/identityfederation/domain"
)

// stubResolver satisfies the whole Resolver port; matchPerson only exercises the JIT-match method, so
// the other two are unreachable in these tests and say so rather than pretending to work.
type stubResolver struct {
	byEmailArg  string
	byEmailHits map[string]string // email -> personID
}

func (s *stubResolver) PersonIDByAccountEmail(_ context.Context, email string) (string, bool, error) {
	s.byEmailArg = email
	id, ok := s.byEmailHits[email]
	return id, ok, nil
}

func (s *stubResolver) Resolve(context.Context, string, string) (domain.Resolution, error) {
	return domain.Resolution{}, errors.New("Resolve is not part of the JIT match path")
}

func (s *stubResolver) LinkOnMatch(context.Context, string, string, string, string) (domain.Resolution, error) {
	return domain.Resolution{}, errors.New("LinkOnMatch runs after the match, not during it")
}

type stubPersons struct {
	byCodeArg  string
	byCodeHits map[string]string // code -> personID
}

func (s *stubPersons) PersonIDByCode(_ context.Context, code string) (string, bool, error) {
	s.byCodeArg = code
	id, ok := s.byCodeHits[code]
	return id, ok, nil
}

// TestJITMatchArms pins WHICH person key each configured match mode consults. The two arms read the
// same claim value, so a mis-wired mode does not fail loudly — it quietly looks the address up as a
// person.code, finds nothing, and rejects every login with the same "unknown identity" as a genuine
// miss. That is indistinguishable from correct reject-unknown behaviour, hence this test.
func TestJITMatchArms(t *testing.T) {
	const email, code, personID = "someone@example.test", "p-42", "019f-person"

	cases := []struct {
		name         string
		match        string
		claims       Claims
		wantPersonID string
		wantOK       bool
		wantCodeArg  string
		wantEmailArg string
	}{
		{
			name:         "code arm (default) matches person.code",
			match:        JITMatchCode,
			claims:       Claims{JITValue: code},
			wantPersonID: personID, wantOK: true, wantCodeArg: code,
		},
		{
			name:         "account-email arm matches the account email",
			match:        JITMatchAccountEmail,
			claims:       Claims{JITValue: email, Email: email, EmailVerified: true},
			wantPersonID: personID, wantOK: true, wantEmailArg: email,
		},
		{
			// The security property of this arm. An unverified address is an unproven assertion; if it
			// matched, anyone able to claim someone else's address at the IdP would take over the
			// account the operator prepared for them.
			name:   "account-email REJECTS an unverified email",
			match:  JITMatchAccountEmail,
			claims: Claims{JITValue: email, Email: email, EmailVerified: false},
			wantOK: false,
		},
		{
			name:   "account-email with no match rejects",
			match:  JITMatchAccountEmail,
			claims: Claims{JITValue: "nobody@example.test", Email: "nobody@example.test", EmailVerified: true},
			wantOK: false, wantEmailArg: "nobody@example.test",
		},
		{
			// An unknown mode must fall back to the historical arm, never to the email one: a typo in
			// install.yml must not silently switch on email matching.
			name:         "an unrecognized mode falls back to the code arm",
			match:        "not-a-mode",
			claims:       Claims{JITValue: code},
			wantPersonID: personID, wantOK: true, wantCodeArg: code,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := &stubResolver{byEmailHits: map[string]string{email: personID}}
			persons := &stubPersons{byCodeHits: map[string]string{code: personID}}
			b := &bound{
				resolver: res,
				persons:  persons,
				jitMatch: NewValidator(Config{JITMatch: tc.match}).JITMatch(),
			}

			got, ok, err := b.matchPerson(context.Background(), tc.claims)
			if err != nil {
				t.Fatalf("matchPerson: %v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("matched=%v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.wantPersonID {
				t.Fatalf("personID=%q, want %q", got, tc.wantPersonID)
			}
			if persons.byCodeArg != tc.wantCodeArg {
				t.Errorf("code arm consulted with %q, want %q", persons.byCodeArg, tc.wantCodeArg)
			}
			if res.byEmailArg != tc.wantEmailArg {
				t.Errorf("email arm consulted with %q, want %q", res.byEmailArg, tc.wantEmailArg)
			}
		})
	}
}

// TestJITMatchDefaultsToCode: an install.yml with jit enabled and no `match` key must keep the
// behaviour it had before the attribute arm existed.
func TestJITMatchDefaultsToCode(t *testing.T) {
	if got := NewValidator(Config{}).JITMatch(); got != JITMatchCode {
		t.Fatalf("unset match = %q, want %q", got, JITMatchCode)
	}
	if got := NewValidator(Config{JITMatch: JITMatchAccountEmail}).JITMatch(); got != JITMatchAccountEmail {
		t.Fatalf("explicit account-email was not honoured: %q", got)
	}
}
