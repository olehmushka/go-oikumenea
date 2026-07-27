// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Domains (org-kind catalog), unit kinds (domain-scoped catalog), and organizations (the realm) —
// the two-tier model above the unit graph (D-TenantOrganizations, M40). Organizations seed their own
// command + operational graphs at creation, in the same transaction. All writes are audited and the
// catalogs/realm are directory attributes — never PDP inputs.
package application

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
)

// noExclude is the zero UUID used as the "exclude nothing" argument to the code-conflict counts on a
// create (a real RID is passed on update). It is a valid uuid that never matches a minted RID, so the
// count query's `id <> @exclude_id` cast succeeds (an empty string would fail the uuid cast).
const noExclude = "00000000-0000-0000-0000-000000000000"

// ---------------------------------------------------------------- domains

// ListDomains returns the org-kind catalog in display order.
func (s *Service) ListDomains(ctx context.Context) ([]domain.Domain, error) {
	return s.newRepo(s.querier(ctx)).ListDomains(ctx)
}

// GetDomain reads one domain by RID.
func (s *Service) GetDomain(ctx context.Context, id string) (domain.Domain, error) {
	return s.newRepo(s.querier(ctx)).GetDomain(ctx, id)
}

// CreateDomain validates and adds a domain to the catalog, recording the action.
func (s *Service) CreateDomain(ctx context.Context, code, name string, sortOrder *int) (domain.Domain, error) {
	d := domain.Domain{Code: code, Name: name}
	if err := d.Validate(); err != nil {
		return domain.Domain{}, err
	}
	var out domain.Domain
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		n, err := repo.CountActiveDomainsByCode(ctx, code, noExclude)
		if err != nil {
			return err
		}
		if n > 0 {
			return domain.ErrDomainCodeConflict
		}
		created, err := repo.InsertDomain(ctx, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "domain.create", "domain", created.ID, "", created)
	})
	return out, err
}

// UpdateDomain applies a partial change (name/status/sortOrder) and records the action.
func (s *Service) UpdateDomain(ctx context.Context, id string, patch domain.DomainPatch) (domain.Domain, error) {
	var out domain.Domain
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateDomain(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "domain.update", "domain", id, "", updated)
	})
	return out, err
}

// ---------------------------------------------------------------- unit kinds

// ListUnitKinds returns a domain's unit-kind catalog in display order (the domain must exist).
func (s *Service) ListUnitKinds(ctx context.Context, domainID string) ([]domain.UnitKind, error) {
	repo := s.newRepo(s.querier(ctx))
	if _, err := repo.GetDomain(ctx, domainID); err != nil {
		return nil, err
	}
	return repo.ListUnitKinds(ctx, domainID)
}

// CreateUnitKind validates and adds a domain-scoped unit kind, recording the action.
func (s *Service) CreateUnitKind(ctx context.Context, k domain.UnitKind) (domain.UnitKind, error) {
	if err := k.Validate(); err != nil {
		return domain.UnitKind{}, err
	}
	var out domain.UnitKind
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetDomain(ctx, k.DomainID); err != nil {
			return err
		}
		n, err := repo.CountActiveUnitKindsByCode(ctx, k.DomainID, k.Code, noExclude)
		if err != nil {
			return err
		}
		if n > 0 {
			return domain.ErrUnitKindCodeConflict
		}
		created, err := repo.InsertUnitKind(ctx, k)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "unit-kind.create", "unit_kind", created.ID, "", created)
	})
	return out, err
}

// UpdateUnitKind applies a partial change and records the action.
func (s *Service) UpdateUnitKind(ctx context.Context, id string, patch domain.UnitKindPatch) (domain.UnitKind, error) {
	var out domain.UnitKind
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateUnitKind(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "unit-kind.update", "unit_kind", id, "", updated)
	})
	return out, err
}

// ---------------------------------------------------------------- organizations

// OrgPage is a page of organizations plus the opaque next-page token.
type OrgPage struct {
	Orgs          []domain.Organization
	NextPageToken string
}

// ListOrganizations returns a keyset-paginated page of organizations, optionally filtered by domain.
func (s *Service) ListOrganizations(ctx context.Context, domainID *string, pageSize int, pageToken string) (OrgPage, error) {
	size := pageSizePolicy.Resolve(pageSize)
	after, err := listing.DecodeCursor(pageToken)
	if err != nil {
		return OrgPage{}, err
	}
	orgs, err := s.newRepo(s.querier(ctx)).ListOrganizations(ctx, domainID, after, size+1)
	if err != nil {
		return OrgPage{}, err
	}
	if len(orgs) > size {
		last := orgs[size-1]
		return OrgPage{Orgs: orgs[:size], NextPageToken: listing.EncodeCursor(last.ID)}, nil
	}
	return OrgPage{Orgs: orgs}, nil
}

// GetOrganization reads one organization by RID.
func (s *Service) GetOrganization(ctx context.Context, id string) (domain.Organization, error) {
	return s.newRepo(s.querier(ctx)).GetOrganization(ctx, id)
}

