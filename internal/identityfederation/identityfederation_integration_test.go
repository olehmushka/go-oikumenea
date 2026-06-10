//go:build integration

// Integration tests for the identity-federation module against a real Postgres (M8 exit criteria,
// D-Bootstrap / D-JIT / D-Audit):
//
//   - an account + its first external identity are created and the (issuer, subject) RESOLVES to the
//     account's person (a valid token would map to that PDP subject);
//
//   - an unknown (issuer, subject) does NOT resolve (the middleware would reject);
//
//   - linking an ADDITIONAL identity is gated by account.identity_linking.enabled;
//
//   - a duplicate (issuer, subject) is rejected (one identity, one account);
//
//   - unlinking removes the login point (resolution then fails); disabling the account stops resolution;
//
//   - just-in-time link-on-match (D-JIT) creates/extends an EXISTING person's account and links;
//
//   - the first-admin bootstrap seeds person+account+identity+instance-admin in one transaction and is
//     idempotent; it skips when an admin already exists and Force is false.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/identityfederation/...
package identityfederation_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzadapters "github.com/olegamysk/go-oikumenea/internal/authorization/adapters"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/adapters"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/application"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/bootstrap"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	personadapters "github.com/olegamysk/go-oikumenea/internal/person/adapters"
	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newAudit(pool *pgxpool.Pool) *auditapp.Service {
	return auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
}

// newService builds the identity-federation application service directly (bypassing Register), with a
// caller-controllable identity_linking.enabled flag.
func newService(t *testing.T, linking *bool) (*application.Service, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(t)
	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	return application.NewService(pool, repoFor, newAudit(pool), func() bool { return *linking }), pool
}

func makePerson(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	repo := personadapters.NewRepository(pool)
	p, err := repo.InsertPerson(context.Background(), persondomain.Person{
		Code: code,
		Name: persondomain.Name{DisplayName: "Test " + code},
		Sex:  persondomain.DefaultSex,
	})
	if err != nil {
		t.Fatalf("insert person: %v", err)
	}
	return p.ID
}

func uniq(prefix string) string { return prefix + "-" + uuid.NewString()[:12] }

func TestAccountLifecycleAndResolution(t *testing.T) {
	linking := true
	svc, pool := newService(t, &linking)
	ctx := context.Background()
	personID := makePerson(t, pool, uniq("p"))
	issuer := "https://idp.example"
	subject := uniq("sub")

	acct, err := svc.CreateAccount(ctx, domain.Account{PersonID: personID}, &domain.ExternalIdentity{Issuer: issuer, Subject: subject})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if len(acct.Identities) != 1 {
		t.Fatalf("expected one initial identity, got %d", len(acct.Identities))
	}

	// Resolution maps the verified (issuer, subject) to the account's person.
	res, err := svc.Resolve(ctx, issuer, subject)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.PersonID != personID || res.AccountID != acct.ID {
		t.Fatalf("resolution mismatch: %+v (want person %s account %s)", res, personID, acct.ID)
	}

	// A second account for the same person is refused.
	if _, err := svc.CreateAccount(ctx, domain.Account{PersonID: personID}, nil); !errors.Is(err, domain.ErrAccountConflict) {
		t.Fatalf("second account: want ErrAccountConflict, got %v", err)
	}

	// Unknown (issuer, subject) does not resolve.
	if _, err := svc.Resolve(ctx, issuer, uniq("nope")); !errors.Is(err, domain.ErrIdentityNotFound) {
		t.Fatalf("unknown identity: want ErrIdentityNotFound, got %v", err)
	}

	// Linking an additional identity is allowed when linking is enabled.
	second := uniq("sub2")
	if _, err := svc.LinkIdentity(ctx, acct.ID, domain.ExternalIdentity{Issuer: issuer, Subject: second}); err != nil {
		t.Fatalf("link second identity: %v", err)
	}

	// A duplicate (issuer, subject) is rejected.
	if _, err := svc.LinkIdentity(ctx, acct.ID, domain.ExternalIdentity{Issuer: issuer, Subject: subject}); !errors.Is(err, domain.ErrIdentityConflict) {
		t.Fatalf("duplicate identity: want ErrIdentityConflict, got %v", err)
	}

	// With linking disabled, a THIRD identity is refused (the first-on-account exception does not apply).
	linking = false
	if _, err := svc.LinkIdentity(ctx, acct.ID, domain.ExternalIdentity{Issuer: issuer, Subject: uniq("sub3")}); !errors.Is(err, domain.ErrLinkingDisabled) {
		t.Fatalf("link with linking disabled: want ErrLinkingDisabled, got %v", err)
	}
	linking = true

	// Disabling the account stops resolution.
	if _, err := svc.DisableAccount(ctx, acct.ID); err != nil {
		t.Fatalf("disable account: %v", err)
	}
	if _, err := svc.Resolve(ctx, issuer, subject); !errors.Is(err, domain.ErrIdentityNotFound) {
		t.Fatalf("resolve after disable: want ErrIdentityNotFound, got %v", err)
	}
}

