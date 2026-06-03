// Package transport implements the person module's generated Conjure PersonService interface: it
// translates the wire contract to/from the application service and maps domain errors to Conjure
// SerializableErrors (D-Conjure). Generated code in internal/conjure is never hand-edited.
//
// Person names are per-record data returned verbatim (a canonical display name + locale-tagged
// variants), NOT the instance localization store (D-i18n) — so unlike rank/tenant this transport
// assembles no locale->text maps. Authorization is intentionally not yet enforced: the endpoints
// declare `auth: header` so the bearer token is parsed, but the `person.*` checks + the read-scope
// rule (D-PersonReadScope) land once authorization (M7) + identity-federation (M8) do. The handlers
// receive the token and ignore it.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olegamysk/go-oikumenea/internal/person/application"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// Service adapts *application.Service to the generated personapi.PersonService interface.
type Service struct {
	app *application.Service
}

// NewService builds the transport adapter over the person application service.
func NewService(app *application.Service) Service { return Service{app: app} }

// compile-time assertion that the transport satisfies the generated server interface.
var _ personapi.PersonService = Service{}

// ---------------------------------------------------------------- persons

func (s Service) CreatePerson(ctx context.Context, _ bearertoken.Token, req personapi.CreatePersonRequest) (personapi.Person, error) {
	created, err := s.app.CreatePerson(ctx, domain.Person{
		Code: derefOr(req.Code, ""),
		Name: nameFromParts(req.DisplayName, req.Title, req.Given, req.Given2, req.Surname,
			req.SurnamePrefix, req.Surname2, req.Generation, req.Credentials, req.Preferred),
		Birthdate:      derefOr(req.Birthdate, ""),
		Sex:            derefOr(req.Sex, ""),
		CountryOfBirth: derefOr(req.CountryOfBirth, ""),
		Attributes:     attrToBytes(req.Attributes),
		RankID:         derefOr(req.RankId, ""),
	})
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, "")
	}
	return toAPIPerson(created), nil
}

func (s Service) GetPerson(ctx context.Context, _ bearertoken.Token, personID string) (personapi.Person, error) {
	p, err := s.app.GetPerson(ctx, personID)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(p), nil
}

func (s Service) UpdatePerson(ctx context.Context, _ bearertoken.Token, personID string, req personapi.UpdatePersonRequest) (personapi.Person, error) {
	updated, err := s.app.UpdatePerson(ctx, personID, domain.PersonPatch{
		DisplayName:    req.DisplayName,
		Title:          req.Title,
		Given:          req.Given,
		Given2:         req.Given2,
		Surname:        req.Surname,
		SurnamePrefix:  req.SurnamePrefix,
		Surname2:       req.Surname2,
		Generation:     req.Generation,
		Credentials:    req.Credentials,
		Preferred:      req.Preferred,
		Birthdate:      req.Birthdate,
		Sex:            req.Sex,
		CountryOfBirth: req.CountryOfBirth,
		Attributes:     attrToBytes(req.Attributes),
	})
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(updated), nil
}

func (s Service) ListPersons(ctx context.Context, _ bearertoken.Token, pageSize *int, pageToken *string) (personapi.PersonPage, error) {
	page, err := s.app.ListPersons(ctx, derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return personapi.PersonPage{}, s.mapError(ctx, err, "")
	}
	persons := make([]personapi.Person, 0, len(page.Persons))
	for _, p := range page.Persons {
		persons = append(persons, toAPIPerson(p))
	}
	return personapi.PersonPage{Persons: persons, NextPageToken: tokenPtr(page.NextPageToken)}, nil
}

func (s Service) SetRank(ctx context.Context, _ bearertoken.Token, personID string, req personapi.SetRankRequest) (personapi.Person, error) {
	updated, err := s.app.SetRank(ctx, personID, req.RankId)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(updated), nil
}

// ---------------------------------------------------------------- lifecycle

func (s Service) DeactivatePerson(ctx context.Context, _ bearertoken.Token, personID string, req personapi.DeactivateRequest) (personapi.Person, error) {
	updated, err := s.app.DeactivatePerson(ctx, personID, derefOr(req.Reason, ""))
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(updated), nil
}

func (s Service) ReactivatePerson(ctx context.Context, _ bearertoken.Token, personID string) (personapi.Person, error) {
	updated, err := s.app.ReactivatePerson(ctx, personID)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(updated), nil
}

func (s Service) PurgePerson(ctx context.Context, _ bearertoken.Token, personID string) (personapi.Person, error) {
	purged, err := s.app.PurgePerson(ctx, personID)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(purged), nil
}

// ---------------------------------------------------------------- name variants

