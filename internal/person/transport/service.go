// Package transport implements the person module's generated Conjure PersonService interface: it
// translates the wire contract to/from the application service and maps domain errors to Conjure
// SerializableErrors (D-Conjure). Generated code in internal/conjure is never hand-edited.
//
// Person names are per-record data returned verbatim (a canonical display name + locale-tagged
// variants), NOT the instance localization store (D-i18n) — so unlike rank/tenant this transport
// assembles no locale->text maps.
//
// Authorization: each endpoint is gated via the PEP on its `person.*` permission, and reads then
// apply the read-scope projection (D-PersonReadScope). Because a person is instance-global with no
// unit column, `GET /persons/{id}` and `GET /persons` intersect the subject's effective readable
// reach (pep.EffectiveReach) with the person's active-membership units: an instance admin sees the
// whole directory; any other reader sees only people reachable through a readable unit. A non-readable
// person is reported as not-found so existence does not leak. The acting subject is resolved from the
// request context by the PEP (identity-federation middleware; internal/authorization/pep).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/person/application"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// Person permission codes (D-BaseRoles); reads project through memberships per D-PersonReadScope (see
// the package note for the interim coarse gate).
const (
	permRead       = string(authzdomain.PermPersonRead)
	permCreate     = string(authzdomain.PermPersonCreate)
	permUpdate     = string(authzdomain.PermPersonUpdate)
	permRankAssign = string(authzdomain.PermPersonRankAssign)
	permLifecycle  = string(authzdomain.PermPersonLifecycle)
	permPurge      = string(authzdomain.PermPersonPurge)
)

// Localization entity-type keys for the translatable contact-kind / platform catalog names (D-i18n).
const (
	emailTypeEntity    = "email_type"
	phoneTypeEntity    = "phone_type"
	platformEntity     = "platform"
	relationTypeEntity = "relation_type"
)

// Service adapts *application.Service to the generated personapi.PersonService interface. It holds the
// localization service to assemble the contact-kind catalog `name` locale->text maps (D-i18n).
type Service struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the person application service, the localization
// service (contact-type name maps), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, loc: loc, pep: enforcer}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ personapi.PersonService = Service{}

// ---------------------------------------------------------------- persons

func (s Service) CreatePerson(ctx context.Context, token bearertoken.Token, req personapi.CreatePersonRequest) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permCreate); err != nil {
		return personapi.Person{}, err
	}
	created, err := s.app.CreatePerson(ctx, domain.Person{
		Code: derefOr(req.Code, ""),
		Name: nameFromParts(req.DisplayName, req.Title, req.Given, req.Given2, req.Surname,
			req.SurnamePrefix, req.Surname2, req.Generation, req.Credentials, req.Preferred),
		Birthdate:      derefOr(req.Birthdate, ""),
		DateOfDeath:    derefOr(req.DateOfDeath, ""),
		Sex:            domain.NormalizeSex(derefOr(req.Sex, "")),
		CountryOfBirth: derefOr(req.CountryOfBirth, ""),
		Attributes:     attrToBytes(req.Attributes),
	})
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, "")
	}
	return toAPIPerson(created), nil
}

func (s Service) GetPerson(ctx context.Context, token bearertoken.Token, personID string) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return personapi.Person{}, err
	}
	// Read-scope projection (D-PersonReadScope): the holder of person.read may only read a person whose
	// active-membership units intersect the subject's effective readable reach (or an instance admin).
	// A non-readable person is reported as not-found, so existence does not leak.
	reach, err := s.pep.EffectiveReach(ctx)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	ok, err := s.app.ReadablePerson(ctx, reach, personID)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	if !ok {
		return personapi.Person{}, personapi.NewPersonNotFound(personID)
	}
	p, err := s.app.GetPerson(ctx, personID)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(p), nil
}

func (s Service) UpdatePerson(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpdatePersonRequest) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Person{}, err
	}
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
		DateOfDeath:    req.DateOfDeath,
		Sex:            normalizeSexPtr(req.Sex),
		CountryOfBirth: req.CountryOfBirth,
		Attributes:     attrToBytes(req.Attributes),
	})
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(updated), nil
}

