// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package adapters is the personprofile module's pgx/sqlc-backed persistence adapter (D-PersonModuleSplit,
// review-2026-07 R-09). It owns the person directory's non-encrypted, person-owned tables: citizenships,
// residences, addresses, the contact channels (email/phone/call-sign/messenger/social), the SPEAKS
// languages, the person<->person relationships, and the non-encrypted institutional ties. It compiles
// against its own generated query package (personprofilesql) and shares the person aggregate's domain
// kernel (internal/person/domain).
package adapters

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	personprofilesql "github.com/olehmushka/go-oikumenea/internal/personprofile/adapters/personprofilesql"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// Repository is the personprofile pgx/sqlc-backed persistence adapter, bound to a single db.DBTX — the
// pool for reads, or a caller-supplied transaction so a write and its audit row commit together (D-Audit).
type Repository struct {
	q *personprofilesql.Queries
	c db.DBTX // raw command surface, for the handful of statements not expressed as sqlc queries
}

// NewRepository binds a repository to the given command surface.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: personprofilesql.New(conn), c: conn}
}

func (r *Repository) UpsertCitizenship(ctx context.Context, c domain.Citizenship) (domain.Citizenship, error) {
	row, err := r.q.UpsertCitizenship(ctx, personprofilesql.UpsertCitizenshipParams{
		PersonID:   c.PersonID,
		CountryID:  c.Country,
		Basis:      c.Basis,
		AcquiredOn: dateText(c.AcquiredOn),
		LostOn:     dateText(c.LostOn),
		IsPrimary:  c.IsPrimary,
	})
	if err != nil {
		return domain.Citizenship{}, mapWriteErr(err)
	}
	return toCitizenship(row), nil
}

func (r *Repository) ClearPrimaryCitizenships(ctx context.Context, personID string) error {
	return r.q.ClearPrimaryCitizenships(ctx, personID)
}

