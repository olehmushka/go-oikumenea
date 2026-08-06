// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport implements the language module's generated Conjure LanguageService interface: it
// translates the wire contract to/from the application service (overview.md; D-Conjure). Generated
// code in internal/conjure is never hand-edited.
//
// Authorization (M7): reads require `language.read` (held anywhere — the language registry is
// instance-global, not unit-keyed), enforced via the PEP. The bearer token carries the acting subject.
package transport

import (
	"context"

	authzdomain "github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	languageapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/language"
	"github.com/olehmushka/go-oikumenea/internal/language/application"
	"github.com/olehmushka/go-oikumenea/internal/language/domain"
	locapp "github.com/olehmushka/go-oikumenea/internal/localization/application"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// i18n entity types the localized `name` maps are stored under (D-i18n: all enabled locales as a
// locale->text map, seeded from each row's default-locale `name` column + the i18n store).
const (
	entityLanguoid      = "languoid"
	entityWritingSystem = "writing_system"
)

// Service adapts *application.Service to the generated languageapi.LanguageService interface. It holds
// the localization service to assemble the `locale -> text` display-name maps responses return.
type Service struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the language application service, the localization
// service (for name-map assembly), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, loc: loc, pep: enforcer}
}

// namesFor assembles the locale->text name maps for a set of entities of one kind (D-i18n: all
// enabled locales, no negotiation), seeded from each entity's default-locale `name` column.
func (s Service) namesFor(ctx context.Context, entityType string, defaults map[string]string) (map[string]map[string]string, error) {
	return s.loc.NamesByID(ctx, entityType, defaults)
}

var _ languageapi.LanguageService = Service{}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ListLanguages implements GET /languages. The four facet args are built through the shared
// languoidFilter so this list and its dashboard read one URL identically (M58 ticket 4); the
// traversal, search and paging fields are added on top, because the aggregate counts none of them.
func (s Service) ListLanguages(ctx context.Context, token bearertoken.Token, level, family, macroarea, status, parent *string, topLevel *bool, query *string, pageSize *int, pageToken *string) (languageapi.LanguoidList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermLanguageRead)); err != nil {
		return languageapi.LanguoidList{}, err
	}
	f := languoidFilter(level, family, macroarea, status, query)
	f.Parent = deref(parent)
	f.TopLevel = topLevel != nil && *topLevel
	f.After = deref(pageToken)
	if pageSize != nil {
		f.Limit = *pageSize
	}
	if err := f.Validate(); err != nil {
		return languageapi.LanguoidList{}, mapLanguoidError(ctx, err, "list languoids failed")
	}
	langs, next, err := s.app.ListLanguoidsPage(ctx, f)
	if err != nil {
		return languageapi.LanguoidList{}, werror.WrapWithContextParams(ctx, err, "list languoids failed")
	}
	defaults := make(map[string]string, len(langs))
	for _, l := range langs {
		defaults[l.ID] = l.Name
	}
	names, err := s.namesFor(ctx, entityLanguoid, defaults)
	if err != nil {
		return languageapi.LanguoidList{}, werror.WrapWithContextParams(ctx, err, "assemble languoid names failed")
	}
	out := make([]languageapi.Languoid, 0, len(langs))
	for _, l := range langs {
		out = append(out, toAPILanguoid(l, names[l.ID]))
	}
	list := languageapi.LanguoidList{Languoids: out}
	if next != "" {
		list.NextPageToken = &next
	}
	return list, nil
}

// GetLanguage implements GET /languages/{id}.
func (s Service) GetLanguage(ctx context.Context, token bearertoken.Token, id string) (languageapi.Languoid, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermLanguageRead)); err != nil {
		return languageapi.Languoid{}, err
	}
	l, found, err := s.app.GetLanguoid(ctx, id)
	if err != nil {
		return languageapi.Languoid{}, werror.WrapWithContextParams(ctx, err, "get languoid failed")
	}
	if !found {
		return languageapi.Languoid{}, languageapi.NewLanguoidNotFound(id)
	}
	names, err := s.namesFor(ctx, entityLanguoid, map[string]string{l.ID: l.Name})
	if err != nil {
		return languageapi.Languoid{}, werror.WrapWithContextParams(ctx, err, "assemble languoid name failed")
	}
	return toAPILanguoid(l, names[l.ID]), nil
}

// ListWritingSystems implements GET /writing-systems.
func (s Service) ListWritingSystems(ctx context.Context, token bearertoken.Token) (languageapi.WritingSystemList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermLanguageRead)); err != nil {
		return languageapi.WritingSystemList{}, err
	}
	wss, err := s.app.ListWritingSystems(ctx)
	if err != nil {
		return languageapi.WritingSystemList{}, werror.WrapWithContextParams(ctx, err, "list writing systems failed")
	}
	defaults := make(map[string]string, len(wss))
	for _, w := range wss {
		defaults[w.ID] = w.Name
	}
	names, err := s.namesFor(ctx, entityWritingSystem, defaults)
	if err != nil {
		return languageapi.WritingSystemList{}, werror.WrapWithContextParams(ctx, err, "assemble writing system names failed")
	}
	out := make([]languageapi.WritingSystem, 0, len(wss))
	for _, w := range wss {
		ws := languageapi.WritingSystem{Id: w.ID, Code: w.Code, Name: names[w.ID]}
		if w.ScriptType != "" {
			st := w.ScriptType
			ws.ScriptType = &st
		}
		out = append(out, ws)
	}
	return languageapi.WritingSystemList{WritingSystems: out}, nil
}

func toAPILanguoid(l domain.Languoid, name map[string]string) languageapi.Languoid {
	out := languageapi.Languoid{
		Id:     l.ID,
		Code:   l.Code,
		Level:  l.Level,
		Name:   name,
		Status: l.Status,
	}
	if l.ParentID != "" {
		v := l.ParentID
		out.ParentId = &v
	}
	if l.FamilyCode != "" {
		v := l.FamilyCode
		out.FamilyCode = &v
	}
	if l.ISO639_3 != "" {
		v := l.ISO639_3
		out.Iso6393 = &v
	}
	if l.Macroarea != "" {
		v := l.Macroarea
		out.Macroarea = &v
	}
	return out
}
