// Package hermenea is the composition seam for the hermenea companion service (M16 / D-Hermenea): it
// wires the store (its own DB), the connector registry, the loader (oikumenea's import endpoint), the
// application service, and the background runtime, then registers the HermeneaService routes. The
// per-object-type mappers are registered here too (geo-countries in-memory + geo-places paged).
// Register returns a cleanup that drains the runtime and closes the pool.
package hermenea

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	hermeneaapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/hermenea"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/adapters"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/application"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/cldrscripts"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/config"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/connector"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/factbookethnicities"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/geocountries"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/glottolog"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/loader"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/runtime"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/transport"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/wikidataorgs"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/wof"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register wires hermenea over its pool and the import service secret (used to call oikumenea), seeds
// the configured sources, registers routes, and starts the runtime. The returned cleanup drains the
// runtime then closes the pool.
func Register(ctx context.Context, info witchcraft.InitInfo, pool *pgxpool.Pool, cfg config.Install, importToken string) (func(), error) {
	store := adapters.NewRepository(pool)

	ld, err := loader.New(cfg.Oikumenea.BaseURL, importToken, cfg.Oikumenea.InsecureSkipVerify)
	if err != nil {
		return nil, werror.WrapWithContextParams(ctx, err, "build oikumenea loader")
	}

	svc := application.NewService(store, connector.Default(), ld)

	// Per-object-type mappers are registered here. Until a mapper exists for a source's object-type, its
	// sync jobs fail with ErrNoMapper (visible in import_runs).
	//   - geo-countries: the simplest, network-free proving consumer (in-memory Mapper over the `file`/
	//     `http` connector) — the milestone's named first consumer.
	//   - geo-places: the first real connector mapper (D-GeoPlaces) — a paged mapper driven by the
	//     wof-sqlite StreamingConnector.
	svc.RegisterMapper(geocountries.ObjectType, geocountries.Mapper{})
	svc.RegisterPagedMapper(wof.ObjectTypeGeoPlaces, wof.GeoPlacesMapper{})
	//   - language-scheme: the Glottolog languoid forest (D-Languages, M18) — hermenea's first NEW
	//     consumer beyond geo. Two source paths: the in-memory Mapper over a bundled/remote preprocessed
	//     JSON (file/http), and the live PagedMapper (CLDFMapper) that transforms the raw Glottolog CLDF
	//     fetched fresh from upstream master by the http-files StreamingConnector.
	//   - language-scripts: the CLDR language→writing-system links (which script a language uses) — same
	//     dual path (JSON Mapper + the live SupplementalMapper over raw CLDR supplementalData + ISO-639).
	svc.RegisterMapper(glottolog.ObjectType, glottolog.Mapper{})
	svc.RegisterMapper(cldrscripts.ObjectType, cldrscripts.Mapper{})
	svc.RegisterPagedMapper(glottolog.ObjectType, glottolog.CLDFMapper{})
	svc.RegisterPagedMapper(cldrscripts.ObjectType, cldrscripts.SupplementalMapper{})
	//   - external-organizations: the M30 registry (D-ExternalOrgs) — an in-memory Mapper over a Wikidata
	//     SPARQL JSON result set fetched live by the `http` connector (?format=json&query=…). Emits
	//     Wikidata-id-keyed party/government/military/NGO/registrant records.
	svc.RegisterMapper(wikidataorgs.ObjectType, wikidataorgs.Mapper{})
	//   - ethnicity-scheme: the ethnicity catalog (D-PhysicalIdentity amendment, M43) fed LIVE from the CIA
	//     World Factbook (public domain). The `factbook` StreamingConnector enumerates the ~260 country files
	//     (one git-tree API call) and stages them; this PagedMapper parses each country's "Ethnic groups"
	//     free-text in Go, derives the country's ISO code from its Internet ccTLD, and dedups group→countries
	//     across all files — a FLAT, country-linked catalog (the Factbook has no hierarchy/language). No
	//     committed preset: the catalog is derived at import time.
	svc.RegisterPagedMapper(factbookethnicities.ObjectType, factbookethnicities.PagedMapper{})

	// Seed declaratively-configured sources (idempotent upsert + schedule).
	for _, cs := range cfg.Sources {
		src := domain.Source{
			Code:          cs.Code,
			Name:          cs.Name,
			ConnectorType: cs.ConnectorType,
			ObjectType:    cs.ObjectType,
			Locator:       cs.Locator,
			Cron:          cs.Cron,
			Enabled:       cs.EnabledOrDefault(),
		}
		if err := svc.SeedSource(ctx, src); err != nil {
			return nil, werror.WrapWithContextParams(ctx, err, "seed hermenea source", werror.SafeParam("source", cs.Code))
		}
	}

	if err := hermeneaapi.RegisterRoutesHermeneaService(info.Router, transport.NewService(svc)); err != nil {
		return nil, werror.WrapWithContextParams(ctx, err, "register hermenea service routes")
	}

	rt := runtime.New(svc, store, runtimeConfig(cfg.Worker))
	stop := rt.Start(ctx)

	return func() {
		stop()
		pool.Close()
	}, nil
}

// runtimeConfig maps the install worker tunables to the runtime config, applying defaults for zero
// fields.
func runtimeConfig(w config.Worker) runtime.Config {
	ms := func(v, def int) time.Duration {
		if v <= 0 {
			v = def
		}
		return time.Duration(v) * time.Millisecond
	}
	return runtime.Config{
		WorkerID:     "hermenea-1",
		PollInterval: ms(w.PollIntervalMs, 2000),
		ScheduleTick: ms(w.ScheduleTickMs, 30000),
		BackoffBase:  ms(w.BackoffBaseMs, 5000),
		BackoffMax:   ms(w.BackoffMaxMs, 300000),
		JobTimeout:   ms(w.JobTimeoutMs, 120000),
		StaleAfter:   ms(w.StaleAfterMs, 600000),
	}
}
