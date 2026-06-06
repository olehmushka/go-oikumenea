// Package transport implements the document module's generated Conjure DocumentService interface: it
// translates the wire contract to/from the application service, assembles localized type/scheme `name`
// maps via the localization service (cross-module query — overview.md), and maps domain errors to
// Conjure SerializableErrors (D-Conjure). Generated code in internal/conjure is never hand-edited.
//
// A type/scheme `name` is a translatable label returned as a `locale -> text` map; documents and
// personal codes reference their holder/type/scheme by id and carry verbatim/decrypted data (no maps).
//
// Authorization: documents and personal codes are scoped THROUGH THE HOLDER (D-PersonReadScope) and
// pass the shadow gate; none of these endpoints carries a unit, so they use the coarse "holds the
// permission anywhere (or is instance admin)" form pending the load-then-check + shadow-gate tightening
// (the shared follow-up across person/membership/document holder-scoped reads). Catalog management uses
// instance-scope permissions, which RequireAnywhere satisfies only from an instance-admin grant.
package transport

import (
	"context"
	"encoding/json"
	"errors"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	documentapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/document"
	"github.com/olegamysk/go-oikumenea/internal/document/application"
	"github.com/olegamysk/go-oikumenea/internal/document/domain"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// Localization entity-type keys for the translatable catalog names (D-i18n).
const (
	typeEntity   = "document_type"
	schemeEntity = "personal_code_scheme"
)

// Service adapts *application.Service to the generated documentapi.DocumentService interface, holding
// the localization service for `name` map assembly and the PEP enforcer for authorization.
type Service struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the document application service, the localization
// service, and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, loc: loc, pep: enforcer}
}

var _ documentapi.DocumentService = Service{}

// ---------------------------------------------------------------- documents (papers)

func (s Service) AttachDocument(ctx context.Context, token bearertoken.Token, personID string, req documentapi.CreateDocumentRequest) (documentapi.Document, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDocumentCreate)); err != nil {
		return documentapi.Document{}, err
	}
	created, err := s.app.AttachDocument(ctx, domain.Document{
		PersonID:       personID,
		TypeID:         req.TypeId,
		Number:         derefOr(req.Number, ""),
		Issuer:         derefOr(req.Issuer, ""),
		IssuingCountry: derefOr(req.IssuingCountry, ""),
		IssuedOn:       derefOr(req.IssuedOn, ""),
		ExpiresOn:      derefOr(req.ExpiresOn, ""),
		Attributes:     attrToBytes(req.Attributes),
	})
	if err != nil {
		return documentapi.Document{}, s.mapError(ctx, err, errCtx{personID: personID})
	}
	return toAPIDocument(created), nil
}

func (s Service) ListPersonDocuments(ctx context.Context, token bearertoken.Token, personID string, pageSize *int, pageToken *string) (documentapi.DocumentPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDocumentRead)); err != nil {
		return documentapi.DocumentPage{}, err
	}
	page, err := s.app.ListPersonDocuments(ctx, personID, derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return documentapi.DocumentPage{}, s.mapError(ctx, err, errCtx{personID: personID})
	}
	docs := make([]documentapi.Document, 0, len(page.Documents))
	for _, d := range page.Documents {
		docs = append(docs, toAPIDocument(d))
	}
	return documentapi.DocumentPage{Documents: docs, NextPageToken: tokenPtr(page.NextPageToken)}, nil
}

func (s Service) GetDocument(ctx context.Context, token bearertoken.Token, documentID string) (documentapi.Document, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDocumentRead)); err != nil {
		return documentapi.Document{}, err
	}
	d, err := s.app.GetDocument(ctx, documentID)
	if err != nil {
		return documentapi.Document{}, s.mapError(ctx, err, errCtx{documentID: documentID})
	}
	return toAPIDocument(d), nil
}

func (s Service) UpdateDocument(ctx context.Context, token bearertoken.Token, documentID string, req documentapi.UpdateDocumentRequest) (documentapi.Document, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDocumentUpdate)); err != nil {
		return documentapi.Document{}, err
	}
	updated, err := s.app.UpdateDocument(ctx, documentID, domain.DocumentPatch{
		Number:         req.Number,
		Issuer:         req.Issuer,
		IssuingCountry: req.IssuingCountry,
		IssuedOn:       req.IssuedOn,
		ExpiresOn:      req.ExpiresOn,
		Attributes:     attrToRawPtr(req.Attributes),
		Status:         req.Status,
	})
	if err != nil {
		return documentapi.Document{}, s.mapError(ctx, err, errCtx{documentID: documentID})
	}
	return toAPIDocument(updated), nil
}

