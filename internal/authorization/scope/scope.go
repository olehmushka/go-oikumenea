// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package scope is the D-VisibilityScope adapter (review-2026-09 R-30): ONE interface answering
// "which of these objects may the subject read", with exactly three canonical implementations
// matching the three row-visibility policies that actually exist in the system — person scope
// (the D-PersonReadScope membership semi-join), unit scope (owning-unit mapping + the shadow gate),
// and catalog scope (the endpoint's read-permission gate is the entire decision; no row trim).
//
// The adapter is ADDITIVE: per-module endpoints keep their own code paths; cross-type surfaces
// (unified search D-UnifiedSearch now, generic link traversal R-27 next) consume it, and its
// correctness contract is differential equality with the owning module's own list endpoint for the
// same subject and fixtures.
//
// The implementations take injected funcs so this package (and the authorization module) imports
// no other module — the composition root wires the concretes (the LocationLookup/WatchlistLookup
// late-bound seam pattern).
package scope

import (
	"context"
)

// Visibility trims a candidate id set to the subset the subject may read, preserving input order.
// isAdmin is the instance-admin flag from pep.SubjectAuthority — every scope short-circuits on it
// (instance admins read everything; matches the per-module endpoints' own admin fast paths).
type Visibility interface {
	ReadableIDs(ctx context.Context, subject string, isAdmin bool, candidateIDs []string) ([]string, error)
}

// ---------------------------------------------------------------- catalog scope

type catalogScope struct{}

// NewCatalogScope is the reference-catalog policy: the type's read permission (enforced by the
// consuming surface, e.g. the search engine's per-provider AllowedAnywhere gate) is the entire
// decision — every row is readable once the gate passes. Identity trim.
func NewCatalogScope() Visibility { return catalogScope{} }

func (catalogScope) ReadableIDs(_ context.Context, _ string, _ bool, candidateIDs []string) ([]string, error) {
	return candidateIDs, nil
}

// ---------------------------------------------------------------- unit scope

type unitScope struct {
	mapUnits func(ctx context.Context, ids []string) (unitOf map[string]string, shadow map[string]bool, err error)
	filter   func(ctx context.Context, subject string, candidates []string, shadow map[string]bool) ([]string, error)
}

// NewUnitScope is the unit-governed policy: map each candidate to its governing unit, then pass
// the units through the shadow-visibility gate (authorization's FilterVisibleUnits — public units
// pass, shadow units only within the subject's readable reach). mapUnits is module-provided
// (identity for tenant units themselves); filter is the authz application's FilterVisibleUnits,
// subject-explicit form.
func NewUnitScope(
	mapUnits func(ctx context.Context, ids []string) (map[string]string, map[string]bool, error),
	filter func(ctx context.Context, subject string, candidates []string, shadow map[string]bool) ([]string, error),
) Visibility {
	return unitScope{mapUnits: mapUnits, filter: filter}
}

func (s unitScope) ReadableIDs(ctx context.Context, subject string, isAdmin bool, candidateIDs []string) ([]string, error) {
	if isAdmin || len(candidateIDs) == 0 {
		return candidateIDs, nil
	}
	unitOf, shadow, err := s.mapUnits(ctx, candidateIDs)
	if err != nil {
		return nil, err
	}
	// Dedupe the governing units, preserving first-seen order; a candidate without a governing
	// unit is dropped (fail closed — an unmapped row must not leak past the gate).
	units := make([]string, 0, len(candidateIDs))
	seen := make(map[string]struct{}, len(candidateIDs))
	for _, id := range candidateIDs {
		u, ok := unitOf[id]
		if !ok {
			continue
		}
		if _, dup := seen[u]; !dup {
			seen[u] = struct{}{}
			units = append(units, u)
		}
	}
	visible, err := s.filter(ctx, subject, units, shadow)
	if err != nil {
		return nil, err
	}
	visibleSet := make(map[string]struct{}, len(visible))
	for _, u := range visible {
		visibleSet[u] = struct{}{}
	}
	out := make([]string, 0, len(candidateIDs))
	for _, id := range candidateIDs {
		if u, ok := unitOf[id]; ok {
			if _, vis := visibleSet[u]; vis {
				out = append(out, id)
			}
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- organization scope

type orgScope struct {
	shadowOf func(ctx context.Context, ids []string) (map[string]bool, error)
	filter   func(ctx context.Context, subject string, candidates []string, shadow map[string]bool) ([]string, error)
}

// NewOrgScope is the organization-governed policy (M58 ticket 5): the candidate RIDs ARE tenant
// organization RIDs, so unlike unitScope there is nothing to map them THROUGH — what is
// module-provided is only each candidate's public/shadow bit. The filter is the authz application's
// FilterVisibleOrgs, subject-explicit form, which applies the DERIVED reach rule (an organization is
// visible when any of its live units is in the subject's reach — D-VisibilityScope, amended M58
// ticket 4).
//
// It exists because the sidecar PROFILE types — company and institution (M41 / D-UnifiedOrgGraph) —
// reach unified search as organization rows and had been registered under the catalog (identity)
// scope, which trimmed nothing: search was the third door on the same leak this ticket closed at the
// list and the point read. A row whose visibility bit is unknown is DROPPED, failing closed.
func NewOrgScope(
	shadowOf func(ctx context.Context, ids []string) (map[string]bool, error),
	filter func(ctx context.Context, subject string, candidates []string, shadow map[string]bool) ([]string, error),
) Visibility {
	return orgScope{shadowOf: shadowOf, filter: filter}
}

func (s orgScope) ReadableIDs(ctx context.Context, subject string, isAdmin bool, candidateIDs []string) ([]string, error) {
	if isAdmin || len(candidateIDs) == 0 {
		return candidateIDs, nil
	}
	shadow, err := s.shadowOf(ctx, candidateIDs)
	if err != nil {
		return nil, err
	}
	known := make([]string, 0, len(candidateIDs))
	for _, id := range candidateIDs {
		if _, ok := shadow[id]; ok {
			known = append(known, id)
		}
	}
	visible, err := s.filter(ctx, subject, known, shadow)
	if err != nil {
		return nil, err
	}
	visibleSet := make(map[string]struct{}, len(visible))
	for _, id := range visible {
		visibleSet[id] = struct{}{}
	}
	out := make([]string, 0, len(candidateIDs))
	for _, id := range candidateIDs {
		if _, ok := visibleSet[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- person scope

type personScope struct {
	probe func(ctx context.Context, subject string, personIDs []string) ([]string, error)
}

// NewPersonScope is the D-PersonReadScope policy: a person is readable when any of their active
// memberships falls in the subject's readable reach. probe is the membership batch semi-join
// (SubjectReadablePersonsAmong) — unordered set semantics; this adapter restores candidate order.
func NewPersonScope(probe func(ctx context.Context, subject string, personIDs []string) ([]string, error)) Visibility {
	return personScope{probe: probe}
}

func (s personScope) ReadableIDs(ctx context.Context, subject string, isAdmin bool, candidateIDs []string) ([]string, error) {
	if isAdmin || len(candidateIDs) == 0 {
		return candidateIDs, nil
	}
	readable, err := s.probe(ctx, subject, candidateIDs)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(readable))
	for _, id := range readable {
		set[id] = struct{}{}
	}
	out := make([]string, 0, len(readable))
	for _, id := range candidateIDs {
		if _, ok := set[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}