func (s Service) UpsertNameVariant(ctx context.Context, _ bearertoken.Token, personID string, req personapi.UpsertNameVariantRequest) (personapi.NameVariant, error) {
	created, err := s.app.UpsertNameVariant(ctx, domain.NameVariant{
		PersonID: personID,
		Locale:   req.Locale,
		Name: nameFromParts(req.DisplayName, req.Title, req.Given, req.Given2, req.Surname,
			req.SurnamePrefix, req.Surname2, req.Generation, req.Credentials, req.Preferred),
		IsPrimary: derefOr(req.IsPrimary, false),
	})
	if err != nil {
		return personapi.NameVariant{}, s.mapError(ctx, err, personID)
	}
	return toAPIVariant(created), nil
}

func (s Service) DeleteNameVariant(ctx context.Context, _ bearertoken.Token, personID, locale string) error {
	if err := s.app.DeleteNameVariant(ctx, personID, locale); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- citizenships

func (s Service) ListCitizenships(ctx context.Context, _ bearertoken.Token, personID string) ([]personapi.Citizenship, error) {
	cs, err := s.app.ListCitizenships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Citizenship, 0, len(cs))
	for _, c := range cs {
		out = append(out, toAPICitizenship(c))
	}
	return out, nil
}

func (s Service) UpsertCitizenship(ctx context.Context, _ bearertoken.Token, personID string, req personapi.UpsertCitizenshipRequest) (personapi.Citizenship, error) {
	created, err := s.app.UpsertCitizenship(ctx, domain.Citizenship{
		PersonID:   personID,
		Country:    req.Country,
		Basis:      derefOr(req.Basis, ""),
		AcquiredOn: derefOr(req.AcquiredOn, ""),
		LostOn:     derefOr(req.LostOn, ""),
		IsPrimary:  derefOr(req.IsPrimary, false),
	})
	if err != nil {
		return personapi.Citizenship{}, s.mapError(ctx, err, personID)
	}
	return toAPICitizenship(created), nil
}

func (s Service) DeleteCitizenship(ctx context.Context, _ bearertoken.Token, personID, country string) error {
	if err := s.app.DeleteCitizenship(ctx, personID, country); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- residences

func (s Service) ListResidences(ctx context.Context, _ bearertoken.Token, personID string) ([]personapi.Residence, error) {
	rs, err := s.app.ListResidences(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Residence, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIResidence(r))
	}
	return out, nil
}

func (s Service) UpsertResidence(ctx context.Context, _ bearertoken.Token, personID string, req personapi.UpsertResidenceRequest) (personapi.Residence, error) {
	created, err := s.app.UpsertResidence(ctx, domain.Residence{
		ID:        derefOr(req.Id, ""),
		PersonID:  personID,
		Country:   req.Country,
		Region:    derefOr(req.Region, ""),
		ValidFrom: req.ValidFrom,
		ValidTo:   derefOr(req.ValidTo, ""),
	})
	if err != nil {
		return personapi.Residence{}, s.mapError(ctx, err, personID)
	}
	return toAPIResidence(created), nil
}

