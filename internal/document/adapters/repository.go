// Package adapters implements the document domain ports against infrastructure: the pgx/sqlc
// repository over the oikumenea.document_* tables. It depends on the database, never the reverse
// (overview.md). Generated sqlc code lives in the documentsql subpackage and is never hand-edited. The
// adapter stays crypto-agnostic — it stores/loads the personal-code value's opaque ciphertext + wrapped
// DEK + key ref + blind index as bytes; encryption lives in the application layer (D-CryptoProvider).
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/document/adapters/documentsql"
	"github.com/olegamysk/go-oikumenea/internal/document/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX —
// the pool for reads, or a caller-supplied transaction so a write and its audit row commit together
// (D-Audit).
type Repository struct {
	q *documentsql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: documentsql.New(conn)}
}

var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- document types

func (r *Repository) InsertDocumentType(ctx context.Context, t domain.DocumentType) (domain.DocumentType, error) {
	row, err := r.q.InsertDocumentType(ctx, documentsql.InsertDocumentTypeParams{
		Code:       t.Code,
		Name:       t.Name,
		AttrSchema: []byte(t.AttrSchema),
		SortOrder:  int4Ptr(t.SortOrder),
	})
	if err != nil {
		return domain.DocumentType{}, mapWriteErr(err)
	}
	return toDocumentType(row), nil
}

func (r *Repository) GetDocumentType(ctx context.Context, id string) (domain.DocumentType, error) {
	row, err := r.q.GetDocumentType(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DocumentType{}, domain.ErrDocumentTypeNotFound
		}
		return domain.DocumentType{}, err
	}
	return toDocumentType(row), nil
}

func (r *Repository) UpdateDocumentType(ctx context.Context, id string, patch domain.DocumentTypePatch) (domain.DocumentType, error) {
	row, err := r.q.UpdateDocumentType(ctx, documentsql.UpdateDocumentTypeParams{
		Name:       textPtr(patch.Name),
		AttrSchema: rawPtr(patch.AttrSchema),
		Status:     textPtr(patch.Status),
		SortOrder:  int4Ptr(patch.SortOrder),
		ID:         id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DocumentType{}, domain.ErrDocumentTypeNotFound
		}
		return domain.DocumentType{}, mapWriteErr(err)
	}
	return toDocumentType(row), nil
}

func (r *Repository) ListDocumentTypes(ctx context.Context) ([]domain.DocumentType, error) {
	rows, err := r.q.ListDocumentTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DocumentType, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDocumentType(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- documents

func (r *Repository) InsertDocument(ctx context.Context, d domain.Document) (domain.Document, error) {
	row, err := r.q.InsertDocument(ctx, documentsql.InsertDocumentParams{
		PersonID:       d.PersonID,
		TypeID:         d.TypeID,
		Number:         text(d.Number),
		Issuer:         text(d.Issuer),
		IssuingCountry: text(d.IssuingCountry),
		IssuedOn:       dateText(d.IssuedOn),
		ExpiresOn:      dateText(d.ExpiresOn),
		Attributes:     d.Attributes,
	})
	if err != nil {
		return domain.Document{}, mapWriteErr(err)
	}
	return toDocument(row), nil
}

func (r *Repository) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	row, err := r.q.GetDocument(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Document{}, domain.ErrDocumentNotFound
		}
		return domain.Document{}, err
	}
	return toDocument(row), nil
}

func (r *Repository) UpdateDocument(ctx context.Context, id string, patch domain.DocumentPatch) (domain.Document, error) {
	row, err := r.q.UpdateDocument(ctx, documentsql.UpdateDocumentParams{
		Number:         textPtr(patch.Number),
		Issuer:         textPtr(patch.Issuer),
		IssuingCountry: textPtr(patch.IssuingCountry),
		IssuedOn:       datePtr(patch.IssuedOn),
		ExpiresOn:      datePtr(patch.ExpiresOn),
		Attributes:     rawPtr(patch.Attributes),
		Status:         textPtr(patch.Status),
		ID:             id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Document{}, domain.ErrDocumentNotFound
		}
		return domain.Document{}, mapWriteErr(err)
	}
	return toDocument(row), nil
}

