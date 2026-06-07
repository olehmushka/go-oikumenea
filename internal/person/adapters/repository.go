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

// ---------------------------------------------------------------- mapping helpers

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
		case strings.Contains(name, "code"):
			return domain.ErrCodeConflict
		}
	case "23503": // foreign_key_violation
		switch {
		case strings.Contains(name, "rank"):
			return domain.ErrUnknownRank
		case strings.Contains(name, "locale"):
			return domain.ErrUnknownLocale
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
