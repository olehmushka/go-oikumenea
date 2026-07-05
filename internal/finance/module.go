// Package finance is the composition seam for the finance module (docs/modules/finance.md / D-Finance,
// M44): it wires the pgx repository, the audited application service (envelope-encrypting IBAN/PAN), and
// the transport, then registers the FinanceService Conjure routes. The account-type / card-network
// reference catalogs are migration-seeded, so there is no boot-time seeding here. Register returns the
// application service so a later PersonPurged subscriber can call ErasePersonAccounts once the event bus
// carries it, and so main can wire SubscribePersonEvents.
package finance

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	financeapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/finance"
	"github.com/olegamysk/go-oikumenea/internal/finance/adapters"
	"github.com/olegamysk/go-oikumenea/internal/finance/application"
	"github.com/olegamysk/go-oikumenea/internal/finance/domain"
	"github.com/olegamysk/go-oikumenea/internal/finance/transport"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the finance module over the platform pool, the audit service (writes record
// in-transaction — D-Audit), the localization service (translatable catalog name maps), the envelope
// cipher (D-CryptoProvider) and the personal-code validator registry (D-PersonalCodes: IBAN/PAN). It
// owns no resources of its own.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, loc *locapp.Service, enforcer *pep.Enforcer, cipher *crypto.Cipher, codes *personalcode.Registry) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, cipher, codes)
	if err := financeapi.RegisterRoutesFinanceService(info.Router, transport.NewService(svc, loc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register finance service routes")
	}
	return svc, nil
}
