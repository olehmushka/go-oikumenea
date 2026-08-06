// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"io/fs"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/migrations"
	werror "github.com/palantir/witchcraft-go-error"
)

// ExpectedSchemaRevision is the schema revision this binary is built against — the revision the
// latest embedded migration records in oikumenea.schema_version. It is derived at init from the
// embedded migration files (architecture review R-15) rather than hand-bumped, so adding a
// migration needs no Go change: the boot-time check (upgrade-safety.md) stays correct automatically.
var ExpectedSchemaRevision = mustDeriveExpectedRevision()

// revisionLiteral matches the two ways a migration records the schema_version marker: the bootstrap
// INSERT (VALUES (true, '0000_...')) and every subsequent migration's closing UPDATE
// (SET revision = '0035_...'). The captured group is the revision string stored in the DB.
var revisionLiteral = regexp.MustCompile(`(?:SET revision = |VALUES \(true, )'([^']+)'`)

func mustDeriveExpectedRevision() string {
	rev, err := deriveExpectedRevision(migrations.FS)
	if err != nil {
		// The migrations are embedded at compile time; a failure here means a broken build.
		panic("db: derive expected schema revision: " + err.Error())
	}
	return rev
}

// deriveExpectedRevision returns the revision the DB will hold after all migrations apply: the last
// revision literal set by the highest-ordered migration that sets one. Migrations apply in filename
// (version) order, and some migrations (e.g. external_orgs) legitimately touch no schema_version
// row — those leave the revision unchanged, which this scan-in-order-track-last mirrors exactly.
func deriveExpectedRevision(fsys fs.FS) (string, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", werror.Error("no embedded migration files")
	}
	sort.Strings(names) // zero-padded version prefixes → lexical order == apply order

	revision := ""
	for _, name := range names {
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return "", err
		}
		// A migration sets the marker at most once, near its end; take the last match to be safe.
		if m := revisionLiteral.FindAllStringSubmatch(string(body), -1); len(m) > 0 {
			revision = m[len(m)-1][1]
		}
	}
	if revision == "" {
		return "", werror.Error("no schema_version revision literal found in embedded migrations")
	}
	return revision, nil
}

// ReadSchemaRevision returns the single-row schema_version marker's revision.
func ReadSchemaRevision(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var revision string
	err := pool.QueryRow(ctx, "SELECT revision FROM oikumenea.schema_version").Scan(&revision)
	if err != nil {
		return "", werror.WrapWithContextParams(ctx, err, "read schema_version")
	}
	return revision, nil
}
