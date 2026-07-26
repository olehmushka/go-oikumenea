// Package adapters is the personsensitive module's pgx/sqlc-backed persistence adapter (D-PersonModuleSplit,
// review-2026-07 R-09). It owns the person directory's sensitive / envelope-encrypted tables: physical
// descriptions & distinguishing marks, ethnicity (crypto-erasable), encrypted declared party memberships,
// watchlist matches & regulatory sanctions, and the M35 overlays (crypto wallets, personality, political
// leaning). It compiles against its own generated query package (personsensitivesql) and shares the person
// aggregate's domain kernel (internal/person/domain); the Cipher and all seal/unseal live in this module's
// application layer.
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
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	personsensitivesql "github.com/olegamysk/go-oikumenea/internal/personsensitive/adapters/personsensitivesql"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the personsensitive pgx/sqlc-backed persistence adapter, bound to a single db.DBTX — the
// pool for reads, or a caller-supplied transaction so a write and its audit row commit together (D-Audit).
type Repository struct {
	q *personsensitivesql.Queries
	c db.DBTX // raw command surface, for the handful of statements not expressed as sqlc queries
}

// NewRepository binds a repository to the given command surface.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: personsensitivesql.New(conn), c: conn}
}

func (r *Repository) UpsertPhysicalDescription(ctx context.Context, d domain.PhysicalDescription) (domain.PhysicalDescription, error) {
	if d.ID == "" {
		row, err := r.q.InsertPhysicalDescription(ctx, personsensitivesql.InsertPhysicalDescriptionParams{
			PersonID:      d.PersonID,
			HeightCm:      int4(d.HeightCm),
			WeightKg:      int4(d.WeightKg),
			EyeColorID:    text(d.EyeColorID),
			HairColorID:   text(d.HairColorID),
			Build:         text(d.Build),
			BloodType:     text(d.BloodType),
			EffectiveFrom: dateText(d.EffectiveFrom),
			EffectiveTo:   dateText(d.EffectiveTo),
			Source:        text(d.Source),
			Confidence:    text(d.Confidence),
		})
		if err != nil {
			return domain.PhysicalDescription{}, mapWriteErr(err)
		}
		return toPhysicalDescription(row), nil
	}
	row, err := r.q.UpdatePhysicalDescription(ctx, personsensitivesql.UpdatePhysicalDescriptionParams{
		HeightCm:      int4(d.HeightCm),
		WeightKg:      int4(d.WeightKg),
		EyeColorID:    text(d.EyeColorID),
		HairColorID:   text(d.HairColorID),
		Build:         text(d.Build),
		BloodType:     text(d.BloodType),
		EffectiveFrom: dateText(d.EffectiveFrom),
		EffectiveTo:   dateText(d.EffectiveTo),
		Source:        text(d.Source),
		Confidence:    text(d.Confidence),
		ID:            d.ID,
		PersonID:      d.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PhysicalDescription{}, domain.ErrPhysicalDescriptionNotFound
		}
		return domain.PhysicalDescription{}, mapWriteErr(err)
	}
	return toPhysicalDescription(row), nil
}

func (r *Repository) DeletePhysicalDescription(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeletePhysicalDescription(ctx, personsensitivesql.DeletePhysicalDescriptionParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrPhysicalDescriptionNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListPhysicalDescriptions(ctx context.Context, personID string) ([]domain.PhysicalDescription, error) {
	rows, err := r.q.ListPhysicalDescriptions(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PhysicalDescription, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPhysicalDescription(row))
	}
	return out, nil
}

// distinguishing marks.

func (r *Repository) UpsertDistinguishingMark(ctx context.Context, m domain.DistinguishingMark) (domain.DistinguishingMark, error) {
	if m.ID == "" {
		row, err := r.q.InsertDistinguishingMark(ctx, personsensitivesql.InsertDistinguishingMarkParams{
			PersonID:     m.PersonID,
			Kind:         m.Kind,
			BodyLocation: text(m.BodyLocation),
			Description:  text(m.Description),
			Source:       text(m.Source),
			Confidence:   text(m.Confidence),
		})
		if err != nil {
			return domain.DistinguishingMark{}, mapWriteErr(err)
		}
		return toDistinguishingMark(row), nil
	}
	row, err := r.q.UpdateDistinguishingMark(ctx, personsensitivesql.UpdateDistinguishingMarkParams{
		Kind:         m.Kind,
		BodyLocation: text(m.BodyLocation),
		Description:  text(m.Description),
		Source:       text(m.Source),
		Confidence:   text(m.Confidence),
		ID:           m.ID,
		PersonID:     m.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DistinguishingMark{}, domain.ErrDistinguishingMarkNotFound
		}
		return domain.DistinguishingMark{}, mapWriteErr(err)
	}
	return toDistinguishingMark(row), nil
}

func (r *Repository) DeleteDistinguishingMark(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteDistinguishingMark(ctx, personsensitivesql.DeleteDistinguishingMarkParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrDistinguishingMarkNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListDistinguishingMarks(ctx context.Context, personID string) ([]domain.DistinguishingMark, error) {
	rows, err := r.q.ListDistinguishingMarks(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DistinguishingMark, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDistinguishingMark(row))
	}
	return out, nil
}

// ethnicity-type catalog.

func (r *Repository) ListEthnicityTypes(ctx context.Context, f domain.EthnicityTypeFilter) ([]domain.EthnicityType, error) {
	lim := int32(f.Limit)
	if lim <= 0 {
		lim = 2000
	}
	rows, err := r.q.ListEthnicityTypes(ctx, personsensitivesql.ListEthnicityTypesParams{
		TopLevel: f.TopLevel,
		Parent:   text(f.Parent),
		Query:    f.Query,
		Lim:      lim,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.EthnicityType, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.EthnicityType{
			ID: row.ID, Code: row.Code, Name: row.Name,
			ParentID: row.ParentID.String, WikidataID: row.WikidataID.String,
			HasChildren: row.HasChildren, Status: row.Status, SortOrder: int4Ptr(row.SortOrder),
		})
	}
	return out, nil
}

func (r *Repository) GetEthnicityTypeByCode(ctx context.Context, code string) (domain.EthnicityType, error) {
	row, err := r.q.GetEthnicityTypeByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EthnicityType{}, domain.ErrUnknownEthnicityType
		}
		return domain.EthnicityType{}, err
	}
	return toEthnicityType(row), nil
}

