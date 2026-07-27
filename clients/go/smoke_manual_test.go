// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build manual

package client_test

import (
	"context"
	"os"
	"testing"

	oik "github.com/olegamysk/go-oikumenea/clients/go"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/identityfederation"
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

// TestFacadeSmoke proves the unified façade (client.New) wires every service onto one Dial + bound
// token, and that hermenea is reachable THROUGH oikumenea's proxy (D-Hermenea) from the same client.
// With a bogus token the auth middleware must reject both an oikumenea-native call (Whoami) and a
// hermenea-proxied call (ListRuns). Run against a running dev server:
//
//	go test -tags manual -run TestFacadeSmoke ./...
func TestFacadeSmoke(t *testing.T) {
	c, err := oik.New("https://localhost:8443", bearertoken.Token("bogus-token"), oik.WithInsecureSkipVerify())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := c.IdentityFederation.Whoami(context.Background()); err == nil {
		t.Fatal("expected an auth error from IdentityFederation.Whoami with a bogus token, got nil")
	}
	if _, err := c.Hermenea.ListRuns(context.Background()); err == nil {
		t.Fatal("expected an auth error from Hermenea.ListRuns (proxied) with a bogus token, got nil")
	}
	t.Log("façade reachable; both oikumenea-native and hermenea-proxied calls rejected as expected")

	// Happy path: with a real token (e.g. OIKUMENEA_TOKEN=$(scripts/keycloak-token.sh)) the SAME façade
	// reaches a hermenea-proxied endpoint through oikumenea and gets a 200 (D-Hermenea / D-ClientSDK).
	if tok := os.Getenv("OIKUMENEA_TOKEN"); tok != "" {
		c2, err := oik.New("https://localhost:8443", bearertoken.Token(tok), oik.WithInsecureSkipVerify())
		if err != nil {
			t.Fatalf("new client (real token): %v", err)
		}
		sources, err := c2.Hermenea.ListSources(context.Background())
		if err != nil {
			t.Fatalf("Hermenea.ListSources (proxied, real token) failed: %v", err)
		}
		t.Logf("hermenea reached through oikumenea via the façade: %d import sources", len(sources))
	}
}
