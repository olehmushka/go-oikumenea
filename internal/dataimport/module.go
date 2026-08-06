// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package dataimport is the composition seam for the data-import module (M16 / D-Hermenea): it wires
// the application service over the platform pool + audit, registers the available object-type upsert
// handlers (geo-countries first), and registers the generic ImportService Conjure route. This is the
// oikumenea SIDE of the ingestion pipeline — the connectors/mappers/scheduler live out of process in
// the hermenea companion (docs/modules/hermenea.md), which calls POST /import/{objectType} here.
package dataimport

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	dataimportapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/dataimport"
	hermeneaapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/hermenea"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/adapters"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/application"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/domain"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/transport"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/palantir/conjure-go-runtime/v2/conjure-go-client/httpclient"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the data-import module over the platform pool + audit service (writes record
// in-transaction — D-Audit), registers the geo-countries upsert handler, and registers the
// ImportService routes onto the witchcraft router. It owns no resources of its own.
//
// hermeneaBaseURL/hermeneaToken/hermeneaInsecureTLS configure the import-control PROXY (the reverse of
// the import endpoint): when hermeneaBaseURL is set, oikumenea also serves the HermeneaService routes
// (/hermenea/v1/*) and forwards UI-triggered sync/list calls to the companion using the trigger token,
// gated on import.manage (D-Hermenea). Empty hermeneaBaseURL leaves the proxy unregistered.
// NewImportService builds the import application service and registers every object-type upsert
// handler over the pool + audit (writes record in-transaction — D-Audit). It registers NO routes, so
// it is reusable off the request path — notably by the `oikumenea seed` CLI and the pinax boot
// autoseeder (D-Pinax, M45), which drive the same handlers without a witchcraft router.
func NewImportService(pool *pgxpool.Pool, audit *auditapp.Service) *application.Service {
	svc := application.NewService(pool, audit)

	// geo-countries: the first importable catalog (M16). The store factory binds the sqlc adapter to
	// the caller's transaction.
	svc.Register(domain.ObjectTypeGeoCountries, application.GeoCountriesHandler(
		func(conn db.DBTX) domain.GeoCountryStore { return adapters.NewGeoCountryRepo(conn) },
	))

	// geo-places: the Who's-On-First administrative gazetteer (D-GeoPlaces) — the first real connector's
	// load target. A placetype=country record also enriches the geo_countries row in the same tx.
	svc.Register(domain.ObjectTypeGeoPlaces, application.GeoPlacesHandler(
		func(conn db.DBTX) domain.GeoPlaceStore { return adapters.NewGeoPlaceRepo(conn) },
	))

	// language-scheme: the Glottolog languoid forest (D-Languages, M18) — the first NEW import consumer.
	// Parent-first upsert; the handler rebuilds the closure + family_code at the end of the batch.
	svc.Register(domain.ObjectTypeLanguageScheme, application.LanguageSchemeHandler(
		func(conn db.DBTX) domain.LanguoidStore { return adapters.NewLanguoidRepo(conn) },
	))

	// language-scripts: the CLDR language→writing-system links (D-Languages). Resolves languoid (by ISO
	// 639-3) + writing system (by ISO-15924) and upserts the link; unresolved records are skipped.
	svc.Register(domain.ObjectTypeLanguageScripts, application.LanguageScriptsHandler(
		func(conn db.DBTX) domain.LanguageScriptStore { return adapters.NewLanguageScriptRepo(conn) },
	))

	// external-organizations: the M30 registry (D-ExternalOrgs) fed from Wikidata / public registries by
	// the hermenea `wikidataorgs` mapper. Idempotent upsert keyed by Wikidata id; unknown kinds skipped.
	svc.Register(domain.ObjectTypeExternalOrgs, application.ExternalOrgsHandler(
		func(conn db.DBTX) domain.ExternalOrgStore { return adapters.NewExternalOrgRepo(conn) },
	))

	// ethnicity-scheme: the hierarchical ethnicity taxonomy (D-PhysicalIdentity amendment, M43) fed from
	// Wikidata by the hermenea `wikidataethnicities` mapper. Parent-first upsert; the group's language +
	// country ties are replaced and the closure rebuilt at the end of the batch. Default catalog is empty
	// (opt-in) — the person's declared ethnicity is unaffected by this reference-data import.
	svc.Register(domain.ObjectTypeEthnicityScheme, application.EthnicitySchemeHandler(
		func(conn db.DBTX) domain.EthnicityStore { return adapters.NewEthnicityRepo(conn) },
	))

	// religion-scheme: the recursive faith taxonomy (D-Religion + D-Pinax, M45) seeded from the bundled
	// `religions` preset. Parent-first upsert; theism classifications replaced per taxon; the handler
	// rebuilds the closure + re-derives the denormalized root religion_id at the end of the batch.
	svc.Register(domain.ObjectTypeReligionScheme, application.ReligionSchemeHandler(
		func(conn db.DBTX) domain.ReligionStore { return adapters.NewReligionRepo(conn) },
	))

	// colors: the per-domain platform_colors palettes (D-Color + D-Pinax, M45) seeded from the bundled
	// `colors` preset. (domain, code)-keyed idempotent upsert; the seeded reference catalogs point at
	// these via color_id (countries fill-if-empty in the geo-countries handler).
	svc.Register(domain.ObjectTypeColors, application.ColorsHandler(
		func(conn db.DBTX) domain.ColorStore { return adapters.NewColorRepo(conn) },
	))

	// translations: the pinax i18n overlay (D-Pinax + D-i18n, M45) — seeds i18n_translations for the
	// seeded reference catalogs (country/languoid/writing_system/religion_taxon/ethnicity_type/rank_*)
	// from bundled CLDR + curated translations. Resolves each entity's natural key to its read-path
	// entity_id and writes create-if-absent. Runs after the entity presets (the preset dependsOn them).
	svc.Register(domain.ObjectTypeTranslations, application.TranslationsHandler(
		func(conn db.DBTX) domain.TranslationStore { return adapters.NewTranslationRepo(conn) },
	))

	// locales: the supported-locale import target (D-DataPacks + D-i18n, M54) — a LOCALE PACK adds a new
	// i18n_locales row create-if-absent here, then its translation overlays via `translations` above. New
	// locales land enabled + non-default; an already-supported code is skipped, never re-flagged.
	svc.Register(domain.ObjectTypeLocales, application.LocalesHandler(
		func(conn db.DBTX) domain.LocaleStore { return adapters.NewLocaleRepo(conn) },
	))

	// person-regulatory-sanctions: the M34 regulatory-exposure overlay (D-Watchlists) — a person-scoped
	// import target. Records reference a person by RID + carry a regulatory action; idempotent by
	// (person, externalId), unresolved-person records skipped. Fed by the hermenea `regulatorysanctions`
	// mapper (an operator-registered source; no committed bulk preset).
	svc.Register(domain.ObjectTypeRegulatorySanctions, application.RegulatorySanctionsHandler(
		func(conn db.DBTX) domain.RegulatorySanctionStore { return adapters.NewRegulatorySanctionRepo(conn) },
	))

	return svc
}

func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, enforcer *pep.Enforcer, hermeneaBaseURL, hermeneaToken string, hermeneaInsecureTLS bool) (*application.Service, error) {
	svc := NewImportService(pool, audit)

	if err := dataimportapi.RegisterRoutesImportService(info.Router, transport.NewService(svc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register import service routes")
	}

	// Import-control proxy (D-Hermenea): only when an operator configured the companion's base URL.
	// oikumenea re-issues UI-triggered calls to hermenea with the trigger secret; callers are gated on
	// import.manage in the proxy handler. The HTTP client mirrors hermenea's loader (no retries — the
	// companion owns retry/backoff; no fixed deadline — list/trigger are quick but a sync enqueue must
	// not race conjure's default).
	if hermeneaBaseURL != "" {
		params := []httpclient.ClientParam{
			httpclient.WithBaseURLs([]string{hermeneaBaseURL}),
			httpclient.WithMaxRetries(0),
		}
		if hermeneaInsecureTLS {
			params = append(params, httpclient.WithTLSInsecureSkipVerify())
		}
		hc, err := httpclient.NewClient(params...)
		if err != nil {
			return nil, werror.Wrap(err, "build hermenea control client")
		}
		proxy := transport.NewHermeneaProxy(enforcer, hermeneaapi.NewHermeneaServiceClient(hc), hermeneaToken)
		if err := hermeneaapi.RegisterRoutesHermeneaService(info.Router, proxy); err != nil {
			return nil, werror.Wrap(err, "register hermenea control proxy routes")
		}
	}
	return svc, nil
}