func (r *Repository) GetEthnicityTypeByID(ctx context.Context, id string) (domain.EthnicityType, error) {
	row, err := r.q.GetEthnicityTypeByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EthnicityType{}, domain.ErrUnknownEthnicityType
		}
		return domain.EthnicityType{}, err
	}
	return domain.EthnicityType{
		ID: row.ID, Code: row.Code, Name: row.Name,
		ParentID: row.ParentID.String, WikidataID: row.WikidataID.String,
		HasChildren: row.HasChildren, Status: row.Status, SortOrder: int4Ptr(row.SortOrder),
	}, nil
}

func (r *Repository) ListEthnicityTypeLanguages(ctx context.Context, ethnicityTypeID string) ([]string, error) {
	return r.q.ListEthnicityTypeLanguages(ctx, ethnicityTypeID)
}

func (r *Repository) ListEthnicityTypeCountries(ctx context.Context, ethnicityTypeID string) ([]string, error) {
	return r.q.ListEthnicityTypeCountries(ctx, ethnicityTypeID)
}

func (r *Repository) UpsertEthnicityType(ctx context.Context, t domain.EthnicityType) (domain.EthnicityType, error) {
	row, err := r.q.UpsertEthnicityType(ctx, personsensitivesql.UpsertEthnicityTypeParams{
		Code:       t.Code,
		Name:       t.Name,
		ParentID:   text(t.ParentID),
		WikidataID: text(t.WikidataID),
		SortOrder:  int4(t.SortOrder),
	})
	if err != nil {
		return domain.EthnicityType{}, mapWriteErr(err)
	}
	return toEthnicityType(row), nil
}

// ethnicities — the encrypted link__has_ethnicity.

func (r *Repository) InsertEthnicity(ctx context.Context, e domain.StoredEthnicity) (domain.StoredEthnicity, error) {
	row, err := r.q.InsertEthnicity(ctx, personsensitivesql.InsertEthnicityParams{
		PersonID:        e.PersonID,
		ValueCiphertext: e.ValueCiphertext,
		WrappedDek:      e.WrappedDEK,
		KeyRef:          text(e.KeyRef),
		ValueBlindIndex: e.ValueBlindIndex,
		LegalBasis:      e.LegalBasis,
		Source:          text(e.Source),
		Confidence:      text(e.Confidence),
	})
	if err != nil {
		return domain.StoredEthnicity{}, mapWriteErr(err)
	}
	return toStoredEthnicity(row), nil
}

func (r *Repository) UpdateEthnicity(ctx context.Context, e domain.StoredEthnicity) (domain.StoredEthnicity, error) {
	row, err := r.q.UpdateEthnicity(ctx, personsensitivesql.UpdateEthnicityParams{
		ValueCiphertext: e.ValueCiphertext,
		WrappedDek:      e.WrappedDEK,
		KeyRef:          text(e.KeyRef),
		ValueBlindIndex: e.ValueBlindIndex,
		LegalBasis:      e.LegalBasis,
		Status:          e.Status,
		Source:          text(e.Source),
		Confidence:      text(e.Confidence),
		ID:              e.ID,
		PersonID:        e.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredEthnicity{}, domain.ErrEthnicityNotFound
		}
		return domain.StoredEthnicity{}, mapWriteErr(err)
	}
	return toStoredEthnicity(row), nil
}

