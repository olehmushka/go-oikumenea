// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

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
// unit column, `GET /persons/{id}` and `GET /persons` intersect the subject's readable reach with
// the person's active-membership units — computed IN SQL since R-02.1 (membership's semi-joins,
// keyed on the subject from pep.SubjectAuthority): an instance admin sees the whole directory; any
// other reader sees only people reachable through a readable unit. A non-readable person is
// reported as not-found so existence does not leak. The acting subject is resolved from the
// request context by the PEP (identity-federation middleware; internal/authorization/pep).
package transport

import (
	"context"

	authzdomain "github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	personapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/person"
	locapp "github.com/olehmushka/go-oikumenea/internal/localization/application"
	"github.com/olehmushka/go-oikumenea/internal/person/application"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	profileapp "github.com/olehmushka/go-oikumenea/internal/personprofile/application"
	sensitiveapp "github.com/olehmushka/go-oikumenea/internal/personsensitive/application"
	"github.com/palantir/pkg/bearertoken"
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
	permMerge      = string(authzdomain.PermPersonMerge)
	// pii:special Art.9 reads — each behind its own code so person.read (base unit-reader) no longer
	// unlocks the ethnicity + politics + party aggregation (D-DataScope, review R-14).
	permEthnicityRead        = string(authzdomain.PermPersonEthnicityRead)
	permPoliticalLeaningRead = string(authzdomain.PermPersonPoliticalLeaningRead)
	permPartyMembershipRead  = string(authzdomain.PermPersonPartyMembershipRead)
	permHealthRead           = string(authzdomain.PermPersonHealthRead)
	// Criminal / arrest / court records (D-LegalRecords, M38). The base read is a need-to-know code in
	// sensitive-reader; the read-suppressed code (NOT in any base role) additionally reveals
	// sealed/expunged records — the strictest gate.
	permLegalRecordRead           = string(authzdomain.PermPersonLegalRecordRead)
	permLegalRecordReadSuppressed = string(authzdomain.PermPersonLegalRecordReadSuppressed)
	// Relationship-graph reads — one code per reified relationship (D-LinkPermissions). The SAME code
	// gates the dedicated list endpoint here and the link-traversal arm in cmd/oikumenea/link_descriptors.go,
	// so the person page and the object graph disclose exactly the same set. Composed into the additive
	// person-relationship-reader base role; NOT in unit-reader.
	permPartnershipRead  = string(authzdomain.PermPersonPartnershipRead)
	permKinshipRead      = string(authzdomain.PermPersonKinshipRead)
	permGuardianshipRead = string(authzdomain.PermPersonGuardianshipRead)
	permSponsorshipRead  = string(authzdomain.PermPersonSponsorshipRead)
	permNextOfKinRead    = string(authzdomain.PermPersonNextOfKinRead)
	permAssociationRead  = string(authzdomain.PermPersonAssociationRead)
	permAddressRead      = string(authzdomain.PermPersonAddressRead)
)

// Localization entity-type keys for the translatable contact-kind / platform catalog names (D-i18n).
const (
	emailTypeEntity     = "email_type"
	phoneTypeEntity     = "phone_type"
	platformEntity      = "platform"
	relationTypeEntity  = "relation_type"
	languoidEntity      = "languoid"
	ethnicityTypeEntity = "ethnicity_type"
)

// Service adapts the person application services to the generated personapi.PersonService interface.
// After the R-09 split the one Conjure surface delegates to three application services: the person core
// (app), and the personsensitive service (sensitive) for physical identity, ethnicity, overlays,
// watchlist and party membership. It holds the localization service to assemble the contact-kind catalog
// `name` locale->text maps (D-i18n).
type Service struct {
	app       *application.Service
	profile   *profileapp.Service
	sensitive *sensitiveapp.Service
	loc       *locapp.Service
	pep       *pep.Enforcer
}

