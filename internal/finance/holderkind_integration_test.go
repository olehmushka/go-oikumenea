// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the account `holderKind` facet (M59 / D-ObjectFacets rule 2) — the first
// facet in the vocabulary that was BORN gated, and the first whose value comes from a one-to-many
// link confined to a single row by a partial unique index.
//
// The gate itself (a 403 for a caller without finance.holder.read) lives in the transport and is
// pinned there structurally, for the reason the education module's holder scope is. What can only be
// checked against a real database is the half below: that the confinement to the ACTIVE PRIMARY
// holder actually partitions, so a jointly held account is counted ONCE and under the right bucket.
// Get that wrong and the chart's segments sum above its own total while every unit test stays green.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/finance/... -run HolderKind
package finance_test

import (
	"context"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/finance/application"
	"github.com/olehmushka/go-oikumenea/internal/finance/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// TestHolderKindPartitionsAndFilters_Integration builds the world the facet's declaration claims to
// handle — a person-held account, a company-held account, and a JOINTLY held one — and holds the
// distribution and the filter to agreeing about it.
func TestHolderKindPartitionsAndFilters_Integration(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`TRUNCATE oikumenea.finance_account_holders, oikumenea.finance_cards, oikumenea.finance_accounts`); err != nil {
		t.Fatalf("reset finance tables: %v", err)
	}

	bank := seedBank(t, pool, "M59 Bank")
	person := seedPerson(t, pool)
	company := seedBank(t, pool, "M59 Holdings")
	current := catalogID(t, pool, "finance_account_types", "current")

	// Three accounts: one held by a person, one by a company, one held by a person AND a company —
	// the joint case that would double-count if the facet grouped the link raw.
	personAcct := mustAccount(t, svc, ctx, bank, "UA903052992990004149123456789", current)
	companyAcct := mustAccount(t, svc, ctx, bank, "UA213223130000026007233566001", current)
	jointAcct := mustAccount(t, svc, ctx, bank, "UA573543470006762462054925026", current)

	addHolder(t, svc, ctx, personAcct, domain.HolderPerson, person, domain.RolePrimary)
	addHolder(t, svc, ctx, companyAcct, domain.HolderCompany, company, domain.RolePrimary)
	// The joint account's PRIMARY holder is the person; the company is joint. The facet describes the
	// primary, so this account belongs in the `person` bucket and nowhere else.
	addHolder(t, svc, ctx, jointAcct, domain.HolderPerson, person, domain.RolePrimary)
	addHolder(t, svc, ctx, jointAcct, domain.HolderCompany, company, domain.RoleJoint)

	sel := accountSel(t)

	t.Run("the distribution partitions", func(t *testing.T) {
		res, err := svc.AccountStats(ctx, domain.AccountFilter{}, sel)
		if err != nil {
			t.Fatalf("AccountStats: %v", err)
		}
		buckets := accountBuckets(t, res, "holderKind")
		var sum int64
		for _, b := range buckets {
			sum += b.Count
		}
		if sum != res.TotalCount {
			t.Errorf("holderKind buckets sum to %d, totalCount=%d — the joint account is counted twice "+
				"(or not at all), so the chart would not add up to the number printed beside it",
				sum, res.TotalCount)
		}
		if got := bucketCount(buckets, "person"); got != 2 {
			t.Errorf("person bucket = %d, want 2 (the person-held account and the JOINT one, whose "+
				"primary holder is the person)", got)
		}
		if got := bucketCount(buckets, "company"); got != 1 {
			t.Errorf("company bucket = %d, want 1", got)
		}
	})

	t.Run("the filter agrees with the distribution", func(t *testing.T) {
		for _, kind := range []string{"person", "company"} {
			k := kind
			res, err := svc.AccountStats(ctx, domain.AccountFilter{HolderKind: &k}, sel)
			if err != nil {
				t.Fatalf("AccountStats(holderKind=%s): %v", k, err)
			}
			rows, err := svc.ListAccounts(ctx, "", domain.AccountFilter{HolderKind: &k}, 50)
			if err != nil {
				t.Fatalf("ListAccounts(holderKind=%s): %v", k, err)
			}
			if int64(len(rows)) != res.TotalCount {
				t.Errorf("holderKind=%s: list returned %d rows, stats totalCount=%d — the filter and "+
					"the aggregate do not see one world", k, len(rows), res.TotalCount)
			}
			// And it is the same number the unfiltered distribution attributed to that bucket.
			all, err := svc.AccountStats(ctx, domain.AccountFilter{}, sel)
			if err != nil {
				t.Fatalf("AccountStats: %v", err)
			}
			if want := bucketCount(accountBuckets(t, all, "holderKind"), k); want != res.TotalCount {
				t.Errorf("holderKind=%s: filtered total %d, but the unfiltered bucket says %d — clicking "+
					"that bar would land on a differently-sized list", k, res.TotalCount, want)
			}
		}
	})

	t.Run("an unheld account lands in (unknown) rather than vanishing", func(t *testing.T) {
		orphan := mustAccount(t, svc, ctx, bank, "UA423052992990004149123456780", current)
		_ = orphan // held by nobody: no holder rows at all
		res, err := svc.AccountStats(ctx, domain.AccountFilter{}, sel)
		if err != nil {
			t.Fatalf("AccountStats: %v", err)
		}
		buckets := accountBuckets(t, res, "holderKind")
		var sum int64
		for _, b := range buckets {
			sum += b.Count
		}
		if sum != res.TotalCount {
			t.Errorf("with an unheld account, buckets sum to %d and totalCount=%d — a row with no "+
				"primary holder was dropped instead of landing in (unknown)", sum, res.TotalCount)
		}
	})
}

