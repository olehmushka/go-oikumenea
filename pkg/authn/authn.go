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

import "context"

// Subject is the resolved PDP context attached to an authenticated request. PersonID is the PDP
// subject the authorization layer decides on; AccountID/Email are the login attachment it came
// through (empty for out-of-band/system contexts that set only a person).
type Subject struct {
	PersonID  string
	AccountID string
	Email     string
}

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