func (r *Repository) DeleteCitizenship(ctx context.Context, personID, country string) error {
	if _, err := r.q.DeleteCitizenship(ctx, personprofilesql.DeleteCitizenshipParams{PersonID: personID, CountryID: country}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrCitizenshipNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListCitizenships(ctx context.Context, personID string) ([]domain.Citizenship, error) {
	rows, err := r.q.ListCitizenships(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Citizenship, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCitizenship(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- residences

// UpsertResidence inserts a new row when r.ID is empty, otherwise replaces the named row.

// UpsertResidence inserts a new row when r.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertResidence(ctx context.Context, res domain.Residence) (domain.Residence, error) {
	if res.ID == "" {
		row, err := r.q.InsertResidence(ctx, personprofilesql.InsertResidenceParams{
			PersonID:  res.PersonID,
			CountryID: res.Country,
			Region:    text(res.Region),
			ValidFrom: dateText(res.ValidFrom),
			ValidTo:   dateText(res.ValidTo),
		})
		if err != nil {
			return domain.Residence{}, mapWriteErr(err)
		}
		return toResidence(row), nil
	}
	row, err := r.q.UpdateResidence(ctx, personprofilesql.UpdateResidenceParams{
		CountryID: res.Country,
		Region:    text(res.Region),
		ValidFrom: dateText(res.ValidFrom),
		ValidTo:   dateText(res.ValidTo),
		ID:        res.ID,
		PersonID:  res.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Residence{}, domain.ErrResidenceNotFound
		}
		return domain.Residence{}, mapWriteErr(err)
	}
	return toResidence(row), nil
}

func (r *Repository) DeleteResidence(ctx context.Context, personID, residenceID string) error {
	if _, err := r.q.DeleteResidence(ctx, personprofilesql.DeleteResidenceParams{ID: residenceID, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrResidenceNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListResidences(ctx context.Context, personID string) ([]domain.Residence, error) {
	rows, err := r.q.ListResidences(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Residence, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResidence(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- addresses (D-PersonAddresses, M32)

// UpsertAddress inserts a new address when a.ID is empty, otherwise replaces the named row.

// UpsertAddress inserts a new address when a.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertAddress(ctx context.Context, a domain.Address) (domain.Address, error) {
	if a.ID == "" {
		row, err := r.q.InsertAddress(ctx, personprofilesql.InsertAddressParams{
			PersonID:       a.PersonID,
			LocationID:     a.LocationID,
			Role:           a.Role,
			ValidFrom:      dateText(a.ValidFrom),
			ValidTo:        dateText(a.ValidTo),
			IsPrimary:      a.IsPrimary,
			PrivacySeeking: a.PrivacySeeking,
			Source:         a.Source,
			Confidence:     a.Confidence,
		})
		if err != nil {
			return domain.Address{}, mapWriteErr(err)
		}
		return toAddress(row), nil
	}
	row, err := r.q.UpdateAddress(ctx, personprofilesql.UpdateAddressParams{
		LocationID:     a.LocationID,
		Role:           a.Role,
		ValidFrom:      dateText(a.ValidFrom),
		ValidTo:        dateText(a.ValidTo),
		IsPrimary:      a.IsPrimary,
		PrivacySeeking: a.PrivacySeeking,
		Source:         a.Source,
		Confidence:     a.Confidence,
		ID:             a.ID,
		PersonID:       a.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Address{}, domain.ErrAddressNotFound
		}
		return domain.Address{}, mapWriteErr(err)
	}
	return toAddress(row), nil
}

func (r *Repository) DeleteAddress(ctx context.Context, personID, addressID string) error {
	if _, err := r.q.DeleteAddress(ctx, personprofilesql.DeleteAddressParams{ID: addressID, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrAddressNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListAddresses(ctx context.Context, personID string) ([]domain.Address, error) {
	rows, err := r.q.ListAddresses(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Address, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAddress(row))
	}
	return out, nil
}

func (r *Repository) DemotePrimaryAddresses(ctx context.Context, personID, exceptID string) error {
	return r.q.DemotePrimaryAddresses(ctx, personprofilesql.DemotePrimaryAddressesParams{PersonID: personID, ExceptID: exceptID})
}

// ---------------------------------------------------------------- emails

// UpsertEmail inserts a new row when e.ID is empty, otherwise replaces the named row.

// UpsertEmail inserts a new row when e.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertEmail(ctx context.Context, e domain.Email) (domain.Email, error) {
	if e.ID == "" {
		row, err := r.q.InsertEmail(ctx, personprofilesql.InsertEmailParams{
			PersonID:  e.PersonID,
			TypeCode:  e.TypeCode,
			Address:   e.Address,
			Provider:  text(e.Provider),
			IsPrimary: e.IsPrimary,
		})
		if err != nil {
			return domain.Email{}, mapWriteErr(err)
		}
		return toEmail(row), nil
	}
	row, err := r.q.UpdateEmail(ctx, personprofilesql.UpdateEmailParams{
		TypeCode:  e.TypeCode,
		Address:   e.Address,
		Provider:  text(e.Provider),
		IsPrimary: e.IsPrimary,
		ID:        e.ID,
		PersonID:  e.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Email{}, domain.ErrEmailNotFound
		}
		return domain.Email{}, mapWriteErr(err)
	}
	return toEmail(row), nil
}

func (r *Repository) ClearPrimaryEmails(ctx context.Context, personID string) error {
	return r.q.ClearPrimaryEmails(ctx, personID)
}

func (r *Repository) DeleteEmail(ctx context.Context, personID, emailID string) error {
	if _, err := r.q.DeleteEmail(ctx, personprofilesql.DeleteEmailParams{ID: emailID, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrEmailNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListEmails(ctx context.Context, personID string) ([]domain.Email, error) {
	rows, err := r.q.ListEmails(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Email, 0, len(rows))
	for _, row := range rows {
		out = append(out, toEmail(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- phones

// UpsertPhone inserts a new row when p.ID is empty, otherwise replaces the named row.

// UpsertPhone inserts a new row when p.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertPhone(ctx context.Context, p domain.Phone) (domain.Phone, error) {
	if p.ID == "" {
		row, err := r.q.InsertPhone(ctx, personprofilesql.InsertPhoneParams{
			PersonID:    p.PersonID,
			TypeCode:    p.TypeCode,
			Number:      p.Number,
			CountryCode: text(p.Country),
			IsPrimary:   p.IsPrimary,
		})
		if err != nil {
			return domain.Phone{}, mapWriteErr(err)
		}
		return toPhone(row), nil
	}
	row, err := r.q.UpdatePhone(ctx, personprofilesql.UpdatePhoneParams{
		TypeCode:    p.TypeCode,
		Number:      p.Number,
		CountryCode: text(p.Country),
		IsPrimary:   p.IsPrimary,
		ID:          p.ID,
		PersonID:    p.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Phone{}, domain.ErrPhoneNotFound
		}
		return domain.Phone{}, mapWriteErr(err)
	}
	return toPhone(row), nil
}

func (r *Repository) ClearPrimaryPhones(ctx context.Context, personID string) error {
	return r.q.ClearPrimaryPhones(ctx, personID)
}

func (r *Repository) DeletePhone(ctx context.Context, personID, phoneID string) error {
	if _, err := r.q.DeletePhone(ctx, personprofilesql.DeletePhoneParams{ID: phoneID, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrPhoneNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListPhones(ctx context.Context, personID string) ([]domain.Phone, error) {
	rows, err := r.q.ListPhones(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Phone, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPhone(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- call signs

// UpsertCallSign inserts a new row when c.ID is empty, otherwise replaces the named row.

// UpsertCallSign inserts a new row when c.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertCallSign(ctx context.Context, c domain.CallSign) (domain.CallSign, error) {
	if c.ID == "" {
		row, err := r.q.InsertCallSign(ctx, personprofilesql.InsertCallSignParams{
			PersonID:  c.PersonID,
			CallSign:  c.CallSign,
			IsPrimary: c.IsPrimary,
		})
		if err != nil {
			return domain.CallSign{}, mapWriteErr(err)
		}
		return toCallSign(row), nil
	}
	row, err := r.q.UpdateCallSign(ctx, personprofilesql.UpdateCallSignParams{
		CallSign:  c.CallSign,
		IsPrimary: c.IsPrimary,
		ID:        c.ID,
		PersonID:  c.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CallSign{}, domain.ErrCallSignNotFound
		}
		return domain.CallSign{}, mapWriteErr(err)
	}
	return toCallSign(row), nil
}

func (r *Repository) ClearPrimaryCallSigns(ctx context.Context, personID string) error {
	return r.q.ClearPrimaryCallSigns(ctx, personID)
}

func (r *Repository) DeleteCallSign(ctx context.Context, personID, callSignID string) error {
	if _, err := r.q.DeleteCallSign(ctx, personprofilesql.DeleteCallSignParams{ID: callSignID, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrCallSignNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListCallSigns(ctx context.Context, personID string) ([]domain.CallSign, error) {
	rows, err := r.q.ListCallSigns(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CallSign, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCallSign(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- contact-kind catalogs

func (r *Repository) ListEmailTypes(ctx context.Context) ([]domain.ContactType, error) {
	rows, err := r.q.ListEmailTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ContactType, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.ContactType{Code: row.Code, Name: row.Name, Status: row.Status, SortOrder: int(row.SortOrder.Int32)})
	}
	return out, nil
}

func (r *Repository) ListPhoneTypes(ctx context.Context) ([]domain.ContactType, error) {
	rows, err := r.q.ListPhoneTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ContactType, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.ContactType{Code: row.Code, Name: row.Name, Status: row.Status, SortOrder: int(row.SortOrder.Int32)})
	}
	return out, nil
}

// ---------------------------------------------------------------- platform catalog

func (r *Repository) ListPlatforms(ctx context.Context) ([]domain.Platform, error) {
	rows, err := r.q.ListPlatforms(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Platform, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPlatform(row))
	}
	return out, nil
}

func (r *Repository) GetPlatform(ctx context.Context, code string) (domain.Platform, error) {
	row, err := r.q.GetPlatform(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Platform{}, domain.ErrUnknownPlatform
		}
		return domain.Platform{}, err
	}
	return toPlatform(row), nil
}

// ---------------------------------------------------------------- messenger links

func (r *Repository) PhonePersonID(ctx context.Context, phoneID string) (string, error) {
	id, err := r.q.PhonePersonID(ctx, phoneID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrPhoneNotFound
		}
		return "", err
	}
	return id, nil
}

func (r *Repository) EmailPersonID(ctx context.Context, emailID string) (string, error) {
	id, err := r.q.EmailPersonID(ctx, emailID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrEmailNotFound
		}
		return "", err
	}
	return id, nil
}

// UpsertMessengerLink inserts a new link when m.ID is empty, otherwise replaces the named row.

// UpsertMessengerLink inserts a new link when m.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertMessengerLink(ctx context.Context, m domain.MessengerLink) (domain.MessengerLink, error) {
	if m.ID == "" {
		row, err := r.q.InsertMessengerLink(ctx, personprofilesql.InsertMessengerLinkParams{
			PhoneID:      text(m.PhoneID),
			EmailID:      text(m.EmailID),
			PlatformCode: m.PlatformCode,
			IsPrimary:    m.IsPrimary,
			VerifiedAt:   ts(m.VerifiedAt),
		})
		if err != nil {
			return domain.MessengerLink{}, mapWriteErr(err)
		}
		return toMessengerLink(row), nil
	}
	row, err := r.q.UpdateMessengerLink(ctx, personprofilesql.UpdateMessengerLinkParams{
		PhoneID:      text(m.PhoneID),
		EmailID:      text(m.EmailID),
		PlatformCode: m.PlatformCode,
		IsPrimary:    m.IsPrimary,
		VerifiedAt:   ts(m.VerifiedAt),
		ID:           m.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.MessengerLink{}, domain.ErrMessengerLinkNotFound
		}
		return domain.MessengerLink{}, mapWriteErr(err)
	}
	return toMessengerLink(row), nil
}

func (r *Repository) ClearPrimaryMessengerLinks(ctx context.Context, personID string) error {
	return r.q.ClearPrimaryMessengerLinks(ctx, personID)
}

func (r *Repository) DeleteMessengerLink(ctx context.Context, personID, linkID string) error {
	if _, err := r.q.DeleteMessengerLink(ctx, personprofilesql.DeleteMessengerLinkParams{ID: linkID, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrMessengerLinkNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListMessengerLinks(ctx context.Context, personID string) ([]domain.MessengerLink, error) {
	rows, err := r.q.ListMessengerLinks(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.MessengerLink, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMessengerLink(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- social accounts

func (r *Repository) InsertSocialAccount(ctx context.Context, a domain.SocialAccount) (domain.SocialAccount, error) {
	row, err := r.q.InsertSocialAccount(ctx, personprofilesql.InsertSocialAccountParams{
		PersonID:             a.PersonID,
		PlatformCode:         a.PlatformCode,
		PlatformUserID:       text(a.PlatformUserID),
		Handle:               a.Handle,
		DisplayName:          text(a.DisplayName),
		ProfileUrl:           text(a.ProfileURL),
		Language:             text(a.Language),
		PlatformVerified:     a.PlatformVerified,
		VerifiedByOperatorAt: ts(a.VerifiedByOperatorAt),
		Source:               a.Source,
		Confidence:           a.Confidence,
		IsPrimary:            a.IsPrimary,
	})
	if err != nil {
		return domain.SocialAccount{}, mapWriteErr(err)
	}
	return toSocialAccount(row), nil
}

func (r *Repository) UpdateSocialAccount(ctx context.Context, a domain.SocialAccount) (domain.SocialAccount, error) {
	row, err := r.q.UpdateSocialAccount(ctx, personprofilesql.UpdateSocialAccountParams{
		PlatformCode:         a.PlatformCode,
		PlatformUserID:       text(a.PlatformUserID),
		Handle:               a.Handle,
		DisplayName:          text(a.DisplayName),
		ProfileUrl:           text(a.ProfileURL),
		Language:             text(a.Language),
		PlatformVerified:     a.PlatformVerified,
		VerifiedByOperatorAt: ts(a.VerifiedByOperatorAt),
		Source:               a.Source,
		Confidence:           a.Confidence,
		IsPrimary:            a.IsPrimary,
		ID:                   a.ID,
		PersonID:             a.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SocialAccount{}, domain.ErrSocialAccountNotFound
		}
		return domain.SocialAccount{}, mapWriteErr(err)
	}
	return toSocialAccount(row), nil
}

func (r *Repository) GetSocialAccount(ctx context.Context, personID, accountID string) (domain.SocialAccount, error) {
	row, err := r.q.GetSocialAccount(ctx, personprofilesql.GetSocialAccountParams{ID: accountID, PersonID: personID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SocialAccount{}, domain.ErrSocialAccountNotFound
		}
		return domain.SocialAccount{}, err
	}
	return toSocialAccount(row), nil
}

func (r *Repository) ClearPrimarySocialAccounts(ctx context.Context, personID string) error {
	return r.q.ClearPrimarySocialAccounts(ctx, personID)
}

func (r *Repository) DeleteSocialAccount(ctx context.Context, personID, accountID string) error {
	if _, err := r.q.DeleteSocialAccount(ctx, personprofilesql.DeleteSocialAccountParams{ID: accountID, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrSocialAccountNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListSocialAccounts(ctx context.Context, personID string) ([]domain.SocialAccount, error) {
	rows, err := r.q.ListSocialAccounts(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.SocialAccount, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSocialAccount(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- social account handle history

func (r *Repository) InsertSocialAccountHandle(ctx context.Context, h domain.SocialAccountHandle) (domain.SocialAccountHandle, error) {
	row, err := r.q.InsertSocialAccountHandle(ctx, personprofilesql.InsertSocialAccountHandleParams{
		AccountID: h.AccountID,
		Handle:    h.Handle,
		ValidFrom: pgtype.Timestamptz{Time: h.ValidFrom, Valid: true},
		ValidTo:   ts(h.ValidTo),
	})
	if err != nil {
		return domain.SocialAccountHandle{}, mapWriteErr(err)
	}
	return toSocialAccountHandle(row), nil
}

func (r *Repository) CloseCurrentSocialAccountHandle(ctx context.Context, accountID string) error {
	return r.q.CloseCurrentSocialAccountHandle(ctx, accountID)
}

func (r *Repository) ListSocialAccountHandles(ctx context.Context, accountID string) ([]domain.SocialAccountHandle, error) {
	rows, err := r.q.ListSocialAccountHandles(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.SocialAccountHandle, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSocialAccountHandle(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- person languages (D-Languages, M18)

func (r *Repository) InsertPersonLanguage(ctx context.Context, l domain.PersonLanguage) error {
	if err := r.q.InsertPersonLanguage(ctx, personprofilesql.InsertPersonLanguageParams{
		PersonID:   l.PersonID,
		LanguageID: l.LanguageID,
		CefrLevel:  text(l.CEFRLevel),
		IsNative:   l.IsNative,
	}); err != nil {
		return mapWriteErr(err)
	}
	return nil
}

func (r *Repository) UpdatePersonLanguage(ctx context.Context, l domain.PersonLanguage) error {
	if err := r.q.UpdatePersonLanguage(ctx, personprofilesql.UpdatePersonLanguageParams{
		PersonID:   l.PersonID,
		LanguageID: l.LanguageID,
		CefrLevel:  text(l.CEFRLevel),
		IsNative:   l.IsNative,
	}); err != nil {
		return mapWriteErr(err)
	}
	return nil
}

func (r *Repository) GetPersonLanguage(ctx context.Context, personID, languageID string) (domain.PersonLanguage, error) {
	row, err := r.q.GetPersonLanguage(ctx, personprofilesql.GetPersonLanguageParams{PersonID: personID, LanguageID: languageID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PersonLanguage{}, domain.ErrLanguageNotFound
		}
		return domain.PersonLanguage{}, err
	}
	return domain.PersonLanguage{
		ID:           row.ID,
		PersonID:     row.PersonID,
		LanguageID:   row.LanguageID,
		LanguageName: row.LanguageName,
		CEFRLevel:    row.CefrLevel.String,
		IsNative:     row.IsNative,
	}, nil
}

func (r *Repository) DeletePersonLanguage(ctx context.Context, personID, languageID string) error {
	if _, err := r.q.DeletePersonLanguage(ctx, personprofilesql.DeletePersonLanguageParams{PersonID: personID, LanguageID: languageID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrLanguageNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListPersonLanguages(ctx context.Context, personID string) ([]domain.PersonLanguage, error) {
	rows, err := r.q.ListPersonLanguages(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PersonLanguage, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PersonLanguage{
			ID:           row.ID,
			PersonID:     row.PersonID,
			LanguageID:   row.LanguageID,
			LanguageName: row.LanguageName,
			CEFRLevel:    row.CefrLevel.String,
			IsNative:     row.IsNative,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------- mapping helpers

func toPlatform(r personprofilesql.OikumeneaPersonPlatform) domain.Platform {
	return domain.Platform{
		Code:      r.Code,
		Name:      r.Name,
		Category:  r.Category,
		Status:    r.Status,
		SortOrder: int(r.SortOrder.Int32),
	}
}

// ---------------------------------------------------------------- person↔person relationships (D-PersonRelationships)

func (r *Repository) ListRelationTypes(ctx context.Context) ([]domain.RelationType, error) {
	rows, err := r.q.ListRelationTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RelationType, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRelationType(row))
	}
	return out, nil
}

func (r *Repository) GetRelationType(ctx context.Context, code string) (domain.RelationType, error) {
	row, err := r.q.GetRelationType(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RelationType{}, domain.ErrUnknownRelationType
		}
		return domain.RelationType{}, err
	}
	return toRelationType(row), nil
}

func (r *Repository) HasActivePartnershipExcept(ctx context.Context, personID, exceptID string) (bool, error) {
	return r.q.HasActivePartnershipExcept(ctx, personprofilesql.HasActivePartnershipExceptParams{ExceptID: exceptID, PersonID: personID})
}

// partnerships

// partnerships
func (r *Repository) UpsertPartnership(ctx context.Context, p domain.Partnership) (domain.Partnership, error) {
	if p.ID == "" {
		row, err := r.q.InsertPartnership(ctx, personprofilesql.InsertPartnershipParams{
			PersonIDA: p.PersonIDA, PersonIDB: p.PersonIDB, Status: p.Status,
			EffectiveFrom: dateText(p.EffectiveFrom), EffectiveTo: dateText(p.EffectiveTo),
		})
		if err != nil {
			return domain.Partnership{}, mapWriteErr(err)
		}
		return toPartnership(row), nil
	}
	row, err := r.q.UpdatePartnership(ctx, personprofilesql.UpdatePartnershipParams{
		ID: p.ID, PersonIDA: p.PersonIDA, PersonIDB: p.PersonIDB, Status: p.Status,
		EffectiveFrom: dateText(p.EffectiveFrom), EffectiveTo: dateText(p.EffectiveTo),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Partnership{}, domain.ErrRelationshipNotFound
		}
		return domain.Partnership{}, mapWriteErr(err)
	}
	return toPartnership(row), nil
}

func (r *Repository) ListPartnerships(ctx context.Context, personID string) ([]domain.Partnership, error) {
	rows, err := r.q.ListPartnerships(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Partnership, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPartnership(row))
	}
	return out, nil
}

func (r *Repository) DeletePartnership(ctx context.Context, personID, id string) error {
	return relDelete(func() (string, error) {
		return r.q.DeletePartnership(ctx, personprofilesql.DeletePartnershipParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllPartnerships(ctx context.Context, personID string) error {
	return r.q.DeleteAllPartnerships(ctx, personID)
}

// kinships

// kinships
func (r *Repository) UpsertKinship(ctx context.Context, k domain.Kinship) (domain.Kinship, error) {
	if k.ID == "" {
		row, err := r.q.InsertKinship(ctx, personprofilesql.InsertKinshipParams{ParentID: k.ParentID, ChildID: k.ChildID, Status: k.Status})
		if err != nil {
			return domain.Kinship{}, mapWriteErr(err)
		}
		return toKinship(row), nil
	}
	row, err := r.q.UpdateKinship(ctx, personprofilesql.UpdateKinshipParams{ID: k.ID, ParentID: k.ParentID, ChildID: k.ChildID, Status: k.Status})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Kinship{}, domain.ErrRelationshipNotFound
		}
		return domain.Kinship{}, mapWriteErr(err)
	}
	return toKinship(row), nil
}

func (r *Repository) ListKinships(ctx context.Context, personID string) ([]domain.Kinship, error) {
	rows, err := r.q.ListKinships(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Kinship, 0, len(rows))
	for _, row := range rows {
		out = append(out, toKinship(row))
	}
	return out, nil
}

func (r *Repository) DeleteKinship(ctx context.Context, personID, id string) error {
	return relDelete(func() (string, error) {
		return r.q.DeleteKinship(ctx, personprofilesql.DeleteKinshipParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllKinships(ctx context.Context, personID string) error {
	return r.q.DeleteAllKinships(ctx, personID)
}

// guardianships

// guardianships
func (r *Repository) UpsertGuardianship(ctx context.Context, g domain.Guardianship) (domain.Guardianship, error) {
	if g.ID == "" {
		row, err := r.q.InsertGuardianship(ctx, personprofilesql.InsertGuardianshipParams{
			GuardianID: g.GuardianID, WardID: g.WardID, RelationCode: text(g.RelationCode), Status: g.Status,
			EffectiveFrom: dateText(g.EffectiveFrom), EffectiveTo: dateText(g.EffectiveTo),
		})
		if err != nil {
			return domain.Guardianship{}, mapWriteErr(err)
		}
		return toGuardianship(row), nil
	}
	row, err := r.q.UpdateGuardianship(ctx, personprofilesql.UpdateGuardianshipParams{
		ID: g.ID, GuardianID: g.GuardianID, WardID: g.WardID, RelationCode: text(g.RelationCode), Status: g.Status,
		EffectiveFrom: dateText(g.EffectiveFrom), EffectiveTo: dateText(g.EffectiveTo),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Guardianship{}, domain.ErrRelationshipNotFound
		}
		return domain.Guardianship{}, mapWriteErr(err)
	}
	return toGuardianship(row), nil
}

func (r *Repository) ListGuardianships(ctx context.Context, personID string) ([]domain.Guardianship, error) {
	rows, err := r.q.ListGuardianships(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Guardianship, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGuardianship(row))
	}
	return out, nil
}

func (r *Repository) DeleteGuardianship(ctx context.Context, personID, id string) error {
	return relDelete(func() (string, error) {
		return r.q.DeleteGuardianship(ctx, personprofilesql.DeleteGuardianshipParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllGuardianships(ctx context.Context, personID string) error {
	return r.q.DeleteAllGuardianships(ctx, personID)
}

// sponsorships

// sponsorships
func (r *Repository) UpsertSponsorship(ctx context.Context, s domain.Sponsorship) (domain.Sponsorship, error) {
	if s.ID == "" {
		row, err := r.q.InsertSponsorship(ctx, personprofilesql.InsertSponsorshipParams{
			SponsorID: s.SponsorID, SponsoredID: s.SponsoredID, RelationCode: s.RelationCode, Status: s.Status,
			EffectiveFrom: dateText(s.EffectiveFrom), EffectiveTo: dateText(s.EffectiveTo),
			EnrollmentID: text(s.EnrollmentID), EducationRole: text(s.EducationRole),
		})
		if err != nil {
			return domain.Sponsorship{}, mapWriteErr(err)
		}
		return toSponsorship(row), nil
	}
	row, err := r.q.UpdateSponsorship(ctx, personprofilesql.UpdateSponsorshipParams{
		ID: s.ID, SponsorID: s.SponsorID, SponsoredID: s.SponsoredID, RelationCode: s.RelationCode, Status: s.Status,
		EffectiveFrom: dateText(s.EffectiveFrom), EffectiveTo: dateText(s.EffectiveTo),
		EnrollmentID: text(s.EnrollmentID), EducationRole: text(s.EducationRole),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Sponsorship{}, domain.ErrRelationshipNotFound
		}
		return domain.Sponsorship{}, mapWriteErr(err)
	}
	return toSponsorship(row), nil
}

func (r *Repository) ListSponsorships(ctx context.Context, personID string) ([]domain.Sponsorship, error) {
	rows, err := r.q.ListSponsorships(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Sponsorship, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSponsorship(row))
	}
	return out, nil
}

func (r *Repository) DeleteSponsorship(ctx context.Context, personID, id string) error {
	return relDelete(func() (string, error) {
		return r.q.DeleteSponsorship(ctx, personprofilesql.DeleteSponsorshipParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllSponsorships(ctx context.Context, personID string) error {
	return r.q.DeleteAllSponsorships(ctx, personID)
}

// next of kin

// next of kin
func (r *Repository) UpsertNextOfKin(ctx context.Context, n domain.NextOfKin) (domain.NextOfKin, error) {
	if n.ID == "" {
		row, err := r.q.InsertNextOfKin(ctx, personprofilesql.InsertNextOfKinParams{
			SubjectID: n.SubjectID, ContactID: n.ContactID, RelationCode: text(n.RelationCode),
			Priority: int32(n.Priority), Status: n.Status,
		})
		if err != nil {
			return domain.NextOfKin{}, mapWriteErr(err)
		}
		return toNextOfKin(row), nil
	}
	row, err := r.q.UpdateNextOfKin(ctx, personprofilesql.UpdateNextOfKinParams{
		ID: n.ID, SubjectID: n.SubjectID, ContactID: n.ContactID, RelationCode: text(n.RelationCode),
		Priority: int32(n.Priority), Status: n.Status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.NextOfKin{}, domain.ErrRelationshipNotFound
		}
		return domain.NextOfKin{}, mapWriteErr(err)
	}
	return toNextOfKin(row), nil
}

func (r *Repository) ListNextOfKin(ctx context.Context, personID string) ([]domain.NextOfKin, error) {
	rows, err := r.q.ListNextOfKin(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.NextOfKin, 0, len(rows))
	for _, row := range rows {
		out = append(out, toNextOfKin(row))
	}
	return out, nil
}

func (r *Repository) DeleteNextOfKin(ctx context.Context, personID, id string) error {
	return relDelete(func() (string, error) {
		return r.q.DeleteNextOfKin(ctx, personprofilesql.DeleteNextOfKinParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllNextOfKin(ctx context.Context, personID string) error {
	return r.q.DeleteAllNextOfKin(ctx, personID)
}

// associations

// associations
func (r *Repository) UpsertAssociation(ctx context.Context, a domain.Association) (domain.Association, error) {
	if a.ID == "" {
		row, err := r.q.InsertAssociation(ctx, personprofilesql.InsertAssociationParams{
			PersonIDA: a.PersonIDA, PersonIDB: a.PersonIDB, RelationCode: text(a.RelationCode), Kind: a.Kind, Status: a.Status,
		})
		if err != nil {
			return domain.Association{}, mapWriteErr(err)
		}
		return toAssociation(row), nil
	}
	row, err := r.q.UpdateAssociation(ctx, personprofilesql.UpdateAssociationParams{
		ID: a.ID, PersonIDA: a.PersonIDA, PersonIDB: a.PersonIDB, RelationCode: text(a.RelationCode), Kind: a.Kind, Status: a.Status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Association{}, domain.ErrRelationshipNotFound
		}
		return domain.Association{}, mapWriteErr(err)
	}
	return toAssociation(row), nil
}

func (r *Repository) ListAssociations(ctx context.Context, personID string) ([]domain.Association, error) {
	rows, err := r.q.ListAssociations(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Association, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAssociation(row))
	}
	return out, nil
}

func (r *Repository) DeleteAssociation(ctx context.Context, personID, id string) error {
	return relDelete(func() (string, error) {
		return r.q.DeleteAssociation(ctx, personprofilesql.DeleteAssociationParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllAssociations(ctx context.Context, personID string) error {
	return r.q.DeleteAllAssociations(ctx, personID)
}

// relDelete maps a person-scoped soft-delete-by-id (RETURNING id) to ErrRelationshipNotFound when no
// row matched (wrong id, already deleted, or the person is not an endpoint).

func toRelationType(r personprofilesql.OikumeneaPersonRelationType) domain.RelationType {
	return domain.RelationType{
		Code:      r.Code,
		Name:      r.Name,
		Category:  r.Category,
		Status:    r.Status,
		SortOrder: int(r.SortOrder.Int32),
	}
}

func toPartnership(r personprofilesql.OikumeneaPersonPartnership) domain.Partnership {
	return domain.Partnership{
		ID: r.ID, PersonIDA: r.PersonIDA, PersonIDB: r.PersonIDB, Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo),
	}
}

func toKinship(r personprofilesql.OikumeneaPersonKinship) domain.Kinship {
	return domain.Kinship{ID: r.ID, ParentID: r.ParentID, ChildID: r.ChildID, Status: r.Status}
}

func toGuardianship(r personprofilesql.OikumeneaPersonGuardianship) domain.Guardianship {
	return domain.Guardianship{
		ID: r.ID, GuardianID: r.GuardianID, WardID: r.WardID, RelationCode: r.RelationCode.String, Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo),
	}
}

func toSponsorship(r personprofilesql.OikumeneaPersonSponsorship) domain.Sponsorship {
	return domain.Sponsorship{
		ID: r.ID, SponsorID: r.SponsorID, SponsoredID: r.SponsoredID, RelationCode: r.RelationCode, Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo),
		EnrollmentID: strText(r.EnrollmentID), EducationRole: strText(r.EducationRole),
	}
}

func toNextOfKin(r personprofilesql.OikumeneaPersonNextOfKin) domain.NextOfKin {
	return domain.NextOfKin{
		ID: r.ID, SubjectID: r.SubjectID, ContactID: r.ContactID, RelationCode: r.RelationCode.String,
		Priority: int(r.Priority), Status: r.Status,
	}
}

func toAssociation(r personprofilesql.OikumeneaPersonAssociation) domain.Association {
	return domain.Association{
		ID: r.ID, PersonIDA: r.PersonIDA, PersonIDB: r.PersonIDB, RelationCode: r.RelationCode.String, Kind: r.Kind, Status: r.Status,
	}
}

func toMessengerLink(r personprofilesql.OikumeneaPersonMessengerLink) domain.MessengerLink {
	return domain.MessengerLink{
		ID:           r.ID,
		PhoneID:      r.PhoneID.String,
		EmailID:      r.EmailID.String,
		PlatformCode: r.PlatformCode,
		IsPrimary:    r.IsPrimary,
		VerifiedAt:   tsPtr(r.VerifiedAt),
	}
}

func toSocialAccount(r personprofilesql.OikumeneaPersonSocialAccount) domain.SocialAccount {
	return domain.SocialAccount{
		ID:                   r.ID,
		PersonID:             r.PersonID,
		PlatformCode:         r.PlatformCode,
		PlatformUserID:       r.PlatformUserID.String,
		Handle:               r.Handle,
		DisplayName:          r.DisplayName.String,
		ProfileURL:           r.ProfileUrl.String,
		Language:             r.Language.String,
		PlatformVerified:     r.PlatformVerified,
		VerifiedByOperatorAt: tsPtr(r.VerifiedByOperatorAt),
		Source:               r.Source,
		Confidence:           r.Confidence,
		IsPrimary:            r.IsPrimary,
	}
}

func toSocialAccountHandle(r personprofilesql.OikumeneaPersonSocialAccountHandle) domain.SocialAccountHandle {
	return domain.SocialAccountHandle{
		ID:        r.ID,
		AccountID: r.AccountID,
		Handle:    r.Handle,
		ValidFrom: r.ValidFrom.Time,
		ValidTo:   tsPtr(r.ValidTo),
	}
}

func toEmail(r personprofilesql.OikumeneaPersonEmail) domain.Email {
	return domain.Email{
		ID:        r.ID,
		PersonID:  r.PersonID,
		TypeCode:  r.TypeCode,
		Address:   r.Address,
		Provider:  r.Provider.String,
		IsPrimary: r.IsPrimary,
	}
}

func toPhone(r personprofilesql.OikumeneaPersonPhone) domain.Phone {
	return domain.Phone{
		ID:        r.ID,
		PersonID:  r.PersonID,
		TypeCode:  r.TypeCode,
		Number:    r.Number,
		Country:   r.CountryID.String,
		IsPrimary: r.IsPrimary,
	}
}

func toCallSign(r personprofilesql.OikumeneaPersonCallSign) domain.CallSign {
	return domain.CallSign{
		ID:        r.ID,
		PersonID:  r.PersonID,
		CallSign:  r.CallSign,
		IsPrimary: r.IsPrimary,
	}
}

func toCitizenship(r personprofilesql.OikumeneaPersonCitizenship) domain.Citizenship {
	return domain.Citizenship{
		ID:         r.ID,
		PersonID:   r.PersonID,
		Country:    r.CountryID,
		Basis:      r.Basis,
		AcquiredOn: dateStr(r.AcquiredOn),
		LostOn:     dateStr(r.LostOn),
		IsPrimary:  r.IsPrimary,
	}
}

func toResidence(r personprofilesql.OikumeneaPersonResidence) domain.Residence {
	return domain.Residence{
		ID:        r.ID,
		PersonID:  r.PersonID,
		Country:   r.CountryID,
		Region:    r.Region.String,
		ValidFrom: dateStr(r.ValidFrom),
		ValidTo:   dateStr(r.ValidTo),
	}
}

func toAddress(r personprofilesql.OikumeneaPersonAddress) domain.Address {
	return domain.Address{
		ID:             r.ID,
		PersonID:       r.PersonID,
		LocationID:     r.LocationID,
		Role:           r.Role,
		ValidFrom:      dateStr(r.ValidFrom),
		ValidTo:        dateStr(r.ValidTo),
		IsPrimary:      r.IsPrimary,
		PrivacySeeking: r.PrivacySeeking,
		Source:         r.Source,
		Confidence:     r.Confidence,
	}
}

// mapWriteErr translates Postgres constraint violations into the module's domain sentinels. Unique
// violations distinguish the person code from the active-citizenship index; FK violations name the
// offending reference (rank / locale / country) so the transport can return a precise error.

// UpsertGovernmentPosition inserts a new row when g.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertGovernmentPosition(ctx context.Context, g domain.GovernmentPosition) (domain.GovernmentPosition, error) {
	if g.ID == "" {
		row, err := r.q.InsertGovernmentPosition(ctx, personprofilesql.InsertGovernmentPositionParams{
			PersonID:   g.PersonID,
			Title:      g.Title,
			Body:       g.Body,
			OrgID:      text(g.OrgID),
			CountryID:  text(g.CountryID),
			Level:      g.Level,
			RoleType:   text(g.RoleType),
			ValidFrom:  dateText(g.ValidFrom),
			ValidTo:    dateText(g.ValidTo),
			PepTrigger: g.PEPTrigger,
			Source:     g.Source,
			Confidence: g.Confidence,
		})
		if err != nil {
			return domain.GovernmentPosition{}, mapWriteErr(err)
		}
		return toGovernmentPosition(row), nil
	}
	row, err := r.q.UpdateGovernmentPosition(ctx, personprofilesql.UpdateGovernmentPositionParams{
		Title:      g.Title,
		Body:       g.Body,
		OrgID:      text(g.OrgID),
		CountryID:  text(g.CountryID),
		Level:      g.Level,
		RoleType:   text(g.RoleType),
		ValidFrom:  dateText(g.ValidFrom),
		ValidTo:    dateText(g.ValidTo),
		PepTrigger: g.PEPTrigger,
		Source:     g.Source,
		Confidence: g.Confidence,
		ID:         g.ID,
		PersonID:   g.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.GovernmentPosition{}, domain.ErrGovernmentPositionNotFound
		}
		return domain.GovernmentPosition{}, mapWriteErr(err)
	}
	return toGovernmentPosition(row), nil
}

func (r *Repository) DeleteGovernmentPosition(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteGovernmentPosition(ctx, personprofilesql.DeleteGovernmentPositionParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrGovernmentPositionNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListGovernmentPositions(ctx context.Context, personID string) ([]domain.GovernmentPosition, error) {
	rows, err := r.q.ListGovernmentPositions(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.GovernmentPosition, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGovernmentPosition(row))
	}
	return out, nil
}

func (r *Repository) IsPoliticallyExposed(ctx context.Context, personID string) (bool, error) {
	n, err := r.q.CountActivePEPPositions(ctx, personID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpsertLobbyingRelationship inserts a new row when l.ID is empty, otherwise replaces the named row.

// UpsertLobbyingRelationship inserts a new row when l.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertLobbyingRelationship(ctx context.Context, l domain.LobbyingRelationship) (domain.LobbyingRelationship, error) {
	if l.Issues == nil {
		l.Issues = []string{}
	}
	if l.ID == "" {
		row, err := r.q.InsertLobbyingRelationship(ctx, personprofilesql.InsertLobbyingRelationshipParams{
			PersonID:        l.PersonID,
			Registrant:      l.Registrant,
			Client:          text(l.Client),
			LegislativeBody: text(l.LegislativeBody),
			Issues:          l.Issues,
			FilingID:        text(l.FilingID),
			SourceUrl:       text(l.SourceURL),
			ValidFrom:       dateText(l.ValidFrom),
			ValidTo:         dateText(l.ValidTo),
			Source:          l.Source,
			Confidence:      l.Confidence,
		})
		if err != nil {
			return domain.LobbyingRelationship{}, mapWriteErr(err)
		}
		return toLobbying(row), nil
	}
	row, err := r.q.UpdateLobbyingRelationship(ctx, personprofilesql.UpdateLobbyingRelationshipParams{
		Registrant:      l.Registrant,
		Client:          text(l.Client),
		LegislativeBody: text(l.LegislativeBody),
		Issues:          l.Issues,
		FilingID:        text(l.FilingID),
		SourceUrl:       text(l.SourceURL),
		ValidFrom:       dateText(l.ValidFrom),
		ValidTo:         dateText(l.ValidTo),
		Source:          l.Source,
		Confidence:      l.Confidence,
		ID:              l.ID,
		PersonID:        l.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.LobbyingRelationship{}, domain.ErrLobbyingNotFound
		}
		return domain.LobbyingRelationship{}, mapWriteErr(err)
	}
	return toLobbying(row), nil
}

func (r *Repository) DeleteLobbyingRelationship(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteLobbyingRelationship(ctx, personprofilesql.DeleteLobbyingRelationshipParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrLobbyingNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListLobbyingRelationships(ctx context.Context, personID string) ([]domain.LobbyingRelationship, error) {
	rows, err := r.q.ListLobbyingRelationships(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LobbyingRelationship, 0, len(rows))
	for _, row := range rows {
		out = append(out, toLobbying(row))
	}
	return out, nil
}

// UpsertExternalReference inserts idempotently by (person, url) when r.ID is empty; otherwise replaces
// the named row by RID.

// UpsertExternalReference inserts idempotently by (person, url) when r.ID is empty; otherwise replaces
// the named row by RID.
func (r *Repository) UpsertExternalReference(ctx context.Context, e domain.ExternalReference) (domain.ExternalReference, error) {
	if e.Categories == nil {
		e.Categories = []string{}
	}
	if e.ID == "" {
		row, err := r.q.UpsertExternalReference(ctx, personprofilesql.UpsertExternalReferenceParams{
			PersonID:    e.PersonID,
			Kind:        e.Kind,
			Url:         e.URL,
			ExternalID:  text(e.ExternalID),
			Categories:  e.Categories,
			LastChecked: ts(e.LastChecked),
			Disputed:    e.Disputed,
			Source:      e.Source,
			Confidence:  e.Confidence,
		})
		if err != nil {
			return domain.ExternalReference{}, mapWriteErr(err)
		}
		return toExternalReference(row), nil
	}
	row, err := r.q.UpdateExternalReference(ctx, personprofilesql.UpdateExternalReferenceParams{
		Kind:        e.Kind,
		Url:         e.URL,
		ExternalID:  text(e.ExternalID),
		Categories:  e.Categories,
		LastChecked: ts(e.LastChecked),
		Disputed:    e.Disputed,
		Source:      e.Source,
		Confidence:  e.Confidence,
		ID:          e.ID,
		PersonID:    e.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ExternalReference{}, domain.ErrExternalReferenceNotFound
		}
		return domain.ExternalReference{}, mapWriteErr(err)
	}
	return toExternalReference(row), nil
}

func (r *Repository) DeleteExternalReference(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteExternalReference(ctx, personprofilesql.DeleteExternalReferenceParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrExternalReferenceNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListExternalReferences(ctx context.Context, personID string) ([]domain.ExternalReference, error) {
	rows, err := r.q.ListExternalReferences(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ExternalReference, 0, len(rows))
	for _, row := range rows {
		out = append(out, toExternalReference(row))
	}
	return out, nil
}

// M33 row mappers.

func toGovernmentPosition(r personprofilesql.OikumeneaPersonGovernmentPosition) domain.GovernmentPosition {
	return domain.GovernmentPosition{
		ID:         r.ID,
		PersonID:   r.PersonID,
		Title:      r.Title,
		Body:       r.Body,
		OrgID:      strText(r.OrgID),
		CountryID:  strText(r.CountryID),
		Level:      r.Level,
		RoleType:   strText(r.RoleType),
		ValidFrom:  dateStr(r.ValidFrom),
		ValidTo:    dateStr(r.ValidTo),
		PEPTrigger: r.PepTrigger,
		Source:     r.Source,
		Confidence: r.Confidence,
		CreatedAt:  r.CreatedAt.Time,
		UpdatedAt:  r.UpdatedAt.Time,
	}
}

func toLobbying(r personprofilesql.OikumeneaPersonLobbyingRelationship) domain.LobbyingRelationship {
	return domain.LobbyingRelationship{
		ID:              r.ID,
		PersonID:        r.PersonID,
		Registrant:      r.Registrant,
		Client:          strText(r.Client),
		LegislativeBody: strText(r.LegislativeBody),
		Issues:          r.Issues,
		FilingID:        strText(r.FilingID),
		SourceURL:       strText(r.SourceUrl),
		ValidFrom:       dateStr(r.ValidFrom),
		ValidTo:         dateStr(r.ValidTo),
		Source:          r.Source,
		Confidence:      r.Confidence,
		CreatedAt:       r.CreatedAt.Time,
		UpdatedAt:       r.UpdatedAt.Time,
	}
}

func toExternalReference(r personprofilesql.OikumeneaPersonExternalReference) domain.ExternalReference {
	return domain.ExternalReference{
		ID:          r.ID,
		PersonID:    r.PersonID,
		Kind:        r.Kind,
		URL:         r.Url,
		ExternalID:  strText(r.ExternalID),
		Categories:  r.Categories,
		LastChecked: tsPtr(r.LastChecked),
		Disputed:    r.Disputed,
		Source:      r.Source,
		Confidence:  r.Confidence,
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}
}

// ---------------------------------------------------------------- watchlists & regulatory exposure (M34)

// UpsertWatchlistMatch inserts or (on the partial-unique person_id) refreshes the single screening result.

// relDelete maps a person-scoped soft-delete-by-id (RETURNING id) to ErrRelationshipNotFound when no
// row matched (wrong id, already deleted, or the person is not an endpoint).
func relDelete(del func() (string, error)) error {
	if _, err := del(); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRelationshipNotFound
		}
		return err
	}
	return nil
}

// mapWriteErr translates Postgres constraint violations into the module's domain sentinels. Unique
// violations distinguish the person code from the active-citizenship index; FK violations name the
// offending reference (rank / locale / country) so the transport can return a precise error.
func mapWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	name := pgErr.ConstraintName
	switch pgErr.Code {
	case "23505": // unique_violation
		switch {
		case strings.Contains(name, "citizenship"):
			return domain.ErrCitizenshipConflict
		case strings.Contains(name, "email"):
			return domain.ErrEmailConflict
		case strings.Contains(name, "phone"):
			return domain.ErrPhoneConflict
		case strings.Contains(name, "call_sign"):
			return domain.ErrCallSignConflict
		case strings.Contains(name, "messenger_link"):
			return domain.ErrMessengerLinkConflict
		case strings.Contains(name, "social_account"):
			return domain.ErrSocialAccountConflict
		case strings.Contains(name, "partnership"):
			return domain.ErrPartnershipConflict
		case strings.Contains(name, "kinship"), strings.Contains(name, "guardianship"),
			strings.Contains(name, "sponsorship"), strings.Contains(name, "next_of_kin"),
			strings.Contains(name, "association"):
			return domain.ErrRelationshipConflict
		case strings.Contains(name, "person_languages"):
			return domain.ErrLanguageConflict
		case strings.Contains(name, "code"):
			return domain.ErrCodeConflict
		}
	case "23503": // foreign_key_violation
		switch {
		case strings.Contains(name, "is_language"):
			return domain.ErrUnknownLanguage
		case strings.Contains(name, "relation_code"):
			return domain.ErrUnknownRelationType
		case strings.Contains(name, "rank"):
			return domain.ErrUnknownRank
		case strings.Contains(name, "locale"):
			return domain.ErrUnknownLocale
		case strings.Contains(name, "platform_code"):
			return domain.ErrUnknownPlatform
		case strings.Contains(name, "type_code"):
			return domain.ErrUnknownContactType
		case strings.Contains(name, "legal_basis"):
			return domain.ErrUnknownLegalBasis
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

// strText reads a nullable text column into a plain string ("" when NULL).

// strText reads a nullable text column into a plain string ("" when NULL).
func strText(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// textPtr maps a patch pointer: nil leaves the column unchanged (NULL narg → COALESCE keeps it); a
// non-nil pointer (including "") sets the column, so an empty string clears an optional name part.

// textPtr maps a patch pointer: nil leaves the column unchanged (NULL narg → COALESCE keeps it); a
// non-nil pointer (including "") sets the column, so an empty string clears an optional name part.
func textPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

// int4 maps an optional int to a nullable integer column (nil => NULL).

// int4 maps an optional int to a nullable integer column (nil => NULL).
func int4(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

// int4Ptr reads a nullable integer column into an *int (nil when NULL).

// int4Ptr reads a nullable integer column into an *int (nil when NULL).
func int4Ptr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	out := int(v.Int32)
	return &out
}

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

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	out := t.Time
	return &out
}

// ts maps an optional instant to a nullable timestamptz column (nil => NULL).

// ts maps an optional instant to a nullable timestamptz column (nil => NULL).
func ts(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// numArg maps an optional float to a nullable numeric column (via its decimal string form; nil => NULL).

// numArg maps an optional float to a nullable numeric column (via its decimal string form; nil => NULL).
func numArg(p *float64) pgtype.Numeric {
	var n pgtype.Numeric
	if p == nil {
		return n
	}
	if err := n.Scan(strconv.FormatFloat(*p, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// numPtr maps a stored numeric back into an optional float64 (via its string Value()).

// numPtr maps a stored numeric back into an optional float64 (via its string Value()).
func numPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Value()
	if err != nil || v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

// float8Arg maps an optional float to a nullable double-precision column (nil => NULL).

// float8Arg maps an optional float to a nullable double-precision column (nil => NULL).
func float8Arg(p *float64) pgtype.Float8 {
	if p == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *p, Valid: true}
}

// float8Ptr reads a nullable double-precision column into an *float64 (nil when NULL).

// float8Ptr reads a nullable double-precision column into an *float64 (nil when NULL).
func float8Ptr(v pgtype.Float8) *float64 {
	if !v.Valid {
		return nil
	}
	out := v.Float64
	return &out
}

// ---------------------------------------------------------------- institutional & political ties (M33)

// InsertPartyMembership stores a new encrypted party membership (the party envelope is sealed upstream).

// nonNilStrs returns s, or an empty (non-nil) slice so a NULL never reaches a NOT NULL text[] column.
func nonNilStrs(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// PersonExists reports whether the person exists and is not soft-deleted — the parent-existence guard
// personprofile writes/reads run before touching a person's directory rows (D-PersonModuleSplit, R-09).
// A reviewed cross-module read of the person core aggregate; ErrNotFound when absent.
func (r *Repository) PersonExists(ctx context.Context, personID string) error {
	if _, err := r.q.PersonExists(ctx, personID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

// ErasePerson hard-deletes all of a person's personprofile-owned rows in FK-safe order — the purge
// erasure path (D-PersonModuleSplit, review-2026-07 R-09). It is invoked in the purge transaction by the
// module's PersonPurged subscriber (SubscribePersonPurge). All of these are non-encrypted, person-owned
// directory data (pii:basic/contact) with no legal-retention requirement, so they are hard-deleted.
func (r *Repository) ErasePerson(ctx context.Context, personID string) error {
	steps := []func(context.Context, string) error{
		r.q.DeleteAllCitizenships,
		r.q.DeleteAllResidences,
		r.q.DeleteAllAddresses, // pii:contact (D-PersonAddresses, M32)
		r.q.DeleteAllEmails,
		r.q.DeleteAllPhones,
		r.q.DeleteAllCallSigns,
		r.q.DeleteAllMessengerLinks,
		r.q.DeleteAllSocialAccountHandles, // handles before their accounts (FK)
		r.q.DeleteAllSocialAccounts,
		r.q.DeleteAllPersonLanguages, // D-Languages, M18
		// person↔person relationships (D-PersonRelationships) — erased on EITHER endpoint's purge.
		r.q.DeleteAllPartnerships,
		r.q.DeleteAllKinships,
		r.q.DeleteAllGuardianships,
		r.q.DeleteAllSponsorships,
		r.q.DeleteAllNextOfKin,
		r.q.DeleteAllAssociations,
		// non-encrypted institutional & political ties (D-InstitutionalTies, M33) — hard-deleted.
		r.q.DeleteAllGovernmentPositions,
		r.q.DeleteAllLobbyingRelationships,
		r.q.DeleteAllExternalReferences,
	}
	for _, step := range steps {
		if err := step(ctx, personID); err != nil {
			return err
		}
	}
	return nil
}