func (s Service) DeleteDocument(ctx context.Context, token bearertoken.Token, documentID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDocumentDelete)); err != nil {
		return err
	}
	if err := s.app.DeleteDocument(ctx, documentID); err != nil {
		return s.mapError(ctx, err, errCtx{documentID: documentID})
	}
	return nil
}

// ---------------------------------------------------------------- document types

func (s Service) ListDocumentTypes(ctx context.Context, token bearertoken.Token) ([]documentapi.DocumentType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDocumentTypeRead)); err != nil {
		return nil, err
	}
	types, err := s.app.ListDocumentTypes(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, errCtx{})
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.ID] = t.Name
	}
	names, err := s.loc.NamesByID(ctx, typeEntity, defaults)
	if err != nil {
		return nil, s.mapError(ctx, err, errCtx{})
	}
	out := make([]documentapi.DocumentType, 0, len(types))
	for _, t := range types {
		out = append(out, toAPIDocumentType(t, names[t.ID]))
	}
	return out, nil
}

func (s Service) CreateDocumentType(ctx context.Context, token bearertoken.Token, req documentapi.CreateDocumentTypeRequest) (documentapi.DocumentType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDocumentTypeManage)); err != nil {
		return documentapi.DocumentType{}, err
	}
	created, err := s.app.CreateDocumentType(ctx, domain.DocumentType{Code: req.Code, Name: req.Name, SortOrder: req.SortOrder})
	if err != nil {
		return documentapi.DocumentType{}, s.mapError(ctx, err, errCtx{})
	}
	return s.documentTypeToAPI(ctx, created)
}

func (s Service) UpdateDocumentType(ctx context.Context, token bearertoken.Token, typeID string, req documentapi.UpdateDocumentTypeRequest) (documentapi.DocumentType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDocumentTypeManage)); err != nil {
		return documentapi.DocumentType{}, err
	}
	updated, err := s.app.UpdateDocumentType(ctx, typeID, domain.DocumentTypePatch{Name: req.Name, Status: req.Status, SortOrder: req.SortOrder})
	if err != nil {
		return documentapi.DocumentType{}, s.mapError(ctx, err, errCtx{typeID: typeID})
	}
	return s.documentTypeToAPI(ctx, updated)
}

// ---------------------------------------------------------------- personal codes

func (s Service) AttachPersonalCode(ctx context.Context, token bearertoken.Token, personID string, req documentapi.CreatePersonalCodeRequest) (documentapi.PersonalCode, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonalCodeCreate)); err != nil {
		return documentapi.PersonalCode{}, err
	}
	created, err := s.app.AttachPersonalCode(ctx, personID, req.SchemeCode, req.Value)
	if err != nil {
		return documentapi.PersonalCode{}, s.mapError(ctx, err, errCtx{personID: personID})
	}
	return toAPIPersonalCode(created), nil
}

func (s Service) ListPersonPersonalCodes(ctx context.Context, token bearertoken.Token, personID string) ([]documentapi.PersonalCode, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonalCodeRead)); err != nil {
		return nil, err
	}
	codes, err := s.app.ListPersonPersonalCodes(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, errCtx{personID: personID})
	}
	out := make([]documentapi.PersonalCode, 0, len(codes))
	for _, c := range codes {
		out = append(out, toAPIPersonalCode(c))
	}
	return out, nil
}

func (s Service) UpdatePersonalCode(ctx context.Context, token bearertoken.Token, codeID string, req documentapi.UpdatePersonalCodeRequest) (documentapi.PersonalCode, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonalCodeUpdate)); err != nil {
		return documentapi.PersonalCode{}, err
	}
	updated, err := s.app.UpdatePersonalCode(ctx, codeID, req.Value, req.Status)
	if err != nil {
		return documentapi.PersonalCode{}, s.mapError(ctx, err, errCtx{codeID: codeID})
	}
	return toAPIPersonalCode(updated), nil
}

func (s Service) DeletePersonalCode(ctx context.Context, token bearertoken.Token, codeID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonalCodeDelete)); err != nil {
		return err
	}
	if err := s.app.DeletePersonalCode(ctx, codeID); err != nil {
		return s.mapError(ctx, err, errCtx{codeID: codeID})
	}
	return nil
}

// ---------------------------------------------------------------- personal-code schemes

