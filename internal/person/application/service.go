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
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	orderevents "github.com/olegamysk/go-oikumenea/internal/order/events"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// Page-size policy (API conventions: token pagination, bounded pages).
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

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
// to resolve which units a person belongs to / which people are reachable through a unit-set. The
// membership application service satisfies it; it is late-bound (SetMembershipReader) because
// membership is composed after person (overview.md composition ordering).
type MembershipReader interface {
	ActiveUnitIDsForPerson(ctx context.Context, personID string) ([]string, error)
	PersonIDsWithActiveMembershipInUnits(ctx context.Context, unitIDs []string, after string, limit int) ([]string, error)
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
}

// NewService wires the service with the pool, the repository factory, the audit service, and the
// (refreshable) purge-grace window in hours. The membership reader is late-bound (SetMembershipReader).
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, graceHours func() int) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, graceHours: graceHours, now: func() time.Time { return time.Now().UTC() }}
}

// SetMembershipReader binds the cross-module membership query seam used by the read-scope projection
// (D-PersonReadScope). Called once at composition time, after membership is built, before serving.
func (s *Service) SetMembershipReader(r MembershipReader) { s.membership = r }

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

// GetPerson reads one person with its ranks, name variants, citizenships, and residences attached.
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
	if p.Citizenships, err = repo.ListCitizenships(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if p.Residences, err = repo.ListResidences(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if p.Emails, err = repo.ListEmails(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if p.Phones, err = repo.ListPhones(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if p.CallSigns, err = repo.ListCallSigns(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if p.MessengerLinks, err = repo.ListMessengerLinks(ctx, id); err != nil {
		return domain.Person{}, err
	}
	if p.SocialAccounts, err = repo.ListSocialAccounts(ctx, id); err != nil {
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

// ListPersons returns a keyset-paginated page of the directory (by time-ordered RID).
func (s *Service) ListPersons(ctx context.Context, pageSize int, pageToken string) (Page, error) {
	size := resolvePageSize(pageSize)
	after, err := decodeCursor(pageToken)
	if err != nil {
		return Page{}, err
	}
	persons, err := s.newRepo(s.pool).ListPersons(ctx, after, size+1)
	if err != nil {
		return Page{}, err
	}
	if len(persons) > size {
		return Page{Persons: persons[:size], NextPageToken: encodeCursor(persons[size-1].ID)}, nil
	}
	return Page{Persons: persons}, nil
}

// ReadablePerson decides whether the request subject (carrying the precomputed effective reach) may
// read person id under D-PersonReadScope: true iff the subject is on the instance plane (an instance
// admin — person.read is never instance-scoped) OR the person's active-membership units intersect the
// subject's effective readable units. A membership-less person has no units, so only an instance admin
// sees them. Intersecting with the readable set subsumes the shadow gate (unreachable shadow units are
// already absent from reach.Readable).
func (s *Service) ReadablePerson(ctx context.Context, reach authzdomain.Reach, personID string) (bool, error) {
	if reach.InstanceAdmin {
		return true, nil
	}
	if s.membership == nil || len(reach.Readable) == 0 {
		return false, nil
	}
	units, err := s.membership.ActiveUnitIDsForPerson(ctx, personID)
	if err != nil {
		return false, err
	}
	for _, u := range units {
		if _, ok := reach.Readable[u]; ok {
			return true, nil
		}
	}
	return false, nil
}

// ListVisiblePersons returns the keyset-paginated union of people a non-instance-admin subject may
// read (D-PersonReadScope): the directory rows whose active memberships fall in the subject's
// effective readable units. The instance-admin case is the unrestricted ListPersons and is handled by
// the caller. An empty readable set yields an empty page. Pagination keys on the person RID, matching
// the membership union's ordering, so the returned rows are already in token order.
func (s *Service) ListVisiblePersons(ctx context.Context, reach authzdomain.Reach, pageSize int, pageToken string) (Page, error) {
	size := resolvePageSize(pageSize)
	after, err := decodeCursor(pageToken)
	if err != nil {
		return Page{}, err
	}
	if s.membership == nil || len(reach.Readable) == 0 {
		return Page{}, nil
	}
	units := make([]string, 0, len(reach.Readable))
	for u := range reach.Readable {
		units = append(units, u)
	}
	ids, err := s.membership.PersonIDsWithActiveMembershipInUnits(ctx, units, after, size+1)
	if err != nil {
		return Page{}, err
	}
	hasMore := len(ids) > size
	if hasMore {
		ids = ids[:size]
	}
	// Both the membership union and ListPersonsByIDs order ascending by person RID, so the hydrated
	// rows are already in token order (a soft-deleted person is simply dropped, never reordered).
	persons, err := s.newRepo(s.pool).ListPersonsByIDs(ctx, ids)
	if err != nil {
		return Page{}, err
	}
	if hasMore && len(ids) > 0 {
		return Page{Persons: persons, NextPageToken: encodeCursor(ids[len(ids)-1])}, nil
	}
	return Page{Persons: persons}, nil
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
		if _, err := repo.GetPerson(ctx, c.PersonID); err != nil {
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
	if _, err := repo.GetPerson(ctx, personID); err != nil {
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
		if _, err := repo.GetPerson(ctx, r.PersonID); err != nil {
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
	if _, err := repo.GetPerson(ctx, personID); err != nil {
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
		if _, err := repo.GetPerson(ctx, e.PersonID); err != nil {
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
	if _, err := repo.GetPerson(ctx, personID); err != nil {
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
		if _, err := repo.GetPerson(ctx, p.PersonID); err != nil {
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
	if _, err := repo.GetPerson(ctx, personID); err != nil {
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
		if _, err := repo.GetPerson(ctx, c.PersonID); err != nil {
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
	if _, err := repo.GetPerson(ctx, personID); err != nil {
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
		if _, err := repo.GetPerson(ctx, personID); err != nil {
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
	if _, err := repo.GetPerson(ctx, personID); err != nil {
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
		if _, err := repo.GetPerson(ctx, a.PersonID); err != nil {
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

// ListSocialAccounts lists a person's social accounts (the person must exist).
func (s *Service) ListSocialAccounts(ctx context.Context, personID string) ([]domain.SocialAccount, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
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

// ---------------------------------------------------------------- person↔person relationships (D-PersonRelationships)

// ListRelationTypes returns the instance-admin relation-label catalog (read; no person scope).
func (s *Service) ListRelationTypes(ctx context.Context) ([]domain.RelationType, error) {
	return s.newRepo(s.pool).ListRelationTypes(ctx)
}

// canonicalPair orders two person ids ascending (the canonical-pair invariant person_id_a < person_id_b).
func canonicalPair(x, y string) (string, string) {
	if x <= y {
		return x, y
	}
	return y, x
}

// requireCounterpart confirms the other endpoint is a real directory person, mapping a missing person to
// ErrUnknownCounterpart (the path person's own existence is checked separately and stays ErrNotFound).
func (s *Service) requireCounterpart(ctx context.Context, repo domain.Repository, id string) error {
	if _, err := repo.GetPerson(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrUnknownCounterpart
		}
		return err
	}
	return nil
}

// checkRelationCode validates an optional relation-type code: "" is allowed; otherwise the code must
// exist and (when wantCategory != "") sit in that category.
func checkRelationCode(ctx context.Context, repo domain.Repository, code, wantCategory string) error {
	if code == "" {
		return nil
	}
	rt, err := repo.GetRelationType(ctx, code)
	if err != nil {
		return err // ErrUnknownRelationType
	}
	if wantCategory != "" && rt.Category != wantCategory {
		return domain.ErrRelationCategory
	}
	return nil
}

// UpsertPartnership records/replaces a partnership between personID and the partner (a symmetric,
// canonically ordered pair), enforcing the single-active-engaged/married-per-person rule for both ends.
func (s *Service) UpsertPartnership(ctx context.Context, personID string, p domain.Partnership) (domain.Partnership, error) {
	if err := p.Validate(); err != nil {
		return domain.Partnership{}, err
	}
	counterpart := otherEndpoint(personID, p.PersonIDA, p.PersonIDB)
	if counterpart == personID {
		return domain.Partnership{}, domain.ErrSelfRelationship
	}
	p.PersonIDA, p.PersonIDB = canonicalPair(personID, counterpart)
	var out domain.Partnership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		if p.IsActivePartnership() {
			for _, who := range []string{personID, counterpart} {
				has, err := repo.HasActivePartnershipExcept(ctx, who, p.ID)
				if err != nil {
					return err
				}
				if has {
					return domain.ErrPartnershipConflict
				}
			}
		}
		saved, err := repo.UpsertPartnership(ctx, p)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.partnership.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertKinship records/replaces a directional parent→child kinship (endpoints set by the transport per role).
func (s *Service) UpsertKinship(ctx context.Context, personID string, k domain.Kinship) (domain.Kinship, error) {
	if k.Status == "" {
		k.Status = "active"
	}
	if err := k.Validate(); err != nil {
		return domain.Kinship{}, err
	}
	if k.ParentID == k.ChildID {
		return domain.Kinship{}, domain.ErrSelfRelationship
	}
	counterpart := otherEndpoint(personID, k.ParentID, k.ChildID)
	var out domain.Kinship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		saved, err := repo.UpsertKinship(ctx, k)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.kinship.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertGuardianship records/replaces a guardian→ward link (relation_code optional, any category).
func (s *Service) UpsertGuardianship(ctx context.Context, personID string, g domain.Guardianship) (domain.Guardianship, error) {
	if g.Status == "" {
		g.Status = "active"
	}
	if err := g.Validate(); err != nil {
		return domain.Guardianship{}, err
	}
	if g.GuardianID == g.WardID {
		return domain.Guardianship{}, domain.ErrSelfRelationship
	}
	counterpart := otherEndpoint(personID, g.GuardianID, g.WardID)
	var out domain.Guardianship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		if err := checkRelationCode(ctx, repo, g.RelationCode, ""); err != nil {
			return err
		}
		saved, err := repo.UpsertGuardianship(ctx, g)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.guardianship.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertSponsorship records/replaces a sponsor→sponsored link (relation_code required, category=sponsorship).
func (s *Service) UpsertSponsorship(ctx context.Context, personID string, sp domain.Sponsorship) (domain.Sponsorship, error) {
	if sp.Status == "" {
		sp.Status = "active"
	}
	if err := sp.Validate(); err != nil {
		return domain.Sponsorship{}, err
	}
	if sp.SponsorID == sp.SponsoredID {
		return domain.Sponsorship{}, domain.ErrSelfRelationship
	}
	counterpart := otherEndpoint(personID, sp.SponsorID, sp.SponsoredID)
	var out domain.Sponsorship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		if err := checkRelationCode(ctx, repo, sp.RelationCode, domain.RelCategorySponsorship); err != nil {
			return err
		}
		saved, err := repo.UpsertSponsorship(ctx, sp)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.sponsorship.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertNextOfKin nominates/replaces a next-of-kin contact for the subject (personID).
func (s *Service) UpsertNextOfKin(ctx context.Context, personID string, n domain.NextOfKin) (domain.NextOfKin, error) {
	if n.Status == "" {
		n.Status = "active"
	}
	if n.Priority == 0 {
		n.Priority = 1
	}
	if err := n.Validate(); err != nil {
		return domain.NextOfKin{}, err
	}
	n.SubjectID = personID
	if n.SubjectID == n.ContactID {
		return domain.NextOfKin{}, domain.ErrSelfRelationship
	}
	var out domain.NextOfKin
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, n.ContactID); err != nil {
			return err
		}
		if err := checkRelationCode(ctx, repo, n.RelationCode, domain.RelCategoryNextOfKin); err != nil {
			return err
		}
		saved, err := repo.UpsertNextOfKin(ctx, n)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.next-of-kin.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertAssociation records/replaces a symmetric association (relation_code optional, category=association).
func (s *Service) UpsertAssociation(ctx context.Context, personID string, a domain.Association) (domain.Association, error) {
	if a.Status == "" {
		a.Status = "active"
	}
	if err := a.Validate(); err != nil {
		return domain.Association{}, err
	}
	counterpart := otherEndpoint(personID, a.PersonIDA, a.PersonIDB)
	if counterpart == personID {
		return domain.Association{}, domain.ErrSelfRelationship
	}
	a.PersonIDA, a.PersonIDB = canonicalPair(personID, counterpart)
	var out domain.Association
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		if err := checkRelationCode(ctx, repo, a.RelationCode, domain.RelCategoryAssociation); err != nil {
			return err
		}
		saved, err := repo.UpsertAssociation(ctx, a)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.association.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// DeleteRelationship removes any person↔person link by id; the link table is decoded from the RID and
// the delete is holder-scoped (the person must be an endpoint). Idempotent at the transport layer.
func (s *Service) DeleteRelationship(ctx context.Context, personID, relationshipID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		var err error
		switch domain.RelationLinkType(relationshipID) {
		case domain.LinkPartnership:
			err = repo.DeletePartnership(ctx, personID, relationshipID)
		case domain.LinkKinship:
			err = repo.DeleteKinship(ctx, personID, relationshipID)
		case domain.LinkGuardianship:
			err = repo.DeleteGuardianship(ctx, personID, relationshipID)
		case domain.LinkSponsorship:
			err = repo.DeleteSponsorship(ctx, personID, relationshipID)
		case domain.LinkNextOfKin:
			err = repo.DeleteNextOfKin(ctx, personID, relationshipID)
		case domain.LinkAssociation:
			err = repo.DeleteAssociation(ctx, personID, relationshipID)
		default:
			return domain.ErrUnknownRelationshipKind
		}
		if err != nil {
			return err
		}
		return s.record(ctx, tx, "person.relationship.delete", personID, map[string]any{"id": personID, "relationshipId": relationshipID})
	})
}

// relationship list reads (holder-scoped: the person must exist; rows touch either endpoint)

func (s *Service) ListPartnerships(ctx context.Context, personID string) ([]domain.Partnership, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListPartnerships(ctx, personID)
}

func (s *Service) ListKinships(ctx context.Context, personID string) ([]domain.Kinship, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListKinships(ctx, personID)
}

func (s *Service) ListGuardianships(ctx context.Context, personID string) ([]domain.Guardianship, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListGuardianships(ctx, personID)
}

func (s *Service) ListSponsorships(ctx context.Context, personID string) ([]domain.Sponsorship, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListSponsorships(ctx, personID)
}

func (s *Service) ListNextOfKin(ctx context.Context, personID string) ([]domain.NextOfKin, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListNextOfKin(ctx, personID)
}

func (s *Service) ListAssociations(ctx context.Context, personID string) ([]domain.Association, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListAssociations(ctx, personID)
}

// otherEndpoint returns whichever of a/b is not personID (b when neither matches — a transport invariant
// guarantees one endpoint is the path person).
func otherEndpoint(personID, a, b string) string {
	if a == personID {
		return b
	}
	if b == personID {
		return a
	}
	return b
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

func resolvePageSize(requested int) int {
	if requested <= 0 {
		return DefaultPageSize
	}
	if requested > MaxPageSize {
		return MaxPageSize
	}
	return requested
}

// encodeCursor/decodeCursor make the keyset position (the last row's RID) into an opaque, URL-safe
// page token (API conventions: token pagination, no offsets).
func encodeCursor(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func decodeCursor(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
