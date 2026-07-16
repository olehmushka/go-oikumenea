package application

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// buildArmQuery builds one arm's keyset query. All identifiers (table + columns) come from the
// compile-time descriptor registry and are passed through pgx.Identifier.Sanitize; the queried RID,
// the polymorphic discriminator, the keyset cursor and the row limit are bound parameters. Shape:
//
//	SELECT id::text, <other>::text[, <otherKind>][, <attr>::text …]
//	FROM <schema.table>
//	WHERE <self> = $1[::uuid] [AND <selfKind> = $2] [AND deleted_at IS NULL] AND id::text > $k
//	ORDER BY id LIMIT $m
//
// id::text keyset ordering matches ORDER BY id (uuid byte order == canonical-text order: the dashes
// sit at fixed positions and hex compares like the bytes) — the same pattern the sqlc link queries use.
func buildArmQuery(a arm, srcUUID, after string, limit int) (string, []any) {
	cols := []string{"id::text", ident(a.other.Column) + "::text"}
	if a.other.KindCol != "" {
		cols = append(cols, ident(a.other.KindCol))
	}
	for _, ac := range a.desc.AttrCols {
		cols = append(cols, ident(ac)+"::text")
	}

	var where []string
	args := make([]any, 0, 4)
	n := 1
	if a.self.KindCol != "" { // polymorphic self end: text id column, discriminator filter
		where = append(where, fmt.Sprintf("%s = $%d", ident(a.self.Column), n))
		args = append(args, srcUUID)
		n++
		where = append(where, fmt.Sprintf("%s = $%d", ident(a.self.KindCol), n))
		args = append(args, a.selfKind)
		n++
	} else { // plain uuid end
		where = append(where, fmt.Sprintf("%s = $%d::uuid", ident(a.self.Column), n))
		args = append(args, srcUUID)
		n++
	}
	if !a.desc.NoSoftDelete {
		where = append(where, "deleted_at IS NULL")
	}
	where = append(where, fmt.Sprintf("id::text > $%d", n))
	args = append(args, after)
	n++

	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY id LIMIT $%d",
		strings.Join(cols, ", "), qualifiedTable(a.desc.Table), strings.Join(where, " AND "), n)
	args = append(args, limit)
	return sql, args
}

func ident(col string) string { return pgx.Identifier{col}.Sanitize() }

func qualifiedTable(t string) string { return pgx.Identifier(strings.Split(t, ".")).Sanitize() }
