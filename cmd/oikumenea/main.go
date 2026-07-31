// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Command oikumenea is the composition root (docs/modules/platform.md). `serve` (the default) boots
// the witchcraft server; bootstrap-admin / recover-admin are the break-glass admin-recovery
// subcommands (D-Bootstrap) that reuse the same idempotent first-admin seed transaction.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/audit"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization"
	authzapp "github.com/olegamysk/go-oikumenea/internal/authorization/application"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	"github.com/olegamysk/go-oikumenea/internal/company"
	companyapp "github.com/olegamysk/go-oikumenea/internal/company/application"
	hermeneaapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/hermenea"
	identityapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/identityfederation"
	"github.com/olegamysk/go-oikumenea/internal/connector"
	"github.com/olegamysk/go-oikumenea/internal/connectorcall"
	"github.com/olegamysk/go-oikumenea/internal/dataimport"
	importdomain "github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/document"
	"github.com/olegamysk/go-oikumenea/internal/education"
	educationapp "github.com/olegamysk/go-oikumenea/internal/education/application"
	"github.com/olegamysk/go-oikumenea/internal/externalorg"
	"github.com/olegamysk/go-oikumenea/internal/finance"
	"github.com/olegamysk/go-oikumenea/internal/geo"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation"
	identityapp "github.com/olegamysk/go-oikumenea/internal/identityfederation/application"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/bootstrap"
	identitydomain "github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/middleware"
	"github.com/olegamysk/go-oikumenea/internal/language"
	"github.com/olegamysk/go-oikumenea/internal/links"
	"github.com/olegamysk/go-oikumenea/internal/localization"
	"github.com/olegamysk/go-oikumenea/internal/membership"
	"github.com/olegamysk/go-oikumenea/internal/order"
	"github.com/olegamysk/go-oikumenea/internal/person"
	"github.com/olegamysk/go-oikumenea/internal/personprofile"
	"github.com/olegamysk/go-oikumenea/internal/personsensitive"
	"github.com/olegamysk/go-oikumenea/internal/pinax"
	"github.com/olegamysk/go-oikumenea/internal/platform"
	"github.com/olegamysk/go-oikumenea/internal/platform/catalog"
	"github.com/olegamysk/go-oikumenea/internal/platform/config"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/platform/outbox"
	"github.com/olegamysk/go-oikumenea/internal/rank"
	rankadapters "github.com/olegamysk/go-oikumenea/internal/rank/adapters"
	rankapp "github.com/olegamysk/go-oikumenea/internal/rank/application"
	rankdomain "github.com/olegamysk/go-oikumenea/internal/rank/domain"
	"github.com/olegamysk/go-oikumenea/internal/religion"
	"github.com/olegamysk/go-oikumenea/internal/search"
	"github.com/olegamysk/go-oikumenea/internal/tenant"
	"github.com/olegamysk/go-oikumenea/internal/vehicle"
	"github.com/olegamysk/go-oikumenea/internal/watchlistclient"
	"github.com/olegamysk/go-oikumenea/pkg/authn"
	"github.com/olegamysk/go-oikumenea/pkg/config/envoverlay"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
	"github.com/palantir/pkg/refreshable"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-logging/wlog"
	_ "github.com/palantir/witchcraft-go-logging/wlog-zap" // register the default (zap) logging provider for CLI-constructed loggers
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
	refreshablefile "github.com/palantir/witchcraft-go-server/v2/witchcraft/refreshable"
	"gopkg.in/yaml.v3"
)

// installConfigPath / runtimeConfigPath are the default YAML locations. Both are OPTIONAL — an absent
// file plus environment variables is a valid env-only boot (D-EnvConfig).
const (
	installConfigPath = "var/conf/install.yml"
	runtimeConfigPath = "var/conf/runtime.yml"
	envPrefix         = "OIKUMENEA"
)

// envAliases preserves the R-16 documented env names on top of the schema-derived overlay: the two
// hermenea shared-secret tokens (OIKUMENEA_HERMENEA_TOKEN -> hermenea.outbound-token,
// HERMENEA_OIKUMENEA_TOKEN -> hermenea.inbound-token). The canonical path-derived names
// (…_OUTBOUND_TOKEN / …_INBOUND_TOKEN) win when both are set.
var envAliases = map[string]envoverlay.Path{
	"OIKUMENEA_HERMENEA_TOKEN": {"hermenea", "outbound-token"},
	"HERMENEA_OIKUMENEA_TOKEN": {"hermenea", "inbound-token"},
}

