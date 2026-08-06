// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package application holds the person module's application service — the orchestrator the transport
// layer calls to read/mutate the directory, recording an audit row in the same transaction as each
// write (D-Audit). It depends on the domain port, the platform DB surface, and the audit service; it
// never imports the adapters package directly (the repository factory is injected by module.go).
//
// Person is the primary PII store, so audit payloads here carry only non-PII identifiers (the id,
// and the changed key/status) — never names or other personal data. A person holds at most one rank
// per rank system, a directory attribute; this service never reads rank to make a decision (D-Rank).
package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	orderevents "github.com/olehmushka/go-oikumenea/internal/order/events"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	personevents "github.com/olehmushka/go-oikumenea/internal/person/events"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/olehmushka/go-oikumenea/pkg/events"
	"github.com/olehmushka/go-oikumenea/pkg/listing"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// Page-size policy (API conventions: token pagination, bounded pages).
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

// pageSize is this module's page-size policy (M56 / pkg/listing): the shared clamp bound to the
// module's own Default/Max, replacing the per-module resolvePageSize copy.
var pageSizePolicy = listing.PageSize{Default: DefaultPageSize, Max: MaxPageSize}

// auditSubsystem labels the interim system actor for person's admin writes. Until authorization (M7)
// + identity-federation (M8) resolve the acting person, these writes are recorded as a `system`
// action under this subsystem (the no-unaudited-mutation ground rule still holds).
const auditSubsystem = "person-admin"

// eventSubsystem labels the system actor for a write made by an ORDER-EVENT SUBSCRIBER (D-OrderApply):
// an order's rank-change effect, auto-applied in the issue transaction, audits as `system` /
// `event-subscriber`, correlated to the human's order.issue row by the shared request_id.
const eventSubsystem = "event-subscriber"

// targetPerson is the audited entity kind; every person-scoped action targets the person id.
const targetPerson = "person"

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a
// caller's transaction for an audited write (D-Audit). Injected by module.go.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// MembershipReader is the cross-module query seam the read-scope projection (D-PersonReadScope) uses
// to resolve who the subject may read. Since review-2026-07 R-02.1 the reach set never leaves the
// database: membership computes visibility as a SQL semi-join over memberships × the subject's authz
// reach. The membership application service satisfies it; it is late-bound (SetMembershipReader)
// because membership is composed after person (overview.md composition ordering).
type MembershipReader interface {
	ActiveUnitIDsForPerson(ctx context.Context, personID string) ([]string, error)
	// VisiblePersonIDsForSubject carries the person facet filter (M56 / D-ObjectFacets) INTO the
	// visibility SQL: every predicate must run before the LIMIT, or a filtered page comes back short
	// with a nextPageToken. The filter type is person's — person owns the vocabulary, membership
	// consumes it, the same direction as the @query predicate R-06 already folded in.
	VisiblePersonIDsForSubject(ctx context.Context, subjectPersonID, after string, f domain.PersonFilter, limit int) ([]string, error)
	SubjectCanReadPerson(ctx context.Context, subjectPersonID, personID string) (bool, error)
	// VisiblePersonStatsForSubject is the dashboard counterpart (M57 / D-ObjectFacets): the same
	// filter and the same visibility predicate, aggregated instead of paged. It lives behind this
	// seam for the same reason the id query does — the reach never leaves the database.
	VisiblePersonStatsForSubject(ctx context.Context, subjectPersonID string, f domain.PersonFilter, sel stats.Selection) ([]stats.Group, error)
}

// Service is the person application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly. graceHours supplies the deactivate->purge window.
type Service struct {
	pool       *pgxpool.Pool
	newRepo    RepositoryFactory
	audit      *auditapp.Service
	graceHours func() int
	now        func() time.Time
	membership MembershipReader
	bus        *events.Bus // set when SubscribeOrderEvents wires the bus; used to publish PersonMerged
	// labeler resolves a dashboard's ref-bucket RIDs to locale->text names (M57). Optional: unset,
	// a chart segment carries its RID and the client falls back to the RID tail.
	labeler stats.Labeler
}

// NewService wires the service with the pool, the repository factory, the audit service, and the
// (refreshable) purge-grace window in hours. The membership reader is late-bound (SetMembershipReader).
// (The envelope cipher + the physical/ethnicity/overlay/watchlist crypto surface moved to the
// personsensitive module under R-09.)
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, graceHours func() int) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, graceHours: graceHours, now: func() time.Time { return time.Now().UTC() }}
}