func (s Service) ListPersons(ctx context.Context, token bearertoken.Token, pageSize *int, pageToken *string) (personapi.PersonPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return personapi.PersonPage{}, err
	}
	// Read-scope projection (D-PersonReadScope): an instance admin sees the whole directory; any other
	// reader sees only the union of people reachable through their effective readable units.
	reach, err := s.pep.EffectiveReach(ctx)
	if err != nil {
		return personapi.PersonPage{}, s.mapError(ctx, err, "")
	}
	var page application.Page
	if reach.InstanceAdmin {
		page, err = s.app.ListPersons(ctx, derefOr(pageSize, 0), derefOr(pageToken, ""))
	} else {
		page, err = s.app.ListVisiblePersons(ctx, reach, derefOr(pageSize, 0), derefOr(pageToken, ""))
	}
	if err != nil {
		return personapi.PersonPage{}, s.mapError(ctx, err, "")
	}
	persons := make([]personapi.Person, 0, len(page.Persons))
	for _, p := range page.Persons {
		persons = append(persons, toAPIPerson(p))
	}
	return personapi.PersonPage{Persons: persons, NextPageToken: tokenPtr(page.NextPageToken)}, nil
}

func (s Service) SetRank(ctx context.Context, token bearertoken.Token, personID string, req personapi.SetRankRequest) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRankAssign); err != nil {
		return personapi.Person{}, err
	}
	// Setting derives the system from the rank; clearing (no rankId) needs the systemId to clear.
	if req.RankId == nil && derefOr(req.SystemId, "") == "" {
		return personapi.Person{}, personapi.NewPersonInvalid("systemId is required to clear a rank")
	}
	updated, err := s.app.SetPersonRank(ctx, personID, derefOr(req.SystemId, ""), req.RankId)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(updated), nil
}

// ---------------------------------------------------------------- lifecycle

func (s Service) DeactivatePerson(ctx context.Context, token bearertoken.Token, personID string, req personapi.DeactivateRequest) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permLifecycle); err != nil {
		return personapi.Person{}, err
	}
	updated, err := s.app.DeactivatePerson(ctx, personID, derefOr(req.Reason, ""))
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(updated), nil
}

func (s Service) ReactivatePerson(ctx context.Context, token bearertoken.Token, personID string) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permLifecycle); err != nil {
		return personapi.Person{}, err
	}
	updated, err := s.app.ReactivatePerson(ctx, personID)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(updated), nil
}

func (s Service) PurgePerson(ctx context.Context, token bearertoken.Token, personID string) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permPurge); err != nil {
		return personapi.Person{}, err
	}
	purged, err := s.app.PurgePerson(ctx, personID)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(purged), nil
}

// ---------------------------------------------------------------- name variants

func (s Service) UpsertNameVariant(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertNameVariantRequest) (personapi.NameVariant, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.NameVariant{}, err
	}
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

