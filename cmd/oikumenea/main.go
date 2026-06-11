// Command oikumenea is the composition root (docs/modules/platform.md). `serve` (the default) boots
// the witchcraft server; bootstrap-admin / recover-admin are the break-glass admin-recovery
// subcommands (D-Bootstrap) that reuse the same idempotent first-admin seed transaction.
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/audit"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	"github.com/olegamysk/go-oikumenea/internal/document"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/bootstrap"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/middleware"
	"github.com/olegamysk/go-oikumenea/internal/localization"
	"github.com/olegamysk/go-oikumenea/internal/membership"
	"github.com/olegamysk/go-oikumenea/internal/order"
	"github.com/olegamysk/go-oikumenea/internal/person"
	"github.com/olegamysk/go-oikumenea/internal/platform"
	"github.com/olegamysk/go-oikumenea/internal/platform/config"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/rank"
	"github.com/olegamysk/go-oikumenea/internal/tenant"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
	"gopkg.in/yaml.v3"
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
		// Break-glass first/lost-admin recovery reuses the bootstrap seed transaction (D-Bootstrap).
		// Operator-host-gated: possession of operator DB/host access is the authorization.
		os.Exit(runAdminCLI(cmd, os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (known: serve, bootstrap-admin, recover-admin)\n", cmd)
		os.Exit(2)
	}
}

func serve() int {
	// The inbound-token validation middleware is created UNBOUND and installed on the server before
	// Start: it (like the PEP) needs the DB pool + services that only exist inside the InitFunc, so it
	// is Bound there before any request is served. WithMiddleware wraps both the app and management
	// routers, so the middleware passes /status and /debug through unauthenticated (see middleware).
	authenticator := middleware.NewUnbound()

	server := witchcraft.NewServer().
		WithInstallConfigType(config.Install{}).
		WithRuntimeConfigType(config.Runtime{}).
		WithInstallConfigFromFile("var/conf/install.yml").
		WithRuntimeConfigFromFile("var/conf/runtime.yml").
		WithSelfSignedCertificate().
		WithMiddleware(authenticator.Handle).
		WithInitFunc(func(ctx context.Context, info witchcraft.InitInfo) (func(), error) {
			return initServer(ctx, info, authenticator)
		})

	if err := server.Start(); err != nil {
		// witchcraft already logged the structured error; signal non-zero exit.
		return 1
	}
	return 0
}

