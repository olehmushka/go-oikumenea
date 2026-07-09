// Package adapters — the person regulatory-sanction import store (D-Watchlists, M34; set-based per
// chunk since R-05). Raw pgx (no sqlc): a person-scoped import target (unusual — most targets are
// instance-global catalogs). One parallel-array merge statement resolves persons inline (an
// unresolved person drops out of the join — skipped, non-destructive) and keys idempotency on
// (person_id, external_id), writing the M34 person_regulatory_sanctions table. Optional
// amount/sanction_date cross as text ('' = NULL) so one array can carry absent values.
package adapters

import (
	"context"

	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// RegulatorySanctionRepo is the raw-pgx implementation of domain.RegulatorySanctionStore, bound to a
// single db.DBTX (the pool for reads, or a caller-supplied transaction so the upsert and its audit row
// commit together).
type RegulatorySanctionRepo struct{ c db.DBTX }

// NewRegulatorySanctionRepo binds a store to the given command surface.
func NewRegulatorySanctionRepo(conn db.DBTX) *RegulatorySanctionRepo {
	return &RegulatorySanctionRepo{c: conn}
}

var _ domain.RegulatorySanctionStore = (*RegulatorySanctionRepo)(nil)

// upsertRegulatorySanctions is the shared body of the merge; the conflict clause differs between the
// update-on-change default and the CreateOnly (pinax) skip-existing variant.
const upsertRegulatorySanctions = `
	WITH r AS (
	  SELECT unnest($1::uuid[]) AS person_id,
	         unnest($2::text[]) AS regulator,
	         unnest($3::text[]) AS action_type,
	         unnest($4::text[]) AS amount,
	         unnest($5::text[]) AS currency,
	         unnest($6::text[]) AS status,
	         unnest($7::text[]) AS sanction_date,
	         unnest($8::text[]) AS source_url,
	         unnest($9::text[]) AS external_id
	)
	INSERT INTO oikumenea.person_regulatory_sanctions
		(person_id, regulator, action_type, amount, currency, status, sanction_date,
		 source_url, external_id, source, confidence)
	SELECT p.id, r.regulator, r.action_type,
	       NULLIF(r.amount, '')::numeric,
	       NULLIF(r.currency, ''),
	       r.status,
	       NULLIF(r.sanction_date, '')::date,
	       NULLIF(r.source_url, ''),
	       NULLIF(r.external_id, ''),
	       'imported', 'probable'
	FROM r
	JOIN oikumenea.person_persons p ON p.id = r.person_id AND p.deleted_at IS NULL
	ON CONFLICT (person_id, external_id) WHERE external_id IS NOT NULL AND deleted_at IS NULL
	`

// BulkUpsert merges one chunk set-based (R-05): a record whose person RID does not resolve is not
// merged (the handler pre-drops RIDs that are not even canonical uuids so the uuid[] parameter
// encodes); an existing (person, externalId) row updates only when a comparable field changed —
// mirroring domain.RegulatorySanction.SameAs — and never under prov.CreateOnly. RETURNING (xmax = 0)
// splits creates from updates; unmerged rows are the caller's skips.
func (r *RegulatorySanctionRepo) BulkUpsert(ctx context.Context, ss []domain.RegulatorySanction, prov domain.Provenance) (created, updated int, err error) {
	if len(ss) == 0 {
		return 0, 0, nil
	}
	n := len(ss)
	personIDs := make([]string, 0, n)
	regulators := make([]string, 0, n)
	actionTypes := make([]string, 0, n)
	amounts := make([]string, 0, n)
	currencies := make([]string, 0, n)
	statuses := make([]string, 0, n)
	dates := make([]string, 0, n)
	urls := make([]string, 0, n)
	externalIDs := make([]string, 0, n)
	for _, s := range ss {
		personIDs = append(personIDs, s.PersonID)
		regulators = append(regulators, s.Regulator)
		actionTypes = append(actionTypes, s.ActionType)
		amounts = append(amounts, floatText(s.Amount))
		currencies = append(currencies, s.Currency)
		statuses = append(statuses, s.Status)
		dates = append(dates, s.SanctionDate)
		urls = append(urls, s.SourceURL)
		externalIDs = append(externalIDs, s.ExternalID)
	}
	query := upsertRegulatorySanctions
	if prov.CreateOnly {
		query += `DO NOTHING
	RETURNING (xmax = 0) AS inserted`
	} else {
		query += `DO UPDATE SET
		regulator     = EXCLUDED.regulator,
		action_type   = EXCLUDED.action_type,
		amount        = EXCLUDED.amount,
		currency      = EXCLUDED.currency,
		status        = EXCLUDED.status,
		sanction_date = EXCLUDED.sanction_date,
		source_url    = EXCLUDED.source_url,
		source        = 'imported',
		updated_at    = now()
	WHERE (oikumenea.person_regulatory_sanctions.regulator,
	       oikumenea.person_regulatory_sanctions.action_type,
	       oikumenea.person_regulatory_sanctions.amount,
	       oikumenea.person_regulatory_sanctions.currency,
	       oikumenea.person_regulatory_sanctions.status,
	       oikumenea.person_regulatory_sanctions.sanction_date,
	       oikumenea.person_regulatory_sanctions.source_url)
	      IS DISTINCT FROM
	      (EXCLUDED.regulator, EXCLUDED.action_type, EXCLUDED.amount, EXCLUDED.currency,
	       EXCLUDED.status, EXCLUDED.sanction_date, EXCLUDED.source_url)
	RETURNING (xmax = 0) AS inserted`
	}
	rows, err := r.c.Query(ctx, query,
		personIDs, regulators, actionTypes, amounts, currencies, statuses, dates, urls, externalIDs)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var inserted bool
		if err := rows.Scan(&inserted); err != nil {
			return 0, 0, err
		}
		if inserted {
			created++
		} else {
			updated++
		}
	}
	return created, updated, rows.Err()
}