// SetMembershipReader binds the cross-module membership query seam used by the read-scope projection
// (D-PersonReadScope). Called once at composition time, after membership is built, before serving.
func (s *Service) SetMembershipReader(r MembershipReader) { s.membership = r }

// SetBucketLabeler binds the optional dashboard label resolver (M57 / D-ObjectFacets), wired at the
// composition root from the same per-type labelers the links service uses — so a unit is named
// identically in a graph row and in a chart segment.
func (s *Service) SetBucketLabeler(l stats.Labeler) { s.labeler = l }

// MustBeBound reports whether the mandatory cross-module seams are wired. The composition root calls
// it at boot (review-2026-07 R-11) so a forgotten setter fails startup instead of surfacing as a
// request-time nil deref or a silently-empty read-scope page (which reads as "no access"). (The location
// lookup seam moved to personprofile and the watchlist seam to personsensitive under R-09.)
func (s *Service) MustBeBound() error {
	if s.membership == nil {
		return errors.New("person service: membership reader seam not bound (SetMembershipReader)")
	}
	return nil
}

// Page is a keyset-paginated slice of the directory.
type Page struct {
	Persons       []domain.Person
	NextPageToken string
}

// ---------------------------------------------------------------- persons

// CreatePerson validates and creates a person (no account/unit required), then records the action.
func (s *Service) CreatePerson(ctx context.Context, p domain.Person) (domain.Person, error) {
	if p.Sex == "" {
		p.Sex = domain.DefaultSex
	}
	if err := p.Validate(); err != nil {
		return domain.Person{}, err
	}
	var out domain.Person
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertPerson(ctx, p)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.create", created.ID, map[string]any{"id": created.ID})
	})
	return out, err
}

// CreateProvisionalPerson creates a minimal-PII stub person (status='provisional') — an unresolved
// external/edge-target node so a relationship or overlay edge points at a real person (D-OverlayFoundation).
// It carries a display name and optional source/confidence attribution; it is resolved later by MergePerson.
func (s *Service) CreateProvisionalPerson(ctx context.Context, p domain.Person) (domain.Person, error) {
	if p.Sex == "" {
		p.Sex = domain.DefaultSex
	}
	if err := p.Validate(); err != nil {
		return domain.Person{}, err
	}
	var out domain.Person
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertProvisionalPerson(ctx, p)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.provisional.create", created.ID, map[string]any{"id": created.ID})
	})
	return out, err
}

// MergePerson resolves a provisional stub (fromID) into a canonical person (intoID): in ONE transaction
// it re-homes the stub's person-owned edges, publishes PersonMerged so every other module re-homes its
// person-referencing rows on the same transaction, then tombstones the stub (PII nulled, status=purged).
// fromID must be provisional; intoID must be a distinct, non-provisional/non-purged person. The
// resulting canonical person is returned. D-OverlayFoundation (M29).
func (s *Service) MergePerson(ctx context.Context, fromID, intoID, confidence string) (domain.Person, error) {
	if confidence == "" {
		confidence = domain.DefaultConfidence
	}
	if !domain.ValidConfidence(confidence) {
		return domain.Person{}, domain.ErrInvalid
	}
	if fromID == intoID {
		return domain.Person{}, domain.ErrMergeIntoInvalid
	}
	var out domain.Person
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		from, err := repo.GetPerson(ctx, fromID)
		if err != nil {
			return err
		}
		if from.Status != domain.StatusProvisional {
			return domain.ErrMergeNotProvisional
		}
		into, err := repo.GetPerson(ctx, intoID)
		if err != nil {
			return err
		}
		if into.Status == domain.StatusProvisional || into.Status == domain.StatusPurged {
			return domain.ErrMergeIntoInvalid
		}
		// Re-home the stub's person-OWNED edges (relationships, names, contacts, …) fromID → intoID.
		if err := repo.RepointPersonOwned(ctx, fromID, intoID); err != nil {
			return err
		}
		// Re-home every OTHER module's person-referencing rows on this same transaction (D-OverlayFoundation):
		// the PersonMerged subscribers (membership, document, authorization, …) run synchronously here.
		if s.bus != nil {
			if err := s.bus.Publish(ctx, tx, personevents.PersonMerged{FromID: fromID, IntoID: intoID, Confidence: confidence}); err != nil {
				return err
			}
		}
		// Tombstone the stub: Purge hard-deletes its core name variants and nulls its residual PII, flipping
		// status → purged (the merged-away marker).
		if _, err := repo.Purge(ctx, fromID); err != nil {
			return err
		}
		// Erase the stub's residual profile/sensitive rows via PersonPurged (D-PersonModuleSplit, R-09): most
		// person-owned rows were re-homed by RepointPersonOwned above, but the single-per-person rows that
		// cannot be re-homed (watchlist match, inferred political leaning) and the stub-only physical /
		// ethnicity / address data are dropped here — the profile/sensitive erasers are no-ops for the rows
		// already re-pointed to intoID. Fired AFTER PersonMerged so the re-point wins.
		if s.bus != nil {
			if err := s.bus.Publish(ctx, tx, personevents.PersonPurged{ID: fromID}); err != nil {
				return err
			}
		}
		if out, err = repo.GetPerson(ctx, intoID); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.merge", intoID, map[string]any{"id": intoID, "mergedFrom": fromID, "confidence": confidence})
	})
	return out, err
}

