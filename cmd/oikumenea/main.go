// Command oikumenea is the composition root (docs/modules/platform.md). `serve` (the default)
// boots the witchcraft server; bootstrap-admin / recover-admin are the break-glass admin-recovery
// subcommands (D-Bootstrap), stubbed until M8.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/olegamysk/go-oikumenea/internal/audit"
	"github.com/olegamysk/go-oikumenea/internal/localization"
	"github.com/olegamysk/go-oikumenea/internal/membership"
	"github.com/olegamysk/go-oikumenea/internal/person"
	"github.com/olegamysk/go-oikumenea/internal/platform"
	"github.com/olegamysk/go-oikumenea/internal/platform/config"
	"github.com/olegamysk/go-oikumenea/internal/rank"
	"github.com/olegamysk/go-oikumenea/internal/tenant"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "serve":
		os.Exit(serve())
	case "bootstrap-admin", "recover-admin":
		// Break-glass first/lost-admin recovery reuses the bootstrap seed transaction
		// (D-Bootstrap). Implemented in M8 (identity-federation + authorization land first).
		fmt.Fprintf(os.Stderr, "%s: not implemented until M8 (identity-federation + first-admin bootstrap)\n", cmd)
		os.Exit(2)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (known: serve, bootstrap-admin, recover-admin)\n", cmd)
		os.Exit(2)
	}
}

func serve() int {
	server := witchcraft.NewServer().
		WithInstallConfigType(config.Install{}).
		WithRuntimeConfigType(config.Runtime{}).
		WithInstallConfigFromFile("var/conf/install.yml").
		WithRuntimeConfigFromFile("var/conf/runtime.yml").
		WithSelfSignedCertificate().
		WithInitFunc(initServer)

	if err := server.Start(); err != nil {
		// witchcraft already logged the structured error; signal non-zero exit.
		return 1
	}
	return 0
}

// initServer is the composition root's InitFunc (overview.md): wire the shared platform services,
// then each module's module.go in dependency order. The audit application service is threaded into
// the domain modules so their writes record in-transaction (D-Audit); localization's returned
// service (TranslationsFor / NamesByID) is threaded into tenant to assemble localized responses.
func initServer(ctx context.Context, info witchcraft.InitInfo) (func(), error) {
	pool, cleanup, err := platform.Bootstrap(ctx, info)
	if err != nil {
		return nil, err
	}

	auditSvc, err := audit.Register(info, pool)
	if err != nil {
		cleanup()
		return nil, err
	}

	locSvc, err := localization.Register(info, pool, auditSvc)
	if err != nil {
		cleanup()
		return nil, err
	}

	if _, err := tenant.Register(info, pool, auditSvc, locSvc); err != nil {
		cleanup()
		return nil, err
	}

	rankSvc, err := rank.Register(info, pool, auditSvc, locSvc)
	if err != nil {
		cleanup()
		return nil, err
	}

	if _, err := person.Register(info, pool, auditSvc, locSvc, rankSvc); err != nil {
		cleanup()
		return nil, err
	}

	if _, err := membership.Register(info, pool, auditSvc, locSvc); err != nil {
		cleanup()
		return nil, err
	}

	return cleanup, nil
}