func (s Service) DeleteNameVariant(ctx context.Context, token bearertoken.Token, personID, locale string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteNameVariant(ctx, personID, locale); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- citizenships

func (s Service) ListCitizenships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Citizenship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
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

func (s Service) UpsertCitizenship(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertCitizenshipRequest) (personapi.Citizenship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Citizenship{}, err
	}
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

func (s Service) DeleteCitizenship(ctx context.Context, token bearertoken.Token, personID, country string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteCitizenship(ctx, personID, country); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- residences

func (s Service) ListResidences(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Residence, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
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

func (s Service) UpsertResidence(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertResidenceRequest) (personapi.Residence, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Residence{}, err
	}
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

func (s Service) DeleteResidence(ctx context.Context, token bearertoken.Token, personID, residenceID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteResidence(ctx, personID, residenceID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- emails

func (s Service) ListEmails(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Email, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	es, err := s.app.ListEmails(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Email, 0, len(es))
	for _, e := range es {
		out = append(out, toAPIEmail(e))
	}
	return out, nil
}

func (s Service) UpsertEmail(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertEmailRequest) (personapi.Email, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Email{}, err
	}
	created, err := s.app.UpsertEmail(ctx, domain.Email{
		ID:        derefOr(req.Id, ""),
		PersonID:  personID,
		TypeCode:  req.TypeCode,
		Address:   req.Address,
		IsPrimary: derefOr(req.IsPrimary, false),
	})
	if err != nil {
		return personapi.Email{}, s.mapError(ctx, err, personID)
	}
	return toAPIEmail(created), nil
}

func (s Service) DeleteEmail(ctx context.Context, token bearertoken.Token, personID, emailID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteEmail(ctx, personID, emailID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- phones

func (s Service) ListPhones(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Phone, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ps, err := s.app.ListPhones(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Phone, 0, len(ps))
	for _, p := range ps {
		out = append(out, toAPIPhone(p))
	}
	return out, nil
}

func (s Service) UpsertPhone(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertPhoneRequest) (personapi.Phone, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Phone{}, err
	}
	created, err := s.app.UpsertPhone(ctx, domain.Phone{
		ID:        derefOr(req.Id, ""),
		PersonID:  personID,
		TypeCode:  req.TypeCode,
		Number:    req.Number,
		IsPrimary: derefOr(req.IsPrimary, false),
	})
	if err != nil {
		return personapi.Phone{}, s.mapError(ctx, err, personID)
	}
	return toAPIPhone(created), nil
}

func (s Service) DeletePhone(ctx context.Context, token bearertoken.Token, personID, phoneID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeletePhone(ctx, personID, phoneID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- call signs

func (s Service) ListCallSigns(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.CallSign, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	cs, err := s.app.ListCallSigns(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.CallSign, 0, len(cs))
	for _, c := range cs {
		out = append(out, toAPICallSign(c))
	}
	return out, nil
}

func (s Service) UpsertCallSign(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertCallSignRequest) (personapi.CallSign, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.CallSign{}, err
	}
	created, err := s.app.UpsertCallSign(ctx, domain.CallSign{
		ID:        derefOr(req.Id, ""),
		PersonID:  personID,
		CallSign:  req.CallSign,
		IsPrimary: derefOr(req.IsPrimary, false),
	})
	if err != nil {
		return personapi.CallSign{}, s.mapError(ctx, err, personID)
	}
	return toAPICallSign(created), nil
}

func (s Service) DeleteCallSign(ctx context.Context, token bearertoken.Token, personID, callSignID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteCallSign(ctx, personID, callSignID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- messenger links (D-PersonSocialChannels)

func (s Service) ListMessengerLinks(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.MessengerLink, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ls, err := s.app.ListMessengerLinks(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	return toAPIMessengerLinks(ls), nil
}

func (s Service) UpsertMessengerLink(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertMessengerLinkRequest) (personapi.MessengerLink, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.MessengerLink{}, err
	}
	created, err := s.app.UpsertMessengerLink(ctx, personID, domain.MessengerLink{
		ID:           derefOr(req.Id, ""),
		PhoneID:      derefOr(req.PhoneId, ""),
		EmailID:      derefOr(req.EmailId, ""),
		PlatformCode: req.PlatformCode,
		IsPrimary:    derefOr(req.IsPrimary, false),
		VerifiedAt:   timePtrFromDT(req.VerifiedAt),
	})
	if err != nil {
		return personapi.MessengerLink{}, s.mapError(ctx, err, personID)
	}
	return toAPIMessengerLink(created), nil
}

func (s Service) DeleteMessengerLink(ctx context.Context, token bearertoken.Token, personID, messengerLinkID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteMessengerLink(ctx, personID, messengerLinkID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- social accounts (D-PersonSocialChannels)

func (s Service) ListSocialAccounts(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.SocialAccount, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	as, err := s.app.ListSocialAccounts(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	return toAPISocialAccounts(as), nil
}

func (s Service) UpsertSocialAccount(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertSocialAccountRequest) (personapi.SocialAccount, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.SocialAccount{}, err
	}
	created, err := s.app.UpsertSocialAccount(ctx, domain.SocialAccount{
		ID:                   derefOr(req.Id, ""),
		PersonID:             personID,
		PlatformCode:         req.PlatformCode,
		PlatformUserID:       derefOr(req.PlatformUserId, ""),
		Handle:               req.Handle,
		DisplayName:          derefOr(req.DisplayName, ""),
		ProfileURL:           derefOr(req.ProfileUrl, ""),
		Language:             derefOr(req.Language, ""),
		PlatformVerified:     derefOr(req.PlatformVerified, false),
		VerifiedByOperatorAt: timePtrFromDT(req.VerifiedByOperatorAt),
		Source:               req.Source,
		Confidence:           derefOr(req.Confidence, ""),
		IsPrimary:            derefOr(req.IsPrimary, false),
	})
	if err != nil {
		return personapi.SocialAccount{}, s.mapError(ctx, err, personID)
	}
	return toAPISocialAccount(created), nil
}

func (s Service) DeleteSocialAccount(ctx context.Context, token bearertoken.Token, personID, socialAccountID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteSocialAccount(ctx, personID, socialAccountID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

func (s Service) ListSocialAccountHandles(ctx context.Context, token bearertoken.Token, personID, socialAccountID string) ([]personapi.SocialAccountHandle, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	hs, err := s.app.ListSocialAccountHandles(ctx, personID, socialAccountID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.SocialAccountHandle, 0, len(hs))
	for _, h := range hs {
		out = append(out, toAPISocialAccountHandle(h))
	}
	return out, nil
}

// ---------------------------------------------------------------- person↔person relationships (D-PersonRelationships)

func (s Service) ListRelationTypes(ctx context.Context, token bearertoken.Token) ([]personapi.RelationType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	types, err := s.app.ListRelationTypes(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.Code] = t.Name
	}
	names, err := s.loc.NamesByID(ctx, relationTypeEntity, defaults)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	out := make([]personapi.RelationType, 0, len(types))
	for _, t := range types {
		out = append(out, personapi.RelationType{
			Code: t.Code, Name: names[t.Code], Category: t.Category, Status: t.Status, SortOrder: sortOrderPtr(t.SortOrder),
		})
	}
	return out, nil
}

// directedEndpoints maps the path person + counterpart + role to a directional (from, to) pair, ok=false
// for an unrecognized role.
func directedEndpoints(personID, counterpart, role, fromRole, toRole string) (from, to string, ok bool) {
	switch role {
	case fromRole:
		return personID, counterpart, true
	case toRole:
		return counterpart, personID, true
	}
	return "", "", false
}

func (s Service) ListPartnerships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Partnership, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	rs, err := s.app.ListPartnerships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Partnership, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIPartnership(r))
	}
	return out, nil
}

func (s Service) UpsertPartnership(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertPartnershipRequest) (personapi.Partnership, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Partnership{}, err
	}
	saved, err := s.app.UpsertPartnership(ctx, personID, domain.Partnership{
		ID:            derefOr(req.Id, ""),
		PersonIDA:     personID,
		PersonIDB:     req.PartnerId,
		Status:        req.Status,
		EffectiveFrom: derefOr(req.EffectiveFrom, ""),
		EffectiveTo:   derefOr(req.EffectiveTo, ""),
	})
	if err != nil {
		return personapi.Partnership{}, s.mapError(ctx, err, personID)
	}
	return toAPIPartnership(saved), nil
}

func (s Service) ListKinships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Kinship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	rs, err := s.app.ListKinships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Kinship, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIKinship(r))
	}
	return out, nil
}

func (s Service) UpsertKinship(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertKinshipRequest) (personapi.Kinship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Kinship{}, err
	}
	parent, child, ok := directedEndpoints(personID, req.CounterpartId, req.Role, "parent", "child")
	if !ok {
		return personapi.Kinship{}, personapi.NewPersonInvalid("role must be parent or child")
	}
	saved, err := s.app.UpsertKinship(ctx, personID, domain.Kinship{
		ID: derefOr(req.Id, ""), ParentID: parent, ChildID: child, Status: derefOr(req.Status, ""),
	})
	if err != nil {
		return personapi.Kinship{}, s.mapError(ctx, err, personID)
	}
	return toAPIKinship(saved), nil
}

func (s Service) ListGuardianships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Guardianship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	rs, err := s.app.ListGuardianships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Guardianship, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIGuardianship(r))
	}
	return out, nil
}