// CreateOrganization validates and creates an organization, then seeds its command (default, locked
// authority-bearing) + operational graphs in the SAME transaction (D-TenantOrganizations, M40), and
// records the action.
func (s *Service) CreateOrganization(ctx context.Context, o domain.Organization) (domain.Organization, error) {
	if o.Visibility == "" {
		o.Visibility = domain.VisibilityPublic
	}
	if o.State == "" {
		o.State = domain.StateActive
	}
	if err := o.Validate(); err != nil {
		return domain.Organization{}, err
	}
	var out domain.Organization
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		dom, err := repo.GetDomain(ctx, o.DomainID)
		if err != nil {
			return err
		}
		n, err := repo.CountActiveOrgsByCode(ctx, o.Code, noExclude)
		if err != nil {
			return err
		}
		if n > 0 {
			return domain.ErrOrgCodeConflict
		}
		created, err := repo.InsertOrganization(ctx, o)
		if err != nil {
			return err
		}
		// Seed this org's graphs ONLY for operational (pdp_scoped) domains. Reference domains
		// (university/company — D-UnifiedOrgGraph, M41) skip the auto-seed to avoid command/operational
		// rows for tens of thousands of bulk-imported orgs; their structure (if any) lazily creates a
		// graph via EnsureGraph.
		if dom.PdpScoped {
			if _, err := repo.InsertGraph(ctx, &created.ID, domain.CommandGraphCode, "Command", true, true); err != nil {
				return err
			}
			if _, err := repo.InsertGraph(ctx, &created.ID, domain.OperationalGraphCode, "Operational", false, true); err != nil {
				return err
			}
		}
		out = created
		return s.record(ctx, tx, "organization.create", "organization", created.ID, "", created)
	})
	return out, err
}

// EnsureGraph returns the organization's graph for `code`, creating it (idempotently) if absent. Used by
// verticals (e.g. education) that build a unit structure under a REFERENCE org whose graphs were not
// auto-seeded (D-UnifiedOrgGraph, M41). The first graph created for an org becomes its default.
func (s *Service) EnsureGraph(ctx context.Context, orgID, code, name string, authorityBearing bool) (domain.Graph, error) {
	var out domain.Graph
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetOrganization(ctx, orgID); err != nil {
			return err
		}
		existing, err := repo.GetGraphForOrgByCode(ctx, &orgID, code)
		switch {
		case err == nil:
			if existing.OrgID != nil && *existing.OrgID == orgID { // an org-owned graph already exists
				out = existing
				return nil
			}
			// only a global graph of this code exists; fall through to create the per-org one.
		case errors.Is(err, domain.ErrGraphNotFound):
			// none — create it.
		default:
			return err
		}
		isDefault, err := repo.CountActiveGraphsForOrg(ctx, &orgID)
		if err != nil {
			return err
		}
		created, err := repo.InsertGraph(ctx, &orgID, code, name, isDefault == 0, authorityBearing)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "graph.create", "graph", created.ID, "", created)
	})
	return out, err
}

// UpdateOrganization applies a partial change (name/domain/visibility/metadata) and records the action.
func (s *Service) UpdateOrganization(ctx context.Context, id string, patch domain.OrgPatch) (domain.Organization, error) {
	if patch.Name != nil && *patch.Name == "" {
		return domain.Organization{}, domain.ErrInvalidUnit
	}
	if patch.Visibility != nil && *patch.Visibility != domain.VisibilityPublic && *patch.Visibility != domain.VisibilityShadow {
		return domain.Organization{}, domain.ErrInvalidUnit
	}
	var out domain.Organization
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if patch.DomainID != nil {
			if _, err := repo.GetDomain(ctx, *patch.DomainID); err != nil {
				return err
			}
		}
		updated, err := repo.UpdateOrganization(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "organization.update", "organization", id, "", updated)
	})
	return out, err
}

// TransitionOrganization moves an organization to a new lifecycle state, appends the append-only
// lifecycle event, and records the action — all in one transaction (mirrors TransitionUnit).
func (s *Service) TransitionOrganization(ctx context.Context, id string, to domain.State, reason string) (domain.Organization, error) {
	if !domain.ValidState(to) {
		return domain.Organization{}, domain.ErrInvalidTransition
	}
	var out domain.Organization
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		current, err := repo.GetOrganization(ctx, id)
		if err != nil {
			return err
		}
		if !current.State.CanTransitionTo(to) {
			return domain.ErrInvalidTransition
		}
		updated, err := repo.SetOrgState(ctx, id, to)
		if err != nil {
			return err
		}
		if err := repo.InsertOrgLifecycleEvent(ctx, id, current.State, to, reason, "", requestID(ctx)); err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "organization.transition", "organization", id, "", map[string]string{
			"from": string(current.State), "to": string(to), "reason": reason,
		})
	})
	return out, err
}
