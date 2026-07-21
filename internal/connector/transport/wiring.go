// The pull-wiring read API (M53 / D-ConnectorPlane), implementing the generated wiringapi.WiringService.
// It is a COMPOSITION layer: it reads other modules' reference catalogs (geo, language, platform's
// legal-basis) and the connector registry (for a connector's own cursors), and returns them to a
// machine subject. Every endpoint is gated on its own `wiring.*` code via RequireService — a human or
// an ungranted machine is denied.
//
// The catalog/registry sources are injected as narrow reader interfaces (satisfied structurally by the
// concrete geo/language/legal-basis/connector services), so this file depends on their read methods,
// not their construction. Cross-module reads are direct calls (per the monolith's query convention).
package transport

import (
	"context"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	wiringapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/wiring"
	connectordomain "github.com/olegamysk/go-oikumenea/internal/connector/domain"
	geodomain "github.com/olegamysk/go-oikumenea/internal/geo/domain"
	langdomain "github.com/olegamysk/go-oikumenea/internal/language/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/catalog"
	"github.com/olegamysk/go-oikumenea/pkg/authn"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

const (
	wiringResolvePerm = string(authzdomain.PermWiringResolve)
	wiringCatalogPerm = string(authzdomain.PermWiringCatalogRead)
	wiringCursorPerm  = string(authzdomain.PermWiringCursorRead)
)

// resolve catalog discriminators (ResolveRequest.catalog).
const (
	catalogCountry       = "country"
	catalogLanguoid      = "languoid"
	catalogWritingSystem = "writing-system"
)

// ---- narrow reader ports (satisfied structurally by the concrete module services) ----

// CountryReader is the geo application service's read surface the wiring API needs.
type CountryReader interface {
	ListCountries(ctx context.Context) ([]geodomain.Country, error)
	ResolveCountries(ctx context.Context, codes []string) (map[string]string, error)
}

// LanguageReader is the language application service's read surface the wiring API needs.
type LanguageReader interface {
	ListLanguoidsPage(ctx context.Context, f langdomain.Filter) ([]langdomain.Languoid, string, error)
	ListWritingSystems(ctx context.Context) ([]langdomain.WritingSystem, error)
	ResolveLanguoids(ctx context.Context, codes []string) (map[string]string, error)
	ResolveWritingSystems(ctx context.Context, codes []string) (map[string]string, error)
}

// LegalBasisReader is the platform legal-basis catalog's read surface.
type LegalBasisReader interface {
	List(ctx context.Context) ([]catalog.LegalBasisKind, error)
}

// CountryNamer assembles country name locale-maps (the localization service's NamesByID).
type CountryNamer interface {
	NamesByID(ctx context.Context, entityType string, defaultText map[string]string) (map[string]map[string]string, error)
}

// WiringService adapts the composed readers to the generated wiringapi.WiringService.
type WiringService struct {
	connectors connectorReader // the connector registry (self/cursors) — the app service
	countries  CountryReader
	langs      LanguageReader
	legal      LegalBasisReader
	names      CountryNamer
	pep        *pep.Enforcer
}

// connectorReader is the connector registry's self-read surface (a subset of *application.Service).
type connectorReader interface {
	GetConnectorByPrincipal(ctx context.Context, principalID string) (connectordomain.Connector, error)
	ListSources(ctx context.Context, connectorID string) ([]connectordomain.Source, error)
	LatestRun(ctx context.Context, sourceID string) (connectordomain.SyncRun, bool, error)
}

// NewWiringService builds the wiring transport over the connector registry, the reference-catalog
// readers, the country namer, and the PEP enforcer.
func NewWiringService(connectors connectorReader, countries CountryReader, langs LanguageReader, legal LegalBasisReader, names CountryNamer, enforcer *pep.Enforcer) WiringService {
	return WiringService{connectors: connectors, countries: countries, langs: langs, legal: legal, names: names, pep: enforcer}
}

var _ wiringapi.WiringService = WiringService{}

// ---- resolve ----

func (s WiringService) ResolveKeys(ctx context.Context, token bearertoken.Token, req wiringapi.ResolveRequest) (wiringapi.ResolveResponse, error) {
	if err := s.pep.RequireService(ctx, token, wiringResolvePerm, ""); err != nil {
		return wiringapi.ResolveResponse{}, err
	}
	var (
		resolved map[string]string
		err      error
	)
	switch req.Catalog {
	case catalogCountry:
		resolved, err = s.countries.ResolveCountries(ctx, req.Codes)
	case catalogLanguoid:
		resolved, err = s.langs.ResolveLanguoids(ctx, req.Codes)
	case catalogWritingSystem:
		resolved, err = s.langs.ResolveWritingSystems(ctx, req.Codes)
	default:
		return wiringapi.ResolveResponse{}, wiringapi.NewWiringInvalid(
			"unknown catalog (want country|languoid|writing-system): " + req.Catalog)
	}
	if err != nil {
		return wiringapi.ResolveResponse{}, err
	}
	// unresolved = the requested codes with no match, so a connector can handle both in one pass.
	unresolved := make([]string, 0)
	for _, c := range req.Codes {
		if _, ok := resolved[c]; !ok {
			unresolved = append(unresolved, c)
		}
	}
	if resolved == nil {
		resolved = map[string]string{}
	}
	return wiringapi.ResolveResponse{Resolved: resolved, Unresolved: unresolved}, nil
}

// ---- catalogs ----

func (s WiringService) ListCountries(ctx context.Context, token bearertoken.Token) (wiringapi.CountryList, error) {
	if err := s.pep.RequireService(ctx, token, wiringCatalogPerm, ""); err != nil {
		return wiringapi.CountryList{}, err
	}
	rows, err := s.countries.ListCountries(ctx)
	if err != nil {
		return wiringapi.CountryList{}, err
	}
	defaults := make(map[string]string, len(rows))
	for _, c := range rows {
		defaults[c.Code] = c.Name
	}
	names, err := s.names.NamesByID(ctx, "country", defaults)
	if err != nil {
		return wiringapi.CountryList{}, err
	}
	out := make([]wiringapi.CountryEntry, 0, len(rows))
	for _, c := range rows {
		out = append(out, wiringapi.CountryEntry{Rid: c.ID, Code: c.Code, Name: names[c.Code]})
	}
	return wiringapi.CountryList{Countries: out}, nil
}

func (s WiringService) ListWritingSystems(ctx context.Context, token bearertoken.Token) (wiringapi.WritingSystemList, error) {
	if err := s.pep.RequireService(ctx, token, wiringCatalogPerm, ""); err != nil {
		return wiringapi.WritingSystemList{}, err
	}
	rows, err := s.langs.ListWritingSystems(ctx)
	if err != nil {
		return wiringapi.WritingSystemList{}, err
	}
	out := make([]wiringapi.WritingSystemEntry, 0, len(rows))
	for _, w := range rows {
		out = append(out, wiringapi.WritingSystemEntry{Rid: w.ID, Code: w.Code, Name: w.Name})
	}
	return wiringapi.WritingSystemList{WritingSystems: out}, nil
}

func (s WiringService) ListLanguoids(ctx context.Context, token bearertoken.Token, query *string, pageSize *int, pageToken *string) (wiringapi.LanguoidPage, error) {
	if err := s.pep.RequireService(ctx, token, wiringCatalogPerm, ""); err != nil {
		return wiringapi.LanguoidPage{}, err
	}
	after, err := decodeToken(pageToken)
	if err != nil {
		return wiringapi.LanguoidPage{}, err
	}
	f := langdomain.Filter{Query: derefStr(query), After: after, Limit: derefInt(pageSize)}
	rows, next, err := s.langs.ListLanguoidsPage(ctx, f)
	if err != nil {
		return wiringapi.LanguoidPage{}, err
	}
	out := make([]wiringapi.LanguoidEntry, 0, len(rows))
	for _, l := range rows {
		e := wiringapi.LanguoidEntry{Rid: l.ID, Code: l.Code, Name: l.Name}
		e.Iso6393 = strPtr(l.ISO639_3)
		e.Level = strPtr(l.Level)
		e.FamilyCode = strPtr(l.FamilyCode)
		out = append(out, e)
	}
	// ListLanguoidsPage returns the next keyset cursor ("" = last page); wrap it as an opaque token.
	var nextTok *string
	if next != "" {
		nextTok = encodeToken(next)
	}
	return wiringapi.LanguoidPage{Languoids: out, NextPageToken: nextTok}, nil
}

func (s WiringService) ListLegalBasisKinds(ctx context.Context, token bearertoken.Token) (wiringapi.LegalBasisList, error) {
	if err := s.pep.RequireService(ctx, token, wiringCatalogPerm, ""); err != nil {
		return wiringapi.LegalBasisList{}, err
	}
	rows, err := s.legal.List(ctx)
	if err != nil {
		return wiringapi.LegalBasisList{}, err
	}
	out := make([]wiringapi.LegalBasisEntry, 0, len(rows))
	for _, k := range rows {
		out = append(out, wiringapi.LegalBasisEntry{Code: k.Code, Name: k.Name, Article: k.Article, Status: k.Status})
	}
	return wiringapi.LegalBasisList{Kinds: out}, nil
}

// ---- self (cursor.read) ----

func (s WiringService) ReadSelf(ctx context.Context, token bearertoken.Token) (wiringapi.WiringSelf, error) {
	if err := s.pep.RequireService(ctx, token, wiringCursorPerm, ""); err != nil {
		return wiringapi.WiringSelf{}, err
	}
	principalID := authn.PrincipalID(ctx)
	c, err := s.connectors.GetConnectorByPrincipal(ctx, principalID)
	if err != nil {
		if err == connectordomain.ErrConnectorNotFound {
			return wiringapi.WiringSelf{}, wiringapi.NewConnectorNotRegistered(principalID)
		}
		return wiringapi.WiringSelf{}, err
	}
	sources, err := s.connectors.ListSources(ctx, c.ID)
	if err != nil {
		return wiringapi.WiringSelf{}, err
	}
	selfSources := make([]wiringapi.SelfSource, 0, len(sources))
	for _, src := range sources {
		ss := wiringapi.SelfSource{
			Id:         src.ID,
			Code:       src.Code,
			ObjectType: strPtr(src.ObjectType),
			Enabled:    src.Enabled,
		}
		run, ok, err := s.connectors.LatestRun(ctx, src.ID)
		if err != nil {
			return wiringapi.WiringSelf{}, err
		}
		if ok {
			ss.LatestRunState = strPtr(run.State)
			ss.LatestExternalRunId = strPtr(run.ExternalRunID)
			if run.FinishedAt != nil {
				dt := datetime.DateTime(*run.FinishedAt)
				ss.LatestFinishedAt = &dt
			}
		}
		selfSources = append(selfSources, ss)
	}
	self := wiringapi.SelfConnector{Id: c.ID, Code: c.Code, Name: c.Name, Status: c.Status}
	if c.LastSeenAt != nil {
		dt := datetime.DateTime(*c.LastSeenAt)
		self.LastSeenAt = &dt
	}
	return wiringapi.WiringSelf{Connector: self, Sources: selfSources}, nil
}