func (r *Repository) SoftDeleteDocument(ctx context.Context, id string) error {
	n, err := r.q.SoftDeleteDocument(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}

func (r *Repository) ListDocumentsByPerson(ctx context.Context, personID, after string, limit int) ([]domain.Document, error) {
	rows, err := r.q.ListDocumentsByPerson(ctx, documentsql.ListDocumentsByPersonParams{
		PersonID: personID, After: after, Lim: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Document, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDocument(row))
	}
	return out, nil
}

func (r *Repository) ErasePersonDocuments(ctx context.Context, personID string) (int64, error) {
	return r.q.ErasePersonDocuments(ctx, personID)
}

// ---------------------------------------------------------------- schemes

func (r *Repository) InsertScheme(ctx context.Context, s domain.PersonalCodeScheme) (domain.PersonalCodeScheme, error) {
	row, err := r.q.InsertScheme(ctx, documentsql.InsertSchemeParams{
		Code:            s.Code,
		CountryIso:      text(s.CountryISO),
		GenericCategory: s.GenericCategory,
		Name:            s.Name,
		ValidationRegex: text(s.ValidationRegex),
		SortOrder:       int4Ptr(s.SortOrder),
	})
	if err != nil {
		return domain.PersonalCodeScheme{}, mapWriteErr(err)
	}
	return toScheme(row), nil
}

func (r *Repository) GetScheme(ctx context.Context, code string) (domain.PersonalCodeScheme, error) {
	row, err := r.q.GetScheme(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PersonalCodeScheme{}, domain.ErrSchemeNotFound
		}
		return domain.PersonalCodeScheme{}, err
	}
	return toScheme(row), nil
}

func (r *Repository) UpdateScheme(ctx context.Context, code string, patch domain.PersonalCodeSchemePatch) (domain.PersonalCodeScheme, error) {
	row, err := r.q.UpdateScheme(ctx, documentsql.UpdateSchemeParams{
		CountryIso:      textPtr(patch.CountryISO),
		GenericCategory: textPtr(patch.GenericCategory),
		Name:            textPtr(patch.Name),
		ValidationRegex: textPtr(patch.ValidationRegex),
		Status:          textPtr(patch.Status),
		SortOrder:       int4Ptr(patch.SortOrder),
		Code:            code,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PersonalCodeScheme{}, domain.ErrSchemeNotFound
		}
		return domain.PersonalCodeScheme{}, mapWriteErr(err)
	}
	return toScheme(row), nil
}

