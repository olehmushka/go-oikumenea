// Package application holds the document module's application service — the orchestrator the transport
// layer calls to read/mutate documents (papers), personal codes (encrypted national identifiers), and
// their catalogs, recording an audit row in the same transaction as each write (D-Audit). It owns the
// crypto: personal-code values are validated against their scheme (D-PersonalCodes), envelope-encrypted
// before persistence and decrypted on read (D-CryptoProvider) — the repository sees only ciphertext.
//
// Existence of referenced persons/types/countries is validated by the DB foreign keys (mapped to domain
// sentinels in the adapter); the scheme is loaded first because its validation_regex feeds the code
// validator. A document/code is a directory attribute and never an authz input. PII discipline: audit
// payloads carry only non-PII ids/keys — never a document number, issuer, or a personal-code value.
package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/document/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// Page-size policy (API conventions: token pagination, bounded pages).
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

// auditSubsystem labels the interim system actor for document admin writes; eraseSubsystem labels the
// purge/crypto-erase path run on a person's records. Until the acting person is threaded through (the
// shared M7/M8 follow-up for holder-scoped endpoints) these are recorded as a `system` action.
const (
	auditSubsystem = "document-admin"
	eraseSubsystem = "event-subscriber"
)

// Audit target types (the audited entity kinds).
const (
	targetDocument     = "document"
	targetDocumentType = "document_type"
	targetPersonalCode = "personal_code"
	targetScheme       = "personal_code_scheme"
	targetPerson       = "person"
)

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a caller's
// transaction for an audited write (D-Audit). Injected by module.go so the application never imports
// adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the document application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly. cipher + codes implement the crypto + validation seams.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
	cipher  *crypto.Cipher
	codes   *personalcode.Registry
}

// NewService wires the service with the pool, the repository factory, the audit service, the envelope
// cipher (D-CryptoProvider), and the personal-code validator registry (D-PersonalCodes).
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, cipher *crypto.Cipher, codes *personalcode.Registry) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, cipher: cipher, codes: codes}
}

// DocumentPage is a keyset-paginated slice of documents plus the opaque next-page token.
type DocumentPage struct {
	Documents     []domain.Document
	NextPageToken string
}

// ---------------------------------------------------------------- document types

func (s *Service) CreateDocumentType(ctx context.Context, t domain.DocumentType) (domain.DocumentType, error) {
	if err := t.Validate(); err != nil {
		return domain.DocumentType{}, err
	}
	var out domain.DocumentType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertDocumentType(ctx, t)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "document.type.create", targetDocumentType, created.ID, map[string]any{"id": created.ID, "code": created.Code})
	})
	return out, err
}

func (s *Service) UpdateDocumentType(ctx context.Context, id string, patch domain.DocumentTypePatch) (domain.DocumentType, error) {
	if err := patch.Validate(); err != nil {
		return domain.DocumentType{}, err
	}
	var out domain.DocumentType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateDocumentType(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "document.type.update", targetDocumentType, id, map[string]any{"id": id, "status": string(updated.Status)})
	})
	return out, err
}

func (s *Service) ListDocumentTypes(ctx context.Context) ([]domain.DocumentType, error) {
	return s.newRepo(s.pool).ListDocumentTypes(ctx)
}

// ---------------------------------------------------------------- documents (papers)

func (s *Service) AttachDocument(ctx context.Context, d domain.Document) (domain.Document, error) {
	if err := d.Validate(); err != nil {
		return domain.Document{}, err
	}
	var out domain.Document
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		t, err := repo.GetDocumentType(ctx, d.TypeID)
		if err != nil {
			if errors.Is(err, domain.ErrDocumentTypeNotFound) {
				return domain.ErrUnknownType // surface as "document type does not exist"
			}
			return err
		}
		if err := domain.ValidateAttributes(t.AttrSchema, d.Attributes); err != nil {
			return err
		}
		created, err := repo.InsertDocument(ctx, d)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "document.create", targetDocument, created.ID,
			map[string]any{"id": created.ID, "personId": created.PersonID, "typeId": created.TypeID})
	})
	return out, err
}

func (s *Service) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	return s.newRepo(s.pool).GetDocument(ctx, id)
}