func (r *Repository) DeleteEthnicity(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteEthnicity(ctx, personsensitivesql.DeleteEthnicityParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrEthnicityNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListEthnicities(ctx context.Context, personID string) ([]domain.StoredEthnicity, error) {
	rows, err := r.q.ListEthnicities(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StoredEthnicity, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStoredEthnicity(row))
	}
	return out, nil
}

func (r *Repository) CryptoEraseEthnicities(ctx context.Context, personID string) (int64, error) {
	return r.q.CryptoEraseEthnicities(ctx, personID)
}

// M31 row mappers.

func toPhysicalDescription(r personsensitivesql.OikumeneaPersonPhysicalDescription) domain.PhysicalDescription {
	return domain.PhysicalDescription{
		ID:            r.ID,
		PersonID:      r.PersonID,
		HeightCm:      int4Ptr(r.HeightCm),
		WeightKg:      int4Ptr(r.WeightKg),
		EyeColorID:    strText(r.EyeColorID),
		HairColorID:   strText(r.HairColorID),
		Build:         strText(r.Build),
		BloodType:     strText(r.BloodType),
		EffectiveFrom: dateStr(r.EffectiveFrom),
		EffectiveTo:   dateStr(r.EffectiveTo),
		Source:        strText(r.Source),
		Confidence:    strText(r.Confidence),
	}
}

func toDistinguishingMark(r personsensitivesql.OikumeneaPersonDistinguishingMark) domain.DistinguishingMark {
	return domain.DistinguishingMark{
		ID:           r.ID,
		PersonID:     r.PersonID,
		Kind:         r.Kind,
		BodyLocation: strText(r.BodyLocation),
		Description:  strText(r.Description),
		Source:       strText(r.Source),
		Confidence:   strText(r.Confidence),
	}
}

func toEthnicityType(r personsensitivesql.OikumeneaPersonEthnicityType) domain.EthnicityType {
	return domain.EthnicityType{
		ID:         r.ID,
		Code:       r.Code,
		Name:       r.Name,
		ParentID:   r.ParentID.String,
		WikidataID: r.WikidataID.String,
		Status:     r.Status,
		SortOrder:  int4Ptr(r.SortOrder),
	}
}

func toStoredEthnicity(r personsensitivesql.OikumeneaPersonEthnicity) domain.StoredEthnicity {
	return domain.StoredEthnicity{
		ID:              r.ID,
		PersonID:        r.PersonID,
		ValueCiphertext: r.ValueCiphertext,
		WrappedDEK:      r.WrappedDek,
		KeyRef:          strText(r.KeyRef),
		ValueBlindIndex: r.ValueBlindIndex,
		LegalBasis:      r.LegalBasis,
		Status:          r.Status,
		Source:          strText(r.Source),
		Confidence:      strText(r.Confidence),
		CreatedAt:       r.CreatedAt.Time,
		UpdatedAt:       r.UpdatedAt.Time,
	}
}

// InsertPartyMembership stores a new encrypted party membership (the party envelope is sealed upstream).
func (r *Repository) InsertPartyMembership(ctx context.Context, p domain.StoredPartyMembership) (domain.StoredPartyMembership, error) {
	row, err := r.q.InsertPartyMembership(ctx, personsensitivesql.InsertPartyMembershipParams{
		PersonID:        p.PersonID,
		PartyCiphertext: p.PartyCiphertext,
		PartyWrappedDek: p.PartyWrappedDEK,
		PartyKeyRef:     text(p.PartyKeyRef),
		PartyBlindIndex: p.PartyBlindIndex,
		Role:            p.Role,
		ValidFrom:       dateText(p.ValidFrom),
		ValidTo:         dateText(p.ValidTo),
		LegalBasis:      p.LegalBasis,
		Source:          p.Source,
		Confidence:      p.Confidence,
	})
	if err != nil {
		return domain.StoredPartyMembership{}, mapWriteErr(err)
	}
	return toStoredParty(row), nil
}

// UpdatePartyMembership re-seals the party and/or flips role/dates/status on an existing row.

// UpdatePartyMembership re-seals the party and/or flips role/dates/status on an existing row.
func (r *Repository) UpdatePartyMembership(ctx context.Context, p domain.StoredPartyMembership) (domain.StoredPartyMembership, error) {
	if p.Status == "" {
		p.Status = "active"
	}
	row, err := r.q.UpdatePartyMembership(ctx, personsensitivesql.UpdatePartyMembershipParams{
		PartyCiphertext: p.PartyCiphertext,
		PartyWrappedDek: p.PartyWrappedDEK,
		PartyKeyRef:     text(p.PartyKeyRef),
		PartyBlindIndex: p.PartyBlindIndex,
		Role:            p.Role,
		ValidFrom:       dateText(p.ValidFrom),
		ValidTo:         dateText(p.ValidTo),
		LegalBasis:      p.LegalBasis,
		Status:          p.Status,
		Source:          p.Source,
		Confidence:      p.Confidence,
		ID:              p.ID,
		PersonID:        p.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredPartyMembership{}, domain.ErrPartyMembershipNotFound
		}
		return domain.StoredPartyMembership{}, mapWriteErr(err)
	}
	return toStoredParty(row), nil
}

func (r *Repository) DeletePartyMembership(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeletePartyMembership(ctx, personsensitivesql.DeletePartyMembershipParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrPartyMembershipNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListPartyMemberships(ctx context.Context, personID string) ([]domain.StoredPartyMembership, error) {
	rows, err := r.q.ListPartyMemberships(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StoredPartyMembership, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStoredParty(row))
	}
	return out, nil
}

func (r *Repository) CryptoErasePartyMemberships(ctx context.Context, personID string) (int64, error) {
	return r.q.CryptoErasePartyMemberships(ctx, personID)
}

// UpsertGovernmentPosition inserts a new row when g.ID is empty, otherwise replaces the named row.

func toStoredParty(r personsensitivesql.OikumeneaPersonPartyMembership) domain.StoredPartyMembership {
	return domain.StoredPartyMembership{
		ID:              r.ID,
		PersonID:        r.PersonID,
		PartyCiphertext: r.PartyCiphertext,
		PartyWrappedDEK: r.PartyWrappedDek,
		PartyKeyRef:     strText(r.PartyKeyRef),
		PartyBlindIndex: r.PartyBlindIndex,
		Role:            r.Role,
		ValidFrom:       dateStr(r.ValidFrom),
		ValidTo:         dateStr(r.ValidTo),
		LegalBasis:      r.LegalBasis,
		Status:          r.Status,
		Source:          r.Source,
		Confidence:      r.Confidence,
		CreatedAt:       r.CreatedAt.Time,
		UpdatedAt:       r.UpdatedAt.Time,
	}
}

// UpsertWatchlistMatch inserts or (on the partial-unique person_id) refreshes the single screening result.
func (r *Repository) UpsertWatchlistMatch(ctx context.Context, m domain.WatchlistMatch) (domain.WatchlistMatch, error) {
	if m.Lists == nil {
		m.Lists = []string{}
	}
	row, err := r.q.UpsertWatchlistMatch(ctx, personsensitivesql.UpsertWatchlistMatchParams{
		PersonID:     m.PersonID,
		OnList:       m.OnList,
		Lists:        m.Lists,
		Program:      text(m.Program),
		MatchScore:   numArg(m.MatchScore),
		Pep:          m.PEP,
		LastChecked:  pgtype.Timestamptz{Time: m.LastChecked, Valid: true},
		NextCheckDue: ts(m.NextCheckDue),
		Source:       m.Source,
		Confidence:   m.Confidence,
	})
	if err != nil {
		return domain.WatchlistMatch{}, mapWriteErr(err)
	}
	return toWatchlistMatch(row), nil
}

func (r *Repository) GetWatchlistMatch(ctx context.Context, personID string) (domain.WatchlistMatch, bool, error) {
	row, err := r.q.GetWatchlistMatch(ctx, personID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.WatchlistMatch{}, false, nil
		}
		return domain.WatchlistMatch{}, false, err
	}
	return toWatchlistMatch(row), true, nil
}

// DeleteWatchlistMatch hard-deletes a person's screening result (purge path).

// DeleteWatchlistMatch hard-deletes a person's screening result (purge path).
func (r *Repository) DeleteWatchlistMatch(ctx context.Context, personID string) error {
	return r.q.DeleteAllWatchlistMatches(ctx, personID)
}

// UpsertRegulatorySanction inserts idempotently by (person, external_id) when x.ID is empty; otherwise
// replaces the named row by RID.

// UpsertRegulatorySanction inserts idempotently by (person, external_id) when x.ID is empty; otherwise
// replaces the named row by RID.
func (r *Repository) UpsertRegulatorySanction(ctx context.Context, x domain.RegulatorySanction) (domain.RegulatorySanction, error) {
	if x.ID == "" {
		row, err := r.q.UpsertRegulatorySanction(ctx, personsensitivesql.UpsertRegulatorySanctionParams{
			PersonID:     x.PersonID,
			Regulator:    x.Regulator,
			ActionType:   x.ActionType,
			Amount:       numArg(x.Amount),
			Currency:     text(x.Currency),
			Status:       x.Status,
			SanctionDate: dateText(x.SanctionDate),
			SourceUrl:    text(x.SourceURL),
			ExternalID:   text(x.ExternalID),
			LegalBasis:   text(x.LegalBasis),
			Source:       x.Source,
			Confidence:   x.Confidence,
		})
		if err != nil {
			return domain.RegulatorySanction{}, mapWriteErr(err)
		}
		return toRegulatorySanction(row), nil
	}
	row, err := r.q.UpdateRegulatorySanction(ctx, personsensitivesql.UpdateRegulatorySanctionParams{
		Regulator:    x.Regulator,
		ActionType:   x.ActionType,
		Amount:       numArg(x.Amount),
		Currency:     text(x.Currency),
		Status:       x.Status,
		SanctionDate: dateText(x.SanctionDate),
		SourceUrl:    text(x.SourceURL),
		ExternalID:   text(x.ExternalID),
		LegalBasis:   text(x.LegalBasis),
		Source:       x.Source,
		Confidence:   x.Confidence,
		ID:           x.ID,
		PersonID:     x.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RegulatorySanction{}, domain.ErrRegulatorySanctionNotFound
		}
		return domain.RegulatorySanction{}, mapWriteErr(err)
	}
	return toRegulatorySanction(row), nil
}

func (r *Repository) DeleteRegulatorySanction(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteRegulatorySanction(ctx, personsensitivesql.DeleteRegulatorySanctionParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRegulatorySanctionNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListRegulatorySanctions(ctx context.Context, personID string) ([]domain.RegulatorySanction, error) {
	rows, err := r.q.ListRegulatorySanctions(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RegulatorySanction, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRegulatorySanction(row))
	}
	return out, nil
}

func toWatchlistMatch(r personsensitivesql.OikumeneaPersonWatchlistMatch) domain.WatchlistMatch {
	return domain.WatchlistMatch{
		ID:           r.ID,
		PersonID:     r.PersonID,
		OnList:       r.OnList,
		Lists:        r.Lists,
		Program:      strText(r.Program),
		MatchScore:   numPtr(r.MatchScore),
		PEP:          r.Pep,
		LastChecked:  r.LastChecked.Time,
		NextCheckDue: tsPtr(r.NextCheckDue),
		Source:       r.Source,
		Confidence:   r.Confidence,
		CreatedAt:    r.CreatedAt.Time,
		UpdatedAt:    r.UpdatedAt.Time,
	}
}

func toRegulatorySanction(r personsensitivesql.OikumeneaPersonRegulatorySanction) domain.RegulatorySanction {
	return domain.RegulatorySanction{
		ID:           r.ID,
		PersonID:     r.PersonID,
		Regulator:    r.Regulator,
		ActionType:   r.ActionType,
		Amount:       numPtr(r.Amount),
		Currency:     strText(r.Currency),
		Status:       r.Status,
		SanctionDate: dateStr(r.SanctionDate),
		SourceURL:    strText(r.SourceUrl),
		ExternalID:   strText(r.ExternalID),
		LegalBasis:   strText(r.LegalBasis),
		Source:       r.Source,
		Confidence:   r.Confidence,
		CreatedAt:    r.CreatedAt.Time,
		UpdatedAt:    r.UpdatedAt.Time,
	}
}

// ---------------------------------------------------------------- overlays (M35, D-PersonOverlays)

// UpsertCryptoWallet inserts a new row when w.ID is empty, otherwise replaces the named row.

// UpsertCryptoWallet inserts a new row when w.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertCryptoWallet(ctx context.Context, w domain.CryptoWallet) (domain.CryptoWallet, error) {
	if w.ID == "" {
		row, err := r.q.InsertCryptoWallet(ctx, personsensitivesql.InsertCryptoWalletParams{
			PersonID:          w.PersonID,
			Address:           w.Address,
			Chain:             w.Chain,
			AttributionMethod: w.AttributionMethod,
			BalanceUsdApprox:  float8Arg(w.BalanceUSDApprox),
			FirstSeen:         dateText(w.FirstSeen),
			LastSeen:          dateText(w.LastSeen),
			Source:            w.Source,
			Confidence:        w.Confidence,
		})
		if err != nil {
			return domain.CryptoWallet{}, mapWriteErr(err)
		}
		return toCryptoWallet(row), nil
	}
	row, err := r.q.UpdateCryptoWallet(ctx, personsensitivesql.UpdateCryptoWalletParams{
		Address:           w.Address,
		Chain:             w.Chain,
		AttributionMethod: w.AttributionMethod,
		BalanceUsdApprox:  float8Arg(w.BalanceUSDApprox),
		FirstSeen:         dateText(w.FirstSeen),
		LastSeen:          dateText(w.LastSeen),
		Source:            w.Source,
		Confidence:        w.Confidence,
		ID:                w.ID,
		PersonID:          w.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CryptoWallet{}, domain.ErrCryptoWalletNotFound
		}
		return domain.CryptoWallet{}, mapWriteErr(err)
	}
	return toCryptoWallet(row), nil
}

func (r *Repository) DeleteCryptoWallet(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteCryptoWallet(ctx, personsensitivesql.DeleteCryptoWalletParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrCryptoWalletNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListCryptoWallets(ctx context.Context, personID string) ([]domain.CryptoWallet, error) {
	rows, err := r.q.ListCryptoWallets(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CryptoWallet, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCryptoWallet(row))
	}
	return out, nil
}

// UpsertPersonality inserts a new row when p.ID is empty, otherwise replaces the named row.

// UpsertPersonality inserts a new row when p.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertPersonality(ctx context.Context, p domain.Personality) (domain.Personality, error) {
	if p.ID == "" {
		row, err := r.q.InsertPersonality(ctx, personsensitivesql.InsertPersonalityParams{
			PersonID:   p.PersonID,
			Framework:  p.Framework,
			Result:     p.Result,
			Instrument: text(p.Instrument),
			Method:     p.Method,
			AssessedAt: dateText(p.AssessedAt),
			Source:     p.Source,
			Confidence: p.Confidence,
		})
		if err != nil {
			return domain.Personality{}, mapWriteErr(err)
		}
		return toPersonality(row), nil
	}
	row, err := r.q.UpdatePersonality(ctx, personsensitivesql.UpdatePersonalityParams{
		Framework:  p.Framework,
		Result:     p.Result,
		Instrument: text(p.Instrument),
		Method:     p.Method,
		AssessedAt: dateText(p.AssessedAt),
		Source:     p.Source,
		Confidence: p.Confidence,
		ID:         p.ID,
		PersonID:   p.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Personality{}, domain.ErrPersonalityNotFound
		}
		return domain.Personality{}, mapWriteErr(err)
	}
	return toPersonality(row), nil
}

func (r *Repository) DeletePersonality(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeletePersonality(ctx, personsensitivesql.DeletePersonalityParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrPersonalityNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListPersonalities(ctx context.Context, personID string) ([]domain.Personality, error) {
	rows, err := r.q.ListPersonalities(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Personality, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPersonality(row))
	}
	return out, nil
}

// InsertPoliticalLeaning stores the single encrypted inferred leaning (the spectrum envelope is sealed
// upstream); UpdatePoliticalLeaning re-seals the existing active row.

// InsertPoliticalLeaning stores the single encrypted inferred leaning (the spectrum envelope is sealed
// upstream); UpdatePoliticalLeaning re-seals the existing active row.
func (r *Repository) InsertPoliticalLeaning(ctx context.Context, l domain.StoredPoliticalLeaning) (domain.StoredPoliticalLeaning, error) {
	row, err := r.q.InsertPoliticalLeaning(ctx, personsensitivesql.InsertPoliticalLeaningParams{
		PersonID:          l.PersonID,
		LeaningCiphertext: l.LeaningCiphertext,
		LeaningWrappedDek: l.LeaningWrappedDEK,
		LeaningKeyRef:     text(l.LeaningKeyRef),
		LeaningBlindIndex: l.LeaningBlindIndex,
		InferenceSources:  nonNilStrs(l.InferenceSources),
		AssessedAt:        dateText(l.AssessedAt),
		LegalBasis:        l.LegalBasis,
		Confidence:        l.Confidence,
	})
	if err != nil {
		return domain.StoredPoliticalLeaning{}, mapWriteErr(err)
	}
	return toStoredLeaning(row), nil
}

func (r *Repository) UpdatePoliticalLeaning(ctx context.Context, l domain.StoredPoliticalLeaning) (domain.StoredPoliticalLeaning, error) {
	row, err := r.q.UpdatePoliticalLeaning(ctx, personsensitivesql.UpdatePoliticalLeaningParams{
		LeaningCiphertext: l.LeaningCiphertext,
		LeaningWrappedDek: l.LeaningWrappedDEK,
		LeaningKeyRef:     text(l.LeaningKeyRef),
		LeaningBlindIndex: l.LeaningBlindIndex,
		InferenceSources:  nonNilStrs(l.InferenceSources),
		AssessedAt:        dateText(l.AssessedAt),
		LegalBasis:        l.LegalBasis,
		Confidence:        l.Confidence,
		PersonID:          l.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredPoliticalLeaning{}, domain.ErrPoliticalLeaningNotFound
		}
		return domain.StoredPoliticalLeaning{}, mapWriteErr(err)
	}
	return toStoredLeaning(row), nil
}

// GetPoliticalLeaning returns the single active inferred leaning, or ErrPoliticalLeaningNotFound.

// GetPoliticalLeaning returns the single active inferred leaning, or ErrPoliticalLeaningNotFound.
func (r *Repository) GetPoliticalLeaning(ctx context.Context, personID string) (domain.StoredPoliticalLeaning, error) {
	row, err := r.q.GetPoliticalLeaning(ctx, personID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredPoliticalLeaning{}, domain.ErrPoliticalLeaningNotFound
		}
		return domain.StoredPoliticalLeaning{}, err
	}
	return toStoredLeaning(row), nil
}

func (r *Repository) DeletePoliticalLeaning(ctx context.Context, personID string) error {
	if _, err := r.q.DeletePoliticalLeaning(ctx, personID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrPoliticalLeaningNotFound
		}
		return err
	}
	return nil
}