// initServer is the composition root's InitFunc (overview.md): wire the shared platform services,
// then each module's module.go in dependency order. The audit application service is threaded into
// the domain modules so their writes record in-transaction (D-Audit); localization's returned service
// assembles localized responses; the PEP enforcer is bound by authorization. M8 adds: the
// identity-federation module, binding the validation middleware (resolver + person directory + JIT),
// and the idempotent first-admin bootstrap (D-Bootstrap).
func initServer(ctx context.Context, info witchcraft.InitInfo, authenticator *middleware.Authenticator) (func(), error) {
	install, ok := info.InstallConfig.(config.Install)
	if !ok {
		return nil, werror.ErrorWithContextParams(ctx, "unexpected install config type")
	}

	pool, cleanup, err := platform.Bootstrap(ctx, info)
	if err != nil {
		return nil, err
	}

	// The PEP enforcer is created UNBOUND and threaded into every module's transport: the PDP it
	// fronts needs tenant's closure, so authorization is built last and binds the enforcer there — all
	// within this InitFunc, before any request is served (see internal/authorization/pep).
	enforcer := pep.NewUnbound()

	// The in-process event bus dispatches cross-module domain events to subscribers WITHIN the
	// publisher's transaction (pkg/events). M10 is its first user: order.issue publishes per-item
	// effect events that the membership/person subscribers (registered below, before serving) apply in
	// the issue transaction — all-or-nothing (D-OrderApply).
	bus := events.NewBus()

	auditSvc, err := audit.Register(info, pool, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}

	locSvc, err := localization.Register(info, pool, auditSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}

	tenantSvc, err := tenant.Register(info, pool, auditSvc, locSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}

	rankSvc, err := rank.Register(info, pool, auditSvc, locSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}

	personSvc, err := person.Register(info, pool, auditSvc, locSvc, rankSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}
	// Person subscribes to order's rank-change effect (D-OrderApply): RankChangeOrdered -> SetRank in
	// the issue transaction.
	personSvc.SubscribeOrderEvents(bus)

	membershipSvc, err := membership.Register(info, pool, auditSvc, locSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}
	// Membership subscribes to order's appointment/removal effects (D-OrderApply): AppointmentOrdered
	// fills/creates, RemovalOrdered ends — all in the issue transaction.
	membershipSvc.SubscribeOrderEvents(bus)
	// Person's read-scope projection (D-PersonReadScope) resolves a person's units through membership;
	// bind that cross-module query seam now that membership exists (late-bound: person is built first).
	personSvc.SetMembershipReader(membershipSvc)

	// Order: administrative orders (наказ). On issue it PUBLISHES the effect events the membership/
	// person subscribers above handle in the same transaction (D-OrderApply); the enforcer it holds is
	// bound by authorization below.
	if _, err := order.Register(info, pool, auditSvc, locSvc, enforcer, bus); err != nil {
		cleanup()
		return nil, err
	}

	// Authorization: builds the PDP over tenant's closure, seeds the base roles, and binds the
	// enforcer the modules above already hold (D-BaseRoles / D-RIDSeeding). Its service also resolves
	// each request's RLS reach for the authenticator's connection-pinning (D-RLSDefenseInDepth).
	authzSvc, err := authorization.Register(info, pool, auditSvc, locSvc, tenantSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}

	// Document: person-held papers and envelope-encrypted personal codes (D-Documents / D-PersonalCodes).
	// The envelope cipher (D-CryptoProvider) + the personal-code validator registry are built from
	// install config; the enforcer it holds is bound by authorization above.
	cipher, err := buildCipher(install)
	if err != nil {
		cleanup()
		return nil, werror.Wrap(err, "build envelope cipher")
	}
	if _, err := document.Register(info, pool, auditSvc, locSvc, enforcer, cipher, personalcode.New(), personSvc); err != nil {
		cleanup()
		return nil, err
	}

	// Identity-federation: the external-IdP seam. Its application service is the (issuer, subject)
	// resolver the validation middleware binds to.
	identitySvc, err := identityfederation.Register(info, pool, auditSvc, enforcer, install.IdentityLinkingEnabled)
	if err != nil {
		cleanup()
		return nil, err
	}

	// Bind the inbound-token validation middleware: the configured issuers' validator, the
	// (issuer, subject) resolver, the person directory (JIT claim -> person.code), and the JIT flag.
	authenticator.Bind(middleware.NewValidator(validatorConfig(install)), identitySvc, personSvc, install.IDP.JIT.Enabled, authzSvc, pool)

	// First-admin bootstrap (D-Bootstrap): idempotent — skips once any instance admin exists.
	if install.BootstrapAdmin != nil {
		res, err := bootstrap.Run(ctx, pool, auditSvc, seedFrom(*install.BootstrapAdmin), bootstrap.Options{Subsystem: "bootstrap"})
		if err != nil {
			cleanup()
			return nil, werror.Wrap(err, "first-admin bootstrap")
		}
		logBootstrap(ctx, res)
	}

	return cleanup, nil
}

// validatorConfig maps the install IDP config into the middleware's validator config, applying the
// documented defaults (60s clock skew, "person_code" JIT claim).
func validatorConfig(install config.Install) middleware.Config {
	issuers := make([]middleware.IssuerConfig, 0, len(install.IDP.Issuers))
	for _, is := range install.IDP.Issuers {
		issuers = append(issuers, middleware.IssuerConfig{
			Issuer:   is.Issuer,
			Audience: is.Audience,
			Type:     is.Type,
			HMACKey:  is.HMACKey,
		})
	}
	skew := time.Duration(install.IDP.ClockSkewSeconds) * time.Second
	if skew <= 0 {
		skew = 60 * time.Second
	}
	claim := install.IDP.JIT.Claim
	if claim == "" {
		claim = "person_code"
	}
	return middleware.Config{Issuers: issuers, ClockSkew: skew, JITEnabled: install.IDP.JIT.Enabled, JITClaim: claim}
}

// defaultDEKCacheTTLSeconds is the unwrapped-DEK cache window when the install config omits it.
const defaultDEKCacheTTLSeconds = 300

// buildCipher constructs the envelope cipher from the install crypto block (D-CryptoProvider): it
// selects the KeyProvider backend (today only local-dev), decodes the base64 KEK + blind-index key, and
// applies the DEK-cache TTL. A missing/short key is a fatal config error (personal codes can't be
// protected without it).
func buildCipher(install config.Install) (*crypto.Cipher, error) {
	c := install.Crypto
	blind, err := base64.StdEncoding.DecodeString(c.BlindIndexKey)
	if err != nil {
		return nil, werror.Wrap(err, "decode crypto.blind-index-key (base64)")
	}

	provider := c.Provider
	if provider == "" {
		provider = "local-dev"
	}
	var kp crypto.KeyProvider
	switch provider {
	case "local-dev":
		kek, err := base64.StdEncoding.DecodeString(c.LocalDev.KEK)
		if err != nil {
			return nil, werror.Wrap(err, "decode crypto.local-dev.kek (base64)")
		}
		kp, err = crypto.NewLocalDevProvider(kek)
		if err != nil {
			return nil, err
		}
	default:
		return nil, werror.Error("unsupported crypto provider (supported: local-dev)", werror.SafeParam("provider", provider))
	}

	ttl := c.DEKCacheTTLSeconds
	if ttl == 0 {
		ttl = defaultDEKCacheTTLSeconds
	}
	return crypto.NewCipher(kp, blind, time.Duration(ttl)*time.Second)
}

