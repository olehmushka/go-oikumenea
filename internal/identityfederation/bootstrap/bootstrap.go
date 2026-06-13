// Package bootstrap seeds the first instance admin (D-Bootstrap). Authentication is delegated
// (L-AuthzOnly) and no-self-escalation means the first admin cannot be granted from inside the API,
// so it is seeded out-of-band from operator config: in ONE transaction it creates (or reuses) a
// person, an account + external identity binding the configured IdP `(issuer, subject)`, and an
// instance-admin grant — all audited as a `system` action. It is idempotent: it skips entirely once
// any active instance admin exists (unless forced — the recover-admin break-glass path).
//
// This is composition-level glue (like main.go): it wires three modules' repositories onto one
// shared transaction so the seed is all-or-nothing. Bootstrap-origin grants set provenance to NULL;
// origin lives in the bootstrap audit row.
package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzadapters "github.com/olegamysk/go-oikumenea/internal/authorization/adapters"
	accountadapters "github.com/olegamysk/go-oikumenea/internal/identityfederation/adapters"
	accountdomain "github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	personadapters "github.com/olegamysk/go-oikumenea/internal/person/adapters"
	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// AdminSeed is the operator-supplied first-admin identity (from install config / CLI flags).
type AdminSeed struct {
	Issuer      string
	Subject     string
	Email       string
	DisplayName string
	PersonCode  string
}

// Options tunes a bootstrap run.
type Options struct {
	// Force runs the seed even when an instance admin already exists (the recover-admin break-glass
	// path; possession of operator DB/host access is the authorization — D-Bootstrap).
	Force bool
	// Subsystem labels the system audit actor: "bootstrap" (first boot) or "recover-admin" (CLI).
	Subsystem string
}

// Result reports what a bootstrap run did.
type Result struct {
	Skipped        bool // an instance admin already existed and Force was false
	PersonID       string
	AccountID      string
	InstanceAdmin  string // the instance-admin grant RID (empty when the person was already an admin)
	CreatedPerson  bool
	CreatedAccount bool
}

// ErrInvalidSeed indicates the operator-supplied seed is incomplete (missing issuer/subject).
var ErrInvalidSeed = errors.New("bootstrap admin seed requires issuer and subject")