func toCryptoWallet(r personsensitivesql.OikumeneaPersonCryptoWallet) domain.CryptoWallet {
	return domain.CryptoWallet{
		ID:                r.ID,
		PersonID:          r.PersonID,
		Address:           r.Address,
		Chain:             r.Chain,
		AttributionMethod: r.AttributionMethod,
		BalanceUSDApprox:  float8Ptr(r.BalanceUsdApprox),
		FirstSeen:         dateStr(r.FirstSeen),
		LastSeen:          dateStr(r.LastSeen),
		Source:            r.Source,
		Confidence:        r.Confidence,
		CreatedAt:         r.CreatedAt.Time,
		UpdatedAt:         r.UpdatedAt.Time,
	}
}

func toPersonality(r personsensitivesql.OikumeneaPersonPersonality) domain.Personality {
	return domain.Personality{
		ID:         r.ID,
		PersonID:   r.PersonID,
		Framework:  r.Framework,
		Result:     r.Result,
		Instrument: strText(r.Instrument),
		Method:     r.Method,
		AssessedAt: dateStr(r.AssessedAt),
		Source:     r.Source,
		Confidence: r.Confidence,
		CreatedAt:  r.CreatedAt.Time,
		UpdatedAt:  r.UpdatedAt.Time,
	}
}

