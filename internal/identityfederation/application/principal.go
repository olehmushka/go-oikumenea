// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// The service-principal registry (M51 / D-ServiceIdentities): the machine counterpart of the
// account/external-identity directory. Registering a principal does not create a credential — the
// external IdP owns the client secret; this only records which (issuer, subject) IS a machine
// subject, so the middleware can resolve its tokens (L-AuthzOnly holds).

const targetPrincipal = "service_principal"

// PrincipalRepositoryFactory binds a domain.PrincipalRepository to a command surface, mirroring
// RepositoryFactory so the application layer never imports adapters.
type PrincipalRepositoryFactory func(conn db.DBTX) domain.PrincipalRepository

// RegisterPrincipal records a machine subject. The (issuer, subject) must not already name a
// principal OR a person external identity — an inbound token resolves to one subject, never two, so
// the collision is rejected from both sides (DB triggers backstop this pre-check).
func (s *Service) RegisterPrincipal(ctx context.Context, p domain.ServicePrincipal) (domain.ServicePrincipal, error) {
	p.Code = strings.TrimSpace(p.Code)
	p.Issuer = strings.TrimSpace(p.Issuer)
	p.Subject = strings.TrimSpace(p.Subject)
	if err := p.Validate(); err != nil {
		return domain.ServicePrincipal{}, err
	}
	// Pre-check the cross-table collision so the caller gets a typed 409 rather than a raw
	// constraint error surfacing as a 500.
	if _, err := s.newRepo(s.pool).ResolveBySubject(ctx, p.Issuer, p.Subject); err == nil {
		return domain.ServicePrincipal{}, domain.ErrPrincipalConflict
	} else if !isNotFound(err, domain.ErrIdentityNotFound) {
		return domain.ServicePrincipal{}, err
	}

	var out domain.ServicePrincipal
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newPrincipalRepo(tx).InsertPrincipal(ctx, p)
		if err != nil {
			return err
		}
		if err := s.record(ctx, tx, "service-principal.register", targetPrincipal, created.ID, principalAudit(created)); err != nil {
			return err
		}
		out = created
		return nil
	})
	return out, err
}

// UpdatePrincipal changes the display fields only. Re-pointing (issuer, subject) would silently
// transfer a machine's authority to a different IdP client, so it is rejected: register a new
// principal and revoke the old one instead.
func (s *Service) UpdatePrincipal(ctx context.Context, p domain.ServicePrincipal) (domain.ServicePrincipal, error) {
	if strings.TrimSpace(p.Name) == "" {
		return domain.ServicePrincipal{}, domain.ErrPrincipalInvalid
	}
	current, err := s.newPrincipalRepo(s.pool).GetPrincipal(ctx, p.ID)
	if err != nil {
		return domain.ServicePrincipal{}, err
	}
	if (p.Issuer != "" && p.Issuer != current.Issuer) || (p.Subject != "" && p.Subject != current.Subject) {
		return domain.ServicePrincipal{}, domain.ErrPrincipalIdentityImmutable
	}

	var out domain.ServicePrincipal
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newPrincipalRepo(tx).UpdatePrincipal(ctx, p)
		if err != nil {
			return err
		}
		if err := s.record(ctx, tx, "service-principal.update", targetPrincipal, updated.ID, principalAudit(updated)); err != nil {
			return err
		}
		out = updated
		return nil
	})
	return out, err
}

// SetPrincipalStatus is the reversible kill switch. A disabled principal fails resolution, so its
// tokens stop working immediately while the audit rows naming it stay intact.
func (s *Service) SetPrincipalStatus(ctx context.Context, id string, status domain.PrincipalStatus) (domain.ServicePrincipal, error) {
	if status != domain.PrincipalActive && status != domain.PrincipalDisabled {
		return domain.ServicePrincipal{}, domain.ErrPrincipalInvalid
	}
	action := "service-principal.disable"
	if status == domain.PrincipalActive {
		action = "service-principal.enable"
	}
	var out domain.ServicePrincipal
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newPrincipalRepo(tx).SetPrincipalStatus(ctx, id, status)
		if err != nil {
			return err
		}
		if err := s.record(ctx, tx, action, targetPrincipal, updated.ID, principalAudit(updated)); err != nil {
			return err
		}
		out = updated
		return nil
	})
	return out, err
}

func (s *Service) GetPrincipal(ctx context.Context, id string) (domain.ServicePrincipal, error) {
	return s.newPrincipalRepo(s.pool).GetPrincipal(ctx, id)
}

func (s *Service) GetPrincipalByCode(ctx context.Context, code string) (domain.ServicePrincipal, error) {
	return s.newPrincipalRepo(s.pool).GetPrincipalByCode(ctx, code)
}

func (s *Service) ListPrincipals(ctx context.Context, afterID string, limit int) ([]domain.ServicePrincipal, error) {
	return s.newPrincipalRepo(s.pool).ListPrincipals(ctx, afterID, limit)
}

// ResolvePrincipal is the middleware's machine-token lookup — the service counterpart of Resolve.
// Unregistered, disabled, and soft-deleted all yield ErrPrincipalNotFound, so the middleware answers
// with the same uniform Unauthorized either way.
func (s *Service) ResolvePrincipal(ctx context.Context, issuer, subject string) (domain.PrincipalResolution, error) {
	return s.newPrincipalRepo(s.pool).ResolvePrincipalBySubject(ctx, issuer, subject)
}

// PrincipalIsActive satisfies the authorization module's PrincipalDirectory port, so a grant write
// validates its principal without reading this module's tables directly.
func (s *Service) PrincipalIsActive(ctx context.Context, principalID string) (bool, error) {
	return s.newPrincipalRepo(s.pool).PrincipalIsActive(ctx, principalID)
}

// EnsurePrincipal is the boot-seed path (create-if-absent) used for the shared-secret
// `hermenea-importer` fallback, so a shared-secret caller resolves to a REAL principal with real
// grants and real audit attribution — one downstream path, not a special case (D-ServiceIdentities).
// It returns the existing principal untouched when the code is already registered.
func (s *Service) EnsurePrincipal(ctx context.Context, p domain.ServicePrincipal) (domain.ServicePrincipal, error) {
	existing, err := s.GetPrincipalByCode(ctx, p.Code)
	if err == nil {
		return existing, nil
	}
	if !isNotFound(err, domain.ErrPrincipalNotFound) {
		return domain.ServicePrincipal{}, err
	}
	created, err := s.RegisterPrincipal(ctx, p)
	if err != nil {
		// A concurrent booting replica may have won the race despite the advisory lock; treat the
		// conflict as success and re-read.
		if isNotFound(err, domain.ErrPrincipalConflict) {
			return s.GetPrincipalByCode(ctx, p.Code)
		}
		return domain.ServicePrincipal{}, err
	}
	return created, nil
}

// principalAudit is the audit payload: identifiers and the issuer only. The subject is not secret
// (it names a machine) but it is the resolution key, so it stays out of the ledger for the same
// reason the person-side identity audit omits it.
func principalAudit(p domain.ServicePrincipal) map[string]any {
	return map[string]any{
		"id":     p.ID,
		"code":   p.Code,
		"issuer": p.Issuer,
		"status": string(p.Status),
	}
}

func isNotFound(err, target error) bool { return err != nil && errors.Is(err, target) }
