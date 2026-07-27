// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Person-owned directory data orchestration (R-09 split from the person core): citizenships, residences,
// contact channels (email/phone/call-sign/messenger/social), SPEAKS languages, person↔person
// relationships, and the profile catalog reads. All writes record an audit row in the same transaction
// (D-Audit) with only non-PII identifiers in the payload.
package application

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

// ---------------------------------------------------------------- citizenships

// UpsertCitizenship adds or replaces the active citizenship for (person, country). When marked
// primary, the person's other active citizenships are demoted in the same transaction.
func (s *Service) UpsertCitizenship(ctx context.Context, c domain.Citizenship) (domain.Citizenship, error) {
	if c.Basis == "" {
		c.Basis = domain.DefaultBasis
	}
	if err := c.Validate(); err != nil {
		return domain.Citizenship{}, err
	}
	var out domain.Citizenship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, c.PersonID); err != nil {
			return err
		}
		if c.IsPrimary {
			if err := repo.ClearPrimaryCitizenships(ctx, c.PersonID); err != nil {
				return err
			}
		}
		created, err := repo.UpsertCitizenship(ctx, c)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.citizenship.upsert", c.PersonID, map[string]any{"id": c.PersonID, "country": c.Country})
	})
	return out, err
}

// DeleteCitizenship removes the active citizenship for a country.
func (s *Service) DeleteCitizenship(ctx context.Context, personID, country string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeleteCitizenship(ctx, personID, country); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.citizenship.delete", personID, map[string]any{"id": personID, "country": country})
	})
}

// ListCitizenships lists a person's citizenships (the person must exist).
func (s *Service) ListCitizenships(ctx context.Context, personID string) ([]domain.Citizenship, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListCitizenships(ctx, personID)
}

// ---------------------------------------------------------------- residences

// UpsertResidence adds a residence row (or replaces one when r.ID is set) and records the action.
func (s *Service) UpsertResidence(ctx context.Context, r domain.Residence) (domain.Residence, error) {
	if err := r.Validate(); err != nil {
		return domain.Residence{}, err
	}
	var out domain.Residence
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, r.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertResidence(ctx, r)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.residence.upsert", r.PersonID, map[string]any{"id": r.PersonID, "residenceId": created.ID})
	})
	return out, err
}

// DeleteResidence removes a person's residence row by id.
func (s *Service) DeleteResidence(ctx context.Context, personID, residenceID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeleteResidence(ctx, personID, residenceID); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.residence.delete", personID, map[string]any{"id": personID, "residenceId": residenceID})
	})
}

// ListResidences lists a person's residence history (the person must exist).
func (s *Service) ListResidences(ctx context.Context, personID string) ([]domain.Residence, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListResidences(ctx, personID)
}

// ---------------------------------------------------------------- emails

// UpsertEmail validates and adds/replaces a contact email, deriving the provider from the address
// domain on write (D-PersonContactChannels). When marked primary, the person's other active emails
// are demoted in the same transaction.
func (s *Service) UpsertEmail(ctx context.Context, e domain.Email) (domain.Email, error) {
	e.Address = normalizeEmail(e.Address)
	if err := e.Validate(); err != nil {
		return domain.Email{}, err
	}
	e.Provider = emailProvider(e.Address)
	var out domain.Email
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, e.PersonID); err != nil {
			return err
		}
		if e.IsPrimary {
			if err := repo.ClearPrimaryEmails(ctx, e.PersonID); err != nil {
				return err
			}
		}
		created, err := repo.UpsertEmail(ctx, e)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.email.upsert", e.PersonID, map[string]any{"id": e.PersonID, "emailId": created.ID})
	})
	return out, err
}

// DeleteEmail removes a person's contact email by id.
func (s *Service) DeleteEmail(ctx context.Context, personID, emailID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeleteEmail(ctx, personID, emailID); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.email.delete", personID, map[string]any{"id": personID, "emailId": emailID})
	})
}

// ListEmails lists a person's contact emails (the person must exist).
func (s *Service) ListEmails(ctx context.Context, personID string) ([]domain.Email, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListEmails(ctx, personID)
}

// ---------------------------------------------------------------- phones

// UpsertPhone validates and adds/replaces a contact phone, normalizing the number to E.164 and
// deriving its country on write (D-PersonContactChannels). When marked primary, the person's other
// active phones are demoted in the same transaction.
func (s *Service) UpsertPhone(ctx context.Context, p domain.Phone) (domain.Phone, error) {
	if err := p.Validate(); err != nil {
		return domain.Phone{}, err
	}
	number, country, err := normalizePhone(p.Number)
	if err != nil {
		return domain.Phone{}, err
	}
	p.Number, p.Country = number, country
	var out domain.Phone
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, p.PersonID); err != nil {
			return err
		}
		if p.IsPrimary {
			if err := repo.ClearPrimaryPhones(ctx, p.PersonID); err != nil {
				return err
			}
		}
		created, err := repo.UpsertPhone(ctx, p)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.phone.upsert", p.PersonID, map[string]any{"id": p.PersonID, "phoneId": created.ID})
	})
	return out, err
}

