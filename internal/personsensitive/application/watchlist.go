// Watchlists & regulatory exposure orchestration (D-Watchlists, M34). CheckWatchlists runs a live
// screening check OUT to the hermenea companion via the late-bound WatchlistLookup seam (the PDP core
// makes no egress call itself), combines the returned match metadata with the locally-derived PEP flag
// (M33 government positions), and upserts the single per-person WatchlistMatch — match metadata only,
// never the lists. RegulatorySanction is a durable pii:sensitive overlay with audited CRUD. Every write
// records an audit row in the same transaction (D-Audit).
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

// ---------------------------------------------------------------- watchlist match (live-lookup)

// CheckWatchlists runs a live watchlist screening check for a person: it screens the person's identity
// via the hermenea seam, snapshots the PEP flag from the M33 government positions, and upserts the single
// per-person WatchlistMatch. Only match metadata is stored. Returns ErrWatchlistUnavailable when no
// companion is configured (the seam is watchlistclient.Disabled{}).
func (s *Service) CheckWatchlists(ctx context.Context, personID string) (domain.WatchlistMatch, error) {
	// The watchlist seam is always wired at boot (MustBeBound, review-2026-07 R-11): the real hermenea
	// client when a companion is configured, else watchlistclient.Disabled{} whose Screen returns
	// ErrWatchlistUnavailable — so no nil check is needed here.
	repo := s.newRepo(s.pool)
	person, err := repo.GetPerson(ctx, personID)
	if err != nil {
		return domain.WatchlistMatch{}, err
	}
	// PEP snapshot from personprofile's government-position ties, read through the late-bound seam
	// (D-PersonModuleSplit, R-09); defaults to false when the seam is unwired (e.g. tests not exercising it).
	pep := false
	if s.pep != nil {
		if pep, err = s.pep.IsPoliticallyExposed(ctx, personID); err != nil {
			return domain.WatchlistMatch{}, err
		}
	}

	res, err := s.watchlist.Screen(ctx, domain.WatchlistQuery{
		SubjectKey:  personID,
		FullName:    person.DisplayName,
		Birthdate:   person.Birthdate,
		Nationality: person.CountryOfBirth,
	})
	if err != nil {
		return domain.WatchlistMatch{}, err
	}

	lists := res.Lists
	if lists == nil {
		lists = []string{}
	}
	match := domain.WatchlistMatch{
		PersonID:     personID,
		OnList:       res.OnList,
		Lists:        lists,
		Program:      res.Program,
		MatchScore:   res.MatchScore,
		PEP:          pep,
		LastChecked:  s.now(),
		NextCheckDue: res.NextCheckDue,
		Source:       "imported",
		Confidence:   screeningConfidence(res),
	}

	var out domain.WatchlistMatch
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).UpsertWatchlistMatch(ctx, match)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.watchlist.check", personID,
			map[string]any{"id": personID, "onList": created.OnList, "pep": created.PEP})
	})
	return out, err
}

// GetWatchlistMatch returns the person's most recent screening result, or (zero,false) if never screened.
func (s *Service) GetWatchlistMatch(ctx context.Context, personID string) (domain.WatchlistMatch, bool, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return domain.WatchlistMatch{}, false, err
	}
	return repo.GetWatchlistMatch(ctx, personID)
}

// screeningConfidence maps a match to the attribution confidence scale: a perfect-score hit is confirmed,
// any other hit probable, no hit possible.
func screeningConfidence(res domain.WatchlistScreenResult) string {
	if !res.OnList {
		return "possible"
	}
	if res.MatchScore != nil && *res.MatchScore >= 1.0 {
		return "confirmed"
	}
	return "probable"
}

// ---------------------------------------------------------------- regulatory sanctions (pii:sensitive)

// ListRegulatorySanctions lists a person's regulatory sanctions (the person must exist).
func (s *Service) ListRegulatorySanctions(ctx context.Context, personID string) ([]domain.RegulatorySanction, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListRegulatorySanctions(ctx, personID)
}

// UpsertRegulatorySanction adds a regulatory sanction (idempotent by externalId when x.ID is empty) or
// replaces the named row by RID.
func (s *Service) UpsertRegulatorySanction(ctx context.Context, x domain.RegulatorySanction) (domain.RegulatorySanction, error) {
	if x.ActionType == "" {
		x.ActionType = "other"
	}
	if x.Status == "" {
		x.Status = "active"
	}
	if x.Source == "" {
		x.Source = "operator_verified"
	}
	if x.Confidence == "" {
		x.Confidence = "possible"
	}
	if err := x.Validate(); err != nil {
		return domain.RegulatorySanction{}, err
	}
	var out domain.RegulatorySanction
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, x.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertRegulatorySanction(ctx, x)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.regulatory_sanction.upsert", x.PersonID,
			map[string]any{"id": x.PersonID, "sanctionId": created.ID, "regulator": created.Regulator})
	})
	return out, err
}

// DeleteRegulatorySanction soft-deletes a regulatory sanction.
func (s *Service) DeleteRegulatorySanction(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteRegulatorySanction(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.regulatory_sanction.delete", personID, map[string]any{"id": personID, "sanctionId": id})
	})
}
