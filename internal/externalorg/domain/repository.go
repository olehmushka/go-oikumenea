// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import "context"

// Repository is the external-organizations persistence port (implemented by adapters over raw pgx). It
// is bound to a single command surface — the pool for reads, or a caller's transaction for an audited
// write (D-Audit). The application layer owns transaction boundaries; the repository never opens its own.
type Repository interface {
	// kind catalog
	ListKinds(ctx context.Context) ([]Kind, error)
	GetKind(ctx context.Context, id string) (Kind, error)
	UpsertKind(ctx context.Context, code, name string, sortOrder *int) (Kind, error)

	// organizations
	InsertOrg(ctx context.Context, in OrgInput) (Organization, error)
	GetOrg(ctx context.Context, id string) (Organization, error)
	UpdateOrg(ctx context.Context, id string, up OrgUpdate) (Organization, error)
	ListOrgs(ctx context.Context, query, kindCode, countryID, status, after string, lim int) ([]Organization, error)
	SoftDeleteOrg(ctx context.Context, id string) (int64, error)
	// MergeOrg tombstones a provisional stub (status flips to nothing — the row is soft-deleted) after
	// validation in the application layer. Returns the surviving canonical org.
	TombstoneOrg(ctx context.Context, id string) (int64, error)

	// label helpers (best-effort default-locale display names)
	KindNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)
	CountryNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)
}
