// Package application holds the data-import module's application service (M16 / D-Hermenea): the
// generic POST /import/{objectType} orchestrator the transport calls. It runs the registered upsert
// handler in ONE transaction and records ONE audited Action as a `system` actor (the bulk-ingest !=
// audited-edit boundary). Connectors/mappers/scheduling are NOT here — they live in the hermenea
// companion; this side only receives canonical envelopes and upserts them.
package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// auditSubsystem labels the `system` actor for import writes. An import is always recorded as a
// system-actor Action regardless of who triggered it (D-Hermenea: ingest != edit).
const auditSubsystem = "data-import"

// Handler applies an object-type's records as an idempotent, non-destructive upsert into its catalog,
// within the caller's transaction, stamping provenance. Registered per object-type at composition time
// (mirrors pkg/events.Bus: adding an importable catalog = registering one handler).
type Handler func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance) (domain.Summary, error)

// StoreFactory binds the geo-country store port to a command surface (the caller's tx). Injected by
// module.go so the application never imports adapters (mirrors rank's RepositoryFactory).
type StoreFactory func(conn db.DBTX) domain.GeoCountryStore

// Envelope is the application-level canonical envelope (the transport maps the Conjure type onto it).
type Envelope struct {
	ObjectType    string
	Source        string
	SourceVersion string
	Records       []domain.Record
}

// Service runs imports over a registry of per-object-type handlers. It owns its writes, so it holds the
// pool to open transactions.
type Service struct {
	pool     *pgxpool.Pool
	audit    *auditapp.Service
	handlers map[string]Handler
}

// NewService builds the service with the pool and the audit service every import records into. The
// handler registry starts empty; module.go registers the available object-types.
func NewService(pool *pgxpool.Pool, audit *auditapp.Service) *Service {
	return &Service{pool: pool, audit: audit, handlers: map[string]Handler{}}
}

// Register adds an upsert handler for an object-type (composition-time wiring, before serving).
func (s *Service) Register(objectType string, h Handler) { s.handlers[objectType] = h }

// Import applies the canonical envelope for objectType as a code-keyed idempotent upsert in one
// transaction, then records one `system`-actor audited Action. An unknown object-type or an empty
// source is rejected before any write.
func (s *Service) Import(ctx context.Context, objectType string, env Envelope) (domain.Summary, error) {
	h, ok := s.handlers[objectType]
	if !ok {
		return domain.Summary{}, domain.ErrUnknownObjectType
	}
	if strings.TrimSpace(env.Source) == "" {
		return domain.Summary{}, domain.ErrInvalidRecord
	}
	prov := domain.Provenance{Source: env.Source, SourceVersion: env.SourceVersion, ImportedAt: time.Now().UTC()}
	var sum domain.Summary
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		out, err := h(ctx, tx, env.Records, prov)
		if err != nil {
			return err
		}
		sum = out
		return s.record(ctx, tx, "import."+objectType, objectType, env.Source, map[string]any{
			"source":        env.Source,
			"sourceVersion": env.SourceVersion,
			"records":       len(env.Records),
			"summary":       out,
		})
	})
	if err != nil {
		return domain.Summary{}, err
	}
	return sum, nil
}

// GeoCountriesHandler builds the geo-countries upsert handler over the injected store factory
// (M16 first catalog). For each record it reads the existing name, inserts when absent, updates only
// when the name changed, and skips otherwise — never deletes. Provenance is stamped on every write.
func GeoCountriesHandler(newStore StoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		for _, rec := range records {
			code, name, err := geoFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			existing, found, err := store.GetName(ctx, code)
			if err != nil {
				return domain.Summary{}, err
			}
			switch {
			case !found:
				if err := store.Insert(ctx, code, name, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Created++
			case existing != name:
				if err := store.UpdateImport(ctx, code, name, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Updated++
			default:
				sum.Skipped++
			}
		}
		return sum, nil
	}
}

// geoFields reads the ISO-3166 alpha-2 `code` + `name` from a record, normalizing the code to upper
// case. A non-2-letter code or empty name is rejected (ErrInvalidRecord).
func geoFields(rec domain.Record) (code, name string, err error) {
	code, _ = rec["code"].(string)
	name, _ = rec["name"].(string)
	code = strings.ToUpper(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	if len(code) != 2 || name == "" {
		return "", "", domain.ErrInvalidRecord
	}
	return code, name, nil
}

// inTx runs fn in a transaction, committing on success and rolling back on any error (so a failing
// upsert reverts the whole import and its audit row — all-or-nothing).
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

// record writes the audited Action for an import in the same transaction (D-Audit), as a `system`
// actor (ingest != edit).
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx)
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

// mintActionRID mints an Action RID (new_id(app, service, kind) — kind 0 placeholder, as the other
// modules do for audit Action ids).
func mintActionRID(ctx context.Context, tx pgx.Tx) (string, error) {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(5, 3, 0)").Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

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