func seedFrom(b config.BootstrapAdmin) bootstrap.AdminSeed {
	return bootstrap.AdminSeed{
		Issuer:      b.Issuer,
		Subject:     b.Subject,
		Email:       b.Email,
		DisplayName: b.DisplayName,
		PersonCode:  b.PersonCode,
	}
}

func logBootstrap(ctx context.Context, res bootstrap.Result) {
	logger := svc1log.FromContext(ctx)
	if res.Skipped {
		logger.Info("first-admin bootstrap skipped: an instance admin already exists")
		return
	}
	logger.Info("first-admin bootstrap seeded instance admin",
		svc1log.SafeParam("personId", res.PersonID),
		svc1log.SafeParam("accountId", res.AccountID),
		svc1log.SafeParam("createdPerson", res.CreatedPerson))
}

// ---------------------------------------------------------------- admin-recovery CLI

// runAdminCLI runs the break-glass bootstrap-admin / recover-admin subcommands (D-Bootstrap). It
// loads the install config, opens the operator pool, respects the boot-time schema-version check, and
// runs the same idempotent seed transaction the first-boot path uses. Its writes audit as a `system`
// actor under subsystem "recover-admin".
func runAdminCLI(cmd string, args []string) int {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	var configPath, issuer, subject, email, display, code string
	var force bool
	fs.StringVar(&configPath, "config", "var/conf/install.yml", "path to the install config")
	fs.StringVar(&issuer, "issuer", "", "IdP issuer (overrides install bootstrap-admin.issuer)")
	fs.StringVar(&subject, "subject", "", "IdP subject (overrides install bootstrap-admin.subject)")
	fs.StringVar(&email, "email", "", "asserted email (optional)")
	fs.StringVar(&display, "display-name", "", "seeded person display name")
	fs.StringVar(&code, "person-code", "", "stable person code (link-to-existing when set)")
	fs.BoolVar(&force, "force", false, "seed even when an instance admin already exists (recover)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	install, err := loadInstall(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: load install config: %v\n", cmd, err)
		return 1
	}

	seed := bootstrap.AdminSeed{Issuer: issuer, Subject: subject, Email: email, DisplayName: display, PersonCode: code}
	if install.BootstrapAdmin != nil {
		seed = mergeSeed(seedFrom(*install.BootstrapAdmin), seed)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, install.Postgres.DSN, install.Environment)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: connect database: %v\n", cmd, err)
		return 1
	}
	defer pool.Close()

	// Respect the boot-time schema-version check (D-Bootstrap): refuse to seed against an
	// unknown/mismatched schema.
	rev, err := db.ReadSchemaRevision(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: read schema version: %v\n", cmd, err)
		return 1
	}
	if rev != db.ExpectedSchemaRevision {
		fmt.Fprintf(os.Stderr, "%s: schema revision %q != expected %q; run migrations first\n", cmd, rev, db.ExpectedSchemaRevision)
		return 1
	}

	auditSvc := auditapp.NewService(pool,
		func(conn db.DBTX) auditdomain.Repository { return auditadapters.NewRepository(conn) },
		func() int { return 50 })

	res, err := bootstrap.Run(ctx, pool, auditSvc, seed, bootstrap.Options{Force: force, Subsystem: "recover-admin"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmd, err)
		return 1
	}
	if res.Skipped {
		fmt.Fprintf(os.Stdout, "%s: an instance admin already exists; nothing to do (use --force to seed anyway)\n", cmd)
		return 0
	}
	fmt.Fprintf(os.Stdout, "%s: seeded instance admin (person=%s account=%s)\n", cmd, res.PersonID, res.AccountID)
	return 0
}

// loadInstall reads and parses the install config (plaintext for local-dev; ECV-decryption of secret
// values is a deployment concern handled by the operator host).
func loadInstall(path string) (config.Install, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return config.Install{}, err
	}
	var install config.Install
	if err := yaml.Unmarshal(raw, &install); err != nil {
		return config.Install{}, err
	}
	return install, nil
}

// mergeSeed overlays non-empty flag values on top of the install-config seed.
func mergeSeed(base, override bootstrap.AdminSeed) bootstrap.AdminSeed {
	if override.Issuer != "" {
		base.Issuer = override.Issuer
	}
	if override.Subject != "" {
		base.Subject = override.Subject
	}
	if override.Email != "" {
		base.Email = override.Email
	}
	if override.DisplayName != "" {
		base.DisplayName = override.DisplayName
	}
	if override.PersonCode != "" {
		base.PersonCode = override.PersonCode
	}
	return base
}
