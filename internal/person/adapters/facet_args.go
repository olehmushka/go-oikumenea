// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
)

// facetArgs is the person facet block bound to pgx wire types — ONE mapping, used by both admin list
// queries (ListPersons and SearchPersons), so the plain list and the trigram search can never bind a
// facet differently (M56 / D-ObjectFacets).
//
// Every field is a nullable pgx type: an unset filter is a SQL NULL that the query's
// `sqlc.narg('x')::type IS NULL OR ...` guard short-circuits. It is never a sentinel value — the
// empty-string-sentinel shape is what D-PersonSearch's R-21 generalization bans, because the
// planner cannot prove a sentinel non-empty under a generic prepared plan and falls back to a scan.
//
// The membership module binds the SAME block into its three visibility queries (a scoped caller
// never reaches these two). The two mappings are kept honest by the SQL narg-parity test, which
// proves every facet's narg name appears in all five queries.
type facetArgs struct {
	sex            pgtype.Text
	status         pgtype.Text
	birthdateFrom  pgtype.Date
	birthdateTo    pgtype.Date
	countryOfBirth pgtype.Text
	rankID         pgtype.Text
	hasAccount     pgtype.Bool
	unitID         pgtype.Text
	graph          pgtype.Text
}

// personFacetArgs projects a validated PersonFilter onto its wire types. Query is deliberately not
// here: it selects WHICH query runs (the R-21 List/Search split), it is not part of the facet block.
func personFacetArgs(f domain.PersonFilter) facetArgs {
	return facetArgs{
		sex:            textPtr(f.Sex),
		status:         textPtr(f.Status),
		birthdateFrom:  dateValue(f.BirthdateFrom),
		birthdateTo:    dateValue(f.BirthdateTo),
		countryOfBirth: textPtr(f.CountryOfBirth),
		rankID:         textPtr(f.RankID),
		hasAccount:     boolValue(f.HasAccount),
		unitID:         textPtr(f.UnitID),
		graph:          textEmptyNull(f.Graph),
	}
}

// dateValue maps an optional calendar day to a nullable date column (nil => NULL).
func dateValue(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// boolValue maps an optional bool to a nullable boolean column (nil => NULL). Note that FALSE is a
// real filter value here (hasAccount=false selects the account-less half of the directory), which is
// exactly why the parameter must be nullable rather than defaulted.
func boolValue(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

// textEmptyNull maps "unset" (the empty string) to NULL for a value carried as a plain string rather
// than a pointer — the graph code, which is only meaningful alongside unitId.
func textEmptyNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
