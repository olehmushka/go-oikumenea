// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package hermeneamigrations embeds hermenea's versioned Atlas migration files so the hermenea
// binary can derive its expected schema revision from the migrations themselves rather than a
// hand-bumped constant (architecture review R-25, mirroring oikumenea's migrations.FS / R-15).
// Kept as its own package under migrations/hermenea because the repo-root migrations package embeds
// only oikumenea's migrations (migrations/*.sql) and deliberately excludes this subdir.
package hermeneamigrations

import (
	"embed"
)

// FS holds the versioned hermenea migration files. atlas ignores this Go file when hashing the
// directory (migrate hash only considers .sql files + atlas.sum), so it does not affect atlas.sum.
//
//go:embed *.sql
var FS embed.FS