func toStoredLeaning(r personsensitivesql.OikumeneaPersonPoliticalLeaning) domain.StoredPoliticalLeaning {
	return domain.StoredPoliticalLeaning{
		ID:                r.ID,
		PersonID:          r.PersonID,
		LeaningCiphertext: r.LeaningCiphertext,
		LeaningWrappedDEK: r.LeaningWrappedDek,
		LeaningKeyRef:     strText(r.LeaningKeyRef),
		LeaningBlindIndex: r.LeaningBlindIndex,
		InferenceSources:  r.InferenceSources,
		AssessedAt:        dateStr(r.AssessedAt),
		LegalBasis:        r.LegalBasis,
		Confidence:        r.Confidence,
		CreatedAt:         r.CreatedAt.Time,
		UpdatedAt:         r.UpdatedAt.Time,
	}
}

// ---------------------------------------------------------------- health & vulnerability (M36, D-HealthVulnerability)

// InsertHealthRecord stores a new encrypted category-level record (the detail envelope is sealed
// upstream); UpdateHealthRecord re-seals the single active row for the given (person, kind).
func (r *Repository) InsertHealthRecord(ctx context.Context, h domain.StoredHealthRecord) (domain.StoredHealthRecord, error) {
	row, err := r.q.InsertHealthRecord(ctx, personsensitivesql.InsertHealthRecordParams{
		PersonID:         h.PersonID,
		Kind:             h.Kind,
		DetailCiphertext: h.DetailCiphertext,
		DetailWrappedDek: h.DetailWrappedDEK,
		DetailKeyRef:     text(h.DetailKeyRef),
		DetailBlindIndex: h.DetailBlindIndex,
		IsPublicRecord:   h.IsPublicRecord,
		AssessedAt:       dateText(h.AssessedAt),
		LegalBasis:       h.LegalBasis,
		Source:           h.Source,
		Confidence:       h.Confidence,
	})
	if err != nil {
		return domain.StoredHealthRecord{}, mapWriteErr(err)
	}
	return toStoredHealthRecord(row), nil
}