// PersonIDByCode resolves an active person's RID from their stable code, reporting whether a match
// was found. It is the cross-module query identity-federation's just-in-time link-on-match (D-JIT)
// uses to map a token claim -> person.code -> person without exposing the person aggregate. An empty
// code never matches.
func (s *Service) PersonIDByCode(ctx context.Context, code string) (string, bool, error) {
	if code == "" {
		return "", false, nil
	}
	p, err := s.newRepo(s.pool).GetActivePersonByCode(ctx, code)
	if errors.Is(err, domain.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return p.ID, true, nil
}

// GetPerson reads one person with its CORE child slices — ranks and name variants — attached. The
// non-encrypted directory child data (citizenships, residences, contact channels, …) is owned by
// personprofile and composed onto the API response by the transport layer (D-PersonModuleSplit, R-09).
func (s *Service) GetPerson(ctx context.Context, id string) (domain.Person, error) {
	repo := s.newRepo(s.pool)
	p, err := repo.GetPerson(ctx, id)
	if err != nil {
		return domain.Person{}, err
	}
	if p.Ranks, err = repo.ListPersonRanks(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if p.NameVariants, err = repo.ListNameVariants(ctx, id); err != nil {
		return domain.Person{}, err
	}
	return p, nil
}

// UpdatePerson applies a partial change to names/bio/attributes and records the action.
func (s *Service) UpdatePerson(ctx context.Context, id string, patch domain.PersonPatch) (domain.Person, error) {
	if err := patch.Validate(); err != nil {
		return domain.Person{}, err
	}
	var out domain.Person
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdatePerson(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "person.update", id, map[string]any{"id": id})
	})
	return out, err
}

// ListPersons returns a keyset-paginated page of the directory (by time-ordered RID), narrowed by
// the person facet set and optionally by a case-insensitive name/code substring — both applied
// server-side, before the LIMIT, so the keyset cursor stays correct (M56 / D-ObjectFacets, R-06).
//
// This is the INSTANCE-ADMIN path; a scoped caller goes through ListVisiblePersons, which carries
// the same filter through membership's visibility queries. The filter is validated HERE, once, so
// both paths reject an ill-formed facet value identically.
func (s *Service) ListPersons(ctx context.Context, f domain.PersonFilter, pageSize int, pageToken string) (Page, error) {
	if err := f.Validate(); err != nil {
		return Page{}, err
	}
	size := pageSizePolicy.Resolve(pageSize)
	after, err := listing.DecodeCursor(pageToken)
	if err != nil {
		return Page{}, err
	}
	// The request-pinned RLS connection, NOT the bare pool. person's own tables carry no row
	// security, so this path used the pool for years — but the M56 `unitId` facet probes
	// `membership_memberships`, which IS RLS-protected, and on an unpinned connection the app.* GUCs
	// are unset: `authz_unit_in_reach` then sees neither the instance-admin bypass nor any grant, the
	// EXISTS matches nothing, and the filter silently returns an EMPTY page to a caller who may read
	// every one of those people. Caught by the live end-to-end run, not by the integration suite,
	// which connects as a superuser and so bypasses RLS entirely.
	persons, err := s.newRepo(db.RequestQuerier(ctx, s.pool)).ListPersons(ctx, f, after, size+1)
	if err != nil {
		return Page{}, err
	}
	if len(persons) > size {
		return Page{Persons: persons[:size], NextPageToken: listing.EncodeCursor(persons[size-1].ID)}, nil
	}
	return Page{Persons: persons}, nil
}

