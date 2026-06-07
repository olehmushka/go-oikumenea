// Package client is the published Go SDK for the go-oikumenea API.
//
// The per-service typed clients live in the generated subpackages under
// github.com/olegamysk/go-oikumenea/client/oikumenea/<module> (e.g. .../oikumenea/person), generated
// from the same Conjure contract as the server (D-Conjure) so the SDK cannot drift from the API.
//
// This file is the only hand-written code in the module: a small Dial helper that builds the
// conjure-go-runtime httpclient.Client every generated client constructor needs. Typical use:
//
//	hc, err := client.Dial("https://localhost:8443", client.WithInsecureSkipVerify())
//	if err != nil { ... }
//	persons := person.NewPersonServiceClient(hc)
//	p, err := persons.GetPerson(ctx, bearertoken.Token(token), personID)
//
// Authentication is delegated: every endpoint takes a bearer token (an OIDC/JWT from the deployment's
// IdP — see deploy/keycloak/ for local testing). Pass it as the bearertoken.Token argument, or wrap a
// client once with the generated NewPersonServiceClientWithAuth(hc, token) /
// NewPersonServiceClientWithTokenProvider(hc, provider) constructors.
package client

import (
	"time"

	"github.com/palantir/conjure-go-runtime/v2/conjure-go-client/httpclient"
)

// Option customizes the httpclient built by Dial.
type Option func(*[]httpclient.ClientParam)

// WithInsecureSkipVerify disables TLS certificate verification — needed for the local dev server's
// self-signed certificate (do NOT use against a real deployment).
func WithInsecureSkipVerify() Option {
	return func(p *[]httpclient.ClientParam) { *p = append(*p, httpclient.WithTLSInsecureSkipVerify()) }
}

// WithTimeout sets the per-request HTTP timeout (default: the conjure-go-runtime default).
func WithTimeout(d time.Duration) Option {
	return func(p *[]httpclient.ClientParam) { *p = append(*p, httpclient.WithHTTPTimeout(d)) }
}

// WithClientParams passes through arbitrary conjure-go-runtime httpclient params for advanced tuning
// (proxies, TLS material, retries, metrics, …) without this package having to wrap each one.
func WithClientParams(params ...httpclient.ClientParam) Option {
	return func(p *[]httpclient.ClientParam) { *p = append(*p, params...) }
}

// Dial builds an httpclient.Client pointed at the server's base URL (e.g. "https://host:8443"), ready
// to hand to any generated service-client constructor. The base path (/person/v1, …) is part of each
// generated client, so pass only the scheme://host[:port] here.
func Dial(baseURL string, opts ...Option) (httpclient.Client, error) {
	params := []httpclient.ClientParam{httpclient.WithBaseURLs([]string{baseURL})}
	for _, opt := range opts {
		opt(&params)
	}
	return httpclient.NewClient(params...)
}
