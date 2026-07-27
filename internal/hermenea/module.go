// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package hermenea is the composition seam for the hermenea companion service (M16 / D-Hermenea): it
// wires the store (its own DB), the connector registry, the loader (oikumenea's import endpoint), the
// application service, and the background runtime, then registers the HermeneaService routes. The
// per-object-type mappers are registered here too (geo-countries in-memory + geo-places paged).
// Register returns a cleanup that drains the runtime and closes the pool.
package hermenea

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	hermeneaapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/hermenea"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/adapters"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/application"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/cldrscripts"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/config"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/fetcher"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/factbookethnicities"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/geocountries"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/glottolog"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/loader"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/regulatorysanctions"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/reporter"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/runtime"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/transport"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/watchlist"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/wikidataorgs"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/wof"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Connector identity in oikumenea's connector registry (M53 / D-ConnectorPlane). ConnectorCode is the
// stable, locale-agnostic handle operators and audit rows reference — it must match the code hermenea
// self-registers under across boots so re-registration converges the same row.
const (
	ConnectorCode        = "hermenea"
	ConnectorName        = "hermenea (ingestion companion)"
	ConnectorDescription = "The M16 ingestion + scheduler companion: fetches, stages and pushes reference data into oikumenea over HTTP."
)

// Register wires hermenea over its pool and the import service secret (used to call oikumenea), seeds
// the configured sources, registers routes, and starts the runtime. The returned cleanup drains the
// runtime then closes the pool.
func Register(ctx context.Context, info witchcraft.InitInfo, pool *pgxpool.Pool, cfg config.Install, importToken string) (func(), error) {
	store := adapters.NewRepository(pool)

	ld, err := loader.New(cfg.Oikumenea.BaseURL, importToken, cfg.Oikumenea.InsecureSkipVerify, loader.Options{
		ChunkSize:   cfg.Oikumenea.ChunkSize, // 0 → loader.DefaultChunkSize
		HTTPTimeout: time.Duration(cfg.Oikumenea.HTTPTimeoutMs) * time.Millisecond,
	})
	if err != nil {
		return nil, werror.WrapWithContextParams(ctx, err, "build oikumenea loader")
	}

	svc := application.NewService(store, fetcher.Default(), ld)

	// Per-object-type mappers are registered here. Until a mapper exists for a source's object-type, its
	// sync jobs fail with ErrNoMapper (visible in import_runs).
	//   - geo-countries: the simplest, network-free proving consumer (in-memory Mapper over the `file`/
	//     `http` connector) — the milestone's named first consumer.
	//   - geo-places: the first real connector mapper (D-GeoPlaces) — a paged mapper driven by the
	//     wof-sqlite StreamingFetcher.
	svc.RegisterMapper(geocountries.ObjectType, geocountries.Mapper{})
	svc.RegisterPagedMapper(wof.ObjectTypeGeoPlaces, wof.GeoPlacesMapper{})
	//   - language-scheme: the Glottolog languoid forest (D-Languages, M18) — hermenea's first NEW
	//     consumer beyond geo. Two source paths: the in-memory Mapper over a bundled/remote preprocessed
	//     JSON (file/http), and the live PagedMapper (CLDFMapper) that transforms the raw Glottolog CLDF
	//     fetched fresh from upstream master by the http-files StreamingFetcher.
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
	//     World Factbook (public domain). The `factbook` StreamingFetcher enumerates the ~260 country files
	//     (one git-tree API call) and stages them; this PagedMapper parses each country's "Ethnic groups"
	//     free-text in Go, derives the country's ISO code from its Internet ccTLD, and dedups group→countries
	//     across all files — a FLAT, country-linked catalog (the Factbook has no hierarchy/language). No
	//     committed preset: the catalog is derived at import time.
	svc.RegisterPagedMapper(factbookethnicities.ObjectType, factbookethnicities.PagedMapper{})
	//   - person-regulatory-sanctions: the M34 regulatory-exposure overlay (D-Watchlists) — a validated
	//     passthrough of an operator-registered source's canonical JSON array into oikumenea's
	//     POST /import/person-regulatory-sanctions (idempotent by (person, externalId)). No committed bulk
	//     source; the connector is http/file per the registered source.
	svc.RegisterMapper(regulatorysanctions.ObjectType, regulatorysanctions.Mapper{})

	// Live watchlist screening (D-Watchlists, M34): the one SYNCHRONOUS surface. hermenea owns egress to
	// the providers + a ≤24h cache; only match metadata is returned. The real INTERPOL Red Notices
	// connector ships; sanctions providers are a documented stub (SanctionsStub returns no match).
	var providers []watchlist.Provider
	if cfg.Watchlist.InterpolEnabledOrDefault() {
		providers = append(providers, watchlist.NewInterpol(cfg.Watchlist.InterpolBaseURL, nil))
	}
	providers = append(providers, watchlist.SanctionsStub{})
	ttl := time.Duration(cfg.Watchlist.TTLMs) * time.Millisecond
	svc.SetWatchlistChecker(watchlist.NewService(providers, watchlist.NewDBCache(pool), ttl))

	// Seed declaratively-configured sources (idempotent upsert + schedule).
	sources := make([]domain.Source, 0, len(cfg.Sources))
	for _, cs := range cfg.Sources {
		src := domain.Source{
			Code:        cs.Code,
			Name:        cs.Name,
			FetcherType: cs.FetcherType,
			ObjectType:  cs.ObjectType,
			Locator:     cs.Locator,
			Cron:        cs.Cron,
			Enabled:     cs.EnabledOrDefault(),
		}
		if err := svc.SeedSource(ctx, src); err != nil {
			return nil, werror.WrapWithContextParams(ctx, err, "seed hermenea source", werror.SafeParam("source", cs.Code))
		}
		sources = append(sources, src)
	}

	// Connector-plane self-registration (M53 / D-ConnectorPlane): make hermenea VISIBLE in oikumenea's
	// connector registry — its row + declared sources — and bind the run reporter the application service
	// pushes each run's open/close through. Reuses the loader's base URL + shared secret (the core
	// resolves it to the `hermenea-importer` service principal), so no separate credential. Registration
	// is BEST-EFFORT: a failure logs and boot continues, because the core registry is a read model and
	// must never gate hermenea's own operation. It is convergent, so the next boot reconciles.
	if cfg.Oikumenea.BaseURL != "" {
		rep, err := reporter.New(cfg.Oikumenea.BaseURL, importToken, cfg.Oikumenea.InsecureSkipVerify,
			time.Duration(cfg.Oikumenea.HTTPTimeoutMs)*time.Millisecond)
		if err != nil {
			return nil, werror.WrapWithContextParams(ctx, err, "build hermenea connector reporter")
		}
		svc.SetReporter(rep)
		if err := rep.Register(ctx, ConnectorCode, ConnectorName, ConnectorDescription, sources); err != nil {
			svc1log.FromContext(ctx).Warn("hermenea: connector-plane self-registration failed (visibility only); will retry next boot",
				svc1log.SafeParam("connector", ConnectorCode), svc1log.Stacktrace(err))
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

// workerID labels this process in worker_jobs.locked_by (the runtime suffixes -w<i> per worker
// goroutine). Hostname-based so replicas are distinguishable (R-13); informational only — claim
// safety comes from SKIP LOCKED, not the label.
func workerID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "hermenea"
	}
	return "hermenea-" + host
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
		WorkerID:     workerID(),
		Concurrency:  w.Concurrency, // <=0 → 1 worker (runtime default)
		PollInterval: ms(w.PollIntervalMs, 2000),
		ScheduleTick: ms(w.ScheduleTickMs, 30000),
		BackoffBase:  ms(w.BackoffBaseMs, 5000),
		BackoffMax:   ms(w.BackoffMaxMs, 300000),
		JobTimeout:   ms(w.JobTimeoutMs, 120000),
		StaleAfter:   ms(w.StaleAfterMs, 600000),
	}
}
