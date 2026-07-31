// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package authn holds the request-scoped PDP subject: the (person, account) an inbound IdP token
// resolved to (docs/modules/identity-federation.md step 4). It is the neutral seam between the
// PRODUCER — the identity-federation validation middleware, which validates the token and attaches
// the subject — and the CONSUMERS — the authorization PEP (internal/authorization/pep) and any
// handler that reads /whoami. Keeping it framework-free and dependency-free lets the consumer read
// the subject without importing the producer (no import cycle).
//
// Authentication is delegated (L-AuthzOnly): this package carries an ALREADY-VALIDATED identity, it
// never validates anything itself.
package authn

import (
	"context"
)

// Subject is the resolved PDP context attached to an authenticated request. It is EITHER a person
// or a machine, never both:
//
//   - PERSON: PersonID is the PDP subject the authorization layer decides on; AccountID/Email are the
//     login attachment it came through (empty for out-of-band/system contexts that set only a person).
//   - SERVICE PRINCIPAL (M51 / D-ServiceIdentities): PrincipalID + Service (the principal's stable
//     code) are set INSTEAD of PersonID, for a machine caller — a facade with standing of its own or
//     a connector — authenticated by the external IdP's client-credentials grant, or by the
//     shared-secret fallback (D-Hermenea). Its authority is the flat per-principal grant set, it has
//     NO unit reach, and it is audited as a `system` actor naming itself.
//
// Because PersonID stays empty for a machine, every person-shaped PEP path denies a principal at its
// existing empty-subject guard — the safety property is structural, not a check someone must
// remember to write.
type Subject struct {
	PersonID    string
	AccountID   string
	Email       string
	Service     string // the service principal's stable code (D-Code)
	PrincipalID string // the service principal's RID
}

// ServiceHermeneaImporter is the stable code of the hermenea companion's importer principal. It is
// now a REGISTERED principal holding an instance-wide `import.manage` grant like any other (M51);
// the shared-secret path boot-seeds it so that caller resolves to the same subject shape a
// client-credentials caller would.
const ServiceHermeneaImporter = "hermenea-importer"

// ServiceID returns the service principal's CODE attached to ctx (empty when the subject is a person
// or absent).
func ServiceID(ctx context.Context) string {
	s, _ := FromContext(ctx)
	return s.Service
}

// PrincipalID returns the service principal's RID attached to ctx, or "" when the request is a person
// or unauthenticated. It is the authorization key for machine callers (the code is a label).
func PrincipalID(ctx context.Context) string {
	s, _ := FromContext(ctx)
	return s.PrincipalID
}

// IsService reports whether the request is acting as a machine subject.
func IsService(ctx context.Context) bool { return PrincipalID(ctx) != "" }

// ctxKey is the unexported context key type (avoids collisions with other packages' keys).
type ctxKey struct{}

// NewContext returns a copy of ctx carrying the resolved subject. The validation middleware calls
// this after mapping a verified token to a person.
func NewContext(ctx context.Context, s Subject) context.Context {
	return context.WithValue(ctx, ctxKey{}, s)
}

// FromContext returns the subject attached to ctx and whether one was present. Absent means the
// request was not authenticated (no validation middleware ran or it rejected) — consumers treat that
// as no subject (the PEP denies).
func FromContext(ctx context.Context) (Subject, bool) {
	s, ok := ctx.Value(ctxKey{}).(Subject)
	return s, ok
}

// PersonID is the convenience accessor the PEP uses: the resolved subject person RID, or "" when the
// request carries no authenticated subject.
func PersonID(ctx context.Context) string {
	s, _ := FromContext(ctx)
	return s.PersonID
}
