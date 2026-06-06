// Package application holds the identity-federation module's application service — the orchestrator
// the transport layer (and the validation middleware) call to read/mutate accounts and external
// identities, recording an audit row in the same transaction as each write (D-Audit). It depends on
// the domain port, the platform DB surface, and the audit service; it never imports the adapters
// package directly (the repository factory is injected by module.go).
//
// go-oikumenea stores no credentials and issues no tokens (L-AuthzOnly): the writes here are
// directory operations (who may log in, mapped to which person), and Resolve is the read the inbound
// middleware uses to turn a verified (issuer, subject) into a PDP subject. Linking ADDITIONAL
// identities beyond the first is gated by account.identity_linking.enabled (an install knob threaded
// in as linkingEnabled); just-in-time link-on-match (D-JIT) is the login-time create/extend path.
package application

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// auditSubsystem labels the interim system actor for admin writes; jitSubsystem labels the
// login-time JIT link. Until the acting person is always resolvable these are recorded as `system`
// actions (the no-unaudited-mutation ground rule still holds). The bootstrap path audits separately
// under its own subsystem (D-Bootstrap).
const (
	auditSubsystem = "identity-federation-admin"
	jitSubsystem   = "identity-federation-jit"
)

// Audit target types.
const (
	targetAccount  = "account"
	targetIdentity = "external_identity"
)

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a
// caller's transaction for an audited write (D-Audit). Injected by module.go so the application layer
// never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the identity-federation application service. It owns its writes, so it holds the pool to
// open transactions; reads run on the pool directly.
type Service struct {
	pool           *pgxpool.Pool
	newRepo        RepositoryFactory
	audit          *auditapp.Service
	linkingEnabled func() bool // account.identity_linking.enabled (install config)
}

// NewService wires the service with the pool, the repository factory, the audit service every write
// records into, and the linking-enabled config accessor.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, linkingEnabled func() bool) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, linkingEnabled: linkingEnabled}
}

// ---------------------------------------------------------------- accounts

// CreateAccount creates an optional login account for a person, optionally linking its first external
// identity in the same transaction (always permitted — the linking cap applies only to ADDITIONAL
// identities). Unknown person -> ErrUnknownPerson (DB FK); a person with an active account ->
// ErrAccountConflict; an already-linked initial (issuer, subject) -> ErrIdentityConflict.
func (s *Service) CreateAccount(ctx context.Context, a domain.Account, initial *domain.ExternalIdentity) (domain.Account, error) {
	if err := a.Validate(); err != nil {
		return domain.Account{}, err
	}
	if initial != nil {
		if err := initial.Validate(); err != nil {
			return domain.Account{}, err
		}
	}
	var out domain.Account
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		created, err := repo.InsertAccount(ctx, a)
		if err != nil {
			return err
		}
		if err := s.record(ctx, tx, "account.create", targetAccount, created.ID, map[string]any{"id": created.ID, "personId": created.PersonID, "status": string(created.Status)}); err != nil {
			return err
		}
		if initial != nil {
			initial.AccountID = created.ID
			id, err := repo.InsertIdentity(ctx, *initial)
			if err != nil {
				return err
			}
			if err := s.recordIdentity(ctx, tx, "identity.link", id); err != nil {
				return err
			}
			created.Identities = []domain.ExternalIdentity{id}
		}
		out = created
		return nil
	})
	return out, err
}

// GetAccount reads one account with its linked identities attached.
func (s *Service) GetAccount(ctx context.Context, id string) (domain.Account, error) {
	repo := s.newRepo(s.pool)
	a, err := repo.GetAccount(ctx, id)
	if err != nil {
		return domain.Account{}, err
	}
	ids, err := repo.ListIdentitiesByAccount(ctx, id)
	if err != nil {
		return domain.Account{}, err
	}
	a.Identities = ids
	return a, nil
}

// DisableAccount disables login on an account (reversible). Idempotent: a not-found account surfaces
// ErrAccountNotFound; disabling an already-disabled account is a harmless re-flip.
func (s *Service) DisableAccount(ctx context.Context, id string) (domain.Account, error) {
	var out domain.Account
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		disabled, err := repo.DisableAccount(ctx, id)
		if err != nil {
			return err
		}
		out = disabled
		return s.record(ctx, tx, "account.disable", targetAccount, id, map[string]any{"id": id, "status": string(disabled.Status)})
	})
	return out, err
}

// ---------------------------------------------------------------- external identities

