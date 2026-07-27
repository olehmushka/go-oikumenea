// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the Finance vertical against a real Postgres (M44 exit criteria, D-Finance /
// D-CryptoProvider / D-Audit). They exercise the audited catalog/account/card CRUD, the envelope-
// encryption of the IBAN + PAN (ciphertext at rest holds no plaintext, blind index present + unique,
// decrypt round-trips), a joint second holder, the duplicate-IBAN/PAN conflict guards, the card
// BIN/last-4 clear display, and the person-purge crypto-erase of a solely-held account (+ its cards)
// while a company-held account survives.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/finance/...
package finance_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/finance/adapters"
	"github.com/olegamysk/go-oikumenea/internal/finance/application"
	"github.com/olegamysk/go-oikumenea/internal/finance/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i + 11)
	}
	provider, err := crypto.NewLocalDevProvider(kek)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	cipher, err := crypto.NewCipher(provider, []byte("finance-blind-index-key"), 0)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return adapters.NewRepository(conn)
	}, audit, cipher, personalcode.New())
}

func uniq(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func catalogID(t *testing.T, pool *pgxpool.Pool, table, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), "SELECT id FROM oikumenea."+table+" WHERE code = $1", code).Scan(&id); err != nil {
		t.Fatalf("resolve %s %s: %v", table, code, err)
	}
	return id
}

func seedPerson(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ('Holder') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return id
}

