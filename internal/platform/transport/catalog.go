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
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/catalog"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// colorEntity is the localization-store entity type for color names (D-i18n). Keyed by the color RID
// (the per-domain `code` is not globally unique).
const colorEntity = "color"

// CatalogService adapts the platform catalog application service to the generated server interface.
type CatalogService struct {
	app   *catalog.Service
	color *catalog.ColorService
	loc   *locapp.Service
	pep   *pep.Enforcer
}

// NewCatalogService builds the transport adapter over the lawful-basis + color catalog services, the
// localization service (for color name locale->text maps), and the PEP enforcer.
func NewCatalogService(app *catalog.Service, color *catalog.ColorService, loc *locapp.Service, enforcer *pep.Enforcer) CatalogService {
	return CatalogService{app: app, color: color, loc: loc, pep: enforcer}
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

// ListColors implements GET /colors (D-Color). Reference data: read anywhere via the PEP.
func (s CatalogService) ListColors(ctx context.Context, token bearertoken.Token, domain *string, locales []string) (platformapi.ColorList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermColorRead)); err != nil {
		return platformapi.ColorList{}, err
	}
	d := ""
	if domain != nil {
		d = *domain
	}
	colors, err := s.color.List(ctx, d)
	if err != nil {
		return platformapi.ColorList{}, werror.WrapWithContextParams(ctx, err, "list colors failed")
	}
	names, err := s.colorNames(ctx, colors)
	if err != nil {
		return platformapi.ColorList{}, werror.WrapWithContextParams(ctx, err, "resolve color names failed")
	}
	want := localeProjection(locales)
	out := make([]platformapi.Color, 0, len(colors))
	for _, c := range colors {
		out = append(out, toAPIColor(c, projectLocales(names[c.ID], want)))
	}
	return platformapi.ColorList{Colors: out}, nil
}

// localeProjection normalizes the `locales=` query projection (D-i18n, review R-19) into a lookup
// set, or nil for "no projection — return all enabled locales" (the default). An absent or empty list
// is treated as no projection (a caller asking for nothing gets everything, never a silently blank
// label).
func localeProjection(locales []string) map[string]struct{} {
	if len(locales) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(locales))
	for _, l := range locales {
		if l != "" {
			want[l] = struct{}{}
		}
	}
	if len(want) == 0 {
		return nil
	}
	return want
}

// projectLocales trims a locale->text label map to the requested locales (intersection with what is
// stored). want == nil means no projection: the full map is returned unchanged.
func projectLocales(m map[string]string, want map[string]struct{}) map[string]string {
	if want == nil {
		return m
	}
	out := make(map[string]string, len(want))
	for locale, text := range m {
		if _, ok := want[locale]; ok {
			out[locale] = text
		}
	}
	return out
}

// UpsertColor implements PUT /colors (instance-admin; `color.manage`).
func (s CatalogService) UpsertColor(ctx context.Context, token bearertoken.Token, req platformapi.UpsertColorRequest) (platformapi.Color, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermColorManage)); err != nil {
		return platformapi.Color{}, err
	}
	saved, err := s.color.Upsert(ctx, catalog.Color{
		Domain:    req.Domain,
		Code:      req.Code,
		Name:      req.Name,
		Hex:       req.Hex,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		if errors.Is(err, catalog.ErrInvalidColor) {
			return platformapi.Color{}, platformapi.NewDemoError("color upsert: domain (eye|hair|vehicle), code, and name are required")
		}
		return platformapi.Color{}, werror.WrapWithContextParams(ctx, err, "upsert color failed")
	}
	names, err := s.colorNames(ctx, []catalog.Color{saved})
	if err != nil {
		return platformapi.Color{}, werror.WrapWithContextParams(ctx, err, "resolve color name failed")
	}
	return toAPIColor(saved, names[saved.ID]), nil
}

// colorNames overlays each color's default-locale name with the localization store (D-i18n), keyed by
// the color RID, returning id -> (locale -> text).
func (s CatalogService) colorNames(ctx context.Context, colors []catalog.Color) (map[string]map[string]string, error) {
	defaults := make(map[string]string, len(colors))
	for _, c := range colors {
		defaults[c.ID] = c.Name
	}
	return s.loc.NamesByID(ctx, colorEntity, defaults)
}

func toAPIColor(c catalog.Color, name map[string]string) platformapi.Color {
	return platformapi.Color{
		Id:        c.ID,
		Domain:    c.Domain,
		Code:      c.Code,
		Name:      name,
		Hex:       c.Hex,
		Status:    c.Status,
		SortOrder: c.SortOrder,
	}
}
