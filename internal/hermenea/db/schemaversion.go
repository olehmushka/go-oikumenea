package db

import (
	"context"
	"io/fs"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
	werror "github.com/palantir/witchcraft-go-error"

	hermeneamigrations "github.com/olegamysk/go-oikumenea/migrations/hermenea"
)

// ExpectedSchemaRevision is the hermenea schema revision this binary is built against — the revision
// the latest embedded hermenea migration records in hermenea.schema_version. Derived at init from the
// embedded migration files (architecture review R-25, mirroring oikumenea's internal/platform/db
// gate / R-15) rather than hand-bumped, so adding a hermenea migration needs no Go change.
var ExpectedSchemaRevision = mustDeriveExpectedRevision()

// revisionLiteral matches the two ways a migration records the schema_version marker: the bootstrap
// INSERT (VALUES (true, '0006_...')) and every subsequent migration's closing UPDATE
// (SET revision = '000N_...'). The captured group is the revision string stored in the DB.
var revisionLiteral = regexp.MustCompile(`(?:SET revision = |VALUES \(true, )'([^']+)'`)

func mustDeriveExpectedRevision() string {
	rev, err := deriveExpectedRevision(hermeneamigrations.FS)
	if err != nil {
		// The migrations are embedded at compile time; a failure here means a broken build.
		panic("hermenea db: derive expected schema revision: " + err.Error())
	}
	return rev
}

// deriveExpectedRevision returns the revision the DB will hold after all migrations apply: the last
// revision literal set by the highest-ordered migration that sets one. Migrations apply in filename
// (version) order; a migration that touches no schema_version row leaves the revision unchanged,
// which this scan-in-order-track-last mirrors exactly.
func deriveExpectedRevision(fsys fs.FS) (string, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", werror.Error("no embedded hermenea migration files")
	}
	sort.Strings(names) // zero-padded version prefixes → lexical order == apply order

	revision := ""
	for _, name := range names {
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return "", err
		}
		if m := revisionLiteral.FindAllStringSubmatch(string(body), -1); len(m) > 0 {
			revision = m[len(m)-1][1]
		}
	}
	if revision == "" {
		return "", werror.Error("no schema_version revision literal found in embedded hermenea migrations")
	}
	return revision, nil
}

// ReadSchemaRevision returns the single-row hermenea.schema_version marker's revision. A DB migrated
// below 0006 (marker table absent) surfaces as an error here — the boot gate treats that as stale.
func ReadSchemaRevision(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var revision string
	err := pool.QueryRow(ctx, "SELECT revision FROM hermenea.schema_version").Scan(&revision)
	if err != nil {
		return "", werror.WrapWithContextParams(ctx, err, "read hermenea schema_version")
	}
	return revision, nil
}
