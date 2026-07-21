package adapters

import (
	"context"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// LoginEventRepo is the raw-pgx implementation of domain.LoginEventRepository over
// oikumenea.account_login_events (M37 / D-LoginSecurityLog). Deliberately raw (not sqlc): the record
// path is a single dedup CTE that reads cleaner as literal SQL, and the store is self-contained. Bound
// to a db.DBTX like the sqlc Repository, so EraseByPerson runs inside the person-purge transaction.
type LoginEventRepo struct {
	conn db.DBTX
}

// NewLoginEventRepo binds the login-event store to a command surface (pool for record/read/sweep, the
// purge tx for EraseByPerson).
func NewLoginEventRepo(conn db.DBTX) *LoginEventRepo { return &LoginEventRepo{conn: conn} }

var _ domain.LoginEventRepository = (*LoginEventRepo)(nil)

// RecordSeen is the bounded-dedup upsert: within `windowSeconds` of an existing (account, context, ip)
// row, bump last_seen_at + occurrence_count (and fill any newly-resolved intel); otherwise insert a
// new row. One atomic statement (a rare concurrent double-insert is acceptable for a best-effort
// security log). The intel columns are COALESCEd so a later resolver can fill a NULL without clobbering.
func (r *LoginEventRepo) RecordSeen(ctx context.Context, accountID string, c domain.LoginContext, ip string, userAgent *string, intel domain.IPIntel, windowSeconds int) error {
	_, err := r.conn.Exec(ctx, `
WITH upd AS (
  UPDATE oikumenea.account_login_events
     SET last_seen_at      = now(),
         occurrence_count  = occurrence_count + 1,
         user_agent        = COALESCE($4, user_agent),
         resolved_country  = COALESCE($5, resolved_country),
         resolved_isp      = COALESCE($6, resolved_isp),
         is_vpn            = COALESCE($7, is_vpn),
         is_tor            = COALESCE($8, is_tor)
   WHERE account_id = $1 AND context = $2 AND ip = $3::inet
     AND last_seen_at > now() - ($9 * interval '1 second')
   RETURNING id
)
INSERT INTO oikumenea.account_login_events
   (account_id, context, ip, user_agent, resolved_country, resolved_isp, is_vpn, is_tor)
SELECT $1, $2, $3::inet, $4, $5, $6, $7, $8
 WHERE NOT EXISTS (SELECT 1 FROM upd)`,
		accountID, string(c), ip, userAgent, intel.Country, intel.ISP, intel.IsVPN, intel.IsTor, windowSeconds)
	return err
}

// ListByAccount pages an account's login history newest-first. Keyset on the time-ordered RID (id DESC
// ≈ first-occurrence order); occurrence_count + last_seen_at carry recency per row. afterID "" = first
// page.
func (r *LoginEventRepo) ListByAccount(ctx context.Context, accountID, afterID string, limit int) ([]domain.LoginEvent, error) {
	rows, err := r.conn.Query(ctx, `
SELECT id, account_id, context, host(ip), first_seen_at, last_seen_at, occurrence_count,
       resolved_country, resolved_isp, is_vpn, is_tor, user_agent
  FROM oikumenea.account_login_events
 WHERE account_id = $1 AND ($2 = '' OR id < $2::uuid)
 ORDER BY id DESC
 LIMIT $3`, accountID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.LoginEvent
	for rows.Next() {
		var e domain.LoginEvent
		if err := rows.Scan(&e.ID, &e.AccountID, &e.Context, &e.IP, &e.FirstSeenAt, &e.LastSeenAt,
			&e.OccurrenceCount, &e.ResolvedCountry, &e.ResolvedISP, &e.IsVPN, &e.IsTor, &e.UserAgent); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EraseByPerson hard-deletes every login event for the person's account(s) — the purge fan-out
// (D-PersonModuleSplit / D-LoginSecurityLog: pii:contact is erased, not retained). Runs on the purge tx.
func (r *LoginEventRepo) EraseByPerson(ctx context.Context, personID string) (int64, error) {
	tag, err := r.conn.Exec(ctx, `
DELETE FROM oikumenea.account_login_events e
 USING oikumenea.account_accounts a
 WHERE a.id = e.account_id AND a.person_id = $1::uuid`, personID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteBefore runs the retention sweep via the migration helper, returning the number of rows deleted.
func (r *LoginEventRepo) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var n int64
	if err := r.conn.QueryRow(ctx, `SELECT oikumenea.delete_login_events_before($1)`, cutoff).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