func (s Service) UpsertGuardianship(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertGuardianshipRequest) (personapi.Guardianship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Guardianship{}, err
	}
	guardian, ward, ok := directedEndpoints(personID, req.CounterpartId, req.Role, "guardian", "ward")
	if !ok {
		return personapi.Guardianship{}, personapi.NewPersonInvalid("role must be guardian or ward")
	}
	saved, err := s.app.UpsertGuardianship(ctx, personID, domain.Guardianship{
		ID: derefOr(req.Id, ""), GuardianID: guardian, WardID: ward, RelationCode: derefOr(req.RelationCode, ""),
		Status: derefOr(req.Status, ""), EffectiveFrom: derefOr(req.EffectiveFrom, ""), EffectiveTo: derefOr(req.EffectiveTo, ""),
	})
	if err != nil {
		return personapi.Guardianship{}, s.mapError(ctx, err, personID)
	}
	return toAPIGuardianship(saved), nil
}

func (s Service) ListSponsorships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Sponsorship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	rs, err := s.app.ListSponsorships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Sponsorship, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPISponsorship(r))
	}
	return out, nil
}

func (s Service) UpsertSponsorship(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertSponsorshipRequest) (personapi.Sponsorship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Sponsorship{}, err
	}
	sponsor, sponsored, ok := directedEndpoints(personID, req.CounterpartId, req.Role, "sponsor", "sponsored")
	if !ok {
		return personapi.Sponsorship{}, personapi.NewPersonInvalid("role must be sponsor or sponsored")
	}
	saved, err := s.app.UpsertSponsorship(ctx, personID, domain.Sponsorship{
		ID: derefOr(req.Id, ""), SponsorID: sponsor, SponsoredID: sponsored, RelationCode: req.RelationCode,
		Status: derefOr(req.Status, ""), EffectiveFrom: derefOr(req.EffectiveFrom, ""), EffectiveTo: derefOr(req.EffectiveTo, ""),
	})
	if err != nil {
		return personapi.Sponsorship{}, s.mapError(ctx, err, personID)
	}
	return toAPISponsorship(saved), nil
}

