# go-oikumenea Go client SDK

A typed Go client for the go-oikumenea API. The per-service clients are **generated from the same
Conjure contract** (`api/*.conjure.yml`) as the server, so the SDK cannot drift from the API. This is a
**nested module** — it versions and is consumed independently of the server.

```
go get github.com/olehmushka/go-oikumenea/clients/go@latest
```

## Layout

- `client` (this package) — the `Dial` helper plus the unified **façade** (`client.New`).
- `clients/go/oikumenea/<module>` — the generated typed clients + request/response/error types, one per API
  module: `person`, `tenant`, `membership`, `authorization`, `identityfederation`, `document`, `order`,
  `rank`, `localization`, `audit`, `platform`, `geo`, `language`, `location`, `education`,
  `educationref`, `company`, `religion`, `dataimport` (the import endpoint), and `hermenea`.

Each module exposes `New<Svc>ServiceClient(httpclient.Client)` plus auth-bound variants
`New<Svc>ServiceClientWithAuth(hc, token)` and `New<Svc>ServiceClientWithTokenProvider(hc, provider)`.

## Unified client (façade)

`client.New` is the one-call entry point: it `Dial`s once and binds your token to every service, so you
don't assemble per-service clients by hand. Every service is a field — **including `Hermenea` and
`Import`**: oikumenea reverse-proxies the hermenea ingestion/scheduler API at `/hermenea/v1/*`
(D-Hermenea), so the same client reaches both oikumenea-native and hermenea-proxied endpoints.

```go
c, err := oik.New("https://localhost:8443", token, oik.WithInsecureSkipVerify())
if err != nil { panic(err) }

who, err := c.IdentityFederation.Whoami(ctx)     // oikumenea-native
runs, err := c.Hermenea.ListRuns(ctx)            // hermenea, through oikumenea
```

Use `oik.NewWithTokenProvider(baseURL, provider, opts...)` for short-lived/refreshing tokens. The
per-service packages remain importable directly for per-call tokens or advanced tuning.

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

	oik "github.com/olehmushka/go-oikumenea/clients/go"
	"github.com/olehmushka/go-oikumenea/clients/go/oikumenea/identityfederation"
	"github.com/olehmushka/go-oikumenea/clients/go/oikumenea/person"
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

Releases are tagged `clients/go/vX.Y.Z` and published to pkg.go.dev independently of the server. The SDK
tracks the server's API contract; pin a version that matches the server you target.
