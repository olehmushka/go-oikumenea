// Package migrations embeds the repo-root Atlas migration files so binaries can derive their
// expected schema revision from the migrations themselves rather than a hand-bumped constant
// (architecture review R-15). Only the oikumenea migrations are embedded (migrations/*.sql);
// the hermenea companion's migrations live in the migrations/hermenea subdir and are not matched.
package migrations

import "embed"

// FS holds the versioned oikumenea migration files. atlas ignores this Go file when hashing the
// directory (migrate hash only considers .sql files + atlas.sum), so it does not affect atlas.sum.
//
//go:embed *.sql
var FS embed.FS
