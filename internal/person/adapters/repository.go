// Package adapters implements the person domain ports against infrastructure: the pgx/sqlc
// repository over the oikumenea.person_* tables. It depends on the database, never the reverse
// (overview.md). Generated sqlc code lives in the personsql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/person/adapters/personsql"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX —
// the pool for reads, or a caller-supplied transaction so a write and its audit row commit together
// (D-Audit).
type Repository struct {
	q *personsql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: personsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- persons

func (r *Repository) InsertPerson(ctx context.Context, p domain.Person) (domain.Person, error) {
	row, err := r.q.InsertPerson(ctx, personsql.InsertPersonParams{
		Code:           text(p.Code),
		DisplayName:    p.DisplayName,
		Title:          text(p.Title),
		Given:          text(p.Given),
		Given2:         text(p.Given2),
		Surname:        text(p.Surname),
		SurnamePrefix:  text(p.SurnamePrefix),
		Surname2:       text(p.Surname2),
		Generation:     text(p.Generation),
		Credentials:    text(p.Credentials),
		Preferred:      text(p.Preferred),
		Birthdate:      dateText(p.Birthdate),
		Sex:            p.Sex,
		CountryOfBirth: text(p.CountryOfBirth),
		Attributes:     p.Attributes,
		RankID:         text(p.RankID),
	})
	if err != nil {
		return domain.Person{}, mapWriteErr(err)
	}
	return toPerson(row), nil
}

func (r *Repository) GetPerson(ctx context.Context, id string) (domain.Person, error) {
	row, err := r.q.GetPerson(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, err
	}
	return toPerson(row), nil
}

// GetActivePersonByCode looks up an active person by their stable `code` (used by
// identity-federation JIT link-on-match and the first-admin bootstrap). ErrNotFound when no active
// person carries that code.
func (r *Repository) GetActivePersonByCode(ctx context.Context, code string) (domain.Person, error) {
	row, err := r.q.GetActivePersonByCode(ctx, pgtype.Text{String: code, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, err
	}
	return toPerson(row), nil
}

func (r *Repository) UpdatePerson(ctx context.Context, id string, patch domain.PersonPatch) (domain.Person, error) {
	row, err := r.q.UpdatePerson(ctx, personsql.UpdatePersonParams{
		DisplayName:    textPtr(patch.DisplayName),
		Title:          textPtr(patch.Title),
		Given:          textPtr(patch.Given),
		Given2:         textPtr(patch.Given2),
		Surname:        textPtr(patch.Surname),
		SurnamePrefix:  textPtr(patch.SurnamePrefix),
		Surname2:       textPtr(patch.Surname2),
		Generation:     textPtr(patch.Generation),
		Credentials:    textPtr(patch.Credentials),
		Preferred:      textPtr(patch.Preferred),
		Birthdate:      datePtr(patch.Birthdate),
		Sex:            textPtr(patch.Sex),
		CountryOfBirth: textPtr(patch.CountryOfBirth),
		Attributes:     patch.Attributes,
		ID:             id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, mapWriteErr(err)
	}
	return toPerson(row), nil
}

func (r *Repository) ListPersons(ctx context.Context, after string, limit int) ([]domain.Person, error) {
	rows, err := r.q.ListPersons(ctx, personsql.ListPersonsParams{After: after, Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Person, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPerson(row))
	}
	return out, nil
}

func (r *Repository) SetRank(ctx context.Context, id string, rankID *string) (domain.Person, error) {
	row, err := r.q.SetRank(ctx, personsql.SetRankParams{RankID: textPtr(rankID), ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, mapWriteErr(err)
	}
	return toPerson(row), nil
}

// ---------------------------------------------------------------- lifecycle

func (r *Repository) Deactivate(ctx context.Context, id string, purgeAfter time.Time) (domain.Person, error) {
	row, err := r.q.DeactivatePerson(ctx, personsql.DeactivatePersonParams{
		PurgeAfter: pgtype.Timestamptz{Time: purgeAfter, Valid: true},
		ID:         id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, err
	}
	return toPerson(row), nil
}

func (r *Repository) Reactivate(ctx context.Context, id string) (domain.Person, error) {
	row, err := r.q.ReactivatePerson(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, err
	}
	return toPerson(row), nil
}

// Purge erases the person's PII and removes all child rows in the same transaction, keeping the id
// row as a tombstone (audit history references it).
func (r *Repository) Purge(ctx context.Context, id string) (domain.Person, error) {
	if err := r.q.DeleteAllNameVariants(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllCitizenships(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllResidences(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllEmails(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllPhones(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllCallSigns(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllMessengerLinks(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllSocialAccountHandles(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllSocialAccounts(ctx, id); err != nil {
		return domain.Person{}, err
	}
	// person↔person relationships (D-PersonRelationships) — erased on EITHER endpoint's purge.
	if err := r.q.DeleteAllPartnerships(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllKinships(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllGuardianships(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllSponsorships(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllNextOfKin(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if err := r.q.DeleteAllAssociations(ctx, id); err != nil {
		return domain.Person{}, err
	}
	row, err := r.q.PurgePerson(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, err
	}
	return toPerson(row), nil
}

// ---------------------------------------------------------------- name variants

func (r *Repository) UpsertNameVariant(ctx context.Context, v domain.NameVariant) (domain.NameVariant, error) {
	row, err := r.q.UpsertNameVariant(ctx, personsql.UpsertNameVariantParams{
		PersonID:      v.PersonID,
		Locale:        v.Locale,
		DisplayName:   v.DisplayName,
		Title:         text(v.Title),
		Given:         text(v.Given),
		Given2:        text(v.Given2),
		Surname:       text(v.Surname),
		SurnamePrefix: text(v.SurnamePrefix),
		Surname2:      text(v.Surname2),
		Generation:    text(v.Generation),
		Credentials:   text(v.Credentials),
		Preferred:     text(v.Preferred),
		IsPrimary:     v.IsPrimary,
	})
	if err != nil {
		return domain.NameVariant{}, mapWriteErr(err)
	}
	return toNameVariant(row), nil
}

func (r *Repository) ClearPrimaryNameVariants(ctx context.Context, personID string) error {
	return r.q.ClearPrimaryNameVariants(ctx, personID)
}

func (r *Repository) DeleteNameVariant(ctx context.Context, personID, locale string) error {
	if _, err := r.q.DeleteNameVariant(ctx, personsql.DeleteNameVariantParams{PersonID: personID, Locale: locale}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNameVariantNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListNameVariants(ctx context.Context, personID string) ([]domain.NameVariant, error) {
	rows, err := r.q.ListNameVariants(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.NameVariant, 0, len(rows))
	for _, row := range rows {
		out = append(out, toNameVariant(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- citizenships

func (r *Repository) UpsertCitizenship(ctx context.Context, c domain.Citizenship) (domain.Citizenship, error) {
	row, err := r.q.UpsertCitizenship(ctx, personsql.UpsertCitizenshipParams{
		PersonID:   c.PersonID,
		Country:    c.Country,
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
	if _, err := r.q.DeleteCitizenship(ctx, personsql.DeleteCitizenshipParams{PersonID: personID, Country: country}); err != nil {
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
func (r *Repository) UpsertResidence(ctx context.Context, res domain.Residence) (domain.Residence, error) {
	if res.ID == "" {
		row, err := r.q.InsertResidence(ctx, personsql.InsertResidenceParams{
			PersonID:  res.PersonID,
			Country:   res.Country,
			Region:    text(res.Region),
			ValidFrom: dateText(res.ValidFrom),
			ValidTo:   dateText(res.ValidTo),
		})
		if err != nil {
			return domain.Residence{}, mapWriteErr(err)
		}
		return toResidence(row), nil
	}
	row, err := r.q.UpdateResidence(ctx, personsql.UpdateResidenceParams{
		Country:   res.Country,
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
	if _, err := r.q.DeleteResidence(ctx, personsql.DeleteResidenceParams{ID: residenceID, PersonID: personID}); err != nil {
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

// ---------------------------------------------------------------- emails

// UpsertEmail inserts a new row when e.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertEmail(ctx context.Context, e domain.Email) (domain.Email, error) {
	if e.ID == "" {
		row, err := r.q.InsertEmail(ctx, personsql.InsertEmailParams{
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
	row, err := r.q.UpdateEmail(ctx, personsql.UpdateEmailParams{
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
	if _, err := r.q.DeleteEmail(ctx, personsql.DeleteEmailParams{ID: emailID, PersonID: personID}); err != nil {
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
func (r *Repository) UpsertPhone(ctx context.Context, p domain.Phone) (domain.Phone, error) {
	if p.ID == "" {
		row, err := r.q.InsertPhone(ctx, personsql.InsertPhoneParams{
			PersonID:  p.PersonID,
			TypeCode:  p.TypeCode,
			Number:    p.Number,
			Country:   text(p.Country),
			IsPrimary: p.IsPrimary,
		})
		if err != nil {
			return domain.Phone{}, mapWriteErr(err)
		}
		return toPhone(row), nil
	}
	row, err := r.q.UpdatePhone(ctx, personsql.UpdatePhoneParams{
		TypeCode:  p.TypeCode,
		Number:    p.Number,
		Country:   text(p.Country),
		IsPrimary: p.IsPrimary,
		ID:        p.ID,
		PersonID:  p.PersonID,
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
	if _, err := r.q.DeletePhone(ctx, personsql.DeletePhoneParams{ID: phoneID, PersonID: personID}); err != nil {
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
func (r *Repository) UpsertCallSign(ctx context.Context, c domain.CallSign) (domain.CallSign, error) {
	if c.ID == "" {
		row, err := r.q.InsertCallSign(ctx, personsql.InsertCallSignParams{
			PersonID:  c.PersonID,
			CallSign:  c.CallSign,
			IsPrimary: c.IsPrimary,
		})
		if err != nil {
			return domain.CallSign{}, mapWriteErr(err)
		}
		return toCallSign(row), nil
	}
	row, err := r.q.UpdateCallSign(ctx, personsql.UpdateCallSignParams{
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
	if _, err := r.q.DeleteCallSign(ctx, personsql.DeleteCallSignParams{ID: callSignID, PersonID: personID}); err != nil {
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
func (r *Repository) UpsertMessengerLink(ctx context.Context, m domain.MessengerLink) (domain.MessengerLink, error) {
	if m.ID == "" {
		row, err := r.q.InsertMessengerLink(ctx, personsql.InsertMessengerLinkParams{
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
	row, err := r.q.UpdateMessengerLink(ctx, personsql.UpdateMessengerLinkParams{
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
	if _, err := r.q.DeleteMessengerLink(ctx, personsql.DeleteMessengerLinkParams{ID: linkID, PersonID: personID}); err != nil {
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
	row, err := r.q.InsertSocialAccount(ctx, personsql.InsertSocialAccountParams{
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
	row, err := r.q.UpdateSocialAccount(ctx, personsql.UpdateSocialAccountParams{
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
	row, err := r.q.GetSocialAccount(ctx, personsql.GetSocialAccountParams{ID: accountID, PersonID: personID})
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
	if _, err := r.q.DeleteSocialAccount(ctx, personsql.DeleteSocialAccountParams{ID: accountID, PersonID: personID}); err != nil {
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
	row, err := r.q.InsertSocialAccountHandle(ctx, personsql.InsertSocialAccountHandleParams{
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

// ---------------------------------------------------------------- mapping helpers

func toPlatform(r personsql.OikumeneaPersonPlatform) domain.Platform {
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
	return r.q.HasActivePartnershipExcept(ctx, personsql.HasActivePartnershipExceptParams{ExceptID: exceptID, PersonID: personID})
}

// partnerships
func (r *Repository) UpsertPartnership(ctx context.Context, p domain.Partnership) (domain.Partnership, error) {
	if p.ID == "" {
		row, err := r.q.InsertPartnership(ctx, personsql.InsertPartnershipParams{
			PersonIDA: p.PersonIDA, PersonIDB: p.PersonIDB, Status: p.Status,
			EffectiveFrom: dateText(p.EffectiveFrom), EffectiveTo: dateText(p.EffectiveTo),
		})
		if err != nil {
			return domain.Partnership{}, mapWriteErr(err)
		}
		return toPartnership(row), nil
	}
	row, err := r.q.UpdatePartnership(ctx, personsql.UpdatePartnershipParams{
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
		return r.q.DeletePartnership(ctx, personsql.DeletePartnershipParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllPartnerships(ctx context.Context, personID string) error {
	return r.q.DeleteAllPartnerships(ctx, personID)
}

// kinships
func (r *Repository) UpsertKinship(ctx context.Context, k domain.Kinship) (domain.Kinship, error) {
	if k.ID == "" {
		row, err := r.q.InsertKinship(ctx, personsql.InsertKinshipParams{ParentID: k.ParentID, ChildID: k.ChildID, Status: k.Status})
		if err != nil {
			return domain.Kinship{}, mapWriteErr(err)
		}
		return toKinship(row), nil
	}
	row, err := r.q.UpdateKinship(ctx, personsql.UpdateKinshipParams{ID: k.ID, ParentID: k.ParentID, ChildID: k.ChildID, Status: k.Status})
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
		return r.q.DeleteKinship(ctx, personsql.DeleteKinshipParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllKinships(ctx context.Context, personID string) error {
	return r.q.DeleteAllKinships(ctx, personID)
}

// guardianships
func (r *Repository) UpsertGuardianship(ctx context.Context, g domain.Guardianship) (domain.Guardianship, error) {
	if g.ID == "" {
		row, err := r.q.InsertGuardianship(ctx, personsql.InsertGuardianshipParams{
			GuardianID: g.GuardianID, WardID: g.WardID, RelationCode: text(g.RelationCode), Status: g.Status,
			EffectiveFrom: dateText(g.EffectiveFrom), EffectiveTo: dateText(g.EffectiveTo),
		})
		if err != nil {
			return domain.Guardianship{}, mapWriteErr(err)
		}
		return toGuardianship(row), nil
	}
	row, err := r.q.UpdateGuardianship(ctx, personsql.UpdateGuardianshipParams{
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
		return r.q.DeleteGuardianship(ctx, personsql.DeleteGuardianshipParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllGuardianships(ctx context.Context, personID string) error {
	return r.q.DeleteAllGuardianships(ctx, personID)
}

// sponsorships
func (r *Repository) UpsertSponsorship(ctx context.Context, s domain.Sponsorship) (domain.Sponsorship, error) {
	if s.ID == "" {
		row, err := r.q.InsertSponsorship(ctx, personsql.InsertSponsorshipParams{
			SponsorID: s.SponsorID, SponsoredID: s.SponsoredID, RelationCode: s.RelationCode, Status: s.Status,
			EffectiveFrom: dateText(s.EffectiveFrom), EffectiveTo: dateText(s.EffectiveTo),
		})
		if err != nil {
			return domain.Sponsorship{}, mapWriteErr(err)
		}
		return toSponsorship(row), nil
	}
	row, err := r.q.UpdateSponsorship(ctx, personsql.UpdateSponsorshipParams{
		ID: s.ID, SponsorID: s.SponsorID, SponsoredID: s.SponsoredID, RelationCode: s.RelationCode, Status: s.Status,
		EffectiveFrom: dateText(s.EffectiveFrom), EffectiveTo: dateText(s.EffectiveTo),
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
		return r.q.DeleteSponsorship(ctx, personsql.DeleteSponsorshipParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllSponsorships(ctx context.Context, personID string) error {
	return r.q.DeleteAllSponsorships(ctx, personID)
}

// next of kin
func (r *Repository) UpsertNextOfKin(ctx context.Context, n domain.NextOfKin) (domain.NextOfKin, error) {
	if n.ID == "" {
		row, err := r.q.InsertNextOfKin(ctx, personsql.InsertNextOfKinParams{
			SubjectID: n.SubjectID, ContactID: n.ContactID, RelationCode: text(n.RelationCode),
			Priority: int32(n.Priority), Status: n.Status,
		})
		if err != nil {
			return domain.NextOfKin{}, mapWriteErr(err)
		}
		return toNextOfKin(row), nil
	}
	row, err := r.q.UpdateNextOfKin(ctx, personsql.UpdateNextOfKinParams{
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
		return r.q.DeleteNextOfKin(ctx, personsql.DeleteNextOfKinParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllNextOfKin(ctx context.Context, personID string) error {
	return r.q.DeleteAllNextOfKin(ctx, personID)
}

// associations
func (r *Repository) UpsertAssociation(ctx context.Context, a domain.Association) (domain.Association, error) {
	if a.ID == "" {
		row, err := r.q.InsertAssociation(ctx, personsql.InsertAssociationParams{
			PersonIDA: a.PersonIDA, PersonIDB: a.PersonIDB, RelationCode: text(a.RelationCode), Kind: a.Kind, Status: a.Status,
		})
		if err != nil {
			return domain.Association{}, mapWriteErr(err)
		}
		return toAssociation(row), nil
	}
	row, err := r.q.UpdateAssociation(ctx, personsql.UpdateAssociationParams{
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
		return r.q.DeleteAssociation(ctx, personsql.DeleteAssociationParams{ID: id, PersonID: personID})
	})
}

func (r *Repository) DeleteAllAssociations(ctx context.Context, personID string) error {
	return r.q.DeleteAllAssociations(ctx, personID)
}

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

func toRelationType(r personsql.OikumeneaPersonRelationType) domain.RelationType {
	return domain.RelationType{
		Code:      r.Code,
		Name:      r.Name,
		Category:  r.Category,
		Status:    r.Status,
		SortOrder: int(r.SortOrder.Int32),
	}
}

func toPartnership(r personsql.OikumeneaPersonPartnership) domain.Partnership {
	return domain.Partnership{
		ID: r.ID, PersonIDA: r.PersonIDA, PersonIDB: r.PersonIDB, Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo),
	}
}

func toKinship(r personsql.OikumeneaPersonKinship) domain.Kinship {
	return domain.Kinship{ID: r.ID, ParentID: r.ParentID, ChildID: r.ChildID, Status: r.Status}
}

func toGuardianship(r personsql.OikumeneaPersonGuardianship) domain.Guardianship {
	return domain.Guardianship{
		ID: r.ID, GuardianID: r.GuardianID, WardID: r.WardID, RelationCode: r.RelationCode.String, Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo),
	}
}

func toSponsorship(r personsql.OikumeneaPersonSponsorship) domain.Sponsorship {
	return domain.Sponsorship{
		ID: r.ID, SponsorID: r.SponsorID, SponsoredID: r.SponsoredID, RelationCode: r.RelationCode, Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo),
	}
}

func toNextOfKin(r personsql.OikumeneaPersonNextOfKin) domain.NextOfKin {
	return domain.NextOfKin{
		ID: r.ID, SubjectID: r.SubjectID, ContactID: r.ContactID, RelationCode: r.RelationCode.String,
		Priority: int(r.Priority), Status: r.Status,
	}
}

func toAssociation(r personsql.OikumeneaPersonAssociation) domain.Association {
	return domain.Association{
		ID: r.ID, PersonIDA: r.PersonIDA, PersonIDB: r.PersonIDB, RelationCode: r.RelationCode.String, Kind: r.Kind, Status: r.Status,
	}
}

func toMessengerLink(r personsql.OikumeneaPersonMessengerLink) domain.MessengerLink {
	return domain.MessengerLink{
		ID:           r.ID,
		PhoneID:      r.PhoneID.String,
		EmailID:      r.EmailID.String,
		PlatformCode: r.PlatformCode,
		IsPrimary:    r.IsPrimary,
		VerifiedAt:   tsPtr(r.VerifiedAt),
	}
}

func toSocialAccount(r personsql.OikumeneaPersonSocialAccount) domain.SocialAccount {
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

func toSocialAccountHandle(r personsql.OikumeneaPersonSocialAccountHandle) domain.SocialAccountHandle {
	return domain.SocialAccountHandle{
		ID:        r.ID,
		AccountID: r.AccountID,
		Handle:    r.Handle,
		ValidFrom: r.ValidFrom.Time,
		ValidTo:   tsPtr(r.ValidTo),
	}
}

func toEmail(r personsql.OikumeneaPersonEmail) domain.Email {
	return domain.Email{
		ID:        r.ID,
		PersonID:  r.PersonID,
		TypeCode:  r.TypeCode,
		Address:   r.Address,
		Provider:  r.Provider.String,
		IsPrimary: r.IsPrimary,
	}
}

func toPhone(r personsql.OikumeneaPersonPhone) domain.Phone {
	return domain.Phone{
		ID:        r.ID,
		PersonID:  r.PersonID,
		TypeCode:  r.TypeCode,
		Number:    r.Number,
		Country:   r.Country.String,
		IsPrimary: r.IsPrimary,
	}
}

func toCallSign(r personsql.OikumeneaPersonCallSign) domain.CallSign {
	return domain.CallSign{
		ID:        r.ID,
		PersonID:  r.PersonID,
		CallSign:  r.CallSign,
		IsPrimary: r.IsPrimary,
	}
}

func toPerson(r personsql.OikumeneaPersonPerson) domain.Person {
	return domain.Person{
		ID:   r.ID,
		Code: r.Code.String,
		Name: domain.Name{
			DisplayName:   r.DisplayName,
			Title:         r.Title.String,
			Given:         r.Given.String,
			Given2:        r.Given2.String,
			Surname:       r.Surname.String,
			SurnamePrefix: r.SurnamePrefix.String,
			Surname2:      r.Surname2.String,
			Generation:    r.Generation.String,
			Credentials:   r.Credentials.String,
			Preferred:     r.Preferred.String,
		},
		Birthdate:      dateStr(r.Birthdate),
		Sex:            r.Sex,
		CountryOfBirth: r.CountryOfBirth.String,
		Attributes:     r.Attributes,
		RankID:         r.RankID.String,
		Status:         domain.Status(r.Status),
		DeactivatedAt:  tsPtr(r.DeactivatedAt),
		PurgeAfter:     tsPtr(r.PurgeAfter),
		CreatedAt:      r.CreatedAt.Time,
		UpdatedAt:      r.UpdatedAt.Time,
	}
}

func toNameVariant(r personsql.OikumeneaPersonNameVariant) domain.NameVariant {
	return domain.NameVariant{
		ID:       r.ID,
		PersonID: r.PersonID,
		Locale:   r.Locale,
		Name: domain.Name{
			DisplayName:   r.DisplayName,
			Title:         r.Title.String,
			Given:         r.Given.String,
			Given2:        r.Given2.String,
			Surname:       r.Surname.String,
			SurnamePrefix: r.SurnamePrefix.String,
			Surname2:      r.Surname2.String,
			Generation:    r.Generation.String,
			Credentials:   r.Credentials.String,
			Preferred:     r.Preferred.String,
		},
		IsPrimary: r.IsPrimary,
	}
}

func toCitizenship(r personsql.OikumeneaPersonCitizenship) domain.Citizenship {
	return domain.Citizenship{
		ID:         r.ID,
		PersonID:   r.PersonID,
		Country:    r.Country,
		Basis:      r.Basis,
		AcquiredOn: dateStr(r.AcquiredOn),
		LostOn:     dateStr(r.LostOn),
		IsPrimary:  r.IsPrimary,
	}
}

func toResidence(r personsql.OikumeneaPersonResidence) domain.Residence {
	return domain.Residence{
		ID:        r.ID,
		PersonID:  r.PersonID,
		Country:   r.Country,
		Region:    r.Region.String,
		ValidFrom: dateStr(r.ValidFrom),
		ValidTo:   dateStr(r.ValidTo),
	}
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
		case strings.Contains(name, "code"):
			return domain.ErrCodeConflict
		}
	case "23503": // foreign_key_violation
		switch {
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
// non-nil pointer (including "") sets the column, so an empty string clears an optional name part.
func textPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
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
func ts(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