func (s *Service) UpdateDocument(ctx context.Context, id string, patch domain.DocumentPatch) (domain.Document, error) {
	if err := patch.Validate(); err != nil {
		return domain.Document{}, err
	}
	var out domain.Document
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		// When the update sets attributes, validate them against the document's type schema
		// (D-DocumentAttrSchema): resolve the document's type, then its schema.
		if patch.Attributes != nil {
			doc, err := repo.GetDocument(ctx, id)
			if err != nil {
				return err
			}
			t, err := repo.GetDocumentType(ctx, doc.TypeID)
			if err != nil {
				return err
			}
			if err := domain.ValidateAttributes(t.AttrSchema, *patch.Attributes); err != nil {
				return err
			}
		}
		updated, err := repo.UpdateDocument(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "document.update", targetDocument, id,
			map[string]any{"id": id, "status": string(updated.Status)})
	})
	return out, err
}

func (s *Service) DeleteDocument(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.SoftDeleteDocument(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "document.delete", targetDocument, id, map[string]any{"id": id, "deleted": true})
	})
}

func (s *Service) ListPersonDocuments(ctx context.Context, personID string, pageSize int, pageToken string) (DocumentPage, error) {
	size := resolvePageSize(pageSize)
	after, err := decodeCursor(pageToken)
	if err != nil {
		return DocumentPage{}, err
	}
	docs, err := s.newRepo(s.pool).ListDocumentsByPerson(ctx, personID, after, size+1)
	if err != nil {
		return DocumentPage{}, err
	}
	if len(docs) > size {
		return DocumentPage{Documents: docs[:size], NextPageToken: encodeCursor(docs[size-1].ID)}, nil
	}
	return DocumentPage{Documents: docs}, nil
}

// ---------------------------------------------------------------- schemes

func (s *Service) CreateScheme(ctx context.Context, sc domain.PersonalCodeScheme) (domain.PersonalCodeScheme, error) {
	if err := sc.Validate(); err != nil {
		return domain.PersonalCodeScheme{}, err
	}
	var out domain.PersonalCodeScheme
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertScheme(ctx, sc)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "personal-code-scheme.create", targetScheme, created.Code,
			map[string]any{"code": created.Code, "country": created.CountryISO, "category": created.GenericCategory})
	})
	return out, err
}

func (s *Service) UpdateScheme(ctx context.Context, code string, patch domain.PersonalCodeSchemePatch) (domain.PersonalCodeScheme, error) {
	if err := patch.Validate(); err != nil {
		return domain.PersonalCodeScheme{}, err
	}
	var out domain.PersonalCodeScheme
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateScheme(ctx, code, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "personal-code-scheme.update", targetScheme, code,
			map[string]any{"code": code, "status": string(updated.Status)})
	})
	return out, err
}

func (s *Service) ListSchemes(ctx context.Context, country, category string) ([]domain.PersonalCodeScheme, error) {
	return s.newRepo(s.pool).ListSchemes(ctx, country, category)
}

// ---------------------------------------------------------------- personal codes

// AttachPersonalCode validates the plaintext value against its scheme (D-PersonalCodes), envelope-
// encrypts it (D-CryptoProvider), and stores ciphertext + blind index, recording the action. The
// returned PersonalCode carries the just-validated plaintext value. The scheme is loaded first (its
// validation_regex is the fallback behind a compiled validator); an unknown scheme is ErrUnknownScheme.
// A failing validator is ErrPersonalCodeInvalid; a duplicate (scheme, value) is ErrPersonalCodeDuplicate.
func (s *Service) AttachPersonalCode(ctx context.Context, personID, schemeCode, value string) (domain.PersonalCode, error) {
	scheme, err := s.newRepo(s.pool).GetScheme(ctx, schemeCode)
	if err != nil {
		if errors.Is(err, domain.ErrSchemeNotFound) {
			return domain.PersonalCode{}, domain.ErrUnknownScheme
		}
		return domain.PersonalCode{}, err
	}
	normalized, err := s.validateValue(ctx, scheme, value)
	if err != nil {
		return domain.PersonalCode{}, err
	}
	sealed, blind, err := s.encrypt(ctx, normalized)
	if err != nil {
		return domain.PersonalCode{}, err
	}

	var stored domain.StoredPersonalCode
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertPersonalCode(ctx, domain.StoredPersonalCode{
			PersonID:        personID,
			SchemeCode:      schemeCode,
			ValueCiphertext: sealed.Ciphertext,
			WrappedDEK:      sealed.WrappedDEK,
			KeyRef:          sealed.KeyRef,
			ValueBlindIndex: blind,
		})
		if err != nil {
			return err
		}
		stored = created
		return s.record(ctx, tx, "personal-code.create", targetPersonalCode, created.ID,
			map[string]any{"id": created.ID, "personId": created.PersonID, "scheme": created.SchemeCode})
	})
	if err != nil {
		return domain.PersonalCode{}, err
	}
	return toPlainCode(stored, normalized), nil
}

