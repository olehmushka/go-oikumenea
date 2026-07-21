// Login security log (M37 / D-LoginSecurityLog) — the application-layer surface over
// domain.LoginEventRepository. The validation middleware records a deduped login/IP occurrence per
// validated request (best-effort, off the request's critical path); an operator reads an account's
// history; the log is purge-erased with the person and retention-swept. All late-bound via
// SetLoginEvents so out-of-request callers (CLI, tests) that never configure it simply no-op.
package application

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
)

// LoginEventRepositoryFactory binds a domain.LoginEventRepository to a command surface (the pool for
// record/read/sweep, the purge tx for the erase fan-out). Injected by module.go.
type LoginEventRepositoryFactory func(conn db.DBTX) domain.LoginEventRepository

// IPIntelResolver is the (deferred) IP-intelligence seam — satisfied by ipintel.NoOp by default.
type IPIntelResolver interface {
	Resolve(ctx context.Context, ip string) domain.IPIntel
}

// SetLoginEvents late-binds the login-security-log dependencies (M37). windowSeconds is the dedup
// window (one row per (account, context, ip) per window); retentionDays bounds the sweep (0 = retain
// forever). Called once at composition, before the server starts.
func (s *Service) SetLoginEvents(newLoginRepo LoginEventRepositoryFactory, ipIntel IPIntelResolver, windowSeconds, retentionDays int) {
	s.newLoginRepo = newLoginRepo
	s.ipIntel = ipIntel
	s.loginWindowSecs = windowSeconds
	s.retentionDays = retentionDays
}

// RecordLoginSeen records a deduped login/IP occurrence for accountID (M37). Best-effort: it resolves
// the (deferred) IP intel, upserts within the dedup window, and logs — never returning an error to the
// caller's control flow — so a logging failure can never fail a request. The middleware calls it
// off the critical path. A nil store (unconfigured), empty account, or empty ip is a silent no-op.
func (s *Service) RecordLoginSeen(ctx context.Context, accountID string, c domain.LoginContext, ip, userAgent string) {
	if s.newLoginRepo == nil || accountID == "" || ip == "" {
		return
	}
	var intel domain.IPIntel
	if s.ipIntel != nil {
		intel = s.ipIntel.Resolve(ctx, ip)
	}
	var uaPtr *string
	if userAgent != "" {
		uaPtr = &userAgent
	}
	if err := s.newLoginRepo(s.pool).RecordSeen(ctx, accountID, c, ip, uaPtr, intel, s.loginWindowSecs); err != nil {
		svc1log.FromContext(ctx).Warn("record login event", svc1log.SafeParam("context", string(c)), svc1log.Stacktrace(err))
	}
}

// ListLoginEvents pages an account's login history newest-first (account.security-log.read). afterID
// "" = first page.
func (s *Service) ListLoginEvents(ctx context.Context, accountID, afterID string, limit int) ([]domain.LoginEvent, error) {
	if s.newLoginRepo == nil {
		return nil, nil
	}
	return s.newLoginRepo(s.pool).ListByAccount(ctx, accountID, afterID, limit)
}

// SubscribePersonPurge erases a purged person's login events in the purge transaction (D-PersonModuleSplit
// / D-LoginSecurityLog: pii:contact is erased, not retained). Mirrors documentSvc.SubscribePersonPurge.
func (s *Service) SubscribePersonPurge(bus *events.Bus) {
	bus.Subscribe(personevents.TypePersonPurged, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(personevents.PersonPurged)
		if !ok || s.newLoginRepo == nil {
			return nil
		}
		_, err := s.newLoginRepo(tx).EraseByPerson(ctx, e.ID)
		return err
	})
}

// SweepLoginEvents deletes events older than the configured retention window (login-security.retention-days;
// 0 = retain forever → no-op), returning the number deleted. The operator/boot enforcer calls it.
func (s *Service) SweepLoginEvents(ctx context.Context) (int64, error) {
	if s.newLoginRepo == nil || s.retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -s.retentionDays)
	return s.newLoginRepo(s.pool).DeleteBefore(ctx, cutoff)
}