func (s Service) ListPersonalCodeSchemes(ctx context.Context, token bearertoken.Token, country *string, category *string) ([]documentapi.PersonalCodeScheme, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonalCodeSchemeRead)); err != nil {
		return nil, err
	}
	schemes, err := s.app.ListSchemes(ctx, derefOr(country, ""), derefOr(category, ""))
	if err != nil {
		return nil, s.mapError(ctx, err, errCtx{})
	}
	defaults := make(map[string]string, len(schemes))
	for _, sc := range schemes {
		defaults[sc.Code] = sc.Name
	}
	names, err := s.loc.NamesByID(ctx, schemeEntity, defaults)
	if err != nil {
		return nil, s.mapError(ctx, err, errCtx{})
	}
	out := make([]documentapi.PersonalCodeScheme, 0, len(schemes))
	for _, sc := range schemes {
		out = append(out, toAPIScheme(sc, names[sc.Code]))
	}
	return out, nil
}

func (s Service) CreatePersonalCodeScheme(ctx context.Context, token bearertoken.Token, req documentapi.CreatePersonalCodeSchemeRequest) (documentapi.PersonalCodeScheme, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonalCodeSchemeManage)); err != nil {
		return documentapi.PersonalCodeScheme{}, err
	}
	created, err := s.app.CreateScheme(ctx, domain.PersonalCodeScheme{
		Code:            req.Code,
		CountryISO:      derefOr(req.CountryIso, ""),
		GenericCategory: req.GenericCategory,
		Name:            req.Name,
		ValidationRegex: derefOr(req.ValidationRegex, ""),
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		return documentapi.PersonalCodeScheme{}, s.mapError(ctx, err, errCtx{schemeCode: req.Code})
	}
	return s.schemeToAPI(ctx, created)
}

func (s Service) UpdatePersonalCodeScheme(ctx context.Context, token bearertoken.Token, schemeCode string, req documentapi.UpdatePersonalCodeSchemeRequest) (documentapi.PersonalCodeScheme, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonalCodeSchemeManage)); err != nil {
		return documentapi.PersonalCodeScheme{}, err
	}
	updated, err := s.app.UpdateScheme(ctx, schemeCode, domain.PersonalCodeSchemePatch{
		CountryISO:      req.CountryIso,
		GenericCategory: req.GenericCategory,
		Name:            req.Name,
		ValidationRegex: req.ValidationRegex,
		Status:          req.Status,
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		return documentapi.PersonalCodeScheme{}, s.mapError(ctx, err, errCtx{schemeCode: schemeCode})
	}
	return s.schemeToAPI(ctx, updated)
}

// ---------------------------------------------------------------- response assembly

func (s Service) documentTypeToAPI(ctx context.Context, t domain.DocumentType) (documentapi.DocumentType, error) {
	names, err := s.loc.NamesByID(ctx, typeEntity, map[string]string{t.ID: t.Name})
	if err != nil {
		return documentapi.DocumentType{}, err
	}
	return toAPIDocumentType(t, names[t.ID]), nil
}

func (s Service) schemeToAPI(ctx context.Context, sc domain.PersonalCodeScheme) (documentapi.PersonalCodeScheme, error) {
	names, err := s.loc.NamesByID(ctx, schemeEntity, map[string]string{sc.Code: sc.Name})
	if err != nil {
		return documentapi.PersonalCodeScheme{}, err
	}
	return toAPIScheme(sc, names[sc.Code]), nil
}

func toAPIDocumentType(t domain.DocumentType, name map[string]string) documentapi.DocumentType {
	return documentapi.DocumentType{
		Id:        t.ID,
		Code:      t.Code,
		Name:      name,
		Status:    string(t.Status),
		SortOrder: t.SortOrder,
		CreatedAt: datetime.DateTime(t.CreatedAt),
		UpdatedAt: datetime.DateTime(t.UpdatedAt),
	}
}

func toAPIDocument(d domain.Document) documentapi.Document {
	return documentapi.Document{
		Id:             d.ID,
		PersonId:       d.PersonID,
		TypeId:         d.TypeID,
		Number:         strPtrOrNil(d.Number),
		Issuer:         strPtrOrNil(d.Issuer),
		IssuingCountry: strPtrOrNil(d.IssuingCountry),
		IssuedOn:       strPtrOrNil(d.IssuedOn),
		ExpiresOn:      strPtrOrNil(d.ExpiresOn),
		Attributes:     attrFromBytes(d.Attributes),
		Status:         string(d.Status),
		CreatedAt:      datetime.DateTime(d.CreatedAt),
		UpdatedAt:      datetime.DateTime(d.UpdatedAt),
	}
}

