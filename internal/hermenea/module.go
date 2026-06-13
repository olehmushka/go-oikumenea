// Package hermenea is the composition seam for the hermenea companion service (M16 / D-Hermenea): it
// wires the store (its own DB), the connector registry, the loader (oikumenea's import endpoint), the
// application service, and the background runtime, then registers the HermeneaService routes. The
// per-object-type mappers are registered here too (geo-countries lands in a follow-up). Register
// returns a cleanup that drains the runtime and closes the pool.
package hermenea

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/adapters"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/application"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/config"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/connector"
	hermeneaapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/hermenea"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/loader"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/runtime"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/transport"
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

	// Per-object-type mappers are registered here. geo-countries lands in a follow-up; until a mapper
	// exists for a source's object-type, its sync jobs fail with ErrNoMapper (visible in import_runs).

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