// Run executes the idempotent first-admin seed in one transaction.
func Run(ctx context.Context, pool *pgxpool.Pool, audit *auditapp.Service, seed AdminSeed, opts Options) (Result, error) {
	if strings.TrimSpace(seed.Issuer) == "" || strings.TrimSpace(seed.Subject) == "" {
		return Result{}, ErrInvalidSeed
	}
	if opts.Subsystem == "" {
		opts.Subsystem = "bootstrap"
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	authzRepo := authzadapters.NewRepository(tx)
	hasAdmin, err := authzRepo.HasActiveInstanceAdmin(ctx)
	if err != nil {
		return Result{}, err
	}
	if hasAdmin && !opts.Force {
		return Result{Skipped: true}, nil // idempotent: an instance admin already exists
	}

	r := newRecorder(audit, opts.Subsystem)
	personRepo := personadapters.NewRepository(tx)
	accountRepo := accountadapters.NewRepository(tx)

	person, createdPerson, err := resolveOrCreatePerson(ctx, personRepo, seed)
	if err != nil {
		return Result{}, err
	}
	res := Result{PersonID: person.ID, CreatedPerson: createdPerson}
	if createdPerson {
		if err := r.record(ctx, tx, "person", "person.create", "person", person.ID, map[string]any{"id": person.ID}); err != nil {
			return Result{}, err
		}
	}

	account, createdAccount, err := resolveOrCreateAccount(ctx, accountRepo, person.ID, seed.Email)
	if err != nil {
		return Result{}, err
	}
	res.AccountID = account.ID
	res.CreatedAccount = createdAccount
	if createdAccount {
		if err := r.record(ctx, tx, "account", "account.create", "account", account.ID, map[string]any{"id": account.ID, "personId": person.ID}); err != nil {
			return Result{}, err
		}
	}

	if err := linkIdentity(ctx, accountRepo, r, tx, account.ID, person.ID, seed); err != nil {
		return Result{}, err
	}

	isAdmin, err := authzRepo.IsActiveInstanceAdmin(ctx, person.ID)
	if err != nil {
		return Result{}, err
	}
	if !isAdmin {
		admin, err := authzRepo.InsertInstanceAdmin(ctx, person.ID, "") // granted_by NULL (D-Bootstrap)
		if err != nil {
			return Result{}, err
		}
		res.InstanceAdmin = admin.ID
		if err := r.record(ctx, tx, "authz", "instance.admin.grant", "instance_admin", admin.ID, map[string]any{"id": admin.ID, "personId": person.ID}); err != nil {
			return Result{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return res, nil
}

// resolveOrCreatePerson reuses an existing person by code (D-JIT/D-Bootstrap link-to-existing) or
// creates one. DisplayName falls back to the code/subject so the NOT NULL display_name is satisfied.
func resolveOrCreatePerson(ctx context.Context, repo *personadapters.Repository, seed AdminSeed) (persondomain.Person, bool, error) {
	if seed.PersonCode != "" {
		p, err := repo.GetActivePersonByCode(ctx, seed.PersonCode)
		if err == nil {
			return p, false, nil
		}
		if !errors.Is(err, persondomain.ErrNotFound) {
			return persondomain.Person{}, false, err
		}
	}
	display := firstNonEmpty(seed.DisplayName, seed.PersonCode, seed.Subject)
	created, err := repo.InsertPerson(ctx, persondomain.Person{
		Code: seed.PersonCode,
		Name: persondomain.Name{DisplayName: display},
		Sex:  persondomain.DefaultSex, // the repo bypasses CreatePerson's default; satisfy the sex CHECK
	})
	if err != nil {
		return persondomain.Person{}, false, err
	}
	return created, true, nil
}

func resolveOrCreateAccount(ctx context.Context, repo *accountadapters.Repository, personID, email string) (accountdomain.Account, bool, error) {
	account, err := repo.GetActiveAccountByPerson(ctx, personID)
	if err == nil {
		return account, false, nil
	}
	if !errors.Is(err, accountdomain.ErrAccountNotFound) {
		return accountdomain.Account{}, false, err
	}
	created, err := repo.InsertAccount(ctx, accountdomain.Account{PersonID: personID, Email: email})
	if err != nil {
		return accountdomain.Account{}, false, err
	}
	return created, true, nil
}

// linkIdentity links the configured (issuer, subject) to the account, tolerating an idempotent
// re-run. It PRE-CHECKS existence rather than catching a unique-violation: a constraint error would
// poison the surrounding transaction (Postgres aborts it), so the follow-up query could not run. An
// identity that already maps to THIS person is a no-op; one mapping elsewhere is refused.
func linkIdentity(ctx context.Context, repo *accountadapters.Repository, r recorder, tx pgx.Tx, accountID, personID string, seed AdminSeed) error {
	existing, err := repo.ResolveBySubject(ctx, seed.Issuer, seed.Subject)
	switch {
	case err == nil:
		if existing.PersonID == personID {
			return nil // already linked to this person — idempotent
		}
		return accountdomain.ErrIdentityConflict // linked to a different person — refuse
	case !errors.Is(err, accountdomain.ErrIdentityNotFound):
		return err
	}
	linked, err := repo.InsertIdentity(ctx, accountdomain.ExternalIdentity{AccountID: accountID, Issuer: seed.Issuer, Subject: seed.Subject})
	if err != nil {
		return err
	}
	return r.record(ctx, tx, "account", "identity.link", "external_identity", linked.ID, map[string]any{"id": linked.ID, "accountId": accountID, "issuer": seed.Issuer})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// recorder writes audit rows for the seed on the shared transaction (D-Audit), as a `system` actor
// under the run's subsystem.
type recorder struct {
	audit     *auditapp.Service
	subsystem string
}

func newRecorder(audit *auditapp.Service, subsystem string) recorder {
	return recorder{audit: audit, subsystem: subsystem}
}

func (r recorder) record(ctx context.Context, tx pgx.Tx, service, action, targetType, targetID string, after map[string]any) error {
	rid, err := mintActionRID(ctx, tx, service, action)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(after)
	return r.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  r.subsystem,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      json.RawMessage(raw),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID mints an Action RID (kind=action=3, generic action type=0) on the seed transaction,
// keyed to the minting service so the rendered RID names its origin. The specific action name is
// recorded separately in audit_log.action (D-Audit); only kind=action matters to the shape CHECK.
func mintActionRID(ctx context.Context, tx pgx.Tx, service, action string) (string, error) {
	_ = action
	var svc int
	switch service {
	case "person":
		svc = 6
	case "account":
		svc = 9
	case "authz":
		svc = 8
	case "audit":
		svc = 3
	default:
		svc = 1 // platform
	}
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id($1, 3, 0)", svc).Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}