// LinkIdentity links an additional external identity to an existing account. The first identity on an
// account is always permitted; linking a SECOND-or-later one is refused with ErrLinkingDisabled when
// account.identity_linking.enabled is false. An (issuer, subject) already linked anywhere ->
// ErrIdentityConflict (the global unique index).
func (s *Service) LinkIdentity(ctx context.Context, accountID string, e domain.ExternalIdentity) (domain.ExternalIdentity, error) {
	if err := e.Validate(); err != nil {
		return domain.ExternalIdentity{}, err
	}
	e.AccountID = accountID
	var out domain.ExternalIdentity
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetAccount(ctx, accountID); err != nil {
			return err
		}
		count, err := repo.CountActiveIdentities(ctx, accountID)
		if err != nil {
			return err
		}
		if count >= 1 && !s.linkingEnabled() {
			return domain.ErrLinkingDisabled
		}
		linked, err := repo.InsertIdentity(ctx, e)
		if err != nil {
			return err
		}
		out = linked
		return s.recordIdentity(ctx, tx, "identity.link", linked)
	})
	return out, err
}

// UnlinkIdentity removes an external identity from an account (hard delete). A missing identity (or
// one belonging to a different account) surfaces ErrIdentityNotFound.
func (s *Service) UnlinkIdentity(ctx context.Context, accountID, identityID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeleteIdentity(ctx, accountID, identityID); err != nil {
			return err
		}
		return s.record(ctx, tx, "identity.unlink", targetIdentity, identityID, map[string]any{"id": identityID, "accountId": accountID})
	})
}

// ---------------------------------------------------------------- resolution (the middleware seam)

// Resolve maps a verified (issuer, subject) to its active account + person (the PDP subject), or
// ErrIdentityNotFound when there is no link or the account is not active. Read-only; the validation
// middleware calls it after checking the token signature/claims.
func (s *Service) Resolve(ctx context.Context, issuer, subject string) (domain.Resolution, error) {
	return s.newRepo(s.pool).ResolveBySubject(ctx, issuer, subject)
}

// LinkOnMatch is the just-in-time provisioning path (D-JIT): on first login of a verified
// (issuer, subject) that matched an EXISTING person (resolved upstream from a token claim ->
// person.code), create-or-extend that person's account and link the identity, returning the
// resolution. It NEVER creates a person. An (issuer, subject) already linked to a DIFFERENT account
// surfaces ErrIdentityConflict (reject).
func (s *Service) LinkOnMatch(ctx context.Context, personID, issuer, subject, email string) (domain.Resolution, error) {
	id := domain.ExternalIdentity{Issuer: issuer, Subject: subject}
	if err := id.Validate(); err != nil {
		return domain.Resolution{}, err
	}
	var out domain.Resolution
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		account, err := repo.GetActiveAccountByPerson(ctx, personID)
		if errors.Is(err, domain.ErrAccountNotFound) {
			account, err = repo.InsertAccount(ctx, domain.Account{PersonID: personID, Email: email})
			if err != nil {
				return err
			}
			if err := s.recordJIT(ctx, tx, "account.create", targetAccount, account.ID, map[string]any{"id": account.ID, "personId": personID}); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		id.AccountID = account.ID
		linked, err := repo.InsertIdentity(ctx, id)
		if err != nil {
			return err
		}
		if err := s.recordJIT(ctx, tx, "identity.link", targetIdentity, linked.ID, map[string]any{"id": linked.ID, "accountId": account.ID, "issuer": issuer}); err != nil {
			return err
		}
		out = domain.Resolution{PersonID: personID, AccountID: account.ID, Email: account.Email}
		return nil
	})
	return out, err
}

// ---------------------------------------------------------------- helpers

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// recordIdentity writes the audit row for an identity link, carrying only non-PII identifiers (the
// identity/account ids + issuer); the subject (pii:basic) is never logged.
func (s *Service) recordIdentity(ctx context.Context, tx pgx.Tx, action string, e domain.ExternalIdentity) error {
	return s.record(ctx, tx, action, targetIdentity, e.ID, map[string]any{"id": e.ID, "accountId": e.AccountID, "issuer": e.Issuer})
}

func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID string, after any) error {
	return s.recordAs(ctx, tx, auditSubsystem, action, targetType, targetID, after)
}

func (s *Service) recordJIT(ctx context.Context, tx pgx.Tx, action, targetType, targetID string, after any) error {
	return s.recordAs(ctx, tx, jitSubsystem, action, targetType, targetID, after)
}

// recordAs mints an Action RID in the caller's transaction and writes the audit row on it, so the
// audit entry commits iff the change commits (D-Audit). The actor is the interim system actor.
func (s *Service) recordAs(ctx context.Context, tx pgx.Tx, subsystem, action, targetType, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx, action)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  subsystem,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID composes the Action RID via the same SQL generator every module uses, so the audit
// log's action__<type> RID-shape CHECK is satisfied. "identity.link" -> "action__identity_link".
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_rid('account', $1)", "action__"+sanitizeAction(action)).Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

func sanitizeAction(action string) string {
	b := make([]byte, len(action))
	for i := 0; i < len(action); i++ {
		if action[i] == '.' {
			b[i] = '_'
		} else {
			b[i] = action[i]
		}
	}
	return string(b)
}

// requestID is the correlation key shared with logs/metrics/traces: the request's trace id, with a
// generated fallback for out-of-request callers (e.g. integration tests).
func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
