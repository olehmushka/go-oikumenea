// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport (R-09 file split): response-assembly, error-mapping and value-helper handlers for the one PersonService Conjure surface.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	personapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

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
		// The non-encrypted directory child slices (citizenships, residences, contact channels) are owned
		// by personprofile (D-PersonModuleSplit, R-09) and composed onto the detail response by
		// GetPerson via composeProfile; they are empty here and in list responses.
	}
}

// composeProfile hydrates the personprofile-owned child slices onto a person detail response
// (D-PersonModuleSplit, R-09): GetPerson reads the core aggregate from the person core service, then
// fills the citizenships/residences/contact-channel arrays from the personprofile service. Any read
// error is returned so the handler reports it (the slices are all-or-nothing for one detail read).
func (s Service) composeProfile(ctx context.Context, personID string, out *personapi.Person) error {
	cits, err := s.profile.ListCitizenships(ctx, personID)
	if err != nil {
		return err
	}
	res, err := s.profile.ListResidences(ctx, personID)
	if err != nil {
		return err
	}
	emails, err := s.profile.ListEmails(ctx, personID)
	if err != nil {
		return err
	}
	phones, err := s.profile.ListPhones(ctx, personID)
	if err != nil {
		return err
	}
	callSigns, err := s.profile.ListCallSigns(ctx, personID)
	if err != nil {
		return err
	}
	messengers, err := s.profile.ListMessengerLinks(ctx, personID)
	if err != nil {
		return err
	}
	socials, err := s.profile.ListSocialAccounts(ctx, personID)
	if err != nil {
		return err
	}
	out.Citizenships = toAPICitizenships(cits)
	out.Residences = toAPIResidences(res)
	out.Emails = toAPIEmails(emails)
	out.Phones = toAPIPhones(phones)
	out.CallSigns = toAPICallSigns(callSigns)
	out.MessengerLinks = toAPIMessengerLinks(messengers)
	out.SocialAccounts = toAPISocialAccounts(socials)
	return nil
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
		VariantKind:   v.VariantKind,
		Source:        strPtrOrNil(v.Source),
		Confidence:    strPtrOrNil(v.Confidence),
	}
}

func toAPIPhysicalDescription(d domain.PhysicalDescription) personapi.PhysicalDescription {
	return personapi.PhysicalDescription{
		Id:            d.ID,
		PersonId:      d.PersonID,
		HeightCm:      d.HeightCm,
		WeightKg:      d.WeightKg,
		EyeColorId:    strPtrOrNil(d.EyeColorID),
		HairColorId:   strPtrOrNil(d.HairColorID),
		Build:         strPtrOrNil(d.Build),
		BloodType:     strPtrOrNil(d.BloodType),
		EffectiveFrom: d.EffectiveFrom,
		EffectiveTo:   strPtrOrNil(d.EffectiveTo),
		Source:        strPtrOrNil(d.Source),
		Confidence:    strPtrOrNil(d.Confidence),
	}
}

func toAPIDistinguishingMark(m domain.DistinguishingMark) personapi.DistinguishingMark {
	return personapi.DistinguishingMark{
		Id:           m.ID,
		PersonId:     m.PersonID,
		Kind:         m.Kind,
		BodyLocation: strPtrOrNil(m.BodyLocation),
		Description:  strPtrOrNil(m.Description),
		Source:       strPtrOrNil(m.Source),
		Confidence:   strPtrOrNil(m.Confidence),
	}
}

func toAPIEthnicity(e domain.Ethnicity) personapi.Ethnicity {
	return personapi.Ethnicity{
		Id:         e.ID,
		PersonId:   e.PersonID,
		Code:       e.Code,
		Name:       strPtrOrNil(e.Name),
		LegalBasis: e.LegalBasis,
		Status:     e.Status,
		Source:     strPtrOrNil(e.Source),
		Confidence: strPtrOrNil(e.Confidence),
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

func toAPIAddress(a domain.Address) personapi.Address {
	return personapi.Address{
		Id:             a.ID,
		PersonId:       a.PersonID,
		LocationId:     a.LocationID,
		Role:           a.Role,
		ValidFrom:      a.ValidFrom,
		ValidTo:        strPtrOrNil(a.ValidTo),
		IsPrimary:      a.IsPrimary,
		PrivacySeeking: a.PrivacySeeking,
		Source:         strPtrOrNil(a.Source),
		Confidence:     strPtrOrNil(a.Confidence),
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
		EnrollmentId: strPtrOrNil(sp.EnrollmentID), EducationRole: strPtrOrNil(sp.EducationRole),
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
		errors.Is(err, domain.ErrLanguageNotFound),
		errors.Is(err, domain.ErrAddressNotFound),
		errors.Is(err, domain.ErrPartyMembershipNotFound),
		errors.Is(err, domain.ErrGovernmentPositionNotFound),
		errors.Is(err, domain.ErrLobbyingNotFound),
		errors.Is(err, domain.ErrExternalReferenceNotFound),
		errors.Is(err, domain.ErrRegulatorySanctionNotFound),
		errors.Is(err, domain.ErrRelationshipNotFound),
		errors.Is(err, domain.ErrEthnicityNotFound),
		errors.Is(err, domain.ErrNameAliasNotFound),
		errors.Is(err, domain.ErrPhysicalDescriptionNotFound),
		errors.Is(err, domain.ErrDistinguishingMarkNotFound),
		errors.Is(err, domain.ErrCryptoWalletNotFound),
		errors.Is(err, domain.ErrPersonalityNotFound),
		errors.Is(err, domain.ErrPoliticalLeaningNotFound),
		errors.Is(err, domain.ErrHealthRecordNotFound),
		errors.Is(err, domain.ErrInsuranceNotFound),
		errors.Is(err, domain.ErrLegalRecordNotFound):
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
	case errors.Is(err, domain.ErrUnknownLanguage):
		return personapi.NewPersonInvalid("language does not exist or is not a level='language' languoid")
	case errors.Is(err, domain.ErrLanguageConflict):
		return personapi.NewPersonConflict("the person already speaks this language")
	case errors.Is(err, domain.ErrMergeNotProvisional):
		return personapi.NewPersonInvalid("merge source is not a provisional person")
	case errors.Is(err, domain.ErrMergeIntoInvalid):
		return personapi.NewPersonInvalid("merge target must be a distinct, non-provisional person")
	case errors.Is(err, domain.ErrColorMismatch):
		return personapi.NewPersonInvalid("color is not in the expected eye/hair palette (D-Color)")
	case errors.Is(err, domain.ErrUnknownLocation):
		return personapi.NewPersonInvalid("location does not exist (D-PersonAddresses)")
	case errors.Is(err, domain.ErrUnknownLegalBasis):
		return personapi.NewPersonInvalid("legal basis does not exist")
	case errors.Is(err, domain.ErrUnknownEthnicityType):
		return personapi.NewPersonInvalid("ethnicity type does not exist")
	case errors.Is(err, domain.ErrWatchlistUnavailable):
		return personapi.NewPersonInvalid("watchlist screening is not configured (D-Watchlists)")
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