// cfgBytesFn adapts a func to witchcraft's ConfigBytesProvider (LoadBytes).
type cfgBytesFn func() ([]byte, error)

func (f cfgBytesFn) LoadBytes() ([]byte, error) { return f() }

// runtimeConfigProvider builds the runtime-config refreshable: it live-reloads var/conf/runtime.yml
// (when present) and overlays the environment on each tick; when the file is absent it is a static,
// env-only refreshable. Env is static — env changes require a restart.
func runtimeConfigProvider(ctx context.Context) (refreshable.Refreshable, error) {
	rt := reflect.TypeOf(config.Runtime{})
	if _, err := os.Stat(runtimeConfigPath); errors.Is(err, os.ErrNotExist) {
		b, aerr := envoverlay.Apply(nil, rt, envPrefix, envoverlay.OSEnviron())
		if aerr != nil {
			return nil, aerr
		}
		return refreshable.NewDefaultRefreshable(b), nil
	}
	base, err := refreshablefile.NewFileRefreshable(ctx, runtimeConfigPath)
	if err != nil {
		return nil, err
	}
	return base.Map(func(v any) any {
		out, aerr := envoverlay.Apply(v.([]byte), rt, envPrefix, envoverlay.OSEnviron())
		if aerr != nil {
			return v.([]byte) // fail-open on a mid-flight bad reload; boot already validated once
		}
		return out
	}), nil
}

