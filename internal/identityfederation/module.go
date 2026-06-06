// Package identityfederation is the composition seam for the identity-federation module
// (docs/modules/identity-federation.md): it wires the pgx/sqlc repository, the application service,
// and the transport, then registers the IdentityFederationService Conjure routes. Register returns
// the application service so the composition root can wire it into (a) the inbound-token validation
// middleware as the (issuer, subject) resolver, and (b) the first-admin bootstrap.
//
// go-oikumenea does NOT authenticate (L-AuthzOnly): this module owns the optional login account + the
// verified external identities, not credentials. The linkingEnabled accessor carries the
// account.identity_linking.enabled install knob (gating ADDITIONAL identity links).
package identityfederation

import (
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	identityapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/identityfederation"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/adapters"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/application"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/transport"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-server/v2/witchcraft"
)

// Register builds the identity-federation module over the platform pool, the audit service (writes
// record in-transaction — D-Audit), the PEP enforcer (account/identity management gates on the linked
// person's `person.*` permissions), and the linking-enabled config accessor. It seeds nothing
// (accounts are created via the API or the first-admin bootstrap) and owns no resources of its own,
// so there is no module-level cleanup.
func Register(info witchcraft.InitInfo, pool *pgxpool.Pool, audit *auditapp.Service, enforcer *pep.Enforcer, linkingEnabled func() bool) (*application.Service, error) {
	repoFor := func(conn db.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, linkingEnabled)

	if err := identityapi.RegisterRoutesIdentityFederationService(info.Router, transport.NewService(svc, enforcer)); err != nil {
		return nil, werror.Wrap(err, "register identity-federation service routes")
	}
	return svc, nil
}