func toAPIScheme(sc domain.PersonalCodeScheme, name map[string]string) documentapi.PersonalCodeScheme {
	return documentapi.PersonalCodeScheme{
		Code:            sc.Code,
		CountryIso:      strPtrOrNil(sc.CountryISO),
		GenericCategory: sc.GenericCategory,
		Name:            name,
		ValidationRegex: strPtrOrNil(sc.ValidationRegex),
		Status:          string(sc.Status),
		SortOrder:       sc.SortOrder,
		CreatedAt:       datetime.DateTime(sc.CreatedAt),
		UpdatedAt:       datetime.DateTime(sc.UpdatedAt),
	}
}

func toAPIPersonalCode(c domain.PersonalCode) documentapi.PersonalCode {
	return documentapi.PersonalCode{
		Id:         c.ID,
		PersonId:   c.PersonID,
		SchemeCode: c.SchemeCode,
		Value:      c.Value,
		Status:     string(c.Status),
		CreatedAt:  datetime.DateTime(c.CreatedAt),
		UpdatedAt:  datetime.DateTime(c.UpdatedAt),
	}
}

// ---------------------------------------------------------------- error mapping

// errCtx carries the identifiers an endpoint can name in a Conjure error (only the relevant fields are
// set per call).
type errCtx struct {
	personID   string
	documentID string
	typeID     string
	codeID     string
	schemeCode string
}

// mapError translates domain/application errors into the Conjure SerializableError contract.
func (s Service) mapError(ctx context.Context, err error, c errCtx) error {
	switch {
	case errors.Is(err, domain.ErrDocumentNotFound):
		return documentapi.NewDocumentNotFound(c.documentID)
	case errors.Is(err, domain.ErrDocumentConflict):
		return documentapi.NewDocumentConflict("the person already holds this (type, number)")
	case errors.Is(err, domain.ErrDocumentTypeNotFound):
		return documentapi.NewDocumentTypeNotFound(c.typeID)
	case errors.Is(err, domain.ErrDocumentTypeConflict):
		return documentapi.NewDocumentTypeConflict("a document type with this code already exists")
	case errors.Is(err, domain.ErrDocumentInvalid):
		return documentapi.NewDocumentInvalid(err.Error())
	case errors.Is(err, domain.ErrUnknownType):
		return documentapi.NewDocumentInvalid("document type does not exist")
	case errors.Is(err, domain.ErrPersonalCodeNotFound):
		return documentapi.NewPersonalCodeNotFound(c.codeID)
	case errors.Is(err, domain.ErrPersonalCodeDuplicate):
		return documentapi.NewPersonalCodeDuplicate("this (scheme, value) is already held")
	case errors.Is(err, domain.ErrPersonalCodeInvalid):
		return documentapi.NewPersonalCodeInvalid(err.Error())
	case errors.Is(err, domain.ErrUnknownScheme):
		return documentapi.NewPersonalCodeInvalid("personal-code scheme does not exist")
	case errors.Is(err, domain.ErrSchemeNotFound):
		return documentapi.NewPersonalCodeSchemeNotFound(c.schemeCode)
	case errors.Is(err, domain.ErrSchemeConflict):
		return documentapi.NewPersonalCodeSchemeConflict("a scheme with this code already exists")
	case errors.Is(err, domain.ErrUnknownPerson):
		return documentapi.NewDocumentInvalid("person does not exist")
	case errors.Is(err, domain.ErrUnknownCountry):
		return documentapi.NewDocumentInvalid("country does not exist")
	default:
		return werror.WrapWithContextParams(ctx, err, "document request failed")
	}
}

// ---------------------------------------------------------------- value helpers

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func tokenPtr(token string) *string {
	if token == "" {
		return nil
	}
	return &token
}

func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

// attrToBytes marshals the optional free-form attributes object to JSONB bytes (nil when absent).
func attrToBytes(a *interface{}) []byte {
	if a == nil {
		return nil
	}
	raw, err := json.Marshal(*a)
	if err != nil {
		return nil
	}
	return raw
}

// attrToRawPtr maps the optional attributes patch to *json.RawMessage: nil leaves attributes unchanged.
func attrToRawPtr(a *interface{}) *json.RawMessage {
	if a == nil {
		return nil
	}
	raw := json.RawMessage(attrToBytes(a))
	return &raw
}

// attrFromBytes unmarshals stored JSONB attributes back into the wire `any` (nil when empty).
func attrFromBytes(b []byte) *interface{} {
	if len(b) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return &v
}