// NewService builds the transport adapter over the person core application service, the personprofile and
// personsensitive application services, the localization service (contact-type name maps), and the PEP
// enforcer. The one Conjure PersonService surface delegates each endpoint to the owning service (R-09).
func NewService(app *application.Service, profile *profileapp.Service, sensitive *sensitiveapp.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, profile: profile, sensitive: sensitive, loc: loc, pep: enforcer}
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

// CreateProvisionalPerson creates a minimal-PII provisional stub (D-OverlayFoundation, M29). Gated on
// person.create (the same create-tier permission); resolution via MergePerson is the privileged step.
func (s Service) CreateProvisionalPerson(ctx context.Context, token bearertoken.Token, req personapi.CreateProvisionalPersonRequest) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permCreate); err != nil {
		return personapi.Person{}, err
	}
	created, err := s.app.CreateProvisionalPerson(ctx, domain.Person{
		Name:       nameFromParts(req.DisplayName, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		Attributes: attrToBytes(req.Attributes),
	})
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, "")
	}
	return toAPIPerson(created), nil
}

// MergePerson resolves a provisional stub into a canonical person (D-OverlayFoundation, M29). Gated on
// the admin-tier person.merge.
func (s Service) MergePerson(ctx context.Context, token bearertoken.Token, personID string, req personapi.MergePersonRequest) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permMerge); err != nil {
		return personapi.Person{}, err
	}
	merged, err := s.app.MergePerson(ctx, personID, req.IntoPersonId, derefOr(req.Confidence, ""))
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return toAPIPerson(merged), nil
}

func (s Service) GetPerson(ctx context.Context, token bearertoken.Token, personID string) (personapi.Person, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return personapi.Person{}, err
	}
	// Read-scope projection (D-PersonReadScope): the holder of person.read may only read a person whose
	// active-membership units intersect the subject's effective readable reach (or an instance admin).
	// A non-readable person is reported as not-found, so existence does not leak. The reach test is a
	// SQL point probe (R-02.1); the admin short-circuit rides the request authority snapshot.
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	if !isAdmin {
		ok, err := s.app.ReadablePerson(ctx, subject, personID)
		if err != nil {
			return personapi.Person{}, s.mapError(ctx, err, personID)
		}
		if !ok {
			return personapi.Person{}, personapi.NewPersonNotFound(personID)
		}
	}
	p, err := s.app.GetPerson(ctx, personID)
	if err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	out := toAPIPerson(p)
	// Compose the personprofile-owned child slices onto the detail response (D-PersonModuleSplit, R-09).
	if err := s.composeProfile(ctx, personID, &out); err != nil {
		return personapi.Person{}, s.mapError(ctx, err, personID)
	}
	return out, nil
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

func (s Service) ListPersons(ctx context.Context, token bearertoken.Token, pageSize *int, pageToken *string, query *string, sex *string, status *string, birthdateFrom *string, birthdateTo *string, countryOfBirth *string, rankID *string, unitID *string, graph *string, hasAccount *bool) (personapi.PersonPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return personapi.PersonPage{}, err
	}
	// One filter for the whole vocabulary (M56 / D-ObjectFacets), built once and passed down BOTH
	// branches of the read-scope dispatch below — the two paths must never see different filters.
	filter, err := personFilter(query, sex, status, birthdateFrom, birthdateTo, countryOfBirth, rankID, unitID, graph, hasAccount)
	if err != nil {
		return personapi.PersonPage{}, s.mapError(ctx, err, "")
	}
	// Read-scope projection (D-PersonReadScope): an instance admin sees the whole directory; any other
	// reader sees only the union of people reachable through their effective readable units — computed
	// as one SQL semi-join keyed on the subject (R-02.1; no reach set is materialized app-side).
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return personapi.PersonPage{}, s.mapError(ctx, err, "")
	}
	var page application.Page
	if isAdmin {
		page, err = s.app.ListPersons(ctx, filter, derefOr(pageSize, 0), derefOr(pageToken, ""))
	} else {
		page, err = s.app.ListVisiblePersons(ctx, subject, filter, derefOr(pageSize, 0), derefOr(pageToken, ""))
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