func (s *Service) ListPersonPersonalCodes(ctx context.Context, personID string) ([]domain.PersonalCode, error) {
	stored, err := s.newRepo(s.pool).ListPersonalCodesByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PersonalCode, 0, len(stored))
	for _, c := range stored {
		value, err := s.decrypt(ctx, c)
		if err != nil {
			return nil, err
		}
		out = append(out, toPlainCode(c, value))
	}
	return out, nil
}

// UpdatePersonalCode re-validates + re-encrypts a new value and/or flips status. When value is nil the
// stored ciphertext is preserved; the scheme is loaded only when the value changes.
func (s *Service) UpdatePersonalCode(ctx context.Context, id string, value *string, status *string) (domain.PersonalCode, error) {
	if status != nil && !validDocStatus(*status) {
		return domain.PersonalCode{}, domain.ErrPersonalCodeInvalid
	}
	current, err := s.newRepo(s.pool).GetPersonalCode(ctx, id)
	if err != nil {
		return domain.PersonalCode{}, err
	}

	upd := domain.StoredPersonalCodeUpdate{
		ValueCiphertext: current.ValueCiphertext,
		WrappedDEK:      current.WrappedDEK,
		KeyRef:          current.KeyRef,
		ValueBlindIndex: current.ValueBlindIndex,
		Status:          current.Status,
	}
	plain := ""
	if value != nil {
		scheme, err := s.newRepo(s.pool).GetScheme(ctx, current.SchemeCode)
		if err != nil {
			if errors.Is(err, domain.ErrSchemeNotFound) {
				return domain.PersonalCode{}, domain.ErrUnknownScheme
			}
			return domain.PersonalCode{}, err
		}
		normalized, err := s.validateValue(ctx, scheme, *value)
		if err != nil {
			return domain.PersonalCode{}, err
		}
		sealed, blind, err := s.encrypt(ctx, normalized)
		if err != nil {
			return domain.PersonalCode{}, err
		}
		upd.ValueCiphertext, upd.WrappedDEK, upd.KeyRef, upd.ValueBlindIndex = sealed.Ciphertext, sealed.WrappedDEK, sealed.KeyRef, blind
		plain = normalized
	}
	if status != nil {
		upd.Status = domain.DocumentStatus(*status)
	}

	var stored domain.StoredPersonalCode
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdatePersonalCode(ctx, id, upd)
		if err != nil {
			return err
		}
		stored = updated
		return s.record(ctx, tx, "personal-code.update", targetPersonalCode, id,
			map[string]any{"id": id, "status": string(updated.Status), "valueChanged": value != nil})
	})
	if err != nil {
		return domain.PersonalCode{}, err
	}
	if plain == "" {
		// value unchanged — decrypt the stored ciphertext for the response.
		plain, err = s.decrypt(ctx, stored)
		if err != nil {
			return domain.PersonalCode{}, err
		}
	}
	return toPlainCode(stored, plain), nil
}

func (s *Service) DeletePersonalCode(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.SoftDeletePersonalCode(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "personal-code.delete", targetPersonalCode, id, map[string]any{"id": id, "deleted": true})
	})
}

// ErasePersonRecords is the person-purge erasure path (D-Documents): it NULLs a person's document
// number/issuer/attributes and CRYPTO-ERASES their personal codes (drop the wrapped DEK + ciphertext),
// keeping row ids as tombstones, all in one transaction with a `system` audit row correlated to the
// originating person.purge by request_id. The PersonPurged event subscriber that triggers this is
// deferred until the event bus lands (open seam); the mechanism is exercised directly today.
func (s *Service) ErasePersonRecords(ctx context.Context, personID string) (docs, codes int64, err error) {
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		d, err := repo.ErasePersonDocuments(ctx, personID)
		if err != nil {
			return err
		}
		c, err := repo.CryptoErasePersonCodes(ctx, personID)
		if err != nil {
			return err
		}
		docs, codes = d, c
		return s.recordErase(ctx, tx, personID, d, c)
	})
	return docs, codes, err
}

