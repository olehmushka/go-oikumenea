// Package adapters is the person core module's pgx/sqlc-backed persistence adapter (D-PersonModuleSplit,
// review-2026-07 R-09). It owns the person aggregate root's core tables only — person_persons, the
// person_ranks link, and the person_name_variants (names incl. aliases) — plus the reversible
// deactivate -> purge lifecycle. The non-encrypted directory data (personprofile) and the sensitive /
// encrypted surface (personsensitive) live in their own adapters over their own generated query packages.
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
	"github.com/olegamysk/go-oikumenea/internal/person/adapters/personsql"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of the person core domain.Repository, bound to a
// single db.DBTX — the pool for reads, or a caller-supplied transaction so a write and its audit row
// commit together (D-Audit).
type Repository struct {
	q *personsql.Queries
	c db.DBTX // raw command surface, for the handful of statements not expressed as sqlc queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: personsql.New(conn), c: conn}
}

// compile-time assertion that the adapter satisfies the person core domain port.
var _ domain.Repository = (*Repository)(nil)

func (r *Repository) InsertPerson(ctx context.Context, p domain.Person) (domain.Person, error) {
	row, err := r.q.InsertPerson(ctx, personsql.InsertPersonParams{
		Code:             text(p.Code),
		DisplayName:      p.DisplayName,
		Title:            text(p.Title),
		Given:            text(p.Given),
		Given2:           text(p.Given2),
		Surname:          text(p.Surname),
		SurnamePrefix:    text(p.SurnamePrefix),
		Surname2:         text(p.Surname2),
		Generation:       text(p.Generation),
		Credentials:      text(p.Credentials),
		Preferred:        text(p.Preferred),
		Birthdate:        dateText(p.Birthdate),
		DateOfDeath:      dateText(p.DateOfDeath),
		Sex:              p.Sex,
		CountryOfBirthID: text(p.CountryOfBirth),
		Attributes:       p.Attributes,
	})
	if err != nil {
		return domain.Person{}, mapWriteErr(err)
	}
	return toPerson(row), nil
}

// InsertProvisionalPerson inserts the person via the normal path, then flips its status to
// 'provisional' in the same transaction (D-OverlayFoundation). The minimal-PII stub keeps the
// display_name (required) and any seeded structured parts; everything else is left empty.

// InsertProvisionalPerson inserts the person via the normal path, then flips its status to
// 'provisional' in the same transaction (D-OverlayFoundation). The minimal-PII stub keeps the
// display_name (required) and any seeded structured parts; everything else is left empty.
func (r *Repository) InsertProvisionalPerson(ctx context.Context, p domain.Person) (domain.Person, error) {
	created, err := r.InsertPerson(ctx, p)
	if err != nil {
		return domain.Person{}, err
	}
	if _, err := r.c.Exec(ctx,
		`UPDATE oikumenea.person_persons SET status = 'provisional' WHERE id = $1`, created.ID); err != nil {
		return domain.Person{}, err
	}
	created.Status = domain.StatusProvisional
	return created, nil
}

// repointOwnedStmts re-homes the person-OWNED rows fromID → toID. Each entry is a single-column
// UPDATE; relationship tables carry the person on two columns, so both are listed. Cross-module rows
// (membership, documents, vehicle/company holders, …) are re-homed by the PersonMerged subscribers,
// not here.

