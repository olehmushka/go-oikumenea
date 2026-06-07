# go-oikumenea Go client SDK

A typed Go client for the go-oikumenea API. The per-service clients are **generated from the same
Conjure contract** (`api/*.conjure.yml`) as the server, so the SDK cannot drift from the API. This is a
**nested module** — it versions and is consumed independently of the server.

```
go get github.com/olegamysk/go-oikumenea/client@latest
```

## Layout

- `client` (this package) — the `Dial` helper that builds the underlying HTTP client.
- `client/oikumenea/<module>` — the generated typed clients + request/response/error types, one per API
  module: `person`, `tenant`, `membership`, `authorization`, `identityfederation`, `document`, `order`,
  `rank`, `localization`, `audit`, `platform`.

Each module exposes `New<Svc>ServiceClient(httpclient.Client)` plus auth-bound variants
`New<Svc>ServiceClientWithAuth(hc, token)` and `New<Svc>ServiceClientWithTokenProvider(hc, provider)`.

## Authentication

Authentication is delegated to the deployment's IdP — every endpoint takes an OIDC/JWT **bearer token**
(see `deploy/keycloak/` for spinning up a local IdP and `scripts/keycloak-token.sh` for minting one).
The server then makes the authorization decision (the PDP). Pass the token as the `bearertoken.Token`
argument, or bind it once with the `…WithAuth` constructor.

## Usage

```go
package main

import (
	"context"
	"fmt"

	oik "github.com/olegamysk/go-oikumenea/client"
	"github.com/olegamysk/go-oikumenea/client/oikumenea/identityfederation"
	"github.com/olegamysk/go-oikumenea/client/oikumenea/person"
	"github.com/palantir/pkg/bearertoken"
)

func main() {
	ctx := context.Background()

	// Build an HTTP client for the server. WithInsecureSkipVerify is only for the local dev
	// server's self-signed cert — drop it against a real deployment.
	hc, err := oik.Dial("https://localhost:8443", oik.WithInsecureSkipVerify())
	if err != nil {
		panic(err)
	}

	token := bearertoken.Token("<paste a token from scripts/keycloak-token.sh>")

	// Who am I? (resolves the token -> person/account)
	who, err := identityfederation.NewIdentityFederationServiceClient(hc).Whoami(ctx, token)
	if err != nil {
		panic(err)
	}
	fmt.Println("personId:", who.PersonId)

	// List the directory (token-paginated). Bind the token once for convenience: WithAuth wraps a
	// base client so per-call signatures drop the bearertoken argument.
	persons := person.NewPersonServiceClientWithAuth(person.NewPersonServiceClient(hc), token)
	page, err := persons.ListPersons(ctx, nil, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println("persons on first page:", len(page.Persons))
}
```

Errors come back as the Conjure `SerializableError` envelope; use the generated typed error helpers in
each module package (e.g. `person.IsPersonNotFound(err)`) to branch on specific failures.

## Versioning

Releases are tagged `client/vX.Y.Z` and published to pkg.go.dev independently of the server. The SDK
tracks the server's API contract; pin a version that matches the server you target.
