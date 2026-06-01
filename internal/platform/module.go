// Package platform is the composition seam for the platform module: it wires the shared services
// (DB pool, readiness gate) and registers the operational Conjure routes onto the witchcraft
// router. Domain modules will expose an analogous Register entrypoint (overview.md).
package platform

import (
	"context"

	platformapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/platform"
	"github.com/olegamysk/go-oikumenea/internal/platform/config"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/platform/health"
	"github.com/olegamysk/go-oikumenea/internal/platform/transport"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Init is the witchcraft InitFunc for the M0 walking skeleton: build the DB pool, install the
// readiness gate, and register the platform ops routes. Returns a cleanup that closes the pool.
func Init(ctx context.Context, info witchcraft.InitInfo) (func(), error) {
	install, ok := info.InstallConfig.(config.Install)
	if !ok {
		return nil, werror.ErrorWithContextParams(ctx, "unexpected install config type")
	}

	pool, err := db.NewPool(ctx, install.Postgres.DSN, install.Environment)
	if err != nil {
		return nil, err
	}

	// Readiness gates on DB reachability + a known schema revision (upgrade-safety.md).
	info.Router.WithReadiness(health.NewReadinessSource(pool))

	// Register the operational Conjure routes. ProductVersion is the binary revision reported by
	// GET /status/version.
	ops := transport.NewOpsService(install.ProductVersion, db.ExpectedSchemaRevision)
	if err := platformapi.RegisterRoutesPlatformOpsService(info.Router, ops); err != nil {
		pool.Close()
		return nil, werror.WrapWithContextParams(ctx, err, "register platform ops routes")
	}

	return func() { pool.Close() }, nil
}