// repointOwnedStmts re-homes the person-OWNED rows fromID → toID. Each entry is a single-column
// UPDATE; relationship tables carry the person on two columns, so both are listed. Cross-module rows
// (membership, documents, vehicle/company holders, …) are re-homed by the PersonMerged subscribers,
// not here.
var repointOwnedStmts = []string{
	`UPDATE oikumenea.person_ranks            SET person_id  = $2 WHERE person_id  = $1`,
	`UPDATE oikumenea.person_name_variants    SET person_id  = $2 WHERE person_id  = $1`,
	`UPDATE oikumenea.person_citizenships     SET person_id  = $2 WHERE person_id  = $1`,
	`UPDATE oikumenea.person_residences       SET person_id  = $2 WHERE person_id  = $1`,
	`UPDATE oikumenea.person_emails           SET person_id  = $2 WHERE person_id  = $1`,
	`UPDATE oikumenea.person_phones           SET person_id  = $2 WHERE person_id  = $1`,
	`UPDATE oikumenea.person_call_signs       SET person_id  = $2 WHERE person_id  = $1`,
	// person_messenger_links has NO person_id — it hangs off a phone/email (re-homed above), so the link
	// follows its channel implicitly.
	`UPDATE oikumenea.person_social_accounts  SET person_id  = $2 WHERE person_id  = $1`,
	`UPDATE oikumenea.person_languages        SET person_id  = $2 WHERE person_id  = $1`,
	`UPDATE oikumenea.person_partnerships     SET person_id_a = $2 WHERE person_id_a = $1`,
	`UPDATE oikumenea.person_partnerships     SET person_id_b = $2 WHERE person_id_b = $1`,
	`UPDATE oikumenea.person_kinships         SET parent_id  = $2 WHERE parent_id  = $1`,
	`UPDATE oikumenea.person_kinships         SET child_id   = $2 WHERE child_id   = $1`,
	`UPDATE oikumenea.person_guardianships    SET guardian_id = $2 WHERE guardian_id = $1`,
	`UPDATE oikumenea.person_guardianships    SET ward_id    = $2 WHERE ward_id    = $1`,
	`UPDATE oikumenea.person_sponsorships     SET sponsor_id = $2 WHERE sponsor_id = $1`,
	`UPDATE oikumenea.person_sponsorships     SET sponsored_id = $2 WHERE sponsored_id = $1`,
	`UPDATE oikumenea.person_next_of_kin      SET subject_id = $2 WHERE subject_id = $1`,
	`UPDATE oikumenea.person_next_of_kin      SET contact_id = $2 WHERE contact_id = $1`,
	`UPDATE oikumenea.person_associations     SET person_id_a = $2 WHERE person_id_a = $1`,
	`UPDATE oikumenea.person_associations     SET person_id_b = $2 WHERE person_id_b = $1`,
	// institutional & political ties (M33): a provisional OSINT stub often carries exactly these edges
	// before it is merged into a canonical person (D-InstitutionalTies / D-OverlayFoundation).
	`UPDATE oikumenea.person_party_memberships      SET person_id = $2 WHERE person_id = $1`,
	`UPDATE oikumenea.person_government_positions   SET person_id = $2 WHERE person_id = $1`,
	`UPDATE oikumenea.person_lobbying_relationships SET person_id = $2 WHERE person_id = $1`,
	`UPDATE oikumenea.person_external_references    SET person_id = $2 WHERE person_id = $1`,
	// watchlists & regulatory exposure (M34): the durable regulatory-sanction overlay is re-homed onto the
	// canonical person. The single-per-person transient watchlist MATCH is NOT re-homed (its partial-unique
	// person_id would collide) — the stub's row is hard-dropped by the merge's Purge step, and a re-check
	// regenerates the screening result on the canonical person (D-Watchlists).
	`UPDATE oikumenea.person_regulatory_sanctions   SET person_id = $2 WHERE person_id = $1`,
	// financial / behavioural overlays (M35): crypto wallets + personality profiles re-home onto the
	// canonical person. The single-per-person inferred political leaning is NOT re-homed (its partial-unique
	// person_id would collide) — the stub's row is hard-dropped by the merge's Purge step (and the inferred
	// leaning is never merged with the declared party membership anyway, D-PersonOverlays).
	`UPDATE oikumenea.person_crypto_wallets         SET person_id = $2 WHERE person_id = $1`,
	`UPDATE oikumenea.person_personality            SET person_id = $2 WHERE person_id = $1`,
}

// RepointPersonOwned runs the person-owned re-point UPDATEs fromID → toID in the caller's transaction.

