// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package client

// client.go is the unified façade over the generated per-service clients (D-ClientSDK) — the Go
// analog of the TypeScript SDK's createOikumeneaClient (clients/typescript/src/index.ts). One Dial,
// one bound token, every service. It is hand-written (like dial.go); the per-service packages under
// oikumenea/<module> remain generated and never hand-edited.
//
// Every oikumenea service is a field here, INCLUDING Hermenea and Import: oikumenea reverse-proxies
// the hermenea ingestion/scheduler API at /hermenea/v1/* (D-Hermenea), so a single Client + base URL
// reaches both oikumenea-native and hermenea-proxied endpoints.
//
//	c, err := client.New("https://localhost:8443", token, client.WithInsecureSkipVerify())
//	if err != nil { ... }
//	who, err := c.IdentityFederation.Whoami(ctx)
//	runs, err := c.Hermenea.ListRuns(ctx) // reaches hermenea through oikumenea
//
// For per-call tokens or advanced tuning, the generated per-service packages
// (oikumenea/<module>) remain importable directly.

import (
	"context"

	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/audit"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/authorization"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/company"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/dataimport"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/document"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/education"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/educationref"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/finance"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/geo"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/hermenea"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/identityfederation"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/language"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/localization"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/location"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/membership"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/order"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/person"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/platform"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/rank"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/religion"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/tenant"
	"github.com/olegamysk/go-oikumenea/clients/go/oikumenea/vehicle"
	"github.com/palantir/conjure-go-runtime/v2/conjure-go-client/httpclient"
	"github.com/palantir/pkg/bearertoken"
)

// Client is a unified, auth-bound handle to every go-oikumenea service. Build it with New (static
// token) or NewWithTokenProvider (refreshing token). Each field is a generated per-service client
// with the bearer already bound, so method calls omit the token argument.
type Client struct {
	// HTTPClient is the shared underlying transport, exposed for advanced/custom calls.
	HTTPClient httpclient.Client

	Audit              audit.AuditServiceClientWithAuth
	Authorization      authorization.AuthorizationServiceClientWithAuth
	Company            company.CompanyServiceClientWithAuth
	Document           document.DocumentServiceClientWithAuth
	Education          education.EducationServiceClientWithAuth
	EducationReference educationref.EducationReferenceServiceClientWithAuth
	Finance            finance.FinanceServiceClientWithAuth
	Geo                geo.GeoServiceClientWithAuth
	IdentityFederation identityfederation.IdentityFederationServiceClientWithAuth
	// Import is the generic reference-data import endpoint (POST /import/{objectType}); hermenea's
	// loader calls this, and so can admins.
	Import       dataimport.ImportServiceClientWithAuth
	Language     language.LanguageServiceClientWithAuth
	Localization localization.LocalizationServiceClientWithAuth
	Location     location.LocationServiceClientWithAuth
	Membership   membership.MembershipServiceClientWithAuth
	Order        order.OrderServiceClientWithAuth
	Person       person.PersonServiceClientWithAuth
	// Platform exposes the unauthenticated operational endpoints (version/status), so it is not
	// token-bound.
	Platform platform.PlatformOpsServiceClient
	Rank     rank.RankServiceClientWithAuth
	Religion religion.ReligionServiceClientWithAuth
	Tenant   tenant.TenantServiceClientWithAuth
	Vehicle  vehicle.VehicleServiceClientWithAuth
	// Hermenea is the ingestion/scheduler companion's control + read API, reached through
	// oikumenea's reverse proxy (D-Hermenea).
	Hermenea hermenea.HermeneaServiceClientWithAuth
}

// New builds a Client pointed at baseURL (scheme://host[:port]; the per-service base paths are part
// of each generated client) with token bound to every authenticated service.
func New(baseURL string, token bearertoken.Token, opts ...Option) (*Client, error) {
	return NewWithTokenProvider(baseURL, func(context.Context) (string, error) {
		return string(token), nil
	}, opts...)
}

// NewWithTokenProvider is like New but resolves the bearer per request from tokenProvider — use it
// when tokens are short-lived/refreshed.
func NewWithTokenProvider(baseURL string, tokenProvider httpclient.TokenProvider, opts ...Option) (*Client, error) {
	hc, err := Dial(baseURL, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		HTTPClient:         hc,
		Audit:              audit.NewAuditServiceClientWithTokenProvider(audit.NewAuditServiceClient(hc), tokenProvider),
		Authorization:      authorization.NewAuthorizationServiceClientWithTokenProvider(authorization.NewAuthorizationServiceClient(hc), tokenProvider),
		Company:            company.NewCompanyServiceClientWithTokenProvider(company.NewCompanyServiceClient(hc), tokenProvider),
		Document:           document.NewDocumentServiceClientWithTokenProvider(document.NewDocumentServiceClient(hc), tokenProvider),
		Education:          education.NewEducationServiceClientWithTokenProvider(education.NewEducationServiceClient(hc), tokenProvider),
		EducationReference: educationref.NewEducationReferenceServiceClientWithTokenProvider(educationref.NewEducationReferenceServiceClient(hc), tokenProvider),
		Finance:            finance.NewFinanceServiceClientWithTokenProvider(finance.NewFinanceServiceClient(hc), tokenProvider),
		Geo:                geo.NewGeoServiceClientWithTokenProvider(geo.NewGeoServiceClient(hc), tokenProvider),
		IdentityFederation: identityfederation.NewIdentityFederationServiceClientWithTokenProvider(identityfederation.NewIdentityFederationServiceClient(hc), tokenProvider),
		Import:             dataimport.NewImportServiceClientWithTokenProvider(dataimport.NewImportServiceClient(hc), tokenProvider),
		Language:           language.NewLanguageServiceClientWithTokenProvider(language.NewLanguageServiceClient(hc), tokenProvider),
		Localization:       localization.NewLocalizationServiceClientWithTokenProvider(localization.NewLocalizationServiceClient(hc), tokenProvider),
		Location:           location.NewLocationServiceClientWithTokenProvider(location.NewLocationServiceClient(hc), tokenProvider),
		Membership:         membership.NewMembershipServiceClientWithTokenProvider(membership.NewMembershipServiceClient(hc), tokenProvider),
		Order:              order.NewOrderServiceClientWithTokenProvider(order.NewOrderServiceClient(hc), tokenProvider),
		Person:             person.NewPersonServiceClientWithTokenProvider(person.NewPersonServiceClient(hc), tokenProvider),
		Platform:           platform.NewPlatformOpsServiceClient(hc),
		Rank:               rank.NewRankServiceClientWithTokenProvider(rank.NewRankServiceClient(hc), tokenProvider),
		Religion:           religion.NewReligionServiceClientWithTokenProvider(religion.NewReligionServiceClient(hc), tokenProvider),
		Tenant:             tenant.NewTenantServiceClientWithTokenProvider(tenant.NewTenantServiceClient(hc), tokenProvider),
		Vehicle:            vehicle.NewVehicleServiceClientWithTokenProvider(vehicle.NewVehicleServiceClient(hc), tokenProvider),
		Hermenea:           hermenea.NewHermeneaServiceClientWithTokenProvider(hermenea.NewHermeneaServiceClient(hc), tokenProvider),
	}, nil
}
