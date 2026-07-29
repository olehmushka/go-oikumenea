// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/document/domain"
)

// documentFacetArgsT is the document facet block bound to pgx wire types — ONE mapping used by BOTH
// top-level list queries (the instance-admin ListDocuments and the holder-scoped
// ListDocumentsForSubject), so the two paths can never bind a facet differently (M56 ticket 3 /
// D-ObjectFacets).
//
// Every field is a nullable pgx type: an unset filter is a SQL NULL that the query's
// `sqlc.narg('x')::type IS NULL OR ...` guard short-circuits. It is never a sentinel value — the
// empty-string-sentinel shape is what R-21 bans, because the planner cannot prove a sentinel
// non-empty under a generic prepared plan and falls back to a scan.
type documentFacetArgsT struct {
	typeID           pgtype.Text
	status           pgtype.Text
	issuingCountryID pgtype.Text
	issuedOnFrom     pgtype.Date
	issuedOnTo       pgtype.Date
	expiresOnFrom    pgtype.Date
	expiresOnTo      pgtype.Date
}

// documentFacetArgs projects a validated DocumentFilter onto its wire types.
func documentFacetArgs(f domain.DocumentFilter) documentFacetArgsT {
	return documentFacetArgsT{
		typeID:           facetText(f.TypeID),
		status:           facetText(f.Status),
		issuingCountryID: facetText(f.IssuingCountryID),
		issuedOnFrom:     facetDate(f.IssuedOnFrom),
		issuedOnTo:       facetDate(f.IssuedOnTo),
		expiresOnFrom:    facetDate(f.ExpiresOnFrom),
		expiresOnTo:      facetDate(f.ExpiresOnTo),
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