func (r *Repository) UpdateHealthRecord(ctx context.Context, h domain.StoredHealthRecord) (domain.StoredHealthRecord, error) {
	row, err := r.q.UpdateHealthRecord(ctx, personsensitivesql.UpdateHealthRecordParams{
		DetailCiphertext: h.DetailCiphertext,
		DetailWrappedDek: h.DetailWrappedDEK,
		DetailKeyRef:     text(h.DetailKeyRef),
		DetailBlindIndex: h.DetailBlindIndex,
		IsPublicRecord:   h.IsPublicRecord,
		AssessedAt:       dateText(h.AssessedAt),
		LegalBasis:       h.LegalBasis,
		Source:           h.Source,
		Confidence:       h.Confidence,
		PersonID:         h.PersonID,
		Kind:             h.Kind,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredHealthRecord{}, domain.ErrHealthRecordNotFound
		}
		return domain.StoredHealthRecord{}, mapWriteErr(err)
	}
	return toStoredHealthRecord(row), nil
}

func (r *Repository) DeleteHealthRecord(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteHealthRecord(ctx, personsensitivesql.DeleteHealthRecordParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrHealthRecordNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListHealthRecords(ctx context.Context, personID string) ([]domain.StoredHealthRecord, error) {
	rows, err := r.q.ListHealthRecords(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StoredHealthRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStoredHealthRecord(row))
	}
	return out, nil
}

func (r *Repository) CryptoEraseHealthRecords(ctx context.Context, personID string) (int64, error) {
	return r.q.CryptoEraseHealthRecords(ctx, personID)
}

// UpsertInsurance inserts a new row when i.ID is empty, otherwise replaces the named row.
func (r *Repository) UpsertInsurance(ctx context.Context, i domain.Insurance) (domain.Insurance, error) {
	if i.ID == "" {
		row, err := r.q.InsertInsurance(ctx, personsensitivesql.InsertInsuranceParams{
			PersonID:          i.PersonID,
			Type:              i.Type,
			Provider:          text(i.Provider),
			PolicyReference:   text(i.PolicyReference),
			EmployerSponsored: i.EmployerSponsored,
			ValidFrom:         dateText(i.ValidFrom),
			ValidTo:           dateText(i.ValidTo),
			Source:            i.Source,
			Confidence:        i.Confidence,
		})
		if err != nil {
			return domain.Insurance{}, mapWriteErr(err)
		}
		return toInsurance(row), nil
	}
	row, err := r.q.UpdateInsurance(ctx, personsensitivesql.UpdateInsuranceParams{
		Type:              i.Type,
		Provider:          text(i.Provider),
		PolicyReference:   text(i.PolicyReference),
		EmployerSponsored: i.EmployerSponsored,
		ValidFrom:         dateText(i.ValidFrom),
		ValidTo:           dateText(i.ValidTo),
		Source:            i.Source,
		Confidence:        i.Confidence,
		ID:                i.ID,
		PersonID:          i.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Insurance{}, domain.ErrInsuranceNotFound
		}
		return domain.Insurance{}, mapWriteErr(err)
	}
	return toInsurance(row), nil
}

func (r *Repository) DeleteInsurance(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteInsurance(ctx, personsensitivesql.DeleteInsuranceParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInsuranceNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListInsurance(ctx context.Context, personID string) ([]domain.Insurance, error) {
	rows, err := r.q.ListInsurance(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Insurance, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInsurance(row))
	}
	return out, nil
}

func toStoredHealthRecord(r personsensitivesql.OikumeneaPersonHealthRecord) domain.StoredHealthRecord {
	return domain.StoredHealthRecord{
		ID:               r.ID,
		PersonID:         r.PersonID,
		Kind:             r.Kind,
		DetailCiphertext: r.DetailCiphertext,
		DetailWrappedDEK: r.DetailWrappedDek,
		DetailKeyRef:     strText(r.DetailKeyRef),
		DetailBlindIndex: r.DetailBlindIndex,
		IsPublicRecord:   r.IsPublicRecord,
		AssessedAt:       dateStr(r.AssessedAt),
		LegalBasis:       r.LegalBasis,
		Source:           r.Source,
		Confidence:       r.Confidence,
		CreatedAt:        r.CreatedAt.Time,
		UpdatedAt:        r.UpdatedAt.Time,
	}
}

func toInsurance(r personsensitivesql.OikumeneaPersonInsurance) domain.Insurance {
	return domain.Insurance{
		ID:                r.ID,
		PersonID:          r.PersonID,
		Type:              r.Type,
		Provider:          strText(r.Provider),
		PolicyReference:   strText(r.PolicyReference),
		EmployerSponsored: r.EmployerSponsored,
		ValidFrom:         dateStr(r.ValidFrom),
		ValidTo:           dateStr(r.ValidTo),
		Source:            r.Source,
		Confidence:        r.Confidence,
		CreatedAt:         r.CreatedAt.Time,
		UpdatedAt:         r.UpdatedAt.Time,
	}
}

// ---------------------------------------------------------------- criminal / arrest / court records (M38, D-LegalRecords)

// ResolveCountryID resolves a jurisdiction ISO code to its geo_countries RID (D-Geo hard FK).
func (r *Repository) ResolveCountryID(ctx context.Context, code string) (string, error) {
	id, err := r.q.GetCountryIDByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrUnknownCountry
		}
		return "", err
	}
	return id, nil
}

