// Package document is the composition seam for the document module (docs/modules/document.md): it wires
// the pgx/sqlc repository, the application service (over the envelope cipher + personal-code validator
// registry), and the transport, then registers the DocumentService Conjure routes. Register returns the
// application service so a later PersonPurged subscriber can call ErasePersonRecords once the event bus
// lands (cross-module path, overview.md).
//
// Papers are typed by the document-type catalog (RID-keyed → seeded at boot here, D-RIDSeeding); personal
// codes are typed by the country-namespaced scheme catalog (natural-key → seeded in the migration). The
// localization service assembles the translatable type/scheme `name` maps; the audit service records
// every write in-transaction (D-Audit).
package document

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	documentapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/document"
	"github.com/olegamysk/go-oikumenea/internal/document/adapters"
	"github.com/olegamysk/go-oikumenea/internal/document/application"
	"github.com/olegamysk/go-oikumenea/internal/document/domain"
	"github.com/olegamysk/go-oikumenea/internal/document/transport"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// seedDocumentTypesSQL idempotently seeds a representative PAPER-type catalog (D-Documents). The RID PKs
// default via new_rid(), which reads the per-connection app.environment GUC — set by db.NewPool but NOT
// by atlas's migration connection — so these RID-keyed reference rows are inserted at BOOT here, on the
// GUC-bearing pool, not in the migration (D-RIDSeeding). ON CONFLICT on the unique code makes this safe
// on every boot. The instance admin adds more (and may retire these) via the API.
const seedDocumentTypesSQL = `
INSERT INTO oikumenea.document_document_types (code, name, sort_order) VALUES
  ('passport',         'Passport',                0),
  ('national-id',      'National ID card',       10),
  ('tax-id',           'Tax ID document',        20),
  ('social-insurance', 'Social insurance card',  30),
  ('driver-license',   'Driver''s licence',      40),
  ('military-id',      'Military ID',            50)
ON CONFLICT (code) DO NOTHING`

// seedMilitaryIDSchemaSQL sets the military-id type's attribute schema (D-DocumentAttrSchema) at boot,
// only when it has none yet (so an operator's later customization is not clobbered). The schema types
// the UA military card's structured fields (VOS specialty, fitness/mobilization category, issuing
// commissariat) that previously rode untyped in the attributes JSONB; documents of this type are
// validated against it on write.
const seedMilitaryIDSchemaSQL = `
UPDATE oikumenea.document_document_types
SET attr_schema = '{
  "fields": {
    "vos": {"type": "string"},
    "fitness_category": {"type": "string", "enum": ["А","Б","В","Г","Д"]},
    "mobilization_category": {"type": "string"},
    "commissariat": {"type": "string"},
    "issued_year": {"type": "number"}
  }
}'::jsonb
WHERE code = 'military-id' AND attr_schema IS NULL AND deleted_at IS NULL`

// Register seeds the document-type catalog, builds the module over the platform pool, the audit service
// (writes record in-transaction — D-Audit), the localization service (name-map assembly), the envelope
// cipher (D-CryptoProvider), and the personal-code validator registry (D-PersonalCodes), and registers
// its routes onto the witchcraft router. It owns no resources of its own (the pool is owned by
// platform), so there is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, enforcer *pep.Enforcer, cipher *crypto.Cipher, codes *personalcode.Registry) (*application.Service, error) {
	if _, err := pool.Exec(context.Background(), seedDocumentTypesSQL); err != nil {
		return nil, werror.Wrap(err, "seed document type catalog")
	}
	if _, err := pool.Exec(context.Background(), seedMilitaryIDSchemaSQL); err != nil {
		return nil, werror.Wrap(err, "seed military-id attribute schema")
	}

	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, cipher, codes)

	if err := documentapi.RegisterRoutesDocumentService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register document service routes")
	}
	return svc, nil
}