func (s Service) DeleteResidence(ctx context.Context, _ bearertoken.Token, personID, residenceID string) error {
	if err := s.app.DeleteResidence(ctx, personID, residenceID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- response assembly

func toAPIPerson(p domain.Person) personapi.Person {
	return personapi.Person{
		Id:             p.ID,
		Code:           strPtrOrNil(p.Code),
		DisplayName:    p.DisplayName,
		Title:          strPtrOrNil(p.Title),
		Given:          strPtrOrNil(p.Given),
		Given2:         strPtrOrNil(p.Given2),
		Surname:        strPtrOrNil(p.Surname),
		SurnamePrefix:  strPtrOrNil(p.SurnamePrefix),
		Surname2:       strPtrOrNil(p.Surname2),
		Generation:     strPtrOrNil(p.Generation),
		Credentials:    strPtrOrNil(p.Credentials),
		Preferred:      strPtrOrNil(p.Preferred),
		Birthdate:      strPtrOrNil(p.Birthdate),
		Sex:            p.Sex,
		CountryOfBirth: strPtrOrNil(p.CountryOfBirth),
		Attributes:     attrFromBytes(p.Attributes),
		RankId:         strPtrOrNil(p.RankID),
		Status:         string(p.Status),
		DeactivatedAt:  dtPtr(p.DeactivatedAt),
		PurgeAfter:     dtPtr(p.PurgeAfter),
		CreatedAt:      datetime.DateTime(p.CreatedAt),
		UpdatedAt:      datetime.DateTime(p.UpdatedAt),
		NameVariants:   toAPIVariants(p.NameVariants),
		Citizenships:   toAPICitizenships(p.Citizenships),
		Residences:     toAPIResidences(p.Residences),
	}
}

func toAPIVariant(v domain.NameVariant) personapi.NameVariant {
	return personapi.NameVariant{
		Id:            v.ID,
		PersonId:      v.PersonID,
		Locale:        v.Locale,
		DisplayName:   v.DisplayName,
		Title:         strPtrOrNil(v.Title),
		Given:         strPtrOrNil(v.Given),
		Given2:        strPtrOrNil(v.Given2),
		Surname:       strPtrOrNil(v.Surname),
		SurnamePrefix: strPtrOrNil(v.SurnamePrefix),
		Surname2:      strPtrOrNil(v.Surname2),
		Generation:    strPtrOrNil(v.Generation),
		Credentials:   strPtrOrNil(v.Credentials),
		Preferred:     strPtrOrNil(v.Preferred),
		IsPrimary:     v.IsPrimary,
	}
}

func toAPIVariants(vs []domain.NameVariant) []personapi.NameVariant {
	out := make([]personapi.NameVariant, 0, len(vs))
	for _, v := range vs {
		out = append(out, toAPIVariant(v))
	}
	return out
}

func toAPICitizenship(c domain.Citizenship) personapi.Citizenship {
	return personapi.Citizenship{
		Id:         c.ID,
		PersonId:   c.PersonID,
		Country:    c.Country,
		Basis:      c.Basis,
		AcquiredOn: strPtrOrNil(c.AcquiredOn),
		LostOn:     strPtrOrNil(c.LostOn),
		IsPrimary:  c.IsPrimary,
	}
}

func toAPICitizenships(cs []domain.Citizenship) []personapi.Citizenship {
	out := make([]personapi.Citizenship, 0, len(cs))
	for _, c := range cs {
		out = append(out, toAPICitizenship(c))
	}
	return out
}

func toAPIResidence(r domain.Residence) personapi.Residence {
	return personapi.Residence{
		Id:        r.ID,
		PersonId:  r.PersonID,
		Country:   r.Country,
		Region:    strPtrOrNil(r.Region),
		ValidFrom: r.ValidFrom,
		ValidTo:   strPtrOrNil(r.ValidTo),
	}
}

func toAPIResidences(rs []domain.Residence) []personapi.Residence {
	out := make([]personapi.Residence, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIResidence(r))
	}
	return out
}

func nameFromParts(displayName string, title, given, given2, surname, surnamePrefix, surname2, generation, credentials, preferred *string) domain.Name {
	return domain.Name{
		DisplayName:   displayName,
		Title:         derefOr(title, ""),
		Given:         derefOr(given, ""),
		Given2:        derefOr(given2, ""),
		Surname:       derefOr(surname, ""),
		SurnamePrefix: derefOr(surnamePrefix, ""),
		Surname2:      derefOr(surname2, ""),
		Generation:    derefOr(generation, ""),
		Credentials:   derefOr(credentials, ""),
		Preferred:     derefOr(preferred, ""),
	}
}

// ---------------------------------------------------------------- error mapping

// mapError translates domain/application errors into the Conjure SerializableError contract. A
// missing child resource (name variant / citizenship / residence) reuses PersonNotFound — the
// targeted sub-resource was not found under the person.
func (s Service) mapError(ctx context.Context, err error, personID string) error {
	switch {
	case errors.Is(err, domain.ErrNotFound),
		errors.Is(err, domain.ErrNameVariantNotFound),
		errors.Is(err, domain.ErrCitizenshipNotFound),
		errors.Is(err, domain.ErrResidenceNotFound):
		return personapi.NewPersonNotFound(personID)
	case errors.Is(err, domain.ErrCodeConflict):
		return personapi.NewPersonConflict("a person with this code already exists")
	case errors.Is(err, domain.ErrCitizenshipConflict):
		return personapi.NewPersonConflict("an active citizenship for this country already exists")
	case errors.Is(err, domain.ErrUnknownRank):
		return personapi.NewPersonInvalid("rank does not exist")
	case errors.Is(err, domain.ErrUnknownCountry):
		return personapi.NewPersonInvalid("country does not exist")
	case errors.Is(err, domain.ErrUnknownLocale):
		return personapi.NewPersonInvalid("locale does not exist")
	case errors.Is(err, domain.ErrInvalid):
		return personapi.NewPersonInvalid(err.Error())
	case errors.Is(err, domain.ErrLifecycle):
		return personapi.NewPersonLifecycleConflict(err.Error())
	default:
		return werror.WrapWithContextParams(ctx, err, "person request failed")
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

func dtPtr(t *time.Time) *datetime.DateTime {
	if t == nil {
		return nil
	}
	d := datetime.DateTime(*t)
	return &d
}

// attrToBytes marshals the optional free-form attributes object to the JSONB bytes stored in the DB
// (nil when absent, so the column keeps its default / current value).
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
