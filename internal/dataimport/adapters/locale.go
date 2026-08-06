// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"

	"github.com/olehmushka/go-oikumenea/internal/dataimport/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// LocaleRepo is the create-if-absent implementation of domain.LocaleStore (D-DataPacks + D-i18n, M54),
// bound to a single db.DBTX (the caller's import transaction). It adds a supported locale to
// i18n_locales without ever touching an existing locale's operator-managed columns — a locale pack may
// ADD a locale, never re-enable/disable one or steal the default. Raw SQL (not sqlc) because it is a
// single create-only insert, like the pinax seeder's own marker writes.
type LocaleRepo struct {
	conn db.DBTX
}

// NewLocaleRepo binds a locale store to the given command surface.
func NewLocaleRepo(conn db.DBTX) *LocaleRepo { return &LocaleRepo{conn: conn} }

var _ domain.LocaleStore = (*LocaleRepo)(nil)

// Insert adds a supported locale create-if-absent. enabled/is_default/sort_order take the schema
// defaults (enabled=true, is_default=false, sort_order=0) on a fresh row; an existing code is left
// entirely untouched (ON CONFLICT DO NOTHING) and reported as created=false so the handler counts it
// skipped. created reflects whether a row was actually written.
func (r *LocaleRepo) Insert(ctx context.Context, code, name string) (bool, error) {
	tag, err := r.conn.Exec(ctx, `
		INSERT INTO oikumenea.i18n_locales (code, name)
		VALUES ($1, $2)
		ON CONFLICT (code) DO NOTHING`, code, name)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
