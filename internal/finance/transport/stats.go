// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	financeapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/finance"
	"github.com/olegamysk/go-oikumenea/internal/finance/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// The dashboard halves of the facet vocabulary for the TWO object types this module owns (M58
// ticket 3 / D-ObjectFacets), plus the instance-wide card registry list the card type needed before
// it could have a dashboard at all.
//
// Both stats endpoints take exactly the filter args their list takes (minus paging) plus the `facets`
// CSV, and both build their filter from the SAME helper the list uses — half of the no-drift contract
// (buildAccountFilter / buildCardFilter in the adapter are the other half).
//
// The gate throughout is the same coarse `finance.read` the list endpoints require. There is no
// scoped arm on either: neither table carries row-level security or a unit reach, so a caller who
// passes this gate may read every row the aggregates count.

// ============================ accounts ============================

func (s FinanceService) AccountStats(
	ctx context.Context,
	token bearertoken.Token,
	facets *string,
	institutionID *string,
	currency *string,
	accountTypeID *string,
	status *string,
	holderKind *string,
) (financeapi.AccountStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return financeapi.AccountStats{}, err
	}
	// The FILTER side of rule 2 applies to the dashboard too: `facets=holderKind` without the code is
	// silently omitted, but holderKind=person as a FILTER narrows the total a caller may not narrow it
	// by, so it is refused on both surfaces identically.
	if err := s.requireFilterCodes(ctx, token, "account", map[string]bool{"holderKind": holderKind != nil}); err != nil {
		return financeapi.AccountStats{}, err
	}
	sel, err := selectFinanceFacets(ctx, s, token, "account", strOr(facets))
	if err != nil {
		return financeapi.AccountStats{}, err
	}
	res, err := s.app.AccountStats(ctx, accountFilter(institutionID, currency, accountTypeID, status, holderKind), sel)
	if err != nil {
		return financeapi.AccountStats{}, s.mapError(ctx, err)
	}
	return financeapi.AccountStats{
		TotalCount: int(res.TotalCount),
		Facets:     toAPIFinanceDistributions(res),
	}, nil
}

// accountFilter is the ONE place a request's account facet args become the domain filter, shared by
// the list and the stats endpoint: the two must read the same arguments the same way, or the same URL
// means two different things depending on which surface renders it.
func accountFilter(institutionID, currency, accountTypeID, status, holderKind *string) domain.AccountFilter {
	return domain.AccountFilter{
		InstitutionID: institutionID,
		Currency:      currency,
		AccountTypeID: accountTypeID,
		Status:        status,
		HolderKind:    holderKind,
	}
}

// requireFilterCodes enforces the LIST side of D-ObjectFacets rule 2 (M59): a gated facet's filter arg
// may be USED only by a caller holding the facet's own read code. facet.FilterReadCodes decides WHICH
// codes a request needs — pure, catalog-driven, so a newly gated facet is covered the moment it is
// declared — and the existing PEP produces the module's own 403. `supplied` is keyed by facet Key.
func (s FinanceService) requireFilterCodes(ctx context.Context, token bearertoken.Token, objectType string, supplied map[string]bool) error {
	o, ok := facet.Default.Get(objectType)
	if !ok {
		return nil // unreachable past the boot-time MustBeBound; a missing catalog gates nothing new
	}
	for _, code := range o.FilterReadCodes(supplied) {
		if err := s.pep.RequireAnywhere(ctx, token, code); err != nil {
			return err
		}
	}
	return nil
}

// ============================ cards ============================

// ListCards is the instance-wide card registry (M58 ticket 3) — the collection-level list the card
// dashboard describes, added because cards were previously reachable only per-account.
//
// It is gated on the SAME readPerm as listAccounts and listAccountCards, and it returns the same
// metadata projection the per-account list already returned: this widens the SCOPE of a read the code
// already permits and discloses no new field. The PAN is never here; getCard decrypts one card at a
// time for an authorized caller (PCI-DSS Req 3; D-DataScope CDE scope).
func (s FinanceService) ListCards(
	ctx context.Context,
	token bearertoken.Token,
	networkID *string,
	cardType *string,
	status *string,
	pageSize *int,
	pageToken *string,
) (financeapi.CardPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return financeapi.CardPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListCards(ctx, decodeToken(pageToken), cardFilter(networkID, cardType, status), limit)
	if err != nil {
		return financeapi.CardPage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	out, err := s.cardsWithLabels(ctx, rows)
	if err != nil {
		return financeapi.CardPage{}, s.mapError(ctx, err)
	}
	page := financeapi.CardPage{Cards: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s FinanceService) CardStats(
	ctx context.Context,
	token bearertoken.Token,
	facets *string,
	networkID *string,
	cardType *string,
	status *string,
) (financeapi.CardStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return financeapi.CardStats{}, err
	}
	sel, err := selectFinanceFacets(ctx, s, token, "card", strOr(facets))
	if err != nil {
		return financeapi.CardStats{}, err
	}
	res, err := s.app.CardStats(ctx, cardFilter(networkID, cardType, status), sel)
	if err != nil {
		return financeapi.CardStats{}, s.mapError(ctx, err)
	}
	return financeapi.CardStats{
		TotalCount: int(res.TotalCount),
		Facets:     toAPIFinanceDistributions(res),
	}, nil
}

// cardFilter is accountFilter's equivalent for the card registry.
func cardFilter(networkID, cardType, status *string) domain.CardFilter {
	return domain.CardFilter{NetworkID: networkID, CardType: cardType, Status: status}
}

// ============================ shared ============================

// selectFinanceFacets resolves the `facets` CSV against the catalog. It takes the object TYPE because
// this module owns two, and each has its own vocabulary — asking for `currency` on the card dashboard
// must be the same 400 as asking for a facet that exists nowhere.
//
// A facet gated on a read code the caller lacks is dropped here — omitted from the response, never a
// zeroed bucket and never a 403 (D-ObjectFacets rule 2). An undeclared facet key IS a caller error:
// it is a typo, not a permission. (Every finance facet is pii:none — the pii:sensitive IBAN and PAN
// have no facet to gate — so the omission arm has no live case here.)
func selectFinanceFacets(ctx context.Context, s FinanceService, token bearertoken.Token, objectType, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get(objectType)
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, financeapi.NewInvalid(fmt.Sprintf("%s facets are not registered", objectType))
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, financeapi.NewInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIFinanceDistributions maps the assembled kernel result onto the wire type, carrying each bucket
// key VERBATIM: it is what the caller passes back as a filter value, synthetic `(unknown)`/`(other)`
// keys included (which the console renders unlinked). ONE mapper for both object types, because
// Conjure generates FacetDistribution once per FILE and both types live in finance.conjure.yml.
func toAPIFinanceDistributions(res stats.Result) []financeapi.FacetDistribution {
	out := make([]financeapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]financeapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := financeapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, financeapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
