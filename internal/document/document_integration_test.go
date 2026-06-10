//go:build integration

// Integration tests for the document module against a real Postgres (M9 exit criteria, D-Documents /
// D-PersonalCodes / D-CryptoProvider / D-Audit):
//   - a paper is attached to a person and reads back; a duplicate (type, number) is rejected;
//   - a personal code is stored as CIPHERTEXT + blind index (the DB never holds the plaintext) and
//     decrypts back to the original value on read;
//   - an invalid value (bad РНОКПП checksum) is rejected; a duplicate (scheme, value) is rejected
//     cross-person; an unknown scheme is rejected;
//   - updating a code re-encrypts the new value;
//   - person purge crypto-erases codes (wrapped DEK + ciphertext destroyed) and NULLs document
//     number/issuer, keeping row ids as tombstones;
//   - a write + its audit row share one transaction.
//
// Run against a throwaway DB that has the migrations applied (the ua-rnokpp scheme is migration-seeded):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/document/...
package document_test

import (
	"bytes"
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/document/adapters"
	"github.com/olegamysk/go-oikumenea/internal/document/application"
	"github.com/olegamysk/go-oikumenea/internal/document/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

// freshRNOKPP builds a РНОКПП with a valid weighted check digit and a random 9-digit prefix, so each
// value is unique across the shared test DB (the blind-index uniqueness is global per scheme).
func freshRNOKPP() string {
	weights := [9]int{-1, 5, 7, 9, 4, 6, 10, 5, 7}
	d := make([]byte, 10)
	sum := 0
	for i := 0; i < 9; i++ {
		n := rand.IntN(10)
		d[i] = byte('0' + n)
		sum += n * weights[i]
	}
	check := ((sum % 11) % 10)
	if check < 0 {
		check += 10
	}
	d[9] = byte('0' + check)
	return string(d)
}

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

func newService(t *testing.T) (*application.Service, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(t)
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })

	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i + 7)
	}
	provider, err := crypto.NewLocalDevProvider(kek)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	cipher, err := crypto.NewCipher(provider, []byte("integration-blind-index-key"), 0)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	svc := application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return adapters.NewRepository(conn)
	}, audit, cipher, personalcode.New())
	return svc, pool
}