// TestHolderKindIsOmittedWithoutTheCode is the stats side of rule 2, at the layer that decides it:
// stats.Select drops a gated facet for a caller who does not hold its code, so the response carries
// no such distribution at all — never a zeroed one, and never an error.
func TestHolderKindIsOmittedWithoutTheCode_Integration(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	o, ok := facet.Default.Get("account")
	if !ok {
		t.Fatal("account is not registered")
	}
	holds := func(held bool) func(string) (bool, error) {
		return func(code string) (bool, error) { return held, nil }
	}

	without, err := stats.Select(o, "", holds(false))
	if err != nil {
		t.Fatalf("Select(without): %v", err)
	}
	with, err := stats.Select(o, "", holds(true))
	if err != nil {
		t.Fatalf("Select(with): %v", err)
	}

	resWithout, err := svc.AccountStats(ctx, domain.AccountFilter{}, without)
	if err != nil {
		t.Fatalf("AccountStats(without the code): %v — rule 2 says omitted, never an error", err)
	}
	for _, d := range resWithout.Distributions {
		if d.Facet == "holderKind" {
			t.Errorf("holderKind is present for a caller without finance.holder.read (%d buckets) — "+
				"rule 2 requires it ABSENT, so the console can tell 'may not read' from 'nothing here'",
				len(d.Buckets))
		}
	}

	resWith, err := svc.AccountStats(ctx, domain.AccountFilter{}, with)
	if err != nil {
		t.Fatalf("AccountStats(with the code): %v", err)
	}
	var found bool
	for _, d := range resWith.Distributions {
		if d.Facet == "holderKind" {
			found = true
		}
	}
	if !found {
		t.Error("holderKind is absent even WITH the code — the omission is unconditional, so the " +
			"assertion above passes for the wrong reason")
	}
	// The totals must be identical: omitting a facet narrows what is DESCRIBED, never what is counted.
	if resWith.TotalCount != resWithout.TotalCount {
		t.Errorf("totalCount differs with (%d) and without (%d) the code — an omitted facet changed the "+
			"candidate set, which would leak the gated attribute through the total",
			resWith.TotalCount, resWithout.TotalCount)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

// mustAccount creates an account or fails. Each caller passes its own checksum-valid IBAN: the blind
// index is unique among ACTIVE rows, so two accounts cannot share one.
func mustAccount(t *testing.T, svc *application.Service, ctx context.Context, bank, iban, accountType string) string {
	t.Helper()
	a, err := svc.CreateAccount(ctx, bank, iban, "UAH", accountType)
	if err != nil {
		t.Fatalf("create account %s: %v", iban, err)
	}
	return a.ID
}

func addHolder(t *testing.T, svc *application.Service, ctx context.Context, accountID, kind, holderID, role string) {
	t.Helper()
	if _, err := svc.AddAccountHolder(ctx, accountID,
		domain.HolderInput{HolderKind: kind, HolderID: holderID, Role: role}); err != nil {
		t.Fatalf("add %s holder (%s): %v", kind, role, err)
	}
}

func accountSel(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("account")
	if !ok {
		t.Fatal("account is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func accountBuckets(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

func bucketCount(bs []stats.Bucket, key string) int64 {
	for _, b := range bs {
		if b.Key == key {
			return b.Count
		}
	}
	return 0
}
