//go:build integration

// Integration test for the M34 person regulatory-sanctions import target (D-Watchlists) against a real
// Postgres. Proves the oikumenea SIDE of the hermenea import path:
//
//   - a record referencing a real person creates the overlay row (Created=1);
//   - a re-import of unchanged data is an idempotent no-op (Skipped=1);
//   - a changed field updates in place (Updated=1), keyed on (person, externalId);
//   - a record whose person RID does not resolve is skipped (Skipped=1), non-destructively.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/dataimport/...
package dataimport_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/adapters"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/application"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

func newRegSanctionsService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	svc := newService(t, pool) // reuses the geo handlers; we only exercise the reg-sanctions one here
	svc.Register(domain.ObjectTypeRegulatorySanctions, application.RegulatorySanctionsHandler(
		func(conn pdb.DBTX) domain.RegulatorySanctionStore { return adapters.NewRegulatorySanctionRepo(conn) },
	))
	return svc
}

func TestRegulatorySanctionsImport(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newRegSanctionsService(t, pool)

	// A real person to attach the sanctions to (inserted directly; the person module owns its own tests).
	var personID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ('Reg Sanction Target') RETURNING id`).Scan(&personID); err != nil {
		t.Fatalf("insert person: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM oikumenea.person_regulatory_sanctions WHERE person_id = $1`, personID)
		_, _ = pool.Exec(ctx, `DELETE FROM oikumenea.person_persons WHERE id = $1`, personID)
	})

	env := func(status string) application.Envelope {
		return application.Envelope{
			ObjectType:    domain.ObjectTypeRegulatorySanctions,
			Source:        "operator-source",
			SourceVersion: "v1",
			Records: []domain.Record{{
				"personId": personID, "regulator": "SEC", "actionType": "fine",
				"amount": 50000.0, "currency": "USD", "status": status,
				"sanctionDate": "2021-06-01", "externalId": "SEC-2021-42",
			}},
		}
	}

	// create
	sum, err := svc.Import(ctx, domain.ObjectTypeRegulatorySanctions, env("active"))
	if err != nil {
		t.Fatalf("import create: %v", err)
	}
	if sum.Created != 1 || sum.Updated != 0 || sum.Skipped != 0 {
		t.Fatalf("create summary = %+v, want Created=1", sum)
	}

	// re-import unchanged -> skip
	sum, err = svc.Import(ctx, domain.ObjectTypeRegulatorySanctions, env("active"))
	if err != nil {
		t.Fatalf("import skip: %v", err)
	}
	if sum.Skipped != 1 || sum.Created != 0 || sum.Updated != 0 {
		t.Fatalf("skip summary = %+v, want Skipped=1", sum)
	}

	// changed status -> update in place (keyed on person+externalId)
	sum, err = svc.Import(ctx, domain.ObjectTypeRegulatorySanctions, env("appealed"))
	if err != nil {
		t.Fatalf("import update: %v", err)
	}
	if sum.Updated != 1 {
		t.Fatalf("update summary = %+v, want Updated=1", sum)
	}

	// exactly one active row for the person, now 'appealed'.
	var n int
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT count(*), max(status) FROM oikumenea.person_regulatory_sanctions
		 WHERE person_id = $1 AND deleted_at IS NULL`, personID).Scan(&n, &status); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if n != 1 || status != "appealed" {
		t.Fatalf("expected one 'appealed' row, got n=%d status=%q", n, status)
	}

	// unresolved person RID -> skipped (non-destructive)
	sum, err = svc.Import(ctx, domain.ObjectTypeRegulatorySanctions, application.Envelope{
		ObjectType:    domain.ObjectTypeRegulatorySanctions,
		Source:        "operator-source",
		SourceVersion: "v1",
		Records: []domain.Record{{
			"personId": "00000000-0000-0000-0000-000000000000", "regulator": "FCA", "externalId": "FCA-1",
		}},
	})
	if err != nil {
		t.Fatalf("import unresolved: %v", err)
	}
	if sum.Skipped != 1 || sum.Created != 0 {
		t.Fatalf("unresolved-person summary = %+v, want Skipped=1", sum)
	}
}
