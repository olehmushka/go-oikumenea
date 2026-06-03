// Package application holds the localization module's application service — the orchestrator the
// transport layer calls to read the locale registry / translation store and to perform the
// instance-admin writes, and that other modules call in-process to assemble localized responses
// (overview.md). It depends on the domain port, the platform DB surface, and the audit service; it
// never imports the adapters package directly (the repository factory is injected by module.go).
package application

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/localization/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// auditSubsystem labels the interim system actor for localization's admin writes. Until
// authorization (M7) + identity-federation (M8) resolve the acting person, these writes cannot be
// attributed to a person, so they are recorded as a `system` action under this subsystem (the
// no-unaudited-mutation ground rule still holds). M7/M8 replace this with the resolved person actor.
const auditSubsystem = "localization-admin"

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a
// caller's transaction for an audited write (D-Audit). Injected by module.go so the application
// layer never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the localization application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
}

// NewService wires the service with the pool, the repository factory, and the audit service every
// write records into.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit}
}

// ListLocales returns the supported locales in display order.
func (s *Service) ListLocales(ctx context.Context) ([]domain.Locale, error) {
	return s.newRepo(s.pool).ListLocales(ctx)
}

// AddLocale adds a supported locale and records the action. New locales are never the default
// (the default is changed via UpdateLocale), so this never disturbs the single-default invariant.
func (s *Service) AddLocale(ctx context.Context, l domain.Locale) (domain.Locale, error) {
	if err := l.Validate(); err != nil {
		return domain.Locale{}, err
	}
	l.IsDefault = false
	var out domain.Locale
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertLocale(ctx, l)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "locale.add", "locale", created.Code, created)
	})
	return out, err
}

// UpdateLocale applies a partial change (rename/enable/default/reorder) and records the action.
// Promoting a new default clears the previous one in the same transaction; the registry invariant
// (>=1 enabled, exactly one default) is enforced by the deferred DB trigger at commit.
func (s *Service) UpdateLocale(ctx context.Context, code string, patch domain.LocalePatch) (domain.Locale, error) {
	if err := domain.ValidateCode(code); err != nil {
		return domain.Locale{}, err
	}
	var out domain.Locale
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if patch.IsDefault != nil && *patch.IsDefault {
			if err := repo.ClearDefault(ctx); err != nil {
				return err
			}
		}
		updated, err := repo.UpdateLocale(ctx, code, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "locale.update", "locale", updated.Code, updated)
	})
	return out, err
}

// GetTranslations returns all translations of one entity (for editing).
func (s *Service) GetTranslations(ctx context.Context, entityType, entityID string) ([]domain.Translation, error) {
	return s.newRepo(s.pool).GetTranslations(ctx, entityType, entityID)
}

// UpsertTranslations validates that every cited locale is a known, enabled locale, upserts the
// rows, records the action, and returns the entity's full translation set. All in one transaction.
func (s *Service) UpsertTranslations(ctx context.Context, entityType, entityID string, ts []domain.Translation) ([]domain.Translation, error) {
	if err := s.validateLocales(ctx, ts); err != nil {
		return nil, err
	}
	var out []domain.Translation
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.UpsertTranslations(ctx, ts); err != nil {
			return err
		}
		if err := s.record(ctx, tx, "translation.upsert", entityType, entityID, ts); err != nil {
			return err
		}
		stored, err := repo.GetTranslations(ctx, entityType, entityID)
		if err != nil {
			return err
		}
		out = stored
		return nil
	})
	return out, err
}

// TranslationsFor is the in-process batch helper later modules call to assemble localized
// responses (tenant, rank, …). It returns the translations of the given fields of the given
// entities, keyed by (entityID, field) -> (locale -> text). The owning module merges in its own
// default-locale `name` fallback to build the final locale-map (docs/modules/localization.md).
func (s *Service) TranslationsFor(ctx context.Context, entityType string, entityIDs, fields []string) (map[domain.TranslationKey]map[string]string, error) {
	rows, err := s.newRepo(s.pool).TranslationsForBatch(ctx, entityType, entityIDs, fields)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.TranslationKey]map[string]string, len(rows))
	for _, t := range rows {
		key := domain.TranslationKey{EntityID: t.EntityID, Field: t.Field}
		m := out[key]
		if m == nil {
			m = make(map[string]string)
			out[key] = m
		}
		m[t.Locale] = t.Text
	}
	return out, nil
}