// ReadablePerson decides whether subjectPersonID may read personID under D-PersonReadScope: true iff
// the person's active-membership units intersect the subject's effective readable reach, computed as
// one SQL point probe (review-2026-07 R-02.1). A membership-less person has no units, so nobody but
// an instance admin sees them — the INSTANCE-ADMIN SHORT-CIRCUIT IS THE CALLER'S (transport checks
// pep.SubjectAuthority first; person.read is never instance-scoped). The reach predicate subsumes the
// shadow gate (an unreachable shadow unit is simply not in reach).
func (s *Service) ReadablePerson(ctx context.Context, subjectPersonID, personID string) (bool, error) {
	// The membership seam is guaranteed wired at boot (MustBeBound, review-2026-07 R-11); only the
	// input guard remains — an unauthenticated (empty) subject reads nobody.
	if subjectPersonID == "" {
		return false, nil
	}
	return s.membership.SubjectCanReadPerson(ctx, subjectPersonID, personID)
}

// ListVisiblePersons returns the keyset-paginated union of people a non-instance-admin subject may
// read (D-PersonReadScope): the directory rows whose active memberships fall in the subject's
// effective readable reach — one SQL semi-join, O(page) regardless of reach size (review-2026-07
// R-02.1; the reach set never leaves the database). The instance-admin case is the unrestricted
// ListPersons and is handled by the caller. Pagination keys on the person RID, matching the
// membership union's ordering, so the returned rows are already in token order.
func (s *Service) ListVisiblePersons(ctx context.Context, subjectPersonID string, f domain.PersonFilter, pageSize int, pageToken string) (Page, error) {
	if err := f.Validate(); err != nil {
		return Page{}, err
	}
	size := pageSizePolicy.Resolve(pageSize)
	after, err := listing.DecodeCursor(pageToken)
	if err != nil {
		return Page{}, err
	}
	if subjectPersonID == "" { // membership seam guaranteed wired at boot (MustBeBound, R-11)
		return Page{}, nil
	}
	// The WHOLE filter — the optional @query (review-2026-07 R-06) and every facet added in M56 — is
	// folded into the membership semi-join SQL: each predicate runs before the LIMIT, so the page is
	// already filtered and the keyset on person RID stays correct. No Go-side re-filter, and so no
	// empty-page-while-hasMore. Nothing here may ever become a post-filter.
	f.Query = strings.TrimSpace(f.Query)
	ids, err := s.membership.VisiblePersonIDsForSubject(ctx, subjectPersonID, after, f, size+1)
	if err != nil {
		return Page{}, err
	}
	hasMore := len(ids) > size
	if hasMore {
		ids = ids[:size]
	}
	// Both the membership union and ListPersonsByIDs order ascending by person RID, so the hydrated
	// rows are already in token order (a soft-deleted person is simply dropped, never reordered).
	persons, err := s.newRepo(db.RequestQuerier(ctx, s.pool)).ListPersonsByIDs(ctx, ids)
	if err != nil {
		return Page{}, err
	}
	if hasMore && len(ids) > 0 {
		return Page{Persons: persons, NextPageToken: listing.EncodeCursor(ids[len(ids)-1])}, nil
	}
	return Page{Persons: persons}, nil
}

// PersonStats is the directory dashboard (M57 / D-ObjectFacets): every selected facet's distribution
// plus the total, over EXACTLY the set ListPersons/ListVisiblePersons would page under the same
// filter. One round-trip, one scan, counts taken inside the visibility predicate.
//
// The admin/scoped dispatch is the caller's, as it is for the list: an instance admin aggregates the
// whole directory, anyone else aggregates their read-scope union (subjectPersonID non-empty). The
// filter is validated here, once, so both arms reject an ill-formed facet value identically.
func (s *Service) PersonStats(ctx context.Context, subjectPersonID string, isAdmin bool, f domain.PersonFilter, sel stats.Selection) (stats.Result, error) {
	if err := f.Validate(); err != nil {
		return stats.Result{}, err
	}
	f.Query = strings.TrimSpace(f.Query)
	// stats.Compute owns the arm convention (an empty subject means the admin arm, and a NON-admin with
	// no subject must never fall into it), so the only thing left here is WHICH repository answers:
	// person's own for the admin arm, membership's reach-scoped query otherwise — the same split
	// ListPersons/ListVisiblePersons make.
	return stats.Compute(ctx, s.labeler, sel, isAdmin, subjectPersonID, func(subject string) ([]stats.Group, error) {
		if subject == "" {
			// The request-pinned RLS connection, not the bare pool: the unitId facet and its filter probe
			// membership_memberships, which IS row-secured, and on an unpinned connection the app.* GUCs
			// are unset — the predicate would then match nothing and report a confident zero (the M56
			// ticket-2 empty-page bug, in its counting form). The db source guard holds this line.
			return s.newRepo(db.RequestQuerier(ctx, s.pool)).PersonStats(ctx, f, sel)
		}
		// The membership seam is guaranteed wired at boot (MustBeBound, R-11).
		return s.membership.VisiblePersonStatsForSubject(ctx, subject, f, sel)
	})
}