// RepointPersonOwned runs the person-owned re-point UPDATEs fromID → toID in the caller's transaction.
func (r *Repository) RepointPersonOwned(ctx context.Context, fromID, toID string) error {
	for _, stmt := range repointOwnedStmts {
		if _, err := r.c.Exec(ctx, stmt, fromID, toID); err != nil {
			return mapWriteErr(err)
		}
	}
	return nil
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
		DisplayName:      textPtr(patch.DisplayName),
		Title:            textPtr(patch.Title),
		Given:            textPtr(patch.Given),
		Given2:           textPtr(patch.Given2),
		Surname:          textPtr(patch.Surname),
		SurnamePrefix:    textPtr(patch.SurnamePrefix),
		Surname2:         textPtr(patch.Surname2),
		Generation:       textPtr(patch.Generation),
		Credentials:      textPtr(patch.Credentials),
		Preferred:        textPtr(patch.Preferred),
		Birthdate:        datePtr(patch.Birthdate),
		DateOfDeath:      datePtr(patch.DateOfDeath),
		Sex:              textPtr(patch.Sex),
		CountryOfBirthID: textPtr(patch.CountryOfBirth),
		Attributes:       patch.Attributes,
		ID:               id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, mapWriteErr(err)
	}
	return toPerson(row), nil
}

// ListPersons returns a keyset page of the directory. A non-empty query routes to the dedicated
// trigram SearchPersons (review R-06) so each match branch stays a GIN bitmap scan; the empty case
// is the unfiltered list. Both queries key on the person RID, so the caller's cursor is identical.

// ListPersons returns a keyset page of the directory. A non-empty query routes to the dedicated
// trigram SearchPersons (review R-06) so each match branch stays a GIN bitmap scan; the empty case
// is the unfiltered list. Both queries key on the person RID, so the caller's cursor is identical.
func (r *Repository) ListPersons(ctx context.Context, after, query string, limit int) ([]domain.Person, error) {
	// The two list queries share the lean R-17 projection, so they return field-identical row structs
	// (convertible to ListPersonsRow) and map through the one leanToPerson mapper.
	var rows []personsql.ListPersonsRow
	if q := strings.TrimSpace(query); q != "" {
		found, err := r.q.SearchPersons(ctx, personsql.SearchPersonsParams{After: after, Query: pgtype.Text{String: q, Valid: true}, Lim: int32(limit)})
		if err != nil {
			return nil, err
		}
		rows = make([]personsql.ListPersonsRow, len(found))
		for i, row := range found {
			rows[i] = personsql.ListPersonsRow(row)
		}
	} else {
		var err error
		if rows, err = r.q.ListPersons(ctx, personsql.ListPersonsParams{After: after, Lim: int32(limit)}); err != nil {
			return nil, err
		}
	}
	out := make([]domain.Person, 0, len(rows))
	for _, row := range rows {
		out = append(out, leanToPerson(row))
	}
	return out, nil
}

func (r *Repository) ListPersonsByIDs(ctx context.Context, ids []string) ([]domain.Person, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.q.ListPersonsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Person, 0, len(rows))
	for _, row := range rows {
		out = append(out, leanToPerson(personsql.ListPersonsRow(row)))
	}
	return out, nil
}

// UpsertPersonRank sets the person's rank in the system DERIVED from rankID (the query SELECTs the
// system from rank_ranks). An unknown/soft-deleted rank produces no row → ErrUnknownRank.

// UpsertPersonRank sets the person's rank in the system DERIVED from rankID (the query SELECTs the
// system from rank_ranks). An unknown/soft-deleted rank produces no row → ErrUnknownRank.
func (r *Repository) UpsertPersonRank(ctx context.Context, personID, rankID string) (domain.PersonRank, error) {
	row, err := r.q.UpsertPersonRank(ctx, personsql.UpsertPersonRankParams{PersonID: personID, RankID: rankID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PersonRank{}, domain.ErrUnknownRank
		}
		return domain.PersonRank{}, mapWriteErr(err)
	}
	return domain.PersonRank{SystemID: row.SystemID, RankID: row.RankID}, nil
}

// ClearPersonRank soft-deletes the person's active rank in systemID (no-op when none is held).