func TestUnlinkIdentity(t *testing.T) {
	linking := true
	svc, pool := newService(t, &linking)
	ctx := context.Background()
	personID := makePerson(t, pool, uniq("p"))
	issuer, subject := "https://idp.example", uniq("sub")

	acct, err := svc.CreateAccount(ctx, domain.Account{PersonID: personID}, &domain.ExternalIdentity{Issuer: issuer, Subject: subject})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	idID := acct.Identities[0].ID

	if err := svc.UnlinkIdentity(ctx, acct.ID, idID); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if _, err := svc.Resolve(ctx, issuer, subject); !errors.Is(err, domain.ErrIdentityNotFound) {
		t.Fatalf("resolve after unlink: want ErrIdentityNotFound, got %v", err)
	}
	// Unlinking again is a not-found.
	if err := svc.UnlinkIdentity(ctx, acct.ID, idID); !errors.Is(err, domain.ErrIdentityNotFound) {
		t.Fatalf("re-unlink: want ErrIdentityNotFound, got %v", err)
	}
}

func TestJITLinkOnMatch(t *testing.T) {
	linking := true
	svc, pool := newService(t, &linking)
	ctx := context.Background()
	personID := makePerson(t, pool, uniq("p"))
	issuer, subject := "https://idp.example", uniq("sub")

	// First login of an unknown identity that matched an existing person -> create account + link.
	res, err := svc.LinkOnMatch(ctx, personID, issuer, subject, uniq("jit")+"@example.test")
	if err != nil {
		t.Fatalf("link on match: %v", err)
	}
	if res.PersonID != personID {
		t.Fatalf("JIT resolved wrong person: %+v", res)
	}
	// Now the identity resolves directly.
	again, err := svc.Resolve(ctx, issuer, subject)
	if err != nil || again.PersonID != personID {
		t.Fatalf("resolve after JIT: %+v err=%v", again, err)
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	audit := newAudit(pool)
	seed := bootstrap.AdminSeed{
		Issuer:      "https://bootstrap.example",
		Subject:     uniq("admin-sub"),
		DisplayName: "Bootstrap Admin",
		PersonCode:  uniq("admin"),
	}

	// Force seeds even though other tests may have created instance admins already.
	first, err := bootstrap.Run(ctx, pool, audit, seed, bootstrap.Options{Force: true, Subsystem: "recover-admin"})
	if err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	if first.PersonID == "" || first.AccountID == "" || first.InstanceAdmin == "" {
		t.Fatalf("bootstrap did not seed fully: %+v", first)
	}
	if !first.CreatedPerson || !first.CreatedAccount {
		t.Fatalf("bootstrap should have created person+account: %+v", first)
	}

	// The seeded identity resolves to the seeded person.
	repo := adapters.NewRepository(pool)
	res, err := repo.ResolveBySubject(ctx, seed.Issuer, seed.Subject)
	if err != nil {
		t.Fatalf("resolve seeded identity: %v", err)
	}
	if res.PersonID != first.PersonID {
		t.Fatalf("seeded identity resolves to %s, want %s", res.PersonID, first.PersonID)
	}

	// The seeded person is an active instance admin.
	authz := authzadapters.NewRepository(pool)
	isAdmin, err := authz.IsActiveInstanceAdmin(ctx, first.PersonID)
	if err != nil || !isAdmin {
		t.Fatalf("seeded person not instance admin: isAdmin=%v err=%v", isAdmin, err)
	}

	// Re-running with the same seed is idempotent: same person, nothing newly created.
	second, err := bootstrap.Run(ctx, pool, audit, seed, bootstrap.Options{Force: true, Subsystem: "recover-admin"})
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if second.PersonID != first.PersonID {
		t.Fatalf("idempotent re-run changed person: %s -> %s", first.PersonID, second.PersonID)
	}
	if second.CreatedPerson || second.CreatedAccount || second.InstanceAdmin != "" {
		t.Fatalf("idempotent re-run created new rows: %+v", second)
	}

	// Without Force, since an instance admin now exists, the run is skipped.
	skipped, err := bootstrap.Run(ctx, pool, audit, seed, bootstrap.Options{Subsystem: "bootstrap"})
	if err != nil {
		t.Fatalf("skip bootstrap: %v", err)
	}
	if !skipped.Skipped {
		t.Fatalf("expected skip when an instance admin exists, got %+v", skipped)
	}
}