// SetPersonRank sets the person's rank in one rank system, or clears it (a directory attribute;
// D-Rank), and records it. When rankID != nil the rank's system is DERIVED from the rank (systemID is
// ignored); when rankID == nil the active rank in systemID is cleared. The returned person carries its
// hydrated ranks.
func (s *Service) SetPersonRank(ctx context.Context, id, systemID string, rankID *string) (domain.Person, error) {
	var out domain.Person
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.setRankTx(ctx, tx, auditSubsystem, id, systemID, rankID, "")
		out = updated
		return err
	})
	return out, err
}

// setRankTx is the shared rank-set core, running on the caller's transaction and recording under
// `subsystem`. orderItemID, when set (the order rank-change effect path), is carried into the audit
// payload as provenance — the HOLDS_RANK link has no provenance FK (D-OrderApply). Returns the person
// with its ranks re-hydrated.
func (s *Service) setRankTx(ctx context.Context, tx pgx.Tx, subsystem, id, systemID string, rankID *string, orderItemID string) (domain.Person, error) {
	repo := s.newRepo(tx)
	after := map[string]any{"id": id}
	if rankID != nil && *rankID != "" {
		pr, err := repo.UpsertPersonRank(ctx, id, *rankID)
		if err != nil {
			return domain.Person{}, err
		}
		after["systemId"], after["rankId"] = pr.SystemID, pr.RankID
	} else {
		if systemID == "" {
			return domain.Person{}, domain.ErrInvalid
		}
		if err := repo.ClearPersonRank(ctx, id, systemID); err != nil {
			return domain.Person{}, err
		}
		after["systemId"], after["rankId"] = systemID, nil
	}
	if orderItemID != "" {
		after["orderItemId"] = orderItemID
	}
	updated, err := repo.GetPerson(ctx, id)
	if err != nil {
		return domain.Person{}, err
	}
	if updated.Ranks, err = repo.ListPersonRanks(ctx, id); err != nil {
		return domain.Person{}, err
	}
	return updated, s.recordWith(ctx, tx, subsystem, "person.rank.assign", id, after)
}

// SubscribeOrderEvents registers the person rank-change handler on the bus: RankChangeOrdered sets the
// person's rank synchronously in the order's issue transaction (D-OrderApply), so a failure rolls the
// whole issue back. Registered once at composition time (module.go), before serving.
func (s *Service) SubscribeOrderEvents(bus *events.Bus) {
	s.bus = bus // retained so MergePerson can publish PersonMerged on the same bus (D-OverlayFoundation)
	bus.Subscribe(orderevents.TypeRankChangeOrdered, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(orderevents.RankChangeOrdered)
		if !ok {
			return nil
		}
		rankID := e.RankID
		// The order rank-change effect always names a concrete rank; its system is derived in SQL.
		_, err := s.setRankTx(ctx, tx, eventSubsystem, e.PersonID, "", &rankID, e.OrderItemID)
		return err
	})
}

// ---------------------------------------------------------------- lifecycle

// DeactivatePerson begins reversible deactivation, opening the purge grace window. Allowed only from
// active.
func (s *Service) DeactivatePerson(ctx context.Context, id, reason string) (domain.Person, error) {
	var out domain.Person
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		p, err := repo.GetPerson(ctx, id)
		if err != nil {
			return err
		}
		if p.Status != domain.StatusActive {
			return domain.ErrLifecycle
		}
		purgeAfter := s.now().Add(time.Duration(s.graceHours()) * time.Hour)
		updated, err := repo.Deactivate(ctx, id, purgeAfter)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "person.deactivate", id, map[string]any{"id": id, "status": string(updated.Status), "reason": reason})
	})
	return out, err
}