// DeletePhone removes a person's contact phone by id.
func (s *Service) DeletePhone(ctx context.Context, personID, phoneID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeletePhone(ctx, personID, phoneID); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.phone.delete", personID, map[string]any{"id": personID, "phoneId": phoneID})
	})
}

// ListPhones lists a person's contact phones (the person must exist).
func (s *Service) ListPhones(ctx context.Context, personID string) ([]domain.Phone, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListPhones(ctx, personID)
}

// ---------------------------------------------------------------- call signs

// UpsertCallSign adds/replaces a call sign (D-PersonContactChannels). When marked primary, the
// person's other active call signs are demoted in the same transaction.
func (s *Service) UpsertCallSign(ctx context.Context, c domain.CallSign) (domain.CallSign, error) {
	if err := c.Validate(); err != nil {
		return domain.CallSign{}, err
	}
	var out domain.CallSign
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, c.PersonID); err != nil {
			return err
		}
		if c.IsPrimary {
			if err := repo.ClearPrimaryCallSigns(ctx, c.PersonID); err != nil {
				return err
			}
		}
		created, err := repo.UpsertCallSign(ctx, c)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.call-sign.upsert", c.PersonID, map[string]any{"id": c.PersonID, "callSignId": created.ID})
	})
	return out, err
}

// DeleteCallSign removes a person's call sign by id.
func (s *Service) DeleteCallSign(ctx context.Context, personID, callSignID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeleteCallSign(ctx, personID, callSignID); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.call-sign.delete", personID, map[string]any{"id": personID, "callSignId": callSignID})
	})
}

// ListCallSigns lists a person's call signs (the person must exist).
func (s *Service) ListCallSigns(ctx context.Context, personID string) ([]domain.CallSign, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListCallSigns(ctx, personID)
}

// ---------------------------------------------------------------- messenger links (D-PersonSocialChannels)

// UpsertMessengerLink adds/replaces a messenger reachability link over one of the person's phones or
// emails. It verifies the channel is held by the person and that the platform is a `messenger`-category
// platform, demoting other primaries when marked primary — all in the same transaction.
func (s *Service) UpsertMessengerLink(ctx context.Context, personID string, m domain.MessengerLink) (domain.MessengerLink, error) {
	if err := m.Validate(); err != nil {
		return domain.MessengerLink{}, err
	}
	var out domain.MessengerLink
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, personID); err != nil {
			return err
		}
		// Holder scope: the annotated phone/email must belong to this person.
		var owner string
		var err error
		if m.PhoneID != "" {
			owner, err = repo.PhonePersonID(ctx, m.PhoneID)
		} else {
			owner, err = repo.EmailPersonID(ctx, m.EmailID)
		}
		if err != nil {
			return err
		}
		if owner != personID {
			return domain.ErrChannelNotOwned
		}
		// The platform must exist and be a messenger platform (D-PersonSocialChannels).
		plat, err := repo.GetPlatform(ctx, m.PlatformCode)
		if err != nil {
			return err
		}
		if !plat.IsMessenger() {
			return domain.ErrPlatformNotMessenger
		}
		if m.IsPrimary {
			if err := repo.ClearPrimaryMessengerLinks(ctx, personID); err != nil {
				return err
			}
		}
		created, err := repo.UpsertMessengerLink(ctx, m)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.messenger-link.upsert", personID, map[string]any{"id": personID, "messengerLinkId": created.ID})
	})
	return out, err
}

// DeleteMessengerLink removes a person's messenger link by id (holder-scoped).
func (s *Service) DeleteMessengerLink(ctx context.Context, personID, linkID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeleteMessengerLink(ctx, personID, linkID); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.messenger-link.delete", personID, map[string]any{"id": personID, "messengerLinkId": linkID})
	})
}

// ListMessengerLinks lists a person's messenger links (the person must exist).
func (s *Service) ListMessengerLinks(ctx context.Context, personID string) ([]domain.MessengerLink, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListMessengerLinks(ctx, personID)
}

// ---------------------------------------------------------------- social accounts (D-PersonSocialChannels)

