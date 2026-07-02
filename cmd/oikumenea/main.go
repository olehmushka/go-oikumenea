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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/audit"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	"github.com/olegamysk/go-oikumenea/internal/company"
	identityapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/identityfederation"
	"github.com/olegamysk/go-oikumenea/internal/dataimport"
	importdomain "github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/document"
	"github.com/olegamysk/go-oikumenea/internal/education"
	"github.com/olegamysk/go-oikumenea/internal/externalorg"
	"github.com/olegamysk/go-oikumenea/internal/geo"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/bootstrap"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/middleware"
	"github.com/olegamysk/go-oikumenea/internal/language"
	"github.com/olegamysk/go-oikumenea/internal/localization"
	"github.com/olegamysk/go-oikumenea/internal/membership"
	"github.com/olegamysk/go-oikumenea/internal/order"
	"github.com/olegamysk/go-oikumenea/internal/person"
	"github.com/olegamysk/go-oikumenea/internal/pinax"
	"github.com/olegamysk/go-oikumenea/internal/platform"
	"github.com/olegamysk/go-oikumenea/internal/platform/config"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/rank"
	rankadapters "github.com/olegamysk/go-oikumenea/internal/rank/adapters"
	rankapp "github.com/olegamysk/go-oikumenea/internal/rank/application"
	rankdomain "github.com/olegamysk/go-oikumenea/internal/rank/domain"
	"github.com/olegamysk/go-oikumenea/internal/religion"
	"github.com/olegamysk/go-oikumenea/internal/tenant"
	"github.com/olegamysk/go-oikumenea/internal/vehicle"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-logging/wlog"
	_ "github.com/palantir/witchcraft-go-logging/wlog-zap" // register the default (zap) logging provider for CLI-constructed loggers
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
	case "seed":
		// pinax reference-plane seed (D-Pinax, M45): manually apply the bundled presets (for
		// pinax.autoseed:false, or an explicit refresh). Operator-host-gated, like the admin CLIs.
		os.Exit(runSeedCLI(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (known: serve, bootstrap-admin, recover-admin, seed)\n", cmd)
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

	// The envelope cipher (D-CryptoProvider) is built here (before person) because person now seals the
	// pii:special declared ethnicity (D-PhysicalIdentity, M31); document/religion reuse the same instance.
	cipher, err := buildCipher(install)
	if err != nil {
		cleanup()
		return nil, werror.Wrap(err, "build envelope cipher")
	}

	// Platform reference catalogs (M29 / D-OverlayFoundation + D-Color): the GDPR lawful-basis catalog
	// and the per-domain color catalog. Composed here (not in platform.Bootstrap) because it needs the
	// audit service + localization + the (unbound-now, bound-later) PEP enforcer. The returned color
	// service is the hard-FK ColorLookup that person + vehicle validate eye/hair/vehicle colors against.
	colorSvc, err := platform.RegisterCatalog(ctx, info, pool, auditSvc, locSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}

	personSvc, err := person.Register(info, pool, auditSvc, locSvc, rankSvc, enforcer, cipher, colorSvc)
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
	// Cross-module re-home on a provisional→canonical merge (D-OverlayFoundation, M29): each module's
	// subscriber re-points its person-referencing rows in the merge transaction (person publishes
	// PersonMerged). Registered here, before serving.
	membershipSvc.SubscribePersonEvents(bus)
	// Person's read-scope projection (D-PersonReadScope) resolves a person's units through membership;
	// bind that cross-module query seam now that membership exists (late-bound: person is built first).
	personSvc.SetMembershipReader(membershipSvc)

	// Order: administrative orders (наказ). On issue it PUBLISHES the effect events the membership/
	// person subscribers above handle in the same transaction (D-OrderApply); the enforcer it holds is
	// bound by authorization below.
	orderSvc, err := order.Register(info, pool, auditSvc, locSvc, enforcer, bus)
	if err != nil {
		cleanup()
		return nil, err
	}
	orderSvc.SubscribePersonEvents(bus)

	// Authorization: builds the PDP over tenant's closure, seeds the base roles, and binds the
	// enforcer the modules above already hold (D-BaseRoles / D-RIDSeeding). Its service also resolves
	// each request's RLS reach for the authenticator's connection-pinning (D-RLSDefenseInDepth).
	authzSvc, err := authorization.Register(info, pool, auditSvc, locSvc, tenantSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}
	authzSvc.SubscribePersonEvents(bus)

	// Document: person-held papers and envelope-encrypted personal codes (D-Documents / D-PersonalCodes).
	// Reuses the envelope cipher built above (D-CryptoProvider) + the personal-code validator registry;
	// the enforcer it holds is bound by authorization above.
	documentSvc, err := document.Register(info, pool, auditSvc, locSvc, enforcer, cipher, personalcode.New(), personSvc)
	if err != nil {
		cleanup()
		return nil, err
	}
	documentSvc.SubscribePersonEvents(bus)

	// Data import (M16 / D-Hermenea): the generic POST /import/{objectType} endpoint the out-of-process
	// hermenea companion calls to load reference data (it never touches this DB). Idempotent,
	// non-destructive, audited as a `system` actor; the enforcer it holds is bound by authorization.
	importSvc, err := dataimport.Register(info, pool, auditSvc, enforcer, install.Hermenea.BaseURL, os.Getenv("OIKUMENEA_HERMENEA_TOKEN"), install.Hermenea.InsecureSkipVerify)
	if err != nil {
		cleanup()
		return nil, err
	}

	// pinax reference-plane autoseed (D-Pinax, M45): self-seed the go:embed-ed bundled presets through
	// the import service above (create-if-absent, version-gated) so a fresh oikumenea is usable without
	// the hermenea companion. A malformed bundle fails boot (NewSeeder); a runtime seed error is logged
	// and NON-fatal (the seed is idempotent — it retries next boot, and `oikumenea seed` surfaces it).
	// Gated by pinax.autoseed (default on); flip to false to seed manually via the `seed` subcommand.
	if install.Pinax.AutoseedEnabled() {
		seeder, err := pinax.NewSeeder(pool, importSvc, pinaxNativeImporters(rankSvc))
		if err != nil {
			cleanup()
			return nil, werror.Wrap(err, "load pinax presets")
		}
		if _, err := seeder.Seed(ctx, false); err != nil {
			svc1log.FromContext(ctx).Error("pinax autoseed failed (non-fatal; retries next boot)",
				svc1log.Stacktrace(err))
		}
	}

	// Geo + Location (M16 / D-Geo + M19 / D-Location): the read-only GET /geo/countries lookup (clients
	// resolve a country to its RID) plus the audited LocationService CRUD + spatial queries over the
	// shared place entity. Both live on the `location` RID service (12); Location writes record via the
	// audit service and assemble place-type name maps via localization.
	if _, err := geo.Register(info, pool, auditSvc, locSvc, enforcer); err != nil {
		cleanup()
		return nil, err
	}

	// Language (M18 / D-Languages): read-only lookup over the Glottolog languoid forest + ISO-15924
	// writing systems. The registry is written by the hermenea import pipeline (language-scheme /
	// language-scripts), not here.
	if _, err := language.Register(info, pool, locSvc, enforcer); err != nil {
		cleanup()
		return nil, err
	}

	// Education (M20 / D-Education): external reference institutions + their structure tree (+ closure),
	// buildings (→ M19 location), groups, positions/appointments (mirror membership), and the person
	// bindings (enrollments, dorm stays). Writes record via the audit service; translatable names
	// assemble via localization.
	educationSvc, err := education.Register(info, pool, auditSvc, locSvc, tenantSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}
	educationSvc.SubscribePersonEvents(bus)

	// Company (M21 / D-Companies): a legal-entity registry over person + the M19 location foundation —
	// companies, registrations, industries, locations, positions/appointments, and the ownership/
	// affiliation graph. Writes record via the audit service; translatable names assemble via localization.
	companySvc, err := company.Register(info, pool, auditSvc, locSvc, tenantSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}
	companySvc.SubscribePersonEvents(bus)

	// Vehicle (M26 / D-Vehicles): a vehicle registry over person + the M21 company registry — brand/
	// model/type catalogs, the vehicle object (VIN), the brand→manufacturer link, and the ownership+
	// plate registration record (plate region → the WOF geo_places gazetteer). Writes record via the
	// audit service; translatable catalog names assemble via localization.
	vehicleSvc, err := vehicle.Register(info, pool, auditSvc, locSvc, enforcer, colorSvc)
	if err != nil {
		cleanup()
		return nil, err
	}
	vehicleSvc.SubscribePersonEvents(bus)

	// External organizations (M30 / D-ExternalOrgs): the registry of external orgs a person is tied to
	// (parties, government bodies, foreign military, NGOs, registrants) — the node-space the M33
	// institutional-tie edges FK. Instance-global reference data, catalog-typed, provisional/resolved +
	// attribution; a hermenea import target (the `external-organizations` object-type is registered on the
	// dataimport side). Writes record via the audit service; translatable names assemble via localization.
	if _, err := externalorg.Register(info, pool, auditSvc, locSvc, enforcer); err != nil {
		cleanup()
		return nil, err
	}

	// Religion (M22 / D-Religion): the multi-faith taxonomy (recursive religion_taxa + closure) with a
	// catalog-driven level marker + theism classification, the per-faith catalogs, and the per-unit
	// organization attributes (profile/classifications/policies). Org nodes reuse tenant units; the
	// canonical/tradition/affiliation graphs are migration-seeded. Reuses tenantSvc for createChildOrg.
	religionSvc, err := religion.Register(info, pool, auditSvc, locSvc, tenantSvc, enforcer, cipher)
	if err != nil {
		cleanup()
		return nil, err
	}
	religionSvc.SubscribePersonEvents(bus)

	// Identity-federation: the external-IdP seam. Its application service is the (issuer, subject)
	// resolver the validation middleware binds to.
	identitySvc, err := identityfederation.Register(info, pool, auditSvc, enforcer, install.IdentityLinkingEnabled, issuerOptions(install))
	if err != nil {
		cleanup()
		return nil, err
	}
	identitySvc.SubscribePersonEvents(bus)

	// Bind the inbound-token validation middleware: the configured issuers' validator, the
	// (issuer, subject) resolver, the person directory (JIT claim -> person.code), and the JIT flag.
	// Refuse symmetric (HS256) issuers outside local/dev before binding — fail closed (F-009).
	vcfg := validatorConfig(install)
	if err := middleware.GuardSymmetricIssuers(vcfg.Issuers, install.Environment); err != nil {
		cleanup()
		return nil, werror.Wrap(err, "reject symmetric issuer outside local/dev")
	}
	authenticator.Bind(middleware.NewValidator(vcfg), identitySvc, personSvc, install.IDP.JIT.Enabled, authzSvc, pool)

	// The hermenea import service-principal shared secret (D-Hermenea / L-AuthzOnly amendment): a
	// RUNTIME secret from the environment (not install config). When set, a bearer matching it
	// authenticates the `hermenea-importer` principal holding exactly import.manage.
	authenticator.SetImportServiceToken(os.Getenv("HERMENEA_OIKUMENEA_TOKEN"))

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
// issuerOptions projects the install issuer config to the PUBLIC fields the identity-federation
// ListIssuers endpoint exposes to binding UIs. The HS256 verification secret is deliberately dropped —
// it is credential-equivalent and never leaves the process.
func issuerOptions(install config.Install) []identityapi.IssuerOption {
	opts := make([]identityapi.IssuerOption, 0, len(install.IDP.Issuers))
	for _, is := range install.IDP.Issuers {
		var audience *string
		if is.Audience != "" {
			a := is.Audience
			audience = &a
		}
		typ := is.Type
		if typ == "" {
			typ = middleware.IssuerOIDC
		}
		opts = append(opts, identityapi.IssuerOption{Issuer: is.Issuer, Audience: audience, Type: typ})
	}
	return opts
}

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

// ---------------------------------------------------------------- pinax seed CLI

// runSeedCLI manually applies the bundled pinax reference-plane presets (D-Pinax, M45) — for a
// pinax.autoseed:false deployment, or an explicit refresh. It mirrors the admin CLI (load config, open
// the operator pool, respect the schema-version gate) and drives the SAME import handlers the boot
// autoseeder uses. `--reconcile` re-runs every preset with update-on-change (create-if-absent otherwise).
func runSeedCLI(args []string) int {
	const cmd = "seed"
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	var configPath string
	var reconcile bool
	fs.StringVar(&configPath, "config", "var/conf/install.yml", "path to the install config")
	fs.BoolVar(&reconcile, "reconcile", false, "update existing seeded rows to the preset version (not just create-if-absent)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	install, err := loadInstall(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: load install config: %v\n", cmd, err)
		return 1
	}

	// Attach a real service logger so the seeder's svc1log calls emit structured lines to stderr instead
	// of the "logger not set on context" warning the bare Background() context triggers off the server path.
	ctx := svc1log.WithLogger(context.Background(), svc1log.New(os.Stderr, wlog.InfoLevel))
	pool, err := db.NewPool(ctx, install.Postgres.DSN, install.Environment)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: connect database: %v\n", cmd, err)
		return 1
	}
	defer pool.Close()

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

	seeder, err := pinax.NewSeeder(pool, dataimport.NewImportService(pool, auditSvc), pinaxNativeImporters(newRankService(pool, auditSvc)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: load pinax presets: %v\n", cmd, err)
		return 1
	}
	summaries, err := seeder.Seed(ctx, reconcile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmd, err)
		return 1
	}
	if len(summaries) == 0 {
		fmt.Fprintf(os.Stdout, "%s: all presets already up to date; nothing to do%s\n", cmd,
			map[bool]string{true: " (use --reconcile to force)", false: ""}[!reconcile])
		return 0
	}
	for name, sum := range summaries {
		fmt.Fprintf(os.Stdout, "%s: %-16s created=%d updated=%d skipped=%d\n", cmd, name, sum.Created, sum.Updated, sum.Skipped)
	}
	return 0
}