func (s Service) ListNextOfKin(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.NextOfKin, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	rs, err := s.app.ListNextOfKin(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.NextOfKin, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPINextOfKin(r))
	}
	return out, nil
}

func (s Service) UpsertNextOfKin(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertNextOfKinRequest) (personapi.NextOfKin, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.NextOfKin{}, err
	}
	saved, err := s.app.UpsertNextOfKin(ctx, personID, domain.NextOfKin{
		ID: derefOr(req.Id, ""), SubjectID: personID, ContactID: req.ContactId,
		RelationCode: derefOr(req.RelationCode, ""), Priority: derefOr(req.Priority, 0), Status: derefOr(req.Status, ""),
	})
	if err != nil {
		return personapi.NextOfKin{}, s.mapError(ctx, err, personID)
	}
	return toAPINextOfKin(saved), nil
}

func (s Service) ListAssociations(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Association, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	rs, err := s.app.ListAssociations(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Association, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIAssociation(r))
	}
	return out, nil
}

func (s Service) UpsertAssociation(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertAssociationRequest) (personapi.Association, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Association{}, err
	}
	saved, err := s.app.UpsertAssociation(ctx, personID, domain.Association{
		ID: derefOr(req.Id, ""), PersonIDA: personID, PersonIDB: req.CounterpartId,
		RelationCode: derefOr(req.RelationCode, ""), Kind: req.Kind, Status: derefOr(req.Status, ""),
	})
	if err != nil {
		return personapi.Association{}, s.mapError(ctx, err, personID)
	}
	return toAPIAssociation(saved), nil
}