// InsertLegalRecord stores a new encrypted category-level record (the detail envelope is sealed
// upstream); UpdateLegalRecord re-seals / re-attributes a person's named record.
func (r *Repository) InsertLegalRecord(ctx context.Context, l domain.StoredLegalRecord) (domain.StoredLegalRecord, error) {
	row, err := r.q.InsertLegalRecord(ctx, personsensitivesql.InsertLegalRecordParams{
		PersonID:              l.PersonID,
		Kind:                  l.Kind,
		Disposition:           l.Disposition,
		DetailCiphertext:      l.DetailCiphertext,
		DetailWrappedDek:      l.DetailWrappedDEK,
		DetailKeyRef:          text(l.DetailKeyRef),
		DetailBlindIndex:      l.DetailBlindIndex,
		JurisdictionCountryID: text(l.JurisdictionCountryID),
		OccurredAt:            dateText(l.OccurredAt),
		DispositionDate:       dateText(l.DispositionDate),
		IsSuppressed:          l.IsSuppressed,
		SuppressedReason:      text(l.SuppressedReason),
		LegalBasis:            l.LegalBasis,
		Source:                l.Source,
		Confidence:            l.Confidence,
	})
	if err != nil {
		return domain.StoredLegalRecord{}, mapWriteErr(err)
	}
	return toStoredLegalRecord(row), nil
}

func (r *Repository) UpdateLegalRecord(ctx context.Context, l domain.StoredLegalRecord) (domain.StoredLegalRecord, error) {
	row, err := r.q.UpdateLegalRecord(ctx, personsensitivesql.UpdateLegalRecordParams{
		Kind:                  l.Kind,
		Disposition:           l.Disposition,
		DetailCiphertext:      l.DetailCiphertext,
		DetailWrappedDek:      l.DetailWrappedDEK,
		DetailKeyRef:          text(l.DetailKeyRef),
		DetailBlindIndex:      l.DetailBlindIndex,
		JurisdictionCountryID: text(l.JurisdictionCountryID),
		OccurredAt:            dateText(l.OccurredAt),
		DispositionDate:       dateText(l.DispositionDate),
		IsSuppressed:          l.IsSuppressed,
		SuppressedReason:      text(l.SuppressedReason),
		LegalBasis:            l.LegalBasis,
		Source:                l.Source,
		Confidence:            l.Confidence,
		ID:                    l.ID,
		PersonID:              l.PersonID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredLegalRecord{}, domain.ErrLegalRecordNotFound
		}
		return domain.StoredLegalRecord{}, mapWriteErr(err)
	}
	return toStoredLegalRecord(row), nil
}

