package client_test

import (
	"context"
	"errors"
	"fmt"

	oik "github.com/olegamysk/go-oikumenea/client"
	"github.com/olegamysk/go-oikumenea/client/oikumenea/identityfederation"
	"github.com/olegamysk/go-oikumenea/client/oikumenea/person"
	"github.com/palantir/pkg/bearertoken"
)

// Example shows the typical SDK flow: Dial the server, then call the typed service clients with a
// bearer token. It is compiled (and vetted) by `go test`, but not executed (no `// Output:` line), so
// it needs no running server.
func Example() {
	ctx := context.Background()

	// WithInsecureSkipVerify is only for the local dev server's self-signed cert.
	hc, err := oik.Dial("https://localhost:8443", oik.WithInsecureSkipVerify())
	if err != nil {
		panic(err)
	}

	token := bearertoken.Token("<token from scripts/keycloak-token.sh>")

	who, err := identityfederation.NewIdentityFederationServiceClient(hc).Whoami(ctx, token)
	if err != nil {
		panic(err)
	}
	fmt.Println(who.PersonId)

	// Typed errors: branch on a specific failure via the generated helper. The WithAuth wrapper binds
	// the token once so per-call signatures drop the bearertoken argument.
	persons := person.NewPersonServiceClientWithAuth(person.NewPersonServiceClient(hc), token)
	if _, err := persons.GetPerson(ctx, "urn:oikumenea:person:local:person:does-not-exist"); err != nil {
		if person.IsPersonNotFound(err) {
			fmt.Println("not found")
		} else {
			fmt.Println("other error:", errors.Unwrap(err))
		}
	}
}