func (r *Repository) ListSchemes(ctx context.Context, country, category string) ([]domain.PersonalCodeScheme, error) {
	rows, err := r.q.ListSchemes(ctx, documentsql.ListSchemesParams{Country: country, Category: category})
	if err != nil {
		return nil, err
	}
	out := make([]domain.PersonalCodeScheme, 0, len(rows))
	for _, row := range rows {
		out = append(out, toScheme(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- personal codes

func (r *Repository) InsertPersonalCode(ctx context.Context, c domain.StoredPersonalCode) (domain.StoredPersonalCode, error) {
	row, err := r.q.InsertPersonalCode(ctx, documentsql.InsertPersonalCodeParams{
		PersonID:        c.PersonID,
		SchemeCode:      c.SchemeCode,
		ValueCiphertext: c.ValueCiphertext,
		WrappedDek:      c.WrappedDEK,
		KeyRef:          c.KeyRef,
		ValueBlindIndex: c.ValueBlindIndex,
	})
	if err != nil {
		return domain.StoredPersonalCode{}, mapWriteErr(err)
	}
	return toStoredCode(row), nil
}

func (r *Repository) GetPersonalCode(ctx context.Context, id string) (domain.StoredPersonalCode, error) {
	row, err := r.q.GetPersonalCode(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredPersonalCode{}, domain.ErrPersonalCodeNotFound
		}
		return domain.StoredPersonalCode{}, err
	}
	return toStoredCode(row), nil
}

func (r *Repository) UpdatePersonalCode(ctx context.Context, id string, upd domain.StoredPersonalCodeUpdate) (domain.StoredPersonalCode, error) {
	row, err := r.q.UpdatePersonalCode(ctx, documentsql.UpdatePersonalCodeParams{
		ValueCiphertext: upd.ValueCiphertext,
		WrappedDek:      upd.WrappedDEK,
		KeyRef:          upd.KeyRef,
		ValueBlindIndex: upd.ValueBlindIndex,
		Status:          string(upd.Status),
		ID:              id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredPersonalCode{}, domain.ErrPersonalCodeNotFound
		}
		return domain.StoredPersonalCode{}, mapWriteErr(err)
	}
	return toStoredCode(row), nil
}

func (r *Repository) SoftDeletePersonalCode(ctx context.Context, id string) error {
	n, err := r.q.SoftDeletePersonalCode(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrPersonalCodeNotFound
	}
	return nil
}

func (r *Repository) ListPersonalCodesByPerson(ctx context.Context, personID string) ([]domain.StoredPersonalCode, error) {
	rows, err := r.q.ListPersonalCodesByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StoredPersonalCode, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStoredCode(row))
	}
	return out, nil
}

func (r *Repository) CryptoErasePersonCodes(ctx context.Context, personID string) (int64, error) {
	return r.q.CryptoErasePersonCodes(ctx, personID)
}

// ---------------------------------------------------------------- mapping helpers

func toDocumentType(r documentsql.OikumeneaDocumentDocumentType) domain.DocumentType {
	return domain.DocumentType{
		ID:         r.ID,
		Code:       r.Code,
		Name:       r.Name,
		AttrSchema: rawOrNil(r.AttrSchema),
		Status:     domain.CatalogStatus(r.Status),
		SortOrder:  int4Val(r.SortOrder),
		CreatedAt:  r.CreatedAt.Time,
		UpdatedAt:  r.UpdatedAt.Time,
	}
}

// rawOrNil maps a stored attr_schema column to json.RawMessage, keeping nil as nil (no schema).
func rawOrNil(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

func toDocument(r documentsql.OikumeneaDocumentDocument) domain.Document {
	return domain.Document{
		ID:             r.ID,
		PersonID:       r.PersonID,
		TypeID:         r.TypeID,
		Number:         r.Number.String,
		Issuer:         r.Issuer.String,
		IssuingCountry: r.IssuingCountry.String,
		IssuedOn:       dateStr(r.IssuedOn),
		ExpiresOn:      dateStr(r.ExpiresOn),
		Attributes:     r.Attributes,
		Status:         domain.DocumentStatus(r.Status),
		CreatedAt:      r.CreatedAt.Time,
		UpdatedAt:      r.UpdatedAt.Time,
	}
}

func toScheme(r documentsql.OikumeneaDocumentPersonalCodeScheme) domain.PersonalCodeScheme {
	return domain.PersonalCodeScheme{
		Code:            r.Code,
		CountryISO:      r.CountryIso.String,
		GenericCategory: r.GenericCategory,
		Name:            r.Name,
		ValidationRegex: r.ValidationRegex.String,
		Status:          domain.CatalogStatus(r.Status),
		SortOrder:       int4Val(r.SortOrder),
		CreatedAt:       r.CreatedAt.Time,
		UpdatedAt:       r.UpdatedAt.Time,
	}
}

func toStoredCode(r documentsql.OikumeneaDocumentPersonalCode) domain.StoredPersonalCode {
	return domain.StoredPersonalCode{
		ID:              r.ID,
		PersonID:        r.PersonID,
		SchemeCode:      r.SchemeCode,
		ValueCiphertext: r.ValueCiphertext,
		WrappedDEK:      r.WrappedDek,
		KeyRef:          r.KeyRef,
		ValueBlindIndex: r.ValueBlindIndex,
		Status:          domain.DocumentStatus(r.Status),
		CreatedAt:       r.CreatedAt.Time,
		UpdatedAt:       r.UpdatedAt.Time,
	}
}

// mapWriteErr translates Postgres constraint violations into the module's domain sentinels. The unique
// indexes distinguish the duplicate-paper, duplicate-code, and catalog-code-clash cases; FK violations
// name the offending reference (person / type / scheme / country) so the transport returns a precise
// error.
func mapWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	name := pgErr.ConstraintName
	switch pgErr.Code {
	case "23505": // unique_violation
		switch {
		case strings.Contains(name, "person_type_number"):
			return domain.ErrDocumentConflict
		case strings.Contains(name, "scheme_value"):
			return domain.ErrPersonalCodeDuplicate
		case strings.Contains(name, "document_types_code") || strings.Contains(name, "document_document_types_pkey"):
			return domain.ErrDocumentTypeConflict
		case strings.Contains(name, "personal_code_schemes_pkey"):
			return domain.ErrSchemeConflict
		}
	case "23503": // foreign_key_violation
		switch {
		case strings.Contains(name, "person"):
			return domain.ErrUnknownPerson
		case strings.Contains(name, "type"):
			return domain.ErrUnknownType
		case strings.Contains(name, "scheme"):
			return domain.ErrUnknownScheme
		case strings.Contains(name, "country"):
			return domain.ErrUnknownCountry
		}
	}
	return err
}

func text(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// textPtr maps a patch pointer: nil leaves the column unchanged (NULL narg → COALESCE keeps it); a
// non-nil pointer sets the column.
func textPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func int4Ptr(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

func int4Val(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int32)
	return &n
}

// dateText maps an ISO-8601 date string to a pgtype.Date; "" or an unparsable value yields NULL (the
// domain validates the format before this is reached).
func dateText(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse(domain.ISODate, s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func datePtr(p *string) pgtype.Date {
	if p == nil {
		return pgtype.Date{}
	}
	return dateText(*p)
}

func dateStr(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(domain.ISODate)
}

// rawPtr maps an optional JSON patch: nil leaves attributes unchanged (NULL narg), a non-nil pointer
// sets them (an explicit empty object clears extras).
func rawPtr(p *json.RawMessage) []byte {
	if p == nil {
		return nil
	}
	return *p
}