// ---------------------------------------------------------------- crypto + validation helpers

// validateValue applies the scheme's validator precedence (D-PersonalCodes), logging the
// accept-with-warning case. It returns the normalized plaintext to encrypt.
func (s *Service) validateValue(ctx context.Context, scheme domain.PersonalCodeScheme, value string) (string, error) {
	res, err := s.codes.Validate(scheme.Code, value, scheme.ValidationRegex)
	if err != nil {
		if errors.Is(err, personalcode.ErrInvalid) {
			return "", errors.Join(domain.ErrPersonalCodeInvalid, err)
		}
		return "", err
	}
	if res.Outcome == personalcode.OutcomeAcceptedWarn {
		svc1log.FromContext(ctx).Warn("personal code accepted without validation (no compiled validator or catalog regex for scheme)",
			svc1log.SafeParam("scheme", scheme.Code))
	}
	return res.Normalized, nil
}

// encrypt seals the normalized plaintext and computes its blind index.
func (s *Service) encrypt(ctx context.Context, normalized string) (crypto.Sealed, []byte, error) {
	sealed, err := s.cipher.Seal(ctx, []byte(normalized))
	if err != nil {
		return crypto.Sealed{}, nil, err
	}
	return sealed, s.cipher.BlindIndex([]byte(normalized)), nil
}

// decrypt recovers the plaintext value; a crypto-erased row (no ciphertext) yields "" (the tombstone).
func (s *Service) decrypt(ctx context.Context, c domain.StoredPersonalCode) (string, error) {
	if len(c.ValueCiphertext) == 0 || len(c.WrappedDEK) == 0 {
		return "", nil
	}
	plain, err := s.cipher.Open(ctx, crypto.Sealed{Ciphertext: c.ValueCiphertext, WrappedDEK: c.WrappedDEK, KeyRef: c.KeyRef})
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func toPlainCode(c domain.StoredPersonalCode, value string) domain.PersonalCode {
	return domain.PersonalCode{
		ID:         c.ID,
		PersonID:   c.PersonID,
		SchemeCode: c.SchemeCode,
		Value:      value,
		Status:     c.Status,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

func validDocStatus(s string) bool {
	switch domain.DocumentStatus(s) {
	case domain.DocumentActive, domain.DocumentSuperseded, domain.DocumentRevoked:
		return true
	}
	return false
}

// ---------------------------------------------------------------- tx + audit plumbing

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record mints an Action RID in the caller's transaction and writes the audit row on it, so the audit
// entry commits iff the change commits (D-Audit). The actor is the interim system actor. PII discipline:
// `after` carries only ids/keys — never a document number, issuer, or personal-code value.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx, action)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// recordErase writes the person-purge erasure audit row under the event-subscriber subsystem,
// correlated to the originating person.purge by request_id.
func (s *Service) recordErase(ctx context.Context, tx pgx.Tx, personID string, docs, codes int64) error {
	rid, err := mintActionRID(ctx, tx, "document.person.erase")
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  eraseSubsystem,
		Action:     "document.person.erase",
		TargetType: targetPerson,
		TargetID:   personID,
		RequestID:  requestID(ctx),
		After:      toJSON(map[string]any{"personId": personID, "documentsErased": docs, "codesCryptoErased": codes}),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID mints an Action RID (document service=10, kind=action=3, generic action type=0).
// The specific action name is recorded separately in audit_log.action (D-Audit).
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	_ = action
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(10, 3, 0)").Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

// requestID is the correlation key shared with logs/metrics/traces: the request's trace id, with a
// generated fallback for out-of-request callers (e.g. integration tests).
func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}

func resolvePageSize(requested int) int {
	if requested <= 0 {
		return DefaultPageSize
	}
	if requested > MaxPageSize {
		return MaxPageSize
	}
	return requested
}

func encodeCursor(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func decodeCursor(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