// seedBank seeds a `company`-domain tenant organization (the bank / a corporate holder, M21/M41).
func seedBank(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO oikumenea.tenant_domains (code, name, pdp_scoped, sort_order)
		VALUES ('company','Company',false,40)
		ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("seed company domain: %v", err)
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
		 SELECT $1, $2, id FROM oikumenea.tenant_domains WHERE code = 'company' AND deleted_at IS NULL LIMIT 1
		 RETURNING id`, uniq("bank"), name).Scan(&id); err != nil {
		t.Fatalf("seed bank: %v", err)
	}
	return id
}

func assertOneAction(t *testing.T, pool *pgxpool.Pool, targetID, action string) {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM oikumenea.audit_log WHERE target_id = $1 AND action = $2 AND actor_type = 'system'",
		targetID, action).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 %s system action for %s, got %d", action, targetID, n)
	}
}

// A pair of Luhn-valid PANs + a valid IBAN reused across the scenario.
const (
	testIBAN     = "UA903052992990004149123456789"
	testPAN      = "4111111111111111"
	testPANBIN   = "411111"
	testPANLast4 = "1111"
)

// TestFinanceVertical drives the whole M44 exit-criteria slice in one ordered scenario.
func TestFinanceVertical(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	// The scenario reuses a fixed, checksum-valid IBAN + PANs (testIBAN/testPAN) that are globally
	// unique among active rows by blind index — so they can't use a uniq() prefix and, being
	// blind-indexed, can't be targeted by plaintext in SQL. Clear the account/card tables up front so
	// the test is re-runnable against a non-reset DB (CI runs a fresh DB; local re-runs otherwise hit
	// the IBAN/PAN blind-index unique constraints). account_holders references accounts, so truncate
	// all three together.
	if _, err := pool.Exec(ctx, `TRUNCATE oikumenea.finance_account_holders, oikumenea.finance_cards, oikumenea.finance_accounts`); err != nil {
		t.Fatalf("reset finance tables: %v", err)
	}

	bank := seedBank(t, pool, "First National")
	person := seedPerson(t, pool)
	company := seedBank(t, pool, "Acme Holdings") // a corporate account holder
	current := catalogID(t, pool, "finance_account_types", "current")
	visa := catalogID(t, pool, "finance_card_networks", "visa")

	// --- 1. a person holds an account: IBAN encrypted, decrypt round-trips ---
	acct, err := svc.CreateAccount(ctx, bank, testIBAN, "UAH", current)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	assertOneAction(t, pool, acct.ID, "finance.account.create")
	if acct.IBAN != testIBAN {
		t.Fatalf("returned IBAN = %q, want %q", acct.IBAN, testIBAN)
	}
	// The primary holder is the person.
	if _, err := svc.AddAccountHolder(ctx, acct.ID, domain.HolderInput{HolderKind: domain.HolderPerson, HolderID: person, Role: domain.RolePrimary}); err != nil {
		t.Fatalf("add person holder: %v", err)
	}

	// IBAN ciphertext at rest holds NO plaintext + blind index present.
	var ct, wdek, blind []byte
	if err := pool.QueryRow(ctx, `SELECT iban_ciphertext, iban_wrapped_dek, iban_blind_index
		FROM oikumenea.finance_accounts WHERE id = $1`, acct.ID).Scan(&ct, &wdek, &blind); err != nil {
		t.Fatalf("read account ciphertext: %v", err)
	}
	if len(ct) == 0 || len(wdek) == 0 || len(blind) == 0 {
		t.Fatalf("expected ciphertext + wrapped DEK + blind index at rest")
	}
	if bytes.Contains(ct, []byte(testIBAN)) {
		t.Fatalf("ciphertext leaks the plaintext IBAN")
	}
	// decrypt round-trips.
	got, err := svc.GetAccount(ctx, acct.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if got.IBAN != testIBAN {
		t.Fatalf("decrypted IBAN = %q, want %q", got.IBAN, testIBAN)
	}

	// --- 2. a duplicate IBAN (blind index) is a conflict ---
	if _, err := svc.CreateAccount(ctx, bank, testIBAN, "UAH", current); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate IBAN should conflict, got %v", err)
	}

	// --- 3. a JOINT second holder (a company) is added ---
	joint, err := svc.AddAccountHolder(ctx, acct.ID, domain.HolderInput{HolderKind: domain.HolderCompany, HolderID: company, Role: domain.RoleJoint})
	if err != nil {
		t.Fatalf("add joint company holder: %v", err)
	}
	if joint.Role != domain.RoleJoint {
		t.Fatalf("expected joint role, got %q", joint.Role)
	}
	holders, err := svc.ListAccountHolders(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list holders: %v", err)
	}
	if len(holders) != 2 {
		t.Fatalf("expected 2 holders, got %d", len(holders))
	}

	// --- 4. a card under the account: PAN encrypted, BIN/last-4 clear, duplicate PAN conflicts ---
	card, err := svc.AddCard(ctx, acct.ID, testPAN, visa, domain.CardDebit, ptrInt(12), ptrInt(2030), person)
	if err != nil {
		t.Fatalf("add card: %v", err)
	}
	assertOneAction(t, pool, card.ID, "finance.card.add")
	if card.BIN != testPANBIN || card.LastFour != testPANLast4 {
		t.Fatalf("card display bin/last4 = %q/%q, want %q/%q", card.BIN, card.LastFour, testPANBIN, testPANLast4)
	}
	var panCT []byte
	if err := pool.QueryRow(ctx, `SELECT pan_ciphertext FROM oikumenea.finance_cards WHERE id = $1`, card.ID).Scan(&panCT); err != nil {
		t.Fatalf("read card ciphertext: %v", err)
	}
	if len(panCT) == 0 || bytes.Contains(panCT, []byte(testPAN)) {
		t.Fatalf("card PAN ciphertext missing or leaks plaintext")
	}
	gotCard, err := svc.GetCard(ctx, card.ID)
	if err != nil {
		t.Fatalf("get card: %v", err)
	}
	if gotCard.PAN != testPAN {
		t.Fatalf("decrypted PAN = %q, want %q", gotCard.PAN, testPAN)
	}
	if _, err := svc.AddCard(ctx, acct.ID, testPAN, visa, domain.CardCredit, nil, nil, ""); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate PAN should conflict, got %v", err)
	}

	// --- 5. an INVALID IBAN / PAN / card type is rejected ---
	if _, err := svc.CreateAccount(ctx, bank, "GB00WEST12345698765432", "GBP", current); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad IBAN should be invalid, got %v", err)
	}
	if _, err := svc.AddCard(ctx, acct.ID, "4111111111111112", visa, domain.CardDebit, nil, nil, ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad PAN (Luhn) should be invalid, got %v", err)
	}

	// --- 5b. R-32: the DB shape CHECK rejects a wrong-RID-type holder, even when an app-layer
	//         existence check is bypassed (a future write path / an M49 import). `company` is a
	//         tenant-org RID (4,1,6); stored under holder_kind='person' it must fail at the DB. ---
	if _, rawErr := pool.Exec(ctx,
		`INSERT INTO oikumenea.finance_account_holders (account_id, holder_kind, holder_id) VALUES ($1, 'person', $2)`,
		acct.ID, company); !isCheckViolation(rawErr, "holder_shape") {
		t.Fatalf("a company RID stored under holder_kind='person' must fail the DB shape check, got %v", rawErr)
	}

	// --- 6. a company-SOLELY-held account (no person holder) that must survive a person purge ---
	corpAcct, err := svc.CreateAccount(ctx, bank, "DE89370400440532013000", "EUR", current)
	if err != nil {
		t.Fatalf("create corp account: %v", err)
	}
	if _, err := svc.AddAccountHolder(ctx, corpAcct.ID, domain.HolderInput{HolderKind: domain.HolderCompany, HolderID: company, Role: domain.RolePrimary}); err != nil {
		t.Fatalf("add corp holder: %v", err)
	}

	// --- 7. person purge: the JOINT account (person + company) is NOT solely-held → survives with its
	//        envelope; but a person-SOLELY-held account IS crypto-erased. Build a solely-held one first. ---
	soleAcct, err := svc.CreateAccount(ctx, bank, "GB82WEST12345698765432", "GBP", current)
	if err != nil {
		t.Fatalf("create sole account: %v", err)
	}
	if _, err := svc.AddAccountHolder(ctx, soleAcct.ID, domain.HolderInput{HolderKind: domain.HolderPerson, HolderID: person, Role: domain.RolePrimary}); err != nil {
		t.Fatalf("add sole holder: %v", err)
	}
	soleCard, err := svc.AddCard(ctx, soleAcct.ID, "5555555555554444", visa, domain.CardCredit, nil, nil, "")
	if err != nil {
		t.Fatalf("add sole card: %v", err)
	}

	erased, err := svc.ErasePersonAccounts(ctx, person)
	if err != nil {
		t.Fatalf("erase person accounts: %v", err)
	}
	if erased != 1 {
		t.Fatalf("expected exactly 1 solely-held account crypto-erased, got %d", erased)
	}

	// The solely-held account row SURVIVES but its IBAN envelope is destroyed (crypto-erase tombstone).
	assertEnvelopeErased(t, pool, "finance_accounts", "iban_ciphertext", "iban_wrapped_dek", soleAcct.ID)
	// Its card's PAN envelope is destroyed too.
	assertEnvelopeErased(t, pool, "finance_cards", "pan_ciphertext", "pan_wrapped_dek", soleCard.ID)
	// The JOINT account (person + company) is NOT solely-held → its envelope survives.
	assertEnvelopeIntact(t, pool, acct.ID)
	// The company-SOLELY-held account is untouched by a PERSON purge.
	assertEnvelopeIntact(t, pool, corpAcct.ID)

	// The person's holder edges are all soft-deleted; the person no longer holds any account.
	remaining, err := svc.ListPersonAccounts(ctx, person)
	if err != nil {
		t.Fatalf("list person accounts after purge: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected 0 held accounts after purge, got %d", len(remaining))
	}

	// --- 8. catalog reads return locale->text name maps (via the transport-less app path here just
	//        confirms the catalog is populated + typed) ---
	types, err := svc.ListAccountTypes(ctx)
	if err != nil || len(types) == 0 {
		t.Fatalf("list account types: %v (n=%d)", err, len(types))
	}
}

func assertEnvelopeErased(t *testing.T, pool *pgxpool.Pool, table, ctCol, dekCol, id string) {
	t.Helper()
	var ct, dek []byte
	var exists bool
	if err := pool.QueryRow(context.Background(),
		fmt.Sprintf("SELECT %s, %s, true FROM oikumenea.%s WHERE id = $1", ctCol, dekCol, table), id).Scan(&ct, &dek, &exists); err != nil {
		t.Fatalf("read %s %s: %v", table, id, err)
	}
	if !exists {
		t.Fatalf("expected %s row %s to SURVIVE the crypto-erase", table, id)
	}
	if len(ct) != 0 || len(dek) != 0 {
		t.Fatalf("expected %s %s envelope destroyed, still has ct=%d dek=%d bytes", table, id, len(ct), len(dek))
	}
}

func assertEnvelopeIntact(t *testing.T, pool *pgxpool.Pool, accountID string) {
	t.Helper()
	var ct, dek []byte
	if err := pool.QueryRow(context.Background(),
		"SELECT iban_ciphertext, iban_wrapped_dek FROM oikumenea.finance_accounts WHERE id = $1", accountID).Scan(&ct, &dek); err != nil {
		t.Fatalf("read account %s: %v", accountID, err)
	}
	if len(ct) == 0 || len(dek) == 0 {
		t.Fatalf("expected account %s envelope INTACT, got ct=%d dek=%d", accountID, len(ct), len(dek))
	}
}

func ptrInt(i int) *int { return &i }

// isCheckViolation reports whether err is a Postgres CHECK-constraint violation (SQLSTATE 23514)
// whose constraint name contains constraintSubstr — used to assert the R-32 holder-shape gate.
func isCheckViolation(err error, constraintSubstr string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514" && strings.Contains(pgErr.ConstraintName, constraintSubstr)
}