// DefaultLocale returns the code of the sole enabled default locale (the fallback locale for
// translatable labels — D-i18n). It is the read helper later modules use when assembling a
// locale->text map from their own default-locale `name` column. Returns "" if no default is set
// (the registry invariant should prevent that; callers treat "" as "no fallback key").
func (s *Service) DefaultLocale(ctx context.Context) (string, error) {
	locales, err := s.ListLocales(ctx)
	if err != nil {
		return "", err
	}
	for _, l := range locales {
		if l.IsDefault {
			return l.Code, nil
		}
	}
	return "", nil
}

// NamesByID assembles the `locale -> text` display-name map for a set of another module's entities
// (D-i18n: all locales in every response, no negotiation). It is LabelsByID over the conventional
// `name` field — the in-process helper the tenant/rank/… response builders call.
func (s *Service) NamesByID(ctx context.Context, entityType string, defaultText map[string]string) (map[string]map[string]string, error) {
	return s.LabelsByID(ctx, entityType, "name", defaultText)
}

// LabelsByID assembles the `locale -> text` map for a translatable FIELD of a set of another
// module's entities (D-i18n: all locales in every response, no negotiation). The caller passes
// entityID -> its own default-locale column value for that field; LabelsByID seeds each map with the
// default locale -> that value, then overlays the additional-locale translation rows from the store.
// It is the in-process helper response builders call for any translatable label (e.g. a unit/rank
// `name`, a position `title`). An entity with no translation rows still gets a single-entry map (the
// default locale).
func (s *Service) LabelsByID(ctx context.Context, entityType, field string, defaultText map[string]string) (map[string]map[string]string, error) {
	out := make(map[string]map[string]string, len(defaultText))
	if len(defaultText) == 0 {
		return out, nil
	}
	defaultLocale, err := s.DefaultLocale(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(defaultText))
	for id := range defaultText {
		ids = append(ids, id)
	}
	translations, err := s.TranslationsFor(ctx, entityType, ids, []string{field})
	if err != nil {
		return nil, err
	}
	for id, text := range defaultText {
		m := make(map[string]string)
		if defaultLocale != "" {
			m[defaultLocale] = text
		}
		for locale, t := range translations[domain.TranslationKey{EntityID: id, Field: field}] {
			m[locale] = t
		}
		out[id] = m
	}
	return out, nil
}

// validateLocales rejects the whole upsert if any translation cites a locale that is not a known,
// enabled locale (domain.ErrUnknownLocale → INVALID_ARGUMENT).
func (s *Service) validateLocales(ctx context.Context, ts []domain.Translation) error {
	if len(ts) == 0 {
		return nil
	}
	wanted := make(map[string]struct{})
	codes := make([]string, 0, len(ts))
	for _, t := range ts {
		if _, ok := wanted[t.Locale]; !ok {
			wanted[t.Locale] = struct{}{}
			codes = append(codes, t.Locale)
		}
	}
	existing, err := s.newRepo(s.pool).ExistingLocaleCodes(ctx, codes)
	if err != nil {
		return err
	}
	for _, c := range existing {
		delete(wanted, c)
	}
	for c := range wanted {
		return domain.UnknownLocaleError{Code: c}
	}
	return nil
}

// inTx runs fn in a transaction, committing on success and rolling back on error (the deferred
// rollback after a successful commit is a no-op).
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	// The locale-registry invariant (>=1 enabled, exactly one default) is a DEFERRABLE constraint
	// trigger, so it fires here at COMMIT; surface it as the domain constraint error.
	if err := tx.Commit(ctx); err != nil {
		if isCheckViolation(err) {
			return errors.Join(domain.ErrLocaleConstraint, err)
		}
		return err
	}
	return nil
}

// isCheckViolation reports whether err is a Postgres check-constraint / RAISE check_violation
// (SQLSTATE 23514) — the class the deferred locale-default trigger raises.
func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

// record mints an Action RID in the caller's transaction and writes the audit row on it, so the
// audit entry commits iff the change commits (D-Audit). The actor is the interim system actor.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx, action)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID composes the Action RID via the same SQL generator every module uses, so the audit
// log's action__<type> RID-shape CHECK is satisfied. The action code (e.g. "locale.add") becomes
// the entity_type slot "action__locale_add".
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	entityType := "action__" + sanitizeAction(action)
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_rid('i18n', $1)", entityType).Scan(&rid); err != nil {
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
// generated fallback for out-of-request callers (e.g. integration tests) so the audit Entry always
// has a non-empty requestId.
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