func (r *Repository) DeleteLegalRecord(ctx context.Context, personID, id string) error {
	if _, err := r.q.DeleteLegalRecord(ctx, personsensitivesql.DeleteLegalRecordParams{ID: id, PersonID: personID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrLegalRecordNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListLegalRecords(ctx context.Context, personID string, includeSuppressed bool) ([]domain.StoredLegalRecord, error) {
	rows, err := r.q.ListLegalRecords(ctx, personsensitivesql.ListLegalRecordsParams{PersonID: personID, IncludeSuppressed: includeSuppressed})
	if err != nil {
		return nil, err
	}
	out := make([]domain.StoredLegalRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStoredLegalRecordRow(row))
	}
	return out, nil
}

func (r *Repository) CryptoEraseLegalRecords(ctx context.Context, personID string) (int64, error) {
	return r.q.CryptoEraseLegalRecords(ctx, personID)
}

func toStoredLegalRecord(r personsensitivesql.OikumeneaPersonLegalRecord) domain.StoredLegalRecord {
	return domain.StoredLegalRecord{
		ID:                    r.ID,
		PersonID:              r.PersonID,
		Kind:                  r.Kind,
		Disposition:           r.Disposition,
		DetailCiphertext:      r.DetailCiphertext,
		DetailWrappedDEK:      r.DetailWrappedDek,
		DetailKeyRef:          strText(r.DetailKeyRef),
		DetailBlindIndex:      r.DetailBlindIndex,
		JurisdictionCountryID: strText(r.JurisdictionCountryID),
		OccurredAt:            dateStr(r.OccurredAt),
		DispositionDate:       dateStr(r.DispositionDate),
		IsSuppressed:          r.IsSuppressed,
		SuppressedReason:      strText(r.SuppressedReason),
		LegalBasis:            r.LegalBasis,
		Source:                r.Source,
		Confidence:            r.Confidence,
		CreatedAt:             r.CreatedAt.Time,
		UpdatedAt:             r.UpdatedAt.Time,
	}
}

func toStoredLegalRecordRow(r personsensitivesql.ListLegalRecordsRow) domain.StoredLegalRecord {
	return domain.StoredLegalRecord{
		ID:                    r.ID,
		PersonID:              r.PersonID,
		Kind:                  r.Kind,
		Disposition:           r.Disposition,
		DetailCiphertext:      r.DetailCiphertext,
		DetailWrappedDEK:      r.DetailWrappedDek,
		DetailKeyRef:          strText(r.DetailKeyRef),
		DetailBlindIndex:      r.DetailBlindIndex,
		JurisdictionCountryID: strText(r.JurisdictionCountryID),
		Jurisdiction:          r.JurisdictionCode,
		OccurredAt:            dateStr(r.OccurredAt),
		DispositionDate:       dateStr(r.DispositionDate),
		IsSuppressed:          r.IsSuppressed,
		SuppressedReason:      strText(r.SuppressedReason),
		LegalBasis:            r.LegalBasis,
		Source:                r.Source,
		Confidence:            r.Confidence,
		CreatedAt:             r.CreatedAt.Time,
		UpdatedAt:             r.UpdatedAt.Time,
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

// PersonExists reports whether the person exists and is not soft-deleted — the parent-existence guard
// personsensitive writes/reads run before touching a person's sensitive rows (D-PersonModuleSplit, R-09).
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

// GetPerson reads the person core aggregate — needed by CheckWatchlists for the subject's identity
// (name / birthdate / nationality). A reviewed cross-module read of the person core aggregate
// (D-PersonModuleSplit, R-09); ErrNotFound when absent.
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

func toPerson(r personsensitivesql.OikumeneaPersonPerson) domain.Person {
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

// ErasePerson erases all of a person's personsensitive-owned rows in FK-safe order — the purge erasure
// path (D-PersonModuleSplit, review-2026-07 R-09). It is invoked in the purge transaction by the module's
// PersonPurged subscriber (SubscribePersonPurge). Physical descriptions (pii:basic), distinguishing marks
// (pii:special ceiling), watchlist matches + regulatory sanctions and the M35 crypto-wallet/personality
// overlays (pii:sensitive) are hard-deleted; the envelope-encrypted ethnicities, declared party
// memberships and the inferred political leaning (pii:special) are CRYPTO-erased — the wrapped DEK +
// ciphertext are dropped but the row is kept as a tombstone.
func (r *Repository) ErasePerson(ctx context.Context, personID string) error {
	deletes := []func(context.Context, string) error{
		r.q.DeleteAllPhysicalDescriptions,
		r.q.DeleteAllDistinguishingMarks,
		r.q.DeleteAllWatchlistMatches,
		r.q.DeleteAllRegulatorySanctions,
		r.q.DeleteAllCryptoWallets,
		r.q.DeleteAllPersonalities,
		r.q.DeleteAllInsurance,
	}
	for _, step := range deletes {
		if err := step(ctx, personID); err != nil {
			return err
		}
	}
	cryptoErases := []func(context.Context, string) (int64, error){
		r.q.CryptoEraseEthnicities,      // D-PhysicalIdentity, M31
		r.q.CryptoErasePartyMemberships, // D-InstitutionalTies, M33
		r.q.CryptoErasePoliticalLeaning, // D-PersonOverlays, M35
		r.q.CryptoEraseHealthRecords,    // D-HealthVulnerability, M36
		r.q.CryptoEraseLegalRecords,     // D-LegalRecords, M38
	}
	for _, step := range cryptoErases {
		if _, err := step(ctx, personID); err != nil {
			return err
		}
	}
	return nil
}
