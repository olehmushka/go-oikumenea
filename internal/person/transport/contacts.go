// Package transport (R-09 file split): citizenship, residence, contact-channel and language handlers for the one PersonService Conjure surface.
package transport

import (
	"context"

	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// ---------------------------------------------------------------- citizenships

func (s Service) ListCitizenships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Citizenship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	cs, err := s.profile.ListCitizenships(ctx, personID)
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
	created, err := s.profile.UpsertCitizenship(ctx, domain.Citizenship{
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
	if err := s.profile.DeleteCitizenship(ctx, personID, country); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- residences

func (s Service) ListResidences(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Residence, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	rs, err := s.profile.ListResidences(ctx, personID)
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
	created, err := s.profile.UpsertResidence(ctx, domain.Residence{
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
	if err := s.profile.DeleteResidence(ctx, personID, residenceID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- emails

func (s Service) ListEmails(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Email, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	es, err := s.profile.ListEmails(ctx, personID)
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
	created, err := s.profile.UpsertEmail(ctx, domain.Email{
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
	if err := s.profile.DeleteEmail(ctx, personID, emailID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- phones

func (s Service) ListPhones(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Phone, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ps, err := s.profile.ListPhones(ctx, personID)
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
	created, err := s.profile.UpsertPhone(ctx, domain.Phone{
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
	if err := s.profile.DeletePhone(ctx, personID, phoneID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- call signs

func (s Service) ListCallSigns(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.CallSign, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	cs, err := s.profile.ListCallSigns(ctx, personID)
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
	created, err := s.profile.UpsertCallSign(ctx, domain.CallSign{
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
	if err := s.profile.DeleteCallSign(ctx, personID, callSignID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- messenger links (D-PersonSocialChannels)

func (s Service) ListMessengerLinks(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.MessengerLink, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ls, err := s.profile.ListMessengerLinks(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	return toAPIMessengerLinks(ls), nil
}

func (s Service) UpsertMessengerLink(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertMessengerLinkRequest) (personapi.MessengerLink, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.MessengerLink{}, err
	}
	created, err := s.profile.UpsertMessengerLink(ctx, personID, domain.MessengerLink{
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
	if err := s.profile.DeleteMessengerLink(ctx, personID, messengerLinkID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- social accounts (D-PersonSocialChannels)

func (s Service) ListSocialAccounts(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.SocialAccount, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	as, err := s.profile.ListSocialAccounts(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	return toAPISocialAccounts(as), nil
}

func (s Service) UpsertSocialAccount(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertSocialAccountRequest) (personapi.SocialAccount, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.SocialAccount{}, err
	}
	created, err := s.profile.UpsertSocialAccount(ctx, domain.SocialAccount{
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
	if err := s.profile.DeleteSocialAccount(ctx, personID, socialAccountID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ListPersonLanguages implements GET /persons/{personId}/languages (D-Languages, M18).
func (s Service) ListPersonLanguages(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.PersonLanguage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ls, err := s.profile.ListPersonLanguages(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	return s.toAPIPersonLanguages(ctx, ls)
}

// UpsertPersonLanguage implements PUT /persons/{personId}/languages.
func (s Service) UpsertPersonLanguage(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertPersonLanguageRequest) (personapi.PersonLanguage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.PersonLanguage{}, err
	}
	saved, err := s.profile.UpsertPersonLanguage(ctx, domain.PersonLanguage{
		PersonID:   personID,
		LanguageID: req.LanguageId,
		CEFRLevel:  derefOr(req.CefrLevel, ""),
		IsNative:   derefOr(req.IsNative, false),
	})
	if err != nil {
		return personapi.PersonLanguage{}, s.mapError(ctx, err, personID)
	}
	out, err := s.toAPIPersonLanguages(ctx, []domain.PersonLanguage{saved})
	if err != nil {
		return personapi.PersonLanguage{}, err
	}
	return out[0], nil
}

// DeletePersonLanguage implements DELETE /persons/{personId}/languages/{languageId}.
func (s Service) DeletePersonLanguage(ctx context.Context, token bearertoken.Token, personID, languageID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.profile.DeletePersonLanguage(ctx, personID, languageID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// toAPIPersonLanguages maps the domain rows to the API shape, assembling each languoid's locale->text
// name map (D-i18n) from its default-locale name + the localization store (entity type "languoid",
// consistent with the language module).
func (s Service) toAPIPersonLanguages(ctx context.Context, ls []domain.PersonLanguage) ([]personapi.PersonLanguage, error) {
	defaults := make(map[string]string, len(ls))
	for _, l := range ls {
		defaults[l.LanguageID] = l.LanguageName
	}
	names, err := s.loc.NamesByID(ctx, languoidEntity, defaults)
	if err != nil {
		return nil, werror.WrapWithContextParams(ctx, err, "assemble languoid names failed")
	}
	out := make([]personapi.PersonLanguage, 0, len(ls))
	for _, l := range ls {
		pl := personapi.PersonLanguage{
			Id:         l.ID,
			PersonId:   l.PersonID,
			LanguageId: l.LanguageID,
			Name:       names[l.LanguageID],
			IsNative:   l.IsNative,
		}
		if l.CEFRLevel != "" {
			c := l.CEFRLevel
			pl.CefrLevel = &c
		}
		out = append(out, pl)
	}
	return out, nil
}

func (s Service) ListSocialAccountHandles(ctx context.Context, token bearertoken.Token, personID, socialAccountID string) ([]personapi.SocialAccountHandle, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	hs, err := s.profile.ListSocialAccountHandles(ctx, personID, socialAccountID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.SocialAccountHandle, 0, len(hs))
	for _, h := range hs {
		out = append(out, toAPISocialAccountHandle(h))
	}
	return out, nil
}
