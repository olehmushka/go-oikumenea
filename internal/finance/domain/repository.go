// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"context"

	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// Repository is the finance persistence port (the domain owns it; adapters implement it). Bound to one
// command surface (pool for reads, tx for writes). All methods are schema-qualified over oikumenea.finance_*.
type Repository interface {
	// catalogs
	ListAccountTypes(ctx context.Context) ([]AccountType, error)
	UpsertAccountType(ctx context.Context, code, name string, sortOrder *int) (AccountType, error)
	ListCardNetworks(ctx context.Context) ([]CardNetwork, error)
	UpsertCardNetwork(ctx context.Context, code, name string, sortOrder *int) (CardNetwork, error)

	// accounts (StoredAccount carries the envelope columns)
	InsertAccount(ctx context.Context, in AccountInput) (StoredAccount, error)
	GetAccount(ctx context.Context, id string) (StoredAccount, error)
	// ListAccounts pages the same set AccountStats aggregates, under the same AccountFilter — one
	// shared predicate, so `totalCount` describes exactly what paging returns (M58 / D-ObjectFacets).
	ListAccounts(ctx context.Context, after string, f AccountFilter, lim int) ([]StoredAccount, error)
	// AccountStats is the dashboard half. ONE arm — finance_accounts has no row-level security and no
	// unit reach, so there is no visibility predicate for a second arm to narrow.
	AccountStats(ctx context.Context, f AccountFilter, sel stats.Selection) ([]stats.Group, error)
	UpdateAccount(ctx context.Context, id string, up AccountUpdate) (StoredAccount, error)
	SoftDeleteAccount(ctx context.Context, id string) (int64, error)

	// holders (the ownership edge)
	InsertHolder(ctx context.Context, accountID string, in HolderInput) (AccountHolder, error)
	GetHolder(ctx context.Context, id string) (AccountHolder, error)
	ListHoldersByAccount(ctx context.Context, accountID string) ([]AccountHolder, error)
	EndHolder(ctx context.Context, id string) (AccountHolder, error)

	// cards (StoredCard carries the PAN envelope)
	InsertCard(ctx context.Context, accountID string, in CardInput) (StoredCard, error)
	GetCard(ctx context.Context, id string) (StoredCard, error)
	ListCardsByAccount(ctx context.Context, accountID string) ([]StoredCard, error)
	// ListCards pages the INSTANCE-WIDE card registry — the collection-level list M58 ticket 3 added
	// so the card dashboard has a set to describe. Metadata only: the PAN is decrypted by GetCard
	// alone, one card at a time (PCI-DSS Req 3; D-DataScope CDE scope).
	ListCards(ctx context.Context, after string, f CardFilter, lim int) ([]StoredCard, error)
	// CardStats is that registry's dashboard half. ONE arm, same reason as AccountStats.
	CardStats(ctx context.Context, f CardFilter, sel stats.Selection) ([]stats.Group, error)
	UpdateCard(ctx context.Context, id string, up CardUpdate) (StoredCard, error)
	SoftDeleteCard(ctx context.Context, id string) (int64, error)

	// person view
	ListAccountsByPersonHolder(ctx context.Context, personID string) ([]PersonAccount, error)

	// person purge (D-Finance): crypto-erase the accounts (+ cards) a person SOLELY holds, and soft-
	// delete their holder edges. Returns the count of crypto-erased accounts.
	ErasePersonHoldings(ctx context.Context, personID string) (accounts int64, err error)

	// cross-reference label helpers
	OrgNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)
	AccountTypeNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)
	NetworkNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)
	// OrgExists reports whether an id is a live tenant_organizations row (a valid bank / company holder).
	OrgExists(ctx context.Context, id string) (bool, error)
	// PersonExists reports whether an id is a live person (a valid cardholder / person holder).
	PersonExists(ctx context.Context, id string) (bool, error)
}