// ReactivatePerson cancels deactivation within the grace window. Allowed only from deactivated.
func (s *Service) ReactivatePerson(ctx context.Context, id string) (domain.Person, error) {
	var out domain.Person
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		p, err := repo.GetPerson(ctx, id)
		if err != nil {
			return err
		}
		if !p.CanReactivate() {
			return domain.ErrLifecycle
		}
		updated, err := repo.Reactivate(ctx, id)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "person.reactivate", id, map[string]any{"id": id, "status": string(updated.Status)})
	})
	return out, err
}

// PurgePerson hard-erases PII after the grace window. Idempotent: a person already purged is returned
// unchanged. Refused before purge_after or when never deactivated (D-PersonReadScope erasure path).
func (s *Service) PurgePerson(ctx context.Context, id string) (domain.Person, error) {
	var out domain.Person
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		p, err := repo.GetPerson(ctx, id)
		if err != nil {
			return err
		}
		if p.Status == domain.StatusPurged {
			out = p // idempotent no-op
			return nil
		}
		if !p.CanPurge(s.now()) {
			return domain.ErrLifecycle
		}
		purged, err := repo.Purge(ctx, id)
		if err != nil {
			return err
		}
		// Every OTHER module that holds this person's rows erases its own rows on this same transaction
		// (D-PersonModuleSplit) — the counterpart to MergePerson's PersonMerged re-point. person no longer
		// deletes education_*/company_* tables inline; those owners subscribe via SubscribeErase.
		if s.bus != nil {
			if err := s.bus.Publish(ctx, tx, personevents.PersonPurged{ID: id}); err != nil {
				return err
			}
		}
		out = purged
		return s.record(ctx, tx, "person.purge", id, map[string]any{"id": id, "status": string(purged.Status)})
	})
	return out, err
}

// ---------------------------------------------------------------- name variants

// UpsertNameVariant adds or replaces the variant for (person, locale). When the variant is marked
// primary, the person's other variants are demoted in the same transaction (at most one primary).
func (s *Service) UpsertNameVariant(ctx context.Context, v domain.NameVariant) (domain.NameVariant, error) {
	if err := v.Validate(); err != nil {
		return domain.NameVariant{}, err
	}
	var out domain.NameVariant
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, v.PersonID); err != nil {
			return err
		}
		if v.IsPrimary {
			if err := repo.ClearPrimaryNameVariants(ctx, v.PersonID); err != nil {
				return err
			}
		}
		created, err := repo.UpsertNameVariant(ctx, v)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.name_variant.upsert", v.PersonID, map[string]any{"id": v.PersonID, "locale": v.Locale})
	})
	return out, err
}

// DeleteNameVariant removes a person's name variant for a locale.
func (s *Service) DeleteNameVariant(ctx context.Context, personID, locale string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeleteNameVariant(ctx, personID, locale); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.name_variant.delete", personID, map[string]any{"id": personID, "locale": locale})
	})
}

// ListNameVariants lists a person's name variants (the person must exist).
func (s *Service) ListNameVariants(ctx context.Context, personID string) ([]domain.NameVariant, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListNameVariants(ctx, personID)
}

// ---------------------------------------------------------------- helpers

// inTx runs fn in a transaction, committing on success and rolling back on error.
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record mints an Action RID in the caller's transaction and writes the audit row on it, so the audit
// entry commits iff the change commits (D-Audit). The actor is the interim system actor; the after
// payload carries only non-PII identifiers (person id + the changed key/status). Person writes are
// instance-scoped, so no unit is attributed.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	return s.recordWith(ctx, tx, auditSubsystem, action, targetID, after)
}

// recordWith is the subsystem-parameterized form: an order-driven rank change records under
// event-subscriber (D-OrderApply); all other person writes use person-admin via record.
func (s *Service) recordWith(ctx context.Context, tx pgx.Tx, subsystem, action, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx, action)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  subsystem,
		Action:     action,
		TargetType: targetPerson,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID mints an Action RID (person service=6, kind=action=3, generic action type=0).
// The specific action name is recorded separately in audit_log.action (D-Audit).
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	_ = action
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(6, 3, 0)").Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

// requestID is the correlation key shared with logs/metrics/traces: the request's trace id, with a
// generated fallback for out-of-request callers (e.g. integration tests).
func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
