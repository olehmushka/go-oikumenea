// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/order/domain"
)

// orderFacetArgsT is the order facet block bound to pgx wire types — ONE mapping used by BOTH
// top-level list queries (the instance-admin ListOrders and the reach-scoped ListOrdersForSubject),
// so the two paths can never bind a facet differently (M56 ticket 3 / D-ObjectFacets).
//
// Every field is a nullable pgx type: an unset filter is a SQL NULL that the query's
// `sqlc.narg('x')::type IS NULL OR ...` guard short-circuits. It is never a sentinel value — the
// empty-string-sentinel shape is what R-21 bans, because the planner cannot prove a sentinel
// non-empty under a generic prepared plan and falls back to a scan.
type orderFacetArgsT struct {
	issuingUnitID pgtype.Text
	orderTypeID   pgtype.Text
	status        pgtype.Text
	issuedOnFrom  pgtype.Date
	issuedOnTo    pgtype.Date
}

// orderFacetArgs projects a validated OrderFilter onto its wire types.
func orderFacetArgs(f domain.OrderFilter) orderFacetArgsT {
	return orderFacetArgsT{
		issuingUnitID: facetText(f.IssuingUnitID),
		orderTypeID:   facetText(f.OrderTypeID),
		status:        facetText(f.Status),
		issuedOnFrom:  facetDate(f.IssuedOnFrom),
		issuedOnTo:    facetDate(f.IssuedOnTo),
	}
}

func facetText(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func facetDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}
