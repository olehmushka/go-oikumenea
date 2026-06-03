// Package platform is the composition seam for the platform module: it wires the shared services
// (DB pool, readiness gate) and registers the operational Conjure routes onto the witchcraft
// router. Domain modules will expose an analogous Register entrypoint (overview.md).
package platform

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	platformapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/platform"
	"github.com/olegamysk/go-oikumenea/internal/platform/config"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/platform/health"
	"github.com/olegamysk/go-oikumenea/internal/platform/transport"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Bootstrap wires the shared platform services: it builds the DB pool, installs the readiness gate
// (DB reachability + a known schema revision — upgrade-safety.md), and registers the platform ops
// routes. It returns the pool so the composition root can hand it to each domain module's Register,
// plus a cleanup that closes the pool. This is the first thing cmd/oikumenea's InitFunc runs
// (overview.md: the composition root wires platform, then each module's module.go).
func Bootstrap(ctx context.Context, info witchcraft.InitInfo) (*pgxpool.Pool, func(), error) {
	install, ok := info.InstallConfig.(config.Install)
	if !ok {
		return nil, nil, werror.ErrorWithContextParams(ctx, "unexpected install config type")
	}

	pool, err := db.NewPool(ctx, install.Postgres.DSN, install.Environment)
	if err != nil {
		return nil, nil, err
	}

	info.Router.WithReadiness(health.NewReadinessSource(pool))

	// Register the operational Conjure routes. ProductVersion is the binary revision reported by
	// GET /status/version.
	ops := transport.NewOpsService(install.ProductVersion, db.ExpectedSchemaRevision)
	if err := platformapi.RegisterRoutesPlatformOpsService(info.Router, ops); err != nil {
		pool.Close()
		return nil, nil, werror.WrapWithContextParams(ctx, err, "register platform ops routes")
	}

	return pool, func() { pool.Close() }, nil
}