// UpsertSocialAccount adds/replaces a standalone social account. The platform must exist; the @handle is
// normalized and a profile URL derived when absent; when marked primary the person's other social
// accounts are demoted; and the account's handle-rename history is maintained (a new period opens on
// create and on every handle change) — all in the same transaction.
func (s *Service) UpsertSocialAccount(ctx context.Context, a domain.SocialAccount) (domain.SocialAccount, error) {
	a.Handle = normalizeHandle(a.Handle)
	if a.Confidence == "" {
		a.Confidence = domain.DefaultConfidence
	}
	if err := a.Validate(); err != nil {
		return domain.SocialAccount{}, err
	}
	if a.ProfileURL == "" {
		a.ProfileURL = deriveProfileURL(a.PlatformCode, a.Handle)
	}
	var out domain.SocialAccount
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, a.PersonID); err != nil {
			return err
		}
		if _, err := repo.GetPlatform(ctx, a.PlatformCode); err != nil {
			return err
		}
		if a.IsPrimary {
			if err := repo.ClearPrimarySocialAccounts(ctx, a.PersonID); err != nil {
				return err
			}
		}
		var prevHandle string
		if a.ID != "" {
			existing, err := repo.GetSocialAccount(ctx, a.PersonID, a.ID)
			if err != nil {
				return err
			}
			prevHandle = existing.Handle
		}
		var saved domain.SocialAccount
		var err error
		if a.ID == "" {
			saved, err = repo.InsertSocialAccount(ctx, a)
		} else {
			saved, err = repo.UpdateSocialAccount(ctx, a)
		}
		if err != nil {
			return err
		}
		// Open a new handle-history period on create, or on a handle rename: close the current period
		// and record the new handle (D-PersonSocialChannels), so a rename never breaks the link.
		if a.ID == "" || saved.Handle != prevHandle {
			if a.ID != "" {
				if err := repo.CloseCurrentSocialAccountHandle(ctx, saved.ID); err != nil {
					return err
				}
			}
			if _, err := repo.InsertSocialAccountHandle(ctx, domain.SocialAccountHandle{
				AccountID: saved.ID,
				Handle:    saved.Handle,
				ValidFrom: s.now(),
			}); err != nil {
				return err
			}
		}
		out = saved
		return s.record(ctx, tx, "person.social-account.upsert", a.PersonID, map[string]any{"id": a.PersonID, "socialAccountId": saved.ID})
	})
	return out, err
}

// DeleteSocialAccount removes a person's social account by id (its handle history cascades).
func (s *Service) DeleteSocialAccount(ctx context.Context, personID, accountID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeleteSocialAccount(ctx, personID, accountID); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.social-account.delete", personID, map[string]any{"id": personID, "socialAccountId": accountID})
	})
}

// UpsertPersonLanguage adds (or updates the proficiency of) a language the person speaks (D-Languages,
// M18; keyed on person+language). The person must exist; the languoid existence + level='language'
// constraint is enforced by the composite FK (a violation surfaces as ErrUnknownLanguage). Returns the
// stored row joined to the languoid name.
func (s *Service) UpsertPersonLanguage(ctx context.Context, l domain.PersonLanguage) (domain.PersonLanguage, error) {
	if err := l.Validate(); err != nil {
		return domain.PersonLanguage{}, err
	}
	var out domain.PersonLanguage
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, l.PersonID); err != nil {
			return err
		}
		_, err := repo.GetPersonLanguage(ctx, l.PersonID, l.LanguageID)
		switch {
		case err == nil:
			if err := repo.UpdatePersonLanguage(ctx, l); err != nil {
				return err
			}
		case errors.Is(err, domain.ErrLanguageNotFound):
			if err := repo.InsertPersonLanguage(ctx, l); err != nil {
				return err
			}
		default:
			return err
		}
		saved, err := repo.GetPersonLanguage(ctx, l.PersonID, l.LanguageID)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.language.upsert", l.PersonID, map[string]any{"id": l.PersonID, "languageId": l.LanguageID})
	})
	return out, err
}

// DeletePersonLanguage removes a language the person speaks, by languoid id.
func (s *Service) DeletePersonLanguage(ctx context.Context, personID, languageID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeletePersonLanguage(ctx, personID, languageID); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.language.delete", personID, map[string]any{"id": personID, "languageId": languageID})
	})
}

// ListPersonLanguages lists the languages a person speaks (the person must exist).
func (s *Service) ListPersonLanguages(ctx context.Context, personID string) ([]domain.PersonLanguage, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListPersonLanguages(ctx, personID)
}

// ListSocialAccounts lists a person's social accounts (the person must exist).
func (s *Service) ListSocialAccounts(ctx context.Context, personID string) ([]domain.SocialAccount, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListSocialAccounts(ctx, personID)
}

// ListSocialAccountHandles lists one social account's handle-rename history (holder-scoped: the account
// must belong to the person).
func (s *Service) ListSocialAccountHandles(ctx context.Context, personID, accountID string) ([]domain.SocialAccountHandle, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetSocialAccount(ctx, personID, accountID); err != nil {
		return nil, err
	}
	return repo.ListSocialAccountHandles(ctx, accountID)
}

// ListPlatforms returns the instance-admin social/messenger platform catalog (read; no person scope).
func (s *Service) ListPlatforms(ctx context.Context) ([]domain.Platform, error) {
	return s.newRepo(s.pool).ListPlatforms(ctx)
}

// ListEmailTypes / ListPhoneTypes return the instance-admin contact-kind catalogs (reads; no person
// scope). The transport assembles the translatable name maps.
func (s *Service) ListEmailTypes(ctx context.Context) ([]domain.ContactType, error) {
	return s.newRepo(s.pool).ListEmailTypes(ctx)
}

func (s *Service) ListPhoneTypes(ctx context.Context) ([]domain.ContactType, error) {
	return s.newRepo(s.pool).ListPhoneTypes(ctx)
}