// ClearPersonRank soft-deletes the person's active rank in systemID (no-op when none is held).
func (r *Repository) ClearPersonRank(ctx context.Context, personID, systemID string) error {
	return r.q.ClearPersonRank(ctx, personsql.ClearPersonRankParams{PersonID: personID, SystemID: systemID})
}

// ListPersonRanks returns the person's active ranks, one per system, ordered by rank-system sort order.

// ListPersonRanks returns the person's active ranks, one per system, ordered by rank-system sort order.
func (r *Repository) ListPersonRanks(ctx context.Context, personID string) ([]domain.PersonRank, error) {
	rows, err := r.q.ListPersonRanks(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PersonRank, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PersonRank{SystemID: row.SystemID, RankID: row.RankID})
	}
	return out, nil
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

// Purge erases the person's PII and removes all child rows in the same transaction, keeping the id
// row as a tombstone (audit history references it).
func (r *Repository) Purge(ctx context.Context, id string) (domain.Person, error) {
	// Core-only erasure (D-PersonModuleSplit, review-2026-07 R-09): the name variants (core-owned) are
	// hard-deleted, then person_persons has its PII scrubbed to a status=purged tombstone. Every other
	// person_* table now belongs to personprofile or personsensitive: the application PurgePerson publishes
	// a PersonPurged event and each owning module erases (or crypto-erases) its own rows via its
	// SubscribePersonPurge handler in the SAME transaction. The cross-module education/company rows are
	// likewise erased by their own PersonPurged subscribers (R-08 boundary).
	if err := r.q.DeleteAllNameVariants(ctx, id); err != nil {
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

// leanToPerson maps the R-17 lean list projection (ListPersonsRow — every column except the wide
// generated search_text and the always-NULL deleted_at) to the domain aggregate. The three list
// queries share this projection (SearchPersonsRow / ListPersonsByIDsRow are field-identical and
// convert to ListPersonsRow), so all directory-list rows map through this one function. Single-row
// gets keep SELECT * and toPerson.
func leanToPerson(r personsql.ListPersonsRow) domain.Person {
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
		DateOfDeath:    dateStr(r.DateOfDeath),
		Sex:            r.Sex,
		CountryOfBirth: r.CountryOfBirthID.String,
		Attributes:     r.Attributes,
		Status:         domain.Status(r.Status),
		DeactivatedAt:  tsPtr(r.DeactivatedAt),
		PurgeAfter:     tsPtr(r.PurgeAfter),
		CreatedAt:      r.CreatedAt.Time,
		UpdatedAt:      r.UpdatedAt.Time,
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
		DateOfDeath:    dateStr(r.DateOfDeath),
		Sex:            r.Sex,
		CountryOfBirth: r.CountryOfBirthID.String,
		Attributes:     r.Attributes,
		Status:         domain.Status(r.Status),
		DeactivatedAt:  tsPtr(r.DeactivatedAt),
		PurgeAfter:     tsPtr(r.PurgeAfter),
		CreatedAt:      r.CreatedAt.Time,
		UpdatedAt:      r.UpdatedAt.Time,
	}
}

// ---------------------------------------------------------------- physical identity (M31)

// name aliases (variant_kind != transliteration), addressed by RID.

func (r *Repository) InsertNameAlias(ctx context.Context, v domain.NameVariant) (domain.NameVariant, error) {
	row, err := r.q.InsertNameAlias(ctx, personsql.InsertNameAliasParams{
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
		VariantKind:   v.VariantKind,
		Source:        text(v.Source),
		Confidence:    text(v.Confidence),
	})
	if err != nil {
		return domain.NameVariant{}, mapWriteErr(err)
	}
	return toNameVariant(row), nil
}

func (r *Repository) DeleteNameAlias(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteNameAlias(ctx, personsql.DeleteNameAliasParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNameAliasNotFound
		}
		return err
	}
	return nil
}

// physical descriptions.

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
		IsPrimary:   r.IsPrimary,
		VariantKind: r.VariantKind,
		Source:      strText(r.Source),
		Confidence:  strText(r.Confidence),
	}
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
