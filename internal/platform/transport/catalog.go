// The platform catalog transport implements the generated PlatformCatalogService (D-OverlayFoundation,
// M29): the GDPR lawful-basis reference catalog. Reads require `legal-basis.read`; writes the
// instance-plane `legal-basis.manage` (both satisfied anywhere via the PEP — the catalog is
// instance-global reference data). Generated code in internal/conjure is never hand-edited.
package transport

import (
	"context"
	"errors"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	platformapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/platform"
	"github.com/olegamysk/go-oikumenea/internal/platform/catalog"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// CatalogService adapts the platform catalog application service to the generated server interface.
type CatalogService struct {
	app *catalog.Service
	pep *pep.Enforcer
}

// NewCatalogService builds the transport adapter over the catalog service + PEP enforcer.
func NewCatalogService(app *catalog.Service, enforcer *pep.Enforcer) CatalogService {
	return CatalogService{app: app, pep: enforcer}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ platformapi.PlatformCatalogService = CatalogService{}

// ListLegalBasisKinds implements GET /legal-basis-kinds.
func (s CatalogService) ListLegalBasisKinds(ctx context.Context, token bearertoken.Token) (platformapi.LegalBasisKindList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermLegalBasisRead)); err != nil {
		return platformapi.LegalBasisKindList{}, err
	}
	kinds, err := s.app.List(ctx)
	if err != nil {
		return platformapi.LegalBasisKindList{}, werror.WrapWithContextParams(ctx, err, "list legal-basis kinds failed")
	}
	out := make([]platformapi.LegalBasisKind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, toAPILegalBasis(k))
	}
	return platformapi.LegalBasisKindList{Kinds: out}, nil
}

// UpsertLegalBasisKind implements PUT /legal-basis-kinds/{code} (instance-admin).
func (s CatalogService) UpsertLegalBasisKind(ctx context.Context, token bearertoken.Token, code string, req platformapi.UpsertLegalBasisKindRequest) (platformapi.LegalBasisKind, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermLegalBasisManage)); err != nil {
		return platformapi.LegalBasisKind{}, err
	}
	saved, err := s.app.Upsert(ctx, catalog.LegalBasisKind{
		Code:      code,
		Name:      req.Name,
		Article:   req.Article,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		if errors.Is(err, catalog.ErrInvalid) {
			return platformapi.LegalBasisKind{}, platformapi.NewDemoError("legal-basis upsert: code, name, and article (art6|art9) are required")
		}
		return platformapi.LegalBasisKind{}, werror.WrapWithContextParams(ctx, err, "upsert legal-basis kind failed")
	}
	return toAPILegalBasis(saved), nil
}

func toAPILegalBasis(k catalog.LegalBasisKind) platformapi.LegalBasisKind {
	return platformapi.LegalBasisKind{
		Code:      k.Code,
		Name:      k.Name,
		Article:   k.Article,
		Status:    k.Status,
		SortOrder: k.SortOrder,
	}
}
