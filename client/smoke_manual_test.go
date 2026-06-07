//go:build manual

package client_test

import (
	"context"
	"testing"

	oik "github.com/olegamysk/go-oikumenea/client"
	"github.com/olegamysk/go-oikumenea/client/oikumenea/identityfederation"
	"github.com/palantir/pkg/bearertoken"
)

// TestSDKSmoke proves the published SDK reaches a live server over the wire (Dial + TLS-skip + base
// path + request + error decode). A bogus token must be rejected by the auth middleware, so Whoami
// returns a non-nil error. Run against a running dev server:
//
//	go test -tags manual -run TestSDKSmoke ./...
func TestSDKSmoke(t *testing.T) {
	hc, err := oik.Dial("https://localhost:8443", oik.WithInsecureSkipVerify())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_, err = identityfederation.NewIdentityFederationServiceClient(hc).
		Whoami(context.Background(), bearertoken.Token("bogus-token"))
	if err == nil {
		t.Fatal("expected an auth error from Whoami with a bogus token, got nil")
	}
	t.Logf("server reachable; bogus token rejected as expected: %v", err)
}
