// Package order is the composition seam for the order module (docs/modules/order.md): it seeds the
// order-type catalog, wires the pgx/sqlc repository, the application service (over the event bus), and
// the transport, then registers the OrderService Conjure routes. Register returns the application
// service for symmetry with the other modules (no in-process caller today — order is the producer of
// the issue-time effect events, not a callee).
//
// Orders are unit-issued and unit/subtree-scoped; the order-type catalog is instance-admin-managed.
// The localization service assembles the translatable type `name` maps; the audit service records
// every write in-transaction (D-Audit); the event bus dispatches the issue-time AppointmentOrdered /
// RemovalOrdered / RankChangeOrdered events to the membership/person subscribers IN the issue
// transaction (D-OrderApply).
package order

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	orderapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/order"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/order/adapters"
	"github.com/olegamysk/go-oikumenea/internal/order/application"
	"github.com/olegamysk/go-oikumenea/internal/order/domain"
	"github.com/olegamysk/go-oikumenea/internal/order/transport"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// seedOrderTypesSQL idempotently seeds a representative order-type catalog (the five UA-army families,
// D-Orders). The RID PKs default via new_id() (D-ResourceIdentifiers), which reads no GUC, so this
// could equally seed in the migration (D-RIDSeeding relaxed, F-014); it stays a boot seed for
// consistency. ON CONFLICT on the unique code makes this safe on every boot. The instance admin adds
// more (or retires these) via the API. The effect mapping drives which target columns an item must
// carry and which intent event issue emits.
const seedOrderTypesSQL = `
INSERT INTO oikumenea.order_order_types (code, name, category, effect, sort_order) VALUES
  ('arrival',          'Arrival / enrollment',  'personnel-list',       'membership-start',   0),
  ('removal',          'Removal / discharge',   'personnel-list',       'membership-end',    10),
  ('appoint',          'Appointment',           'appointment',          'membership-start',  20),
  ('dismiss',          'Dismissal',             'appointment',          'membership-end',    30),
  ('rank-award',       'Rank award',            'discipline-incentive', 'rank-change',       40),
  ('rank-deprivation', 'Rank deprivation',      'discipline-incentive', 'rank-change',       50),
  ('leave-annual',     'Annual leave',          'leave-travel',         'record-only',       60),
  ('business-trip',    'Business trip',         'leave-travel',         'record-only',       70),
  ('reprimand',        'Reprimand',             'discipline-incentive', 'record-only',       80),
  ('duty-detail',      'Daily duty detail',     'duty-roster',          'record-only',       90)
ON CONFLICT (code) DO NOTHING`

// Register seeds the order-type catalog, builds the module over the platform pool, the audit service
// (writes record in-transaction — D-Audit), the localization service (name-map assembly), the PEP
// enforcer, and the event bus (issue-time effect dispatch), and registers its routes onto the
// witchcraft router. It owns no resources of its own (the pool is owned by platform), so there is no
// module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, enforcer *pep.Enforcer, bus *events.Bus) (*application.Service, error) {
	if _, err := pool.Exec(context.Background(), seedOrderTypesSQL); err != nil {
		return nil, werror.Wrap(err, "seed order type catalog")
	}

	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, bus)

	if err := orderapi.RegisterRoutesOrderService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register order service routes")
	}
	return svc, nil
}
