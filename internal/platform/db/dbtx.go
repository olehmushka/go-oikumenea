package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the minimal command surface shared by *pgxpool.Pool and pgx.Tx. Module repositories
// accept a DBTX so the application layer controls transaction boundaries (conventions.md): a read
// runs on the pool, while an audited write hands the repository the caller's open transaction so
// the audit row commits iff the change commits (D-Audit). Its method set matches the interface
// sqlc generates per module, so a DBTX value satisfies those generated interfaces directly.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