func main() {
	// Load ./.env (if present) into the process environment before anything reads config — for serve
	// AND every CLI subcommand. Real env vars always win over .env (D-EnvConfig).
	if err := envoverlay.LoadDotEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: reading .env: %v\n", err)
	}

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
	case "outbox-selftest-enqueue":
		// Env-gated outbox self-test driver (review R-13, Phase 8): enqueue N notify events onto the
		// shared outbox for the scripts/scale-e2e.sh two-replica verification. Inert unless OIKUMENEA_OUTBOX_SELFTEST=1.
		os.Exit(runOutboxSelftestEnqueue(os.Args[2:]))
	case "rewrap":
		// Envelope key-rotation maintenance (review R-22 / D-CryptoProvider): re-wrap every DEK under the
		// active KEK (optionally reindex blind indexes). Operator-host-gated, like the seed/admin CLIs.
		os.Exit(runRewrapCLI(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (known: serve, bootstrap-admin, recover-admin, seed, rewrap)\n", cmd)
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
		// Install/runtime config = the YAML file (optional) with the environment overlaid on top; the
		// bytes we return are still ECV-decrypted + unmarshaled by witchcraft (D-EnvConfig).
		WithInstallConfigProvider(cfgBytesFn(func() ([]byte, error) {
			return envoverlay.LoadFileOverlayWithAliases(installConfigPath, reflect.TypeOf(config.Install{}), envPrefix, envAliases)
		})).
		WithRuntimeConfigProviderFunc(runtimeConfigProvider).
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

	// Fail fast if the Go RID registry (pkg/rid) has drifted from the migration-seeded
	// platform_rid_services / platform_rid_types tables: a mismatched registry would mint wrong RIDs
	// (R-28, review-2026-09). conventions.md § Resource identifiers documents this as boot-asserted.
	if err := rid.AssertMatches(ctx, pool); err != nil {
		cleanup()
		return nil, werror.Wrap(err, "rid registry drifted from platform_rid_* seed")
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

	// personsensitive (R-09): the person directory's envelope-encrypted / pii:special data (physical
	// identity, declared ethnicity, overlays, encrypted party membership, watchlist matches + sanctions).
	// Built before person core because the one PersonService transport composes it; it reuses the envelope
	// cipher (D-CryptoProvider) and validates eye/hair colors against the color catalog.
	sensitiveSvc := personsensitive.Register(pool, auditSvc, cipher)
	sensitiveSvc.SetColorLookup(colorSvc)

	// personprofile (R-09): the person directory's non-encrypted, person-owned directory data
	// (citizenships, residences, addresses, contact channels, SPEAKS languages, relationships,
	// non-encrypted institutional ties). Composed into the one PersonService transport; its address FK
	// check binds the location seam once geo exists (SetLocationLookup, below).
	profileSvc := personprofile.Register(pool, auditSvc)

	// personsensitive's watchlist screening snapshots the PEP flag from personprofile's government-position
	// ties (the M33/M34 seam) — bound now that both split services exist (D-PersonModuleSplit, R-09).
	sensitiveSvc.SetPEPStatusReader(profileSvc)

	personSvc, err := person.Register(info, pool, auditSvc, locSvc, rankSvc, enforcer, profileSvc, sensitiveSvc)
	if err != nil {
		cleanup()
		return nil, err
	}
	// Person subscribes to order's rank-change effect (D-OrderApply): RankChangeOrdered -> SetRank in
	// the issue transaction.
	personSvc.SubscribeOrderEvents(bus)
	// On a person purge, the personprofile / personsensitive modules erase (hard-delete or crypto-erase)
	// their own person_* rows in the purge transaction via PersonPurged (D-PersonModuleSplit, R-09).
	profileSvc.SubscribePersonPurge(bus)
	sensitiveSvc.SubscribePersonPurge(bus)

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
	// A disabled vertical's permission codes are not grantable (D-DataPacks, M54): the authz service
	// rejects a role or principal grant naming a `finance.*` / `religion.*` / … code while that module
	// is off. The codes stay in the static catalog (so re-enabling is a config flip), just non-grantable.
	authzSvc.SetDisabledModulePrefixes(install.DisabledModulePrefixes())

	// Document: person-held papers and envelope-encrypted personal codes (D-Documents / D-PersonalCodes).
	// Reuses the envelope cipher built above (D-CryptoProvider) + the personal-code validator registry;
	// the enforcer it holds is bound by authorization above.
	documentSvc, err := document.Register(info, pool, auditSvc, locSvc, enforcer, cipher, personalcode.New(), personSvc)
	if err != nil {
		cleanup()
		return nil, err
	}
	documentSvc.SubscribePersonEvents(bus)
	documentSvc.SubscribePersonPurge(bus) // erase this module's rows on PersonPurged (D-PersonModuleSplit)

	// Data import (M16 / D-Hermenea): the generic POST /import/{objectType} endpoint the out-of-process
	// hermenea companion calls to load reference data (it never touches this DB). Idempotent,
	// non-destructive, audited as a `system` actor; the enforcer it holds is bound by authorization.
	importSvc, err := dataimport.Register(info, pool, auditSvc, enforcer, install.Hermenea.BaseURL, install.Hermenea.ResolveOutboundToken(), install.Hermenea.InsecureSkipVerify)
	if err != nil {
		cleanup()
		return nil, err
	}

	// Roll the audit_log monthly partition window forward (review-2026-07 R-07 / D-AuditRetention):
	// ensure the current + next month's range partition exists so every audited write lands in a real
	// partition, never the DEFAULT catch-all. Advisory-locked (R-13) so replicas booting a fresh DB
	// don't race the CREATE; idempotent, and non-fatal (a DEFAULT partition backstops any gap).
	if err := db.WithAdvisoryLock(ctx, pool, db.LockBootSeed, func(ctx context.Context) error {
		return auditSvc.EnsureCurrentPartitions(ctx)
	}); err != nil {
		svc1log.FromContext(ctx).Error("audit partition roll-forward failed (non-fatal; DEFAULT partition backstops)",
			svc1log.Stacktrace(err))
	}
	// Surface the operator's audit-retention intent (D-AuditRetention): enforcement is manual (the
	// detach_audit_partitions_before helper + dump/drop runbook), so a configured window is a posture
	// note, not an automated action.
	if m := install.Audit.RetentionMonths; m > 0 {
		svc1log.FromContext(ctx).Info("audit retention configured (operator-enforced via detach_audit_partitions_before)",
			svc1log.SafeParam("retentionMonths", m))
	}

	// pinax reference-plane autoseed (D-Pinax, M45): self-seed the go:embed-ed bundled presets through
	// the import service above (create-if-absent, version-gated) so a fresh oikumenea is usable without
	// the hermenea companion. A malformed bundle fails boot (NewSeeder); a runtime seed error is logged
	// and NON-fatal (the seed is idempotent — it retries next boot, and `oikumenea seed` surfaces it).
	// Gated by pinax.autoseed (default on); flip to false to seed manually via the `seed` subcommand.
	// Serialized across replicas by the boot-seed advisory lock (R-13): a second replica booting the
	// same fresh DB waits, then finds every preset already applied (version-gated no-op).
	if install.Pinax.AutoseedEnabled() {
		seeder, err := pinax.NewSeeder(pool, importSvc, pinaxNativeImporters(rankSvc), install.Pinax.Packs)
		if err != nil {
			cleanup()
			return nil, werror.Wrap(err, "load pinax presets")
		}
		if err := db.WithAdvisoryLock(ctx, pool, db.LockBootSeed, func(ctx context.Context) error {
			_, err := seeder.Seed(ctx, false)
			return err
		}); err != nil {
			svc1log.FromContext(ctx).Error("pinax autoseed failed (non-fatal; retries next boot)",
				svc1log.Stacktrace(err))
		}
	}

	// Geo + Location (M16 / D-Geo + M19 / D-Location): the read-only GET /geo/countries lookup (clients
	// resolve a country to its RID) plus the audited LocationService CRUD + spatial queries over the
	// shared place entity. Both live on the `location` RID service (12); Location writes record via the
	// audit service and assemble place-type name maps via localization.
	geoSvc, err := geo.Register(info, pool, auditSvc, locSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}
	// Person addresses (M32 / D-PersonAddresses) verify their location_id against the location service
	// before writing; bind that cross-module query seam now that geo exists (late-bound: person is built
	// above, before geo — mirrors SetMembershipReader).
	profileSvc.SetLocationLookup(geoSvc)

	// Watchlist screening seam (M34 / D-Watchlists): CheckWatchlists runs a live screening check OUT to
	// the hermenea companion (which owns the OFAC/EU/UN/INTERPOL egress + the ≤24h cache). Wire the seam
	// only when the companion is configured — otherwise CheckWatchlists returns a clear "not configured"
	// error. Reuses the OIKUMENEA_HERMENEA trigger secret + trust direction (the web tier never reaches
	// the companion directly). Late-bound, mirroring SetLocationLookup.
	if install.Hermenea.BaseURL != "" {
		// The connector-call seam (M53) dials a deadline-bounded client — the R-12 deadline + no-retry
		// policy are enforced inside connectorcall.Dial, so this call site cannot forget them. The
		// watchlist is the first on-demand-lookup kind over that seam; a second kind dials the same way.
		wlHTTP, err := connectorcall.Dial(install.Hermenea.BaseURL, install.Hermenea.InsecureSkipVerify)
		if err != nil {
			cleanup()
			return nil, werror.Wrap(err, "build hermenea watchlist client")
		}
		sensitiveSvc.SetWatchlistLookup(watchlistclient.New(
			hermeneaapi.NewHermeneaServiceClient(wlHTTP), install.Hermenea.ResolveOutboundToken()))
	} else {
		// No companion configured: bind an explicit disabled no-op so the seam is always non-nil
		// (review-2026-07 R-11). CheckWatchlists then returns the clear "not configured" error via the
		// Disabled implementation rather than relying on a nil check in the person service.
		sensitiveSvc.SetWatchlistLookup(watchlistclient.Disabled{})
	}

	// Language (M18 / D-Languages): read-only lookup over the Glottolog languoid forest + ISO-15924
	// writing systems. The registry is written by the hermenea import pipeline (language-scheme /
	// language-scripts), not here.
	languageSvc, err := language.Register(info, pool, locSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}

	// Education (M20 / D-Education): external reference institutions + their structure tree (+ closure),
	// buildings (→ M19 location), groups, positions/appointments (mirror membership), and the person
	// bindings (enrollments, dorm stays). Writes record via the audit service; translatable names
	// assemble via localization.
	// M54 (D-DataPacks): the six enrichment verticals below are gated on `modules.<name>.enabled`
	// (default on). A disabled module registers NO routes (→ 404), skips its event subscriptions, and
	// leaves its service nil — but its schema still migrated (atlas is independent), so re-enabling is a
	// config flip. education/company keep a lifted service var because unified search fans them in below.
	var educationSvc *educationapp.Service
	if install.ModuleEnabled("education") {
		educationSvc, err = education.Register(info, pool, auditSvc, locSvc, tenantSvc, enforcer)
		if err != nil {
			cleanup()
			return nil, err
		}
		educationSvc.SubscribePersonEvents(bus)
		educationSvc.SubscribePersonPurge(bus) // erase education person-owned rows on PersonPurged (D-PersonModuleSplit)
	}

	// Company (M21 / D-Companies): a legal-entity registry over person + the M19 location foundation —
	// companies, registrations, industries, locations, positions/appointments, and the ownership/
	// affiliation graph. Writes record via the audit service; translatable names assemble via localization.
	var companySvc *companyapp.Service
	if install.ModuleEnabled("company") {
		companySvc, err = company.Register(info, pool, auditSvc, locSvc, tenantSvc, enforcer)
		if err != nil {
			cleanup()
			return nil, err
		}
		companySvc.SubscribePersonEvents(bus)
		companySvc.SubscribePersonPurge(bus) // erase company person-link rows on PersonPurged (D-PersonModuleSplit)
	}

	// Vehicle (M26 / D-Vehicles): a vehicle registry over person + the M21 company registry — brand/
	// model/type catalogs, the vehicle object (VIN), the brand→manufacturer link, and the ownership+
	// plate registration record (plate region → the WOF geo_places gazetteer). Writes record via the
	// audit service; translatable catalog names assemble via localization.
	if install.ModuleEnabled("vehicle") {
		vehicleSvc, err := vehicle.Register(info, pool, auditSvc, locSvc, enforcer, colorSvc)
		if err != nil {
			cleanup()
			return nil, err
		}
		vehicleSvc.SubscribePersonEvents(bus)
		vehicleSvc.SubscribePersonPurge(bus) // erase this module's rows on PersonPurged (D-PersonModuleSplit)
	}

	// Finance (M44 / D-Finance): bank accounts (envelope-encrypted IBAN) + payment cards (envelope-
	// encrypted PAN, no CVV) as authoritative first-party directory data. A bank is a `company`-domain
	// tenant organization (M21/M41); ownership is a polymorphic person|company holder link. Reuses the
	// shared cipher (D-CryptoProvider) + the personal-code validator registry (D-PersonalCodes: IBAN/PAN)
	// already built for the document module. A person purge crypto-erases solely-held accounts + cards.
	if install.ModuleEnabled("finance") {
		financeSvc, err := finance.Register(info, pool, auditSvc, locSvc, enforcer, cipher, personalcode.New())
		if err != nil {
			cleanup()
			return nil, err
		}
		financeSvc.SubscribePersonEvents(bus)
		financeSvc.SubscribePersonPurge(bus) // erase this module's rows on PersonPurged (D-PersonModuleSplit)
	}

	// External organizations (M30 / D-ExternalOrgs): the registry of external orgs a person is tied to
	// (parties, government bodies, foreign military, NGOs, registrants) — the node-space the M33
	// institutional-tie edges FK. Instance-global reference data, catalog-typed, provisional/resolved +
	// attribution; a hermenea import target (the `external-organizations` object-type is registered on the
	// dataimport side). Writes record via the audit service; translatable names assemble via localization.
	if install.ModuleEnabled("externalorg") {
		if _, err := externalorg.Register(info, pool, auditSvc, locSvc, enforcer); err != nil {
			cleanup()
			return nil, err
		}
	}

	// Connector plane (M53 / D-ConnectorPlane): the fleet registry. Connectors (deployable agents beside
	// the core — hermenea is the first) self-register and report their sync runs here; operators read the
	// fleet. Visibility, not orchestration — no endpoint triggers a run. Writes record via the audit
	// service under the reporting principal (the M51 machine-actor shape).
	connectorSvc, err := connector.Register(info, pool, auditSvc, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}
	// The pull-wiring read API (M53): resolve natural keys to RIDs, read reference catalogs, and let a
	// connector read its own cursors. Composed over the connector registry + the geo/language/legal-basis
	// catalogs (all constructed above) + localization for country names; each surface its own wiring.* code.
	if err := connector.RegisterWiring(info, connectorSvc, geoSvc, languageSvc, catalog.NewService(pool, auditSvc), locSvc, enforcer); err != nil {
		cleanup()
		return nil, err
	}

	// Religion (M22 / D-Religion): the multi-faith taxonomy (recursive religion_taxa + closure) with a
	// catalog-driven level marker + theism classification, the per-faith catalogs, and the per-unit
	// organization attributes (profile/classifications/policies). Org nodes reuse tenant units; the
	// canonical/tradition/affiliation graphs are migration-seeded. Reuses tenantSvc for createChildOrg.
	if install.ModuleEnabled("religion") {
		religionSvc, err := religion.Register(info, pool, auditSvc, locSvc, tenantSvc, enforcer, cipher)
		if err != nil {
			cleanup()
			return nil, err
		}
		religionSvc.SubscribePersonEvents(bus)
		religionSvc.SubscribePersonPurge(bus) // erase this module's rows on PersonPurged (D-PersonModuleSplit)
	}

	// Unified search (review-2026-09 R-26 / D-UnifiedSearch): ONE cross-type endpoint fanning in the
	// per-module trigram queries. The module owns no tables; providers register here with their
	// D-VisibilityScope adapters (search_providers.go), and the engine joins the boot seam loop so an
	// empty provider set fails startup instead of serving an empty (or untrimmed) search.
	searchSvc, err := search.Register(info, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := registerSearchProviders(searchSvc, personSvc, membershipSvc, languageSvc, geoSvc, educationSvc, companySvc); err != nil {
		cleanup()
		return nil, werror.Wrap(err, "composition root: search provider registration")
	}

	// Generic link traversal (review-2026-09 R-27 / D-LinkTraversal): ONE endpoint answering "what
	// links does object X have?" as a fan-in over the reified link tables. The module owns no tables;
	// descriptors register here (link_descriptors.go), and the engine's coverage assertion joins the
	// boot seam loop so a kind=link type that is neither registered nor exempt fails startup.
	linksSvc, err := links.Register(info, pool, enforcer)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := registerLinkDescriptors(linksSvc, pool, membershipSvc, authzSvc, locSvc); err != nil {
		cleanup()
		return nil, werror.Wrap(err, "composition root: link descriptor registration")
	}

	// Dashboard bucket labels (M57 / D-ObjectFacets): the ref-facet RIDs a stats response returns are
	// named through the SAME per-type resolvers the link engine uses (stats_labelers.go), so a unit
	// reads identically in a graph row and in a chart segment. The coverage assertion is pure and runs
	// here rather than in the seam loop below, because it checks a compile-time table against the
	// catalog, not a wired holder.
	if err := assertBucketLabelersBound(); err != nil {
		cleanup()
		return nil, werror.Wrap(err, "composition root: stats bucket labelers")
	}
	statsLabeler := bucketLabeler(pool, locSvc)
	personSvc.SetBucketLabeler(statsLabeler)
	tenantSvc.SetBucketLabeler(statsLabeler)
	membershipSvc.SetBucketLabeler(statsLabeler)
	orderSvc.SetBucketLabeler(statsLabeler)
	documentSvc.SetBucketLabeler(statsLabeler)
	auditSvc.SetBucketLabeler(statsLabeler)

	// Identity-federation: the external-IdP seam. Its application service is the (issuer, subject)
	// resolver the validation middleware binds to.
	identitySvc, err := identityfederation.Register(info, pool, auditSvc, enforcer, install.IdentityLinkingEnabled, issuerOptions(install),
		install.LoginSecurity.DedupWindow(), install.LoginSecurity.RetentionDays)
	if err != nil {
		cleanup()
		return nil, err
	}
	identitySvc.SubscribePersonEvents(bus)
	// Erase a purged person's login/IP security-log rows in the purge transaction (M37 /
	// D-LoginSecurityLog: pii:contact is erased, not retained).
	identitySvc.SubscribePersonPurge(bus)

	// Bind the inbound-token validation middleware: the configured issuers' validator, the
	// (issuer, subject) resolver, the person directory (JIT claim -> person.code), and the JIT flag.
	// Refuse symmetric (HS256) issuers outside local/dev before binding — fail closed (F-009).
	vcfg := validatorConfig(install)
	if err := middleware.GuardSymmetricIssuers(vcfg.Issuers, install.Environment); err != nil {
		cleanup()
		return nil, werror.Wrap(err, "reject symmetric issuer outside local/dev")
	}
	// The reserved local issuer backs the shared-secret fallback principal and must never be a real
	// configured issuer, or an operator's IdP could mint tokens for it (M51 / D-ServiceIdentities).
	if err := middleware.GuardReservedIssuer(vcfg.Issuers); err != nil {
		cleanup()
		return nil, werror.Wrap(err, "reject reserved issuer in idp config")
	}
	// Late-bind the machine-subject registry into authorization (M51): identity-federation registers
	// after authorization, so the grant writer gets its principal directory here — asserted below.
	authzSvc.BindPrincipalDirectory(identitySvc)
	authenticator.Bind(middleware.NewValidator(vcfg), identitySvc, personSvc, install.IDP.JIT.Enabled, authzSvc, pool, identitySvc, authzSvc)
	// Login security log (M37 / D-LoginSecurityLog): the validation middleware emits a deduped login/IP
	// occurrence per validated human request via identitySvc. trust-forwarded-for must be on only when a
	// facade sets an authoritative X-Forwarded-For (D-HeadlessTopology amended).
	authenticator.SetLoginRecorder(identitySvc, install.LoginSecurity.TrustForwardedFor)
	// Best-effort retention sweep at boot (retain-forever when retention-days is 0). A scheduled
	// enforcer is an explicit open seam (mirrors D-AuditRetention); a boot sweep gives retention teeth
	// without a scheduler.
	if n, err := identitySvc.SweepLoginEvents(ctx); err != nil {
		svc1log.FromContext(ctx).Warn("login-event retention sweep", svc1log.Stacktrace(err))
	} else if n > 0 {
		svc1log.FromContext(ctx).Info("login-event retention sweep", svc1log.SafeParam("deleted", n))
	}

	// The hermenea import shared secret (D-Hermenea, retained by D-ServiceIdentities as the
	// minimal-install fallback): the inbound token from install config (hermenea.inbound-token),
	// honouring the HERMENEA_OIKUMENEA_TOKEN env override via ResolveInboundToken (review R-16).
	//
	// Since M51 the secret no longer grants a hard-coded exemption: it authenticates a REGISTERED
	// principal seeded here (create-if-absent, under the boot-seed advisory lock so replicas do not
	// race) holding an instance-wide import.manage grant. A shared-secret caller therefore produces
	// the same subject shape, the same grants and the same audit attribution as a client-credentials
	// one — one downstream path, and the operator can revoke it like any other machine.
	if inbound := install.Hermenea.ResolveInboundToken(); inbound != "" {
		authenticator.SetImportServiceToken(inbound)
		if err := db.WithAdvisoryLock(ctx, pool, db.LockBootSeed, func(ctx context.Context) error {
			return seedSharedSecretPrincipal(ctx, identitySvc, authzSvc)
		}); err != nil {
			cleanup()
			return nil, werror.Wrap(err, "seed shared-secret import principal")
		}
	}

	// First-admin bootstrap (D-Bootstrap): idempotent — skips once any instance admin exists. The
	// has-admin check is read-then-write, so it is additionally serialized across replicas by the
	// boot-seed advisory lock (R-13): the losing replica waits, re-checks, and skips.
	if install.BootstrapAdmin != nil {
		var res bootstrap.Result
		err := db.WithAdvisoryLock(ctx, pool, db.LockBootSeed, func(ctx context.Context) error {
			var err error
			res, err = bootstrap.Run(ctx, pool, auditSvc, seedFrom(*install.BootstrapAdmin), bootstrap.Options{Subsystem: "bootstrap"})
			return err
		})
		if err != nil {
			cleanup()
			return nil, werror.Wrap(err, "first-admin bootstrap")
		}
		logBootstrap(ctx, res)
	}

	// Boot-time seam assertion (review-2026-07 R-11): every late-bound holder must be wired before we
	// serve. A forgotten Set*/Bind otherwise compiles and surfaces at request time — as a nil deref or,
	// worse, a silently-empty read-scope page that reads as "no access" rather than "mis-wired server".
	// Fail fast here, naming the missing seam, instead.
	// facet.Default is a package-level catalog rather than a wired service, so it cannot be
	// mis-injected — but it CAN be emptied or left inconsistent by an edit, and every list filter and
	// (from M57) every stats bucket reads it. Asserting it here puts it on the same footing as the
	// other seams: a broken vocabulary fails boot, not the first filtered request (D-ObjectFacets).
	for _, seam := range []interface{ MustBeBound() error }{authenticator, enforcer, personSvc, profileSvc, sensitiveSvc, searchSvc, linksSvc, facet.Default} {
		if err := seam.MustBeBound(); err != nil {
			cleanup()
			return nil, werror.Wrap(err, "composition root: late-bound seam not wired")
		}
	}
	if !authzSvc.PrincipalDirectoryBound() {
		cleanup()
		return nil, werror.Error("composition root: authorization principal directory not bound (M51)")
	}

	// Seal the atomic event bus (review-2026-07 R-10): every module has now wired its same-transaction
	// subscribers, so any later Subscribe is a mis-wire (it would race Publish and silently widen a
	// publisher's transaction). A late Subscribe now panics naming the type.
	bus.Seal()

	// Start the transactional-outbox dispatcher (R-10 / D-EventOutbox): it drains the after-commit
	// `notify` queue (oikumenea.platform_outbox) out of the write path. No notify producers/handlers
	// exist yet — every domain event is `atomic` today — so it runs live over an empty queue as a proven
	// seam; consumers register on it (before Seal) as `notify` needs land. Sealed with no handlers today.
	dispatcher := outbox.New(pool, outbox.Config{})
	// Env-gated multi-replica self-test seam (review R-13, Phase 8): wires a counting notify handler
	// (before Seal) only when OIKUMENEA_OUTBOX_SELFTEST=1, so the scale-e2e scenario can prove
	// cross-replica outbox delivery. A no-op otherwise — no handler, no table, no schema change.
	registerOutboxSelftest(ctx, pool, dispatcher)
	dispatcher.Seal()
	stopDispatcher := dispatcher.Start(ctx)
	baseCleanup := cleanup
	cleanup = func() { stopDispatcher(); baseCleanup() }

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
		// Previous KEKs are unwrap-only, retained across a rotation until `rewrap` completes (R-22).
		previous := make([][]byte, 0, len(c.LocalDev.PreviousKEKs))
		for i, pk := range c.LocalDev.PreviousKEKs {
			b, err := base64.StdEncoding.DecodeString(pk)
			if err != nil {
				return nil, werror.Wrap(err, "decode crypto.local-dev.previous-keks (base64)", werror.SafeParam("index", i))
			}
			previous = append(previous, b)
		}
		kp, err = crypto.NewLocalDevProviderWithPrevious(kek, previous)
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

// seedSharedSecretPrincipal registers the shared-secret import caller as a REAL service principal
// (M51 / D-ServiceIdentities) and grants it its instance-wide code set, all create-if-absent.
//
// This is what lets the D-Hermenea shared-secret path keep working without a hard-coded PEP
// exemption: the caller resolves through the same registry lookup as a client-credentials caller, so
// it carries a principal RID, real grants and real audit attribution. Idempotent — a second boot (or
// a replica that lost the advisory-lock race) re-runs the now-no-op checks.
//
// The set is import.manage (the push gate) plus the M53 connector-plane self-service codes
// connector.register + connector.report, so the same principal that pushes imports can also make
// hermenea VISIBLE in the connector registry (D-ConnectorPlane). NOT the wiring.* codes — hermenea
// pushes and reports; it does not pull-wire, so it holds no read grant it never exercises.
func seedSharedSecretPrincipal(ctx context.Context, identitySvc *identityapp.Service, authzSvc *authzapp.Service) error {
	principal, err := identitySvc.EnsurePrincipal(ctx, identitydomain.ServicePrincipal{
		Code:        authn.ServiceHermeneaImporter,
		Name:        "hermenea importer (shared secret)",
		Description: "Boot-seeded for the HERMENEA_OIKUMENEA_TOKEN fallback path (D-Hermenea).",
		Issuer:      middleware.ReservedLocalIssuer,
		Subject:     authn.ServiceHermeneaImporter,
	})
	if err != nil {
		return err
	}
	grants, err := authzSvc.ListPrincipalGrants(ctx, principal.ID)
	if err != nil {
		return err
	}
	held := map[authzdomain.Permission]bool{}
	for _, g := range grants {
		if g.OrgID == "" {
			held[g.Permission] = true
		}
	}
	for _, code := range []authzdomain.Permission{
		authzdomain.PermImportManage,
		authzdomain.PermConnectorRegister,
		authzdomain.PermConnectorReport,
	} {
		if held[code] {
			continue // already granted instance-wide
		}
		if _, err := authzSvc.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
			PrincipalID: principal.ID,
			Permission:  code,
		}); err != nil {
			return err
		}
	}
	return nil
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

	seeder, err := pinax.NewSeeder(pool, dataimport.NewImportService(pool, auditSvc), pinaxNativeImporters(newRankService(pool, auditSvc)), install.Pinax.Packs)
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

// loadInstall reads and parses the install config for the CLI subcommands, applying the SAME env
// overlay as serve (D-EnvConfig): a missing file is tolerated (env-only), and env vars override the
// YAML. Plaintext for local-dev; ECV-decryption of secret values is a deployment concern handled by
// the operator host (the CLI path does not decrypt ECV — env secrets here are plaintext).
func loadInstall(path string) (config.Install, error) {
	overlaid, err := envoverlay.LoadFileOverlayWithAliases(path, reflect.TypeOf(config.Install{}), envPrefix, envAliases)
	if err != nil {
		return config.Install{}, err
	}
	var install config.Install
	if err := yaml.Unmarshal(overlaid, &install); err != nil {
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
