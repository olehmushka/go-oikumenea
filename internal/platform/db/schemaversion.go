package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	werror "github.com/palantir/witchcraft-go-error"
)

// ExpectedSchemaRevision is the schema revision this binary is built against. It must match the
// revision recorded in oikumenea.schema_version by the latest applied migration. Bump this in the
// same change that adds a migration (upgrade-safety.md, boot-time schema-version check).
const ExpectedSchemaRevision = "0015_person_relationships"

// ReadSchemaRevision returns the single-row schema_version marker's revision.
func ReadSchemaRevision(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var revision string
	err := pool.QueryRow(ctx, "SELECT revision FROM oikumenea.schema_version").Scan(&revision)
	if err != nil {
		return "", werror.WrapWithContextParams(ctx, err, "read schema_version")
	}
	return revision, nil
}
