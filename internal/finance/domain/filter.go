// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"

	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/rid"
)

// The finance facet vocabulary in Go (M58 ticket 3 / D-ObjectFacets), shared verbatim by each list
// path and its stats path.
//
// Neither table carries row-level security or a unit reach, so `finance.read` held anywhere is the
// whole visibility decision and both aggregates ship ONE arm — external_organization's reason, not
// the audit ledger's. These structs exist for the OTHER half of the no-drift contract: the list and
// the dashboard must select the same rows from the same arguments, or a chart segment and a filter
// stop being the same act.
//
// Neither struct has a field for an encrypted column, and neither can. The IBAN and the PAN are
// envelope-encrypted with no plaintext to compare or group, and D-DataScope's aggregation rule
// forbids the surface independently of that.

// AccountFilter is the account facet vocabulary.
type AccountFilter struct {
	// InstitutionID is the holding BANK — a `company`-domain tenant_organizations RID (M21/M41),
	// never a finance-owned entity.
	InstitutionID *string

	// Currency is an ISO 4217 code matched as an OPEN value set: the column carries no CHECK, so
	// there is nothing to validate against and the code is its own bucket label.
	Currency *string

	AccountTypeID *string
	Status        *string

	// HolderKind is `person` | `company` — the kind of the account's ACTIVE PRIMARY holder, matched
	// through the finance_account_holders link. Supplying it requires finance.holder.read
	// (facet.FilterReadCodes, enforced in the transport): who holds an account is a disclosure the
	// holder endpoints already gate separately, and a filter over it would otherwise recover the same
	// fact one value at a time.
	HolderKind *string
}

// CardFilter is the card facet vocabulary, for the instance-wide registry M58 added (cards were
// previously reachable only per-account, so there was no collection for a dashboard to describe).
type CardFilter struct {
	NetworkID *string
	// CardType is `debit` | `credit`. Named for the card rather than bare `type`, which beside
	// NetworkID would read as the card's network.
	CardType *string
	Status   *string
}

// Validate rejects a caller value the SQL would otherwise accept and silently return nothing for.
// Every enum check reads the SHIPPED facet catalog rather than a local copy, so a CHECK constraint
// and its filter cannot drift apart.
func (f AccountFilter) Validate() error {
	if err := validateFacetEnum("account", "status", f.Status); err != nil {
		return err
	}
	if err := validateFacetEnum("account", "holderKind", f.HolderKind); err != nil {
		return err
	}
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"institutionId", f.InstitutionID},
		{"accountTypeId", f.AccountTypeID},
	} {
		if r.val != nil && !rid.IsRID(*r.val) {
			return fmt.Errorf("%w: %s must be a RID", ErrInvalid, r.arg)
		}
	}
	// `currency` is deliberately NOT validated against a value set: the column carries no CHECK, so
	// there is nothing to validate against. An unknown code matches no rows, which is the honest
	// answer for an open set — the KindCode case (audit.action set the precedent).
	return nil
}

// Validate is the card registry's equivalent.
func (f CardFilter) Validate() error {
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"cardType", f.CardType},
		{"status", f.Status},
	} {
		if err := validateFacetEnum("card", r.arg, r.val); err != nil {
			return err
		}
	}
	if f.NetworkID != nil && !rid.IsRID(*f.NetworkID) {
		return fmt.Errorf("%w: networkId must be a RID", ErrInvalid)
	}
	return nil
}

// validateFacetEnum checks a value against the shipped facet's declared Values (the CHECK set in
// chart order). It takes the object type because this module owns TWO — accounts and cards — and a
// facet key alone would not say which vocabulary to read. An unknown key is a programming error in
// this file, not a caller error.
func validateFacetEnum(objectType, facetKey string, v *string) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get(objectType)
	if !ok {
		return fmt.Errorf("%s is not a registered facet type", objectType)
	}
	for _, ft := range o.Facets {
		if ft.Key != facetKey {
			continue
		}
		for _, allowed := range ft.Values {
			if *v == allowed {
				return nil
			}
		}
		return fmt.Errorf("%w: %s must be one of %v", ErrInvalid, facetKey, ft.Values)
	}
	return fmt.Errorf("%s is not a declared %s facet", facetKey, objectType)
}
