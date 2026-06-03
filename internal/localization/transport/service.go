// Package transport implements the localization module's generated Conjure LocalizationService
// interface: it translates the wire contract to/from the application service and maps domain errors
// to Conjure SerializableErrors (overview.md; D-Conjure). Generated code in internal/conjure is
// never hand-edited.
//
// Authorization is intentionally not yet enforced here: the endpoints declare `auth: header` so the
// bearer token is parsed, but the `locale.read`/`locale.manage`/`translation.read`/
// `translation.manage` permission checks are wired once authorization (M7) + identity-federation
// (M8) land (docs/modules/localization.md). The handlers receive the token and ignore it for now.
package transport

import (
	"context"
	"errors"

	locapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/localization"
	"github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/localization/domain"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// Service adapts *application.Service to the generated locapi.LocalizationService interface.
type Service struct {
	app *application.Service
}

// NewService builds the transport adapter over the localization application service.
func NewService(app *application.Service) Service {
	return Service{app: app}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ locapi.LocalizationService = Service{}

// ListLocales implements GET /locales.
func (s Service) ListLocales(ctx context.Context, _ bearertoken.Token) (locapi.LocaleList, error) {
	locales, err := s.app.ListLocales(ctx)
	if err != nil {
		return locapi.LocaleList{}, mapError(ctx, err, "")
	}
	out := make([]locapi.Locale, 0, len(locales))
	for _, l := range locales {
		out = append(out, toAPILocale(l))
	}
	return locapi.LocaleList{Locales: out}, nil
}

// AddLocale implements POST /locales.
func (s Service) AddLocale(ctx context.Context, _ bearertoken.Token, req locapi.AddLocaleRequest) (locapi.Locale, error) {
	l := domain.Locale{
		Code:      req.Code,
		Name:      req.Name,
		Enabled:   derefOr(req.Enabled, true),
		SortOrder: derefOr(req.SortOrder, 0),
	}
	created, err := s.app.AddLocale(ctx, l)
	if err != nil {
		return locapi.Locale{}, mapError(ctx, err, req.Code)
	}
	return toAPILocale(created), nil
}

// UpdateLocale implements PUT /locales/{localeCode}.
func (s Service) UpdateLocale(ctx context.Context, _ bearertoken.Token, localeCode string, req locapi.UpdateLocaleRequest) (locapi.Locale, error) {
	updated, err := s.app.UpdateLocale(ctx, localeCode, domain.LocalePatch{
		Name:      req.Name,
		Enabled:   req.Enabled,
		IsDefault: req.IsDefault,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return locapi.Locale{}, mapError(ctx, err, localeCode)
	}
	return toAPILocale(updated), nil
}

// GetTranslations implements GET /translations/{entityType}/{entityId}.
func (s Service) GetTranslations(ctx context.Context, _ bearertoken.Token, entityType, entityID string) (map[string]map[string]string, error) {
	ts, err := s.app.GetTranslations(ctx, entityType, entityID)
	if err != nil {
		return nil, mapError(ctx, err, "")
	}
	return toAPITranslations(ts), nil
}

// PutTranslations implements PUT /translations/{entityType}/{entityId}.
func (s Service) PutTranslations(ctx context.Context, _ bearertoken.Token, entityType, entityID string, translations map[string]map[string]string) (map[string]map[string]string, error) {
	ts := make([]domain.Translation, 0, len(translations))
	for field, byLocale := range translations {
		for locale, text := range byLocale {
			ts = append(ts, domain.Translation{
				EntityType: entityType,
				EntityID:   entityID,
				Field:      field,
				Locale:     locale,
				Text:       text,
			})
		}
	}
	stored, err := s.app.UpsertTranslations(ctx, entityType, entityID, ts)
	if err != nil {
		return nil, mapError(ctx, err, "")
	}
	return toAPITranslations(stored), nil
}

// mapError translates domain/application errors into the Conjure SerializableError contract.
// localeCode is the code in scope (path/request) for the errors that name one.
func mapError(ctx context.Context, err error, localeCode string) error {
	var unknown domain.UnknownLocaleError
	switch {
	case errors.As(err, &unknown):
		return locapi.NewUnknownLocale(unknown.Code)
	case errors.Is(err, domain.ErrLocaleNotFound):
		return locapi.NewLocaleNotFound(localeCode)
	case errors.Is(err, domain.ErrLocaleConflict):
		return locapi.NewLocaleCodeConflict(localeCode)
	case errors.Is(err, domain.ErrInvalidLocale):
		return locapi.NewLocaleInvalid(err.Error())
	case errors.Is(err, domain.ErrLocaleConstraint):
		return locapi.NewLocaleConstraintViolation(err.Error())
	default:
		return werror.WrapWithContextParams(ctx, err, "localization request failed")
	}
}

func toAPILocale(l domain.Locale) locapi.Locale {
	return locapi.Locale{
		Code:      l.Code,
		Name:      l.Name,
		Enabled:   l.Enabled,
		IsDefault: l.IsDefault,
		SortOrder: l.SortOrder,
	}
}

// toAPITranslations assembles the field -> (locale -> text) response map.
func toAPITranslations(ts []domain.Translation) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for _, t := range ts {
		byLocale := out[t.Field]
		if byLocale == nil {
			byLocale = make(map[string]string)
			out[t.Field] = byLocale
		}
		byLocale[t.Locale] = t.Text
	}
	return out
}

func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}
