// Package application holds the personsensitive module's application service — the orchestrator for the
// person directory's most sensitive, envelope-encrypted or pii:special data (R-09 split): physical
// identity (descriptions, distinguishing marks), the self-declared encrypted ethnicity, the financial /
// behavioural / psychological overlays (crypto wallets, personality, the inferred political leaning),
// the encrypted declared party membership (M33), and watchlist matches + regulatory sanctions.
//
// Consolidating these here puts the crypto Cipher seam and its reviewers in one module (the R-09
// rationale). It shares the person aggregate root's domain kernel (internal/person/domain) and the
// transaction+audit plumbing (internal/person/appkit); it does not import the person core application or
// adapters packages. Audit payloads carry only non-PII identifiers (D-Audit).
package application

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/person/appkit"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
)

// auditSubsystem labels the interim system actor for personsensitive admin writes (mirrors the person
// core subsystem — these are still person-scoped admin actions).
const auditSubsystem = "person-admin"

// RepositoryFactory binds a domain.SensitiveRepository to a command surface — the pool for reads, or a caller's
// transaction for an audited write (D-Audit). Injected by module.go. For PR-2a the factory returns the
// unified person adapters repository (the repository/query split lands in PR-2b).
type RepositoryFactory func(conn db.DBTX) domain.SensitiveRepository

// Service is the personsensitive application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly.
type Service struct {
	pool      *pgxpool.Pool
	newRepo   RepositoryFactory
	rec       *appkit.Recorder
	now       func() time.Time
	cipher    *crypto.Cipher         // envelope cipher for pii:special declared ethnicity / party / political leaning
	colors    domain.ColorLookup     // late-bound color catalog (D-Color): eye/hair hard-FK palette check
	watchlist domain.WatchlistLookup // late-bound hermenea screening seam (D-Watchlists, M34)
	pep       domain.PEPStatusReader // late-bound PEP snapshot from personprofile government ties (R-09 split)
}

// NewService wires the service with the pool, the repository factory, the audit service, and the envelope
// cipher (D-CryptoProvider). The color and watchlist seams are late-bound (SetColorLookup / SetWatchlistLookup).
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, cipher *crypto.Cipher) *Service {
	return &Service{pool: pool, newRepo: newRepo, rec: appkit.NewRecorder(audit), cipher: cipher, now: func() time.Time { return time.Now().UTC() }}
}

// SetColorLookup binds the cross-module color catalog query seam (D-Color) used to enforce the eye/hair
// physical-description hard FKs against their palettes. Late-bound at composition time; when unset (e.g.
// tests that don't exercise color), the palette check is skipped.
func (s *Service) SetColorLookup(c domain.ColorLookup) { s.colors = c }

// SetWatchlistLookup binds the cross-module watchlist screening seam (D-Watchlists, M34) used by
// CheckWatchlists to run a live check out to the hermenea companion. Late-bound at composition time;
// when set to watchlistclient.Disabled{} the companion is not configured and CheckWatchlists returns
// domain.ErrWatchlistUnavailable.
func (s *Service) SetWatchlistLookup(w domain.WatchlistLookup) { s.watchlist = w }

// SetPEPStatusReader binds the personprofile-owned PEP snapshot seam (D-PersonModuleSplit, R-09) used by
// CheckWatchlists to record whether the person holds any active PEP-triggering government position. Late-
// bound at composition time (personprofile is built alongside personsensitive); when unset the screening
// still runs and the PEP flag defaults to false.
func (s *Service) SetPEPStatusReader(p domain.PEPStatusReader) { s.pep = p }

// MustBeBound reports whether the mandatory cross-module seams are wired (review-2026-07 R-11): the
// composition root calls it at boot so a forgotten setter fails startup instead of a request-time nil
// deref. The watchlist seam counts as bound when set to watchlistclient.Disabled{}; the color seam is
// deliberately optional and not asserted.
func (s *Service) MustBeBound() error {
	if s.watchlist == nil {
		return errors.New("personsensitive service: watchlist lookup seam not bound (SetWatchlistLookup)")
	}
	return nil
}

// inTx runs fn in a single transaction on the pool (a write and its audit row commit together).
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return appkit.InTx(ctx, s.pool, fn)
}

// record writes a person-admin audit entry in the caller's transaction (non-PII payload only).
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	return s.rec.Record(ctx, tx, auditSubsystem, action, targetID, after)
}

// checkColor enforces that a physical-description color id belongs to the expected eye/hair palette
// (D-Color). A no-op when the id is empty or the color seam is unbound.
func (s *Service) checkColor(ctx context.Context, colorID, wantDomain string) error {
	if colorID == "" || s.colors == nil {
		return nil
	}
	d, err := s.colors.ColorDomain(ctx, colorID)
	if err != nil || d != wantDomain {
		return domain.ErrColorMismatch
	}
	return nil
}
