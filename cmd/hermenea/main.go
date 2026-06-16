// Command hermenea is the companion ingestion + scheduler service (M16 / D-Hermenea): a witchcraft
// server with its OWN PostgreSQL that fetches → stages → maps external reference data and loads it
// into oikumenea over HTTP (POST /import/{objectType}) — it never touches oikumenea's database.
//
// Two runtime shared secrets come from the environment (not install config):
//   - OIKUMENEA_HERMENEA_TOKEN — authenticates inbound push triggers (validated by the Authenticator).
//   - HERMENEA_OIKUMENEA_TOKEN — authorizes hermenea's outbound calls to oikumenea's import endpoint.
package main

import (
	"context"
	"os"

	"github.com/olegamysk/go-oikumenea/internal/hermenea"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/config"
	hdb "github.com/olegamysk/go-oikumenea/internal/hermenea/db"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/transport"
	werror "github.com/palantir/witchcraft-go-error"
	wconfig "github.com/palantir/witchcraft-go-server/v2/config"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

func main() { os.Exit(serve()) }

func serve() int {
	// Runtime shared secrets (env, not install config).
	triggerToken := os.Getenv("OIKUMENEA_HERMENEA_TOKEN") // inbound: authenticate oikumenea's push triggers
	importToken := os.Getenv("HERMENEA_OIKUMENEA_TOKEN")  // outbound: authorize calls to oikumenea's import API

	auth := transport.NewAuthenticator(triggerToken)

	server := witchcraft.NewServer().
		WithInstallConfigType(config.Install{}).
		WithRuntimeConfigType(wconfig.Runtime{}).
		WithInstallConfigFromFile("var/conf/hermenea-install.yml").
		WithRuntimeConfigFromFile("var/conf/hermenea-runtime.yml").
		WithSelfSignedCertificate().
		WithMiddleware(auth.Handle).
		WithInitFunc(func(ctx context.Context, info witchcraft.InitInfo) (func(), error) {
			install, ok := info.InstallConfig.(config.Install)
			if !ok {
				return nil, werror.ErrorWithContextParams(ctx, "unexpected install config type")
			}
			pool, err := hdb.NewPool(ctx, install.Postgres.DSN)
			if err != nil {
				return nil, err
			}
			cleanup, err := hermenea.Register(ctx, info, pool, install, importToken)
			if err != nil {
				pool.Close()
				return nil, err
			}
			return cleanup, nil
		})

	if err := server.Start(); err != nil {
		return 1
	}
	return 0
}