func code(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + uuid.NewString()[:8]
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

// TestDocumentAttrSchema exercises the per-type attribute schema (D-DocumentAttrSchema): a document of
// a type that declares a schema is validated against it on write — valid attributes are accepted,
// unknown keys / wrong types / bad enum values are rejected as ErrDocumentInvalid.
func TestDocumentAttrSchema(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	person := seedPerson(t, pool)

	schema := []byte(`{"fields":{
	  "vos":{"type":"string"},
	  "fitness_category":{"type":"string","enum":["А","Б","В","Г","Д"]},
	  "issued_year":{"type":"number"}
	}}`)
	typ, err := svc.CreateDocumentType(ctx, domain.DocumentType{Code: code(t, "milid"), Name: "Military ID", AttrSchema: schema})
	if err != nil {
		t.Fatalf("create typed type: %v", err)
	}

	// Valid attributes are accepted.
	if _, err := svc.AttachDocument(ctx, domain.Document{
		PersonID: person, TypeID: typ.ID, Number: code(t, "mil"),
		Attributes: []byte(`{"vos":"100","fitness_category":"А","issued_year":2014}`),
	}); err != nil {
		t.Fatalf("valid attributes rejected: %v", err)
	}

	// Unknown key, wrong type, and bad enum are each rejected.
	for name, attrs := range map[string]string{
		"unknown key": `{"unknown":"x"}`,
		"wrong type":  `{"issued_year":"not-a-number"}`,
		"bad enum":    `{"fitness_category":"Z"}`,
	} {
		if _, err := svc.AttachDocument(ctx, domain.Document{
			PersonID: person, TypeID: typ.ID, Number: code(t, "mil"), Attributes: []byte(attrs),
		}); !errors.Is(err, domain.ErrDocumentInvalid) {
			t.Fatalf("%s: want ErrDocumentInvalid, got %v", name, err)
		}
	}

	// A type with no schema accepts free-form attributes.
	free, err := svc.CreateDocumentType(ctx, domain.DocumentType{Code: code(t, "free"), Name: "Freeform"})
	if err != nil {
		t.Fatalf("create freeform type: %v", err)
	}
	if _, err := svc.AttachDocument(ctx, domain.Document{
		PersonID: person, TypeID: free.ID, Number: code(t, "free"), Attributes: []byte(`{"anything":"goes","n":1}`),
	}); err != nil {
		t.Fatalf("freeform attributes rejected: %v", err)
	}
}

func TestDocumentLifecycle(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	person := seedPerson(t, pool)

	typ, err := svc.CreateDocumentType(ctx, domain.DocumentType{Code: code(t, "passport"), Name: "Passport"})
	if err != nil {
		t.Fatalf("create type: %v", err)
	}

	doc, err := svc.AttachDocument(ctx, domain.Document{
		PersonID: person, TypeID: typ.ID, Number: "AA123456", Issuer: "DMS", IssuingCountry: "UA", IssuedOn: "2020-01-02",
	})
	if err != nil {
		t.Fatalf("attach document: %v", err)
	}
	if doc.Status != domain.DocumentActive {
		t.Fatalf("new document should be active, got %q", doc.Status)
	}

	// Duplicate (person, type, number) is rejected.
	if _, err := svc.AttachDocument(ctx, domain.Document{PersonID: person, TypeID: typ.ID, Number: "AA123456"}); !errors.Is(err, domain.ErrDocumentConflict) {
		t.Fatalf("duplicate document should conflict, got %v", err)
	}

	// A write recorded an audit row in the same transaction.
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM oikumenea.audit_log WHERE action = 'document.create' AND target_id = $1`, doc.ID).Scan(&n); err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 document.create audit row, got %d", n)
	}

	// List + soft-delete.
	page, err := svc.ListPersonDocuments(ctx, person, 10, "")
	if err != nil || len(page.Documents) != 1 {
		t.Fatalf("list documents: got %d err %v", len(page.Documents), err)
	}
	if err := svc.DeleteDocument(ctx, doc.ID); err != nil {
		t.Fatalf("delete document: %v", err)
	}
	if _, err := svc.GetDocument(ctx, doc.ID); !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("deleted document should be gone, got %v", err)
	}
}

func TestPersonalCodeEncryptionRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	person := seedPerson(t, pool)

	val := freshRNOKPP()
	created, err := svc.AttachPersonalCode(ctx, person, "ua-rnokpp", val)
	if err != nil {
		t.Fatalf("attach personal code: %v", err)
	}
	if created.Value != val {
		t.Fatalf("returned value mismatch: %q", created.Value)
	}

	// The DB stores ciphertext, never the plaintext.
	var ciphertext []byte
	if err := pool.QueryRow(ctx,
		`SELECT value_ciphertext FROM oikumenea.document_personal_codes WHERE id = $1`, created.ID).Scan(&ciphertext); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if len(ciphertext) == 0 || bytes.Contains(ciphertext, []byte(val)) {
		t.Fatal("value must be stored as ciphertext, not plaintext")
	}

	// Read decrypts back to the original.
	codes, err := svc.ListPersonPersonalCodes(ctx, person)
	if err != nil || len(codes) != 1 {
		t.Fatalf("list codes: got %d err %v", len(codes), err)
	}
	if codes[0].Value != val {
		t.Fatalf("decrypt round-trip mismatch: %q", codes[0].Value)
	}

	// Update re-encrypts a new (valid) value.
	newVal := freshRNOKPP()
	upd, err := svc.UpdatePersonalCode(ctx, created.ID, &newVal, nil)
	if err != nil {
		t.Fatalf("update code: %v", err)
	}
	if upd.Value != newVal {
		t.Fatalf("updated value mismatch: %q", upd.Value)
	}
}

func TestPersonalCodeValidationAndUniqueness(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	p1 := seedPerson(t, pool)
	p2 := seedPerson(t, pool)

	// Bad checksum rejected.
	if _, err := svc.AttachPersonalCode(ctx, p1, "ua-rnokpp", "1234567890"); !errors.Is(err, domain.ErrPersonalCodeInvalid) {
		t.Fatalf("bad checksum should be invalid, got %v", err)
	}
	// Unknown scheme rejected.
	if _, err := svc.AttachPersonalCode(ctx, p1, "zz-nonexistent", "1234567899"); !errors.Is(err, domain.ErrUnknownScheme) {
		t.Fatalf("unknown scheme should be rejected, got %v", err)
	}

	dup := freshRNOKPP()
	if _, err := svc.AttachPersonalCode(ctx, p1, "ua-rnokpp", dup); err != nil {
		t.Fatalf("first attach: %v", err)
	}
	// Same (scheme, value) for another person is rejected cross-person (over the blind index).
	if _, err := svc.AttachPersonalCode(ctx, p2, "ua-rnokpp", dup); !errors.Is(err, domain.ErrPersonalCodeDuplicate) {
		t.Fatalf("cross-person duplicate should be rejected, got %v", err)
	}
}

func TestPurgeCryptoErase(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	person := seedPerson(t, pool)

	typ, err := svc.CreateDocumentType(ctx, domain.DocumentType{Code: code(t, "id"), Name: "ID"})
	if err != nil {
		t.Fatalf("create type: %v", err)
	}
	if _, err := svc.AttachDocument(ctx, domain.Document{PersonID: person, TypeID: typ.ID, Number: "N-1", Issuer: "Authority"}); err != nil {
		t.Fatalf("attach document: %v", err)
	}
	codeRec, err := svc.AttachPersonalCode(ctx, person, "ua-rnokpp", freshRNOKPP())
	if err != nil {
		t.Fatalf("attach code: %v", err)
	}

	docs, codes, err := svc.ErasePersonRecords(ctx, person)
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	if docs != 1 || codes != 1 {
		t.Fatalf("expected 1 doc + 1 code erased, got %d / %d", docs, codes)
	}

	// Document PII NULLed.
	var number *string
	if err := pool.QueryRow(ctx,
		`SELECT number FROM oikumenea.document_documents WHERE person_id = $1`, person).Scan(&number); err != nil {
		t.Fatalf("read document: %v", err)
	}
	if number != nil {
		t.Fatalf("document number should be NULLed on purge, got %q", *number)
	}

	// Code crypto-erased: wrapped DEK + ciphertext destroyed, row id kept as a tombstone.
	var ct, dek []byte
	if err := pool.QueryRow(ctx,
		`SELECT value_ciphertext, wrapped_dek FROM oikumenea.document_personal_codes WHERE id = $1`, codeRec.ID).Scan(&ct, &dek); err != nil {
		t.Fatalf("read code: %v", err)
	}
	if ct != nil || dek != nil {
		t.Fatal("ciphertext and wrapped DEK must be destroyed on crypto-erase")
	}

	// Decryption of an erased code yields the empty tombstone value.
	got, err := svc.ListPersonPersonalCodes(ctx, person)
	if err != nil || len(got) != 1 {
		t.Fatalf("list after erase: %d err %v", len(got), err)
	}
	if got[0].Value != "" {
		t.Fatalf("crypto-erased value should read empty, got %q", got[0].Value)
	}
}