// pinaxNativeImporters builds the pinax native-importer registry (D-Pinax, M45): presets that own their
// own transaction / nested decoding and cannot go through the flat canonical-record import path.
// Currently just `ranks` — rank.Service.ImportPreset over the system→category→type→rank subtree, which
// manages its own transaction (so it can't run inside the shared import tx the flat handlers use).
func pinaxNativeImporters(rankSvc *rankapp.Service) map[string]pinax.NativeImporter {
	return map[string]pinax.NativeImporter{
		"ranks": func(ctx context.Context, records []map[string]any, _ bool) (importdomain.Summary, error) {
			// ImportPreset is an idempotent upsert; each record is one rank system. Boot autoseed is
			// version-gated (systems start empty), so create-if-absent vs reconcile collapses here.
			var sum importdomain.Summary
			for _, rec := range records {
				p, err := rankapp.PresetFromMap(rec)
				if err != nil {
					return importdomain.Summary{}, err
				}
				s, err := rankSvc.ImportPreset(ctx, p)
				if err != nil {
					return importdomain.Summary{}, err
				}
				sum.Created += s.Created
				sum.Updated += s.Updated
				sum.Skipped += s.Skipped
			}
			return sum, nil
		},
	}
}

// newRankService builds the rank application service off the pool + audit alone (no router) — for the
// pinax seed CLI (the boot path reuses the already-registered rankSvc).
func newRankService(pool *pgxpool.Pool, auditSvc *auditapp.Service) *rankapp.Service {
	return rankapp.NewService(pool,
		func(conn db.DBTX) rankdomain.Repository { return rankadapters.NewRepository(conn) }, auditSvc)
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