func (s Service) DeleteRelationship(ctx context.Context, token bearertoken.Token, personID, relationshipID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteRelationship(ctx, personID, relationshipID); err != nil {
		if errors.Is(err, domain.ErrRelationshipNotFound) { // idempotent: a missing link is a no-op
			return nil
		}
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- platform catalog

func (s Service) ListPlatforms(ctx context.Context, token bearertoken.Token) ([]personapi.Platform, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	platforms, err := s.app.ListPlatforms(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	defaults := make(map[string]string, len(platforms))
	for _, p := range platforms {
		defaults[p.Code] = p.Name
	}
	names, err := s.loc.NamesByID(ctx, platformEntity, defaults)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	out := make([]personapi.Platform, 0, len(platforms))
	for _, p := range platforms {
		out = append(out, personapi.Platform{
			Code: p.Code, Name: names[p.Code], Category: p.Category, Status: p.Status, SortOrder: sortOrderPtr(p.SortOrder),
		})
	}
	return out, nil
}

// ---------------------------------------------------------------- contact-kind catalogs

func (s Service) ListEmailTypes(ctx context.Context, token bearertoken.Token) ([]personapi.EmailType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	types, err := s.app.ListEmailTypes(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	names, err := s.contactTypeNames(ctx, emailTypeEntity, types)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	out := make([]personapi.EmailType, 0, len(types))
	for _, t := range types {
		out = append(out, personapi.EmailType{
			Code: t.Code, Name: names[t.Code], Status: t.Status, SortOrder: sortOrderPtr(t.SortOrder),
		})
	}
	return out, nil
}

func (s Service) ListPhoneTypes(ctx context.Context, token bearertoken.Token) ([]personapi.PhoneType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	types, err := s.app.ListPhoneTypes(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	names, err := s.contactTypeNames(ctx, phoneTypeEntity, types)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	out := make([]personapi.PhoneType, 0, len(types))
	for _, t := range types {
		out = append(out, personapi.PhoneType{
			Code: t.Code, Name: names[t.Code], Status: t.Status, SortOrder: sortOrderPtr(t.SortOrder),
		})
	}
	return out, nil
}

// contactTypeNames assembles each catalog row's translatable `name` as a locale->text map, keyed by
// the type code, with the default-locale `name` as the fallback (D-i18n).
func (s Service) contactTypeNames(ctx context.Context, entity string, types []domain.ContactType) (map[string]map[string]string, error) {
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.Code] = t.Name
	}
	return s.loc.NamesByID(ctx, entity, defaults)
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
		DateOfDeath:    strPtrOrNil(p.DateOfDeath),
		Sex:            p.Sex,
		CountryOfBirth: strPtrOrNil(p.CountryOfBirth),
		Attributes:     attrFromBytes(p.Attributes),
		Ranks:          toAPIPersonRanks(p.Ranks),
		Status:         string(p.Status),
		DeactivatedAt:  dtPtr(p.DeactivatedAt),
		PurgeAfter:     dtPtr(p.PurgeAfter),
		CreatedAt:      datetime.DateTime(p.CreatedAt),
		UpdatedAt:      datetime.DateTime(p.UpdatedAt),
		NameVariants:   toAPIVariants(p.NameVariants),
		Citizenships:   toAPICitizenships(p.Citizenships),
		Residences:     toAPIResidences(p.Residences),
		Emails:         toAPIEmails(p.Emails),
		Phones:         toAPIPhones(p.Phones),
		CallSigns:      toAPICallSigns(p.CallSigns),
		MessengerLinks: toAPIMessengerLinks(p.MessengerLinks),
		SocialAccounts: toAPISocialAccounts(p.SocialAccounts),
	}
}

// toAPIPersonRanks maps the person's held ranks (one per rank system; D-Rank) to the API shape.
func toAPIPersonRanks(rs []domain.PersonRank) []personapi.PersonRank {
	out := make([]personapi.PersonRank, 0, len(rs))
	for _, r := range rs {
		out = append(out, personapi.PersonRank{SystemId: r.SystemID, RankId: r.RankID})
	}
	return out
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

func toAPIEmail(e domain.Email) personapi.Email {
	return personapi.Email{
		Id:        e.ID,
		PersonId:  e.PersonID,
		TypeCode:  e.TypeCode,
		Address:   e.Address,
		Provider:  strPtrOrNil(e.Provider),
		IsPrimary: e.IsPrimary,
	}
}

func toAPIEmails(es []domain.Email) []personapi.Email {
	out := make([]personapi.Email, 0, len(es))
	for _, e := range es {
		out = append(out, toAPIEmail(e))
	}
	return out
}

func toAPIPhone(p domain.Phone) personapi.Phone {
	return personapi.Phone{
		Id:        p.ID,
		PersonId:  p.PersonID,
		TypeCode:  p.TypeCode,
		Number:    p.Number,
		Country:   strPtrOrNil(p.Country),
		IsPrimary: p.IsPrimary,
	}
}

func toAPIPhones(ps []domain.Phone) []personapi.Phone {
	out := make([]personapi.Phone, 0, len(ps))
	for _, p := range ps {
		out = append(out, toAPIPhone(p))
	}
	return out
}

func toAPICallSign(c domain.CallSign) personapi.CallSign {
	return personapi.CallSign{
		Id:        c.ID,
		PersonId:  c.PersonID,
		CallSign:  c.CallSign,
		IsPrimary: c.IsPrimary,
	}
}

func toAPICallSigns(cs []domain.CallSign) []personapi.CallSign {
	out := make([]personapi.CallSign, 0, len(cs))
	for _, c := range cs {
		out = append(out, toAPICallSign(c))
	}
	return out
}

func toAPIMessengerLink(m domain.MessengerLink) personapi.MessengerLink {
	return personapi.MessengerLink{
		Id:           m.ID,
		PhoneId:      strPtrOrNil(m.PhoneID),
		EmailId:      strPtrOrNil(m.EmailID),
		PlatformCode: m.PlatformCode,
		IsPrimary:    m.IsPrimary,
		VerifiedAt:   dtPtr(m.VerifiedAt),
	}
}

func toAPIMessengerLinks(ls []domain.MessengerLink) []personapi.MessengerLink {
	out := make([]personapi.MessengerLink, 0, len(ls))
	for _, m := range ls {
		out = append(out, toAPIMessengerLink(m))
	}
	return out
}

func toAPISocialAccount(a domain.SocialAccount) personapi.SocialAccount {
	return personapi.SocialAccount{
		Id:                   a.ID,
		PersonId:             a.PersonID,
		PlatformCode:         a.PlatformCode,
		PlatformUserId:       strPtrOrNil(a.PlatformUserID),
		Handle:               a.Handle,
		DisplayName:          strPtrOrNil(a.DisplayName),
		ProfileUrl:           strPtrOrNil(a.ProfileURL),
		Language:             strPtrOrNil(a.Language),
		PlatformVerified:     a.PlatformVerified,
		VerifiedByOperatorAt: dtPtr(a.VerifiedByOperatorAt),
		Source:               a.Source,
		Confidence:           a.Confidence,
		IsPrimary:            a.IsPrimary,
	}
}

func toAPISocialAccounts(as []domain.SocialAccount) []personapi.SocialAccount {
	out := make([]personapi.SocialAccount, 0, len(as))
	for _, a := range as {
		out = append(out, toAPISocialAccount(a))
	}
	return out
}

func toAPISocialAccountHandle(h domain.SocialAccountHandle) personapi.SocialAccountHandle {
	return personapi.SocialAccountHandle{
		Id:        h.ID,
		AccountId: h.AccountID,
		Handle:    h.Handle,
		ValidFrom: datetime.DateTime(h.ValidFrom),
		ValidTo:   dtPtr(h.ValidTo),
	}
}

func toAPIPartnership(p domain.Partnership) personapi.Partnership {
	return personapi.Partnership{
		Id: p.ID, PersonIdA: p.PersonIDA, PersonIdB: p.PersonIDB, Status: p.Status,
		EffectiveFrom: strPtrOrNil(p.EffectiveFrom), EffectiveTo: strPtrOrNil(p.EffectiveTo),
	}
}

func toAPIKinship(k domain.Kinship) personapi.Kinship {
	return personapi.Kinship{Id: k.ID, ParentId: k.ParentID, ChildId: k.ChildID, Status: k.Status}
}

func toAPIGuardianship(g domain.Guardianship) personapi.Guardianship {
	return personapi.Guardianship{
		Id: g.ID, GuardianId: g.GuardianID, WardId: g.WardID, RelationCode: strPtrOrNil(g.RelationCode),
		Status: g.Status, EffectiveFrom: strPtrOrNil(g.EffectiveFrom), EffectiveTo: strPtrOrNil(g.EffectiveTo),
	}
}

func toAPISponsorship(sp domain.Sponsorship) personapi.Sponsorship {
	return personapi.Sponsorship{
		Id: sp.ID, SponsorId: sp.SponsorID, SponsoredId: sp.SponsoredID, RelationCode: sp.RelationCode,
		Status: sp.Status, EffectiveFrom: strPtrOrNil(sp.EffectiveFrom), EffectiveTo: strPtrOrNil(sp.EffectiveTo),
	}
}

func toAPINextOfKin(n domain.NextOfKin) personapi.NextOfKin {
	return personapi.NextOfKin{
		Id: n.ID, SubjectId: n.SubjectID, ContactId: n.ContactID,
		RelationCode: strPtrOrNil(n.RelationCode), Priority: n.Priority, Status: n.Status,
	}
}

func toAPIAssociation(a domain.Association) personapi.Association {
	return personapi.Association{
		Id: a.ID, PersonIdA: a.PersonIDA, PersonIdB: a.PersonIDB,
		RelationCode: strPtrOrNil(a.RelationCode), Kind: a.Kind, Status: a.Status,
	}
}

// sortOrderPtr maps a catalog sort order (0 == unset by convention) to the optional API field.
func sortOrderPtr(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
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
		errors.Is(err, domain.ErrResidenceNotFound),
		errors.Is(err, domain.ErrEmailNotFound),
		errors.Is(err, domain.ErrPhoneNotFound),
		errors.Is(err, domain.ErrCallSignNotFound),
		errors.Is(err, domain.ErrMessengerLinkNotFound),
		errors.Is(err, domain.ErrSocialAccountNotFound),
		errors.Is(err, domain.ErrRelationshipNotFound):
		return personapi.NewPersonNotFound(personID)
	case errors.Is(err, domain.ErrCodeConflict):
		return personapi.NewPersonConflict("a person with this code already exists")
	case errors.Is(err, domain.ErrCitizenshipConflict):
		return personapi.NewPersonConflict("an active citizenship for this country already exists")
	case errors.Is(err, domain.ErrEmailConflict):
		return personapi.NewPersonConflict("an active email with this address already exists")
	case errors.Is(err, domain.ErrPhoneConflict):
		return personapi.NewPersonConflict("an active phone with this number already exists")
	case errors.Is(err, domain.ErrCallSignConflict):
		return personapi.NewPersonConflict("an active call sign with this value already exists")
	case errors.Is(err, domain.ErrMessengerLinkConflict):
		return personapi.NewPersonConflict("an active messenger link for this channel and platform already exists")
	case errors.Is(err, domain.ErrSocialAccountConflict):
		return personapi.NewPersonConflict("an active social account for this platform and identity already exists")
	case errors.Is(err, domain.ErrPartnershipConflict):
		return personapi.NewPersonConflict("a person already has an active engaged/married partnership")
	case errors.Is(err, domain.ErrRelationshipConflict):
		return personapi.NewPersonConflict("an equivalent active relationship already exists")
	case errors.Is(err, domain.ErrUnknownRelationType):
		return personapi.NewPersonInvalid("relation type does not exist")
	case errors.Is(err, domain.ErrRelationCategory):
		return personapi.NewPersonInvalid("relation type is not in the expected category")
	case errors.Is(err, domain.ErrSelfRelationship):
		return personapi.NewPersonInvalid("a person cannot be related to themselves")
	case errors.Is(err, domain.ErrUnknownCounterpart):
		return personapi.NewPersonInvalid("the counterpart person does not exist")
	case errors.Is(err, domain.ErrUnknownRelationshipKind):
		return personapi.NewPersonInvalid("unknown relationship id")
	case errors.Is(err, domain.ErrUnknownPlatform):
		return personapi.NewPersonInvalid("platform does not exist")
	case errors.Is(err, domain.ErrPlatformNotMessenger):
		return personapi.NewPersonInvalid("platform is not a messenger platform")
	case errors.Is(err, domain.ErrChannelNotOwned):
		return personapi.NewPersonInvalid("the phone/email is not held by this person")
	case errors.Is(err, domain.ErrUnknownRank):
		return personapi.NewPersonInvalid("rank does not exist")
	case errors.Is(err, domain.ErrUnknownContactType):
		return personapi.NewPersonInvalid("contact type does not exist")
	case errors.Is(err, domain.ErrUnparseablePhone):
		return personapi.NewPersonInvalid("phone number could not be parsed")
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

// normalizeSexPtr collapses an optional ISO/IEC 5218 sex value to its canonical readable text,
// leaving nil (unchanged-in-patch) as nil.
func normalizeSexPtr(p *string) *string {
	if p == nil {
		return nil
	}
	s := domain.NormalizeSex(*p)
	return &s
}

func dtPtr(t *time.Time) *datetime.DateTime {
	if t == nil {
		return nil
	}
	d := datetime.DateTime(*t)
	return &d
}

// timePtrFromDT maps an optional wire datetime back to a *time.Time (nil stays nil).
func timePtrFromDT(d *datetime.DateTime) *time.Time {
	if d == nil {
		return nil
	}
	t := time.Time(*d)
	return &t
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
