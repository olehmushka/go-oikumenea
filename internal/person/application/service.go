// Package application holds the person module's application service — the orchestrator the transport
// layer calls to read/mutate the directory, recording an audit row in the same transaction as each
// write (D-Audit). It depends on the domain port, the platform DB surface, and the audit service; it
// never imports the adapters package directly (the repository factory is injected by module.go).
//
// Person is the primary PII store, so audit payloads here carry only non-PII identifiers (the id,
// and the changed key/status) — never names or other personal data. A person holds at most one rank,
// a directory attribute; this service never reads rank to make a decision (D-Rank).
package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
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

// targetPerson is the audited entity kind; every person-scoped action targets the person id.
const targetPerson = "person"

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a
// caller's transaction for an audited write (D-Audit). Injected by module.go.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the person application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly. graceHours supplies the deactivate->purge window.
type Service struct {
	pool       *pgxpool.Pool
	newRepo    RepositoryFactory
	audit      *auditapp.Service
	graceHours func() int
	now        func() time.Time
}

// NewService wires the service with the pool, the repository factory, the audit service, and the
// (refreshable) purge-grace window in hours.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, graceHours func() int) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, graceHours: graceHours, now: func() time.Time { return time.Now().UTC() }}
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

// GetPerson reads one person with its name variants, citizenships, and residences attached.
func (s *Service) GetPerson(ctx context.Context, id string) (domain.Person, error) {
	repo := s.newRepo(s.pool)
	p, err := repo.GetPerson(ctx, id)
	if err != nil {
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

// SetRank sets or clears the person's one rank (a directory attribute; D-Rank) and records it.
func (s *Service) SetRank(ctx context.Context, id string, rankID *string) (domain.Person, error) {
	var out domain.Person
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).SetRank(ctx, id, rankID)
		if err != nil {
			return err
		}
		out = updated
		after := map[string]any{"id": id, "rankId": nil}
		if rankID != nil {
			after["rankId"] = *rankID
		}
		return s.record(ctx, tx, "person.rank.assign", id, after)
	})
	return out, err
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
	rid, err := mintActionRID(ctx, tx, action)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: targetPerson,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID composes the Action RID via the same SQL generator every module uses, so the audit
// log's action__<type> RID-shape CHECK is satisfied. The action code (e.g. "person.citizenship.
// upsert") becomes the entity_type slot "action__person_citizenship_upsert".
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	entityType := "action__" + sanitizeAction(action)
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_rid('person', $1)", entityType).Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

func sanitizeAction(action string) string {
	b := make([]byte, len(action))
	for i := 0; i < len(action); i++ {
		if action[i] == '.' {
			b[i] = '_'
		} else {
			b[i] = action[i]
		}
	}
	return string(b)
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
