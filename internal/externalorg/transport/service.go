// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport implements the generated externalorgapi.ExternalOrganizationService (D-ExternalOrgs,
// M30). It PEP-gates each op (external orgs are instance-global external reference data, so reads gate on
// `externalorg.read` and writes on the instance-scope `externalorg.manage`, both satisfied anywhere via
// the PEP), assembles the translatable kind/org names as locale->text maps via the localization service,
// resolves best-effort default-locale kind/country labels, and maps domain sentinels to the Conjure
// ExternalOrg:* SerializableErrors. Generated code is never hand-edited.
package transport

import (
	"context"
	"errors"
	"time"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	externalorgapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/externalorg"
	"github.com/olegamysk/go-oikumenea/internal/externalorg/application"
	"github.com/olegamysk/go-oikumenea/internal/externalorg/domain"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// i18n entity types the translatable names are stored under (localization store).
const (
	entKind = "external_org_kind"
	entOrg  = "external_organization"
)

const (
	readPerm   = string(authzdomain.PermExternalOrgRead)
	managePerm = string(authzdomain.PermExternalOrgManage)
)

// ExternalOrganizationService adapts *application.Service to the generated Conjure interface.
type ExternalOrganizationService struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the application service, the localization service (name
// maps), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) ExternalOrganizationService {
	return ExternalOrganizationService{app: app, loc: loc, pep: enforcer}
}

var _ externalorgapi.ExternalOrganizationService = ExternalOrganizationService{}

// ============================ kind catalog ============================

func (s ExternalOrganizationService) ListExternalOrgKinds(ctx context.Context, token bearertoken.Token) (externalorgapi.ExternalOrgKindList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return externalorgapi.ExternalOrgKindList{}, err
	}
	rows, err := s.app.ListKinds(ctx)
	if err != nil {
		return externalorgapi.ExternalOrgKindList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(rows))
	for _, k := range rows {
		defaults[k.ID] = k.Name
	}
	names, err := s.loc.NamesByID(ctx, entKind, defaults)
	if err != nil {
		return externalorgapi.ExternalOrgKindList{}, s.mapError(ctx, err)
	}
	out := make([]externalorgapi.ExternalOrgKind, 0, len(rows))
	for _, k := range rows {
		out = append(out, kindAPI(k, names[k.ID]))
	}
	return externalorgapi.ExternalOrgKindList{Kinds: out}, nil
}

func (s ExternalOrganizationService) UpsertExternalOrgKind(ctx context.Context, token bearertoken.Token, req externalorgapi.UpsertExternalOrgKindRequest) (externalorgapi.ExternalOrgKind, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return externalorgapi.ExternalOrgKind{}, err
	}
	k, err := s.app.UpsertKind(ctx, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return externalorgapi.ExternalOrgKind{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entKind, k.ID, k.Name)
	if err != nil {
		return externalorgapi.ExternalOrgKind{}, s.mapError(ctx, err)
	}
	return kindAPI(k, name), nil
}

// ============================ organizations ============================

func (s ExternalOrganizationService) ListExternalOrgs(ctx context.Context, token bearertoken.Token, query, kind, country, status *string, pageSize *int, pageToken *string) (externalorgapi.ExternalOrgPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return externalorgapi.ExternalOrgPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListOrgs(ctx, strOr(query), strOr(kind), strOr(country), strOr(status), decodeToken(pageToken), limit)
	if err != nil {
		return externalorgapi.ExternalOrgPage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	out, err := s.orgsWithLabels(ctx, rows)
	if err != nil {
		return externalorgapi.ExternalOrgPage{}, s.mapError(ctx, err)
	}
	page := externalorgapi.ExternalOrgPage{Orgs: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s ExternalOrganizationService) CreateExternalOrg(ctx context.Context, token bearertoken.Token, req externalorgapi.CreateExternalOrgRequest) (externalorgapi.ExternalOrganization, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return externalorgapi.ExternalOrganization{}, err
	}
	o, err := s.app.CreateOrg(ctx, domain.OrgInput{
		KindID:     req.KindId,
		Name:       req.Name,
		Code:       strOr(req.Code),
		CountryID:  strOr(req.CountryId),
		WikidataID: strOr(req.WikidataId),
		Status:     strOr(req.Status),
		Source:     strOr(req.Source),
		Confidence: strOr(req.Confidence),
		AsOf:       timeOr(req.AsOf),
	})
	if err != nil {
		return externalorgapi.ExternalOrganization{}, s.mapError(ctx, err)
	}
	return s.orgWithLabels(ctx, o)
}

func (s ExternalOrganizationService) GetExternalOrg(ctx context.Context, token bearertoken.Token, orgID string) (externalorgapi.ExternalOrganization, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return externalorgapi.ExternalOrganization{}, err
	}
	o, err := s.app.GetOrg(ctx, orgID)
	if err != nil {
		return externalorgapi.ExternalOrganization{}, s.mapError(ctx, err)
	}
	return s.orgWithLabels(ctx, o)
}

func (s ExternalOrganizationService) UpdateExternalOrg(ctx context.Context, token bearertoken.Token, orgID string, req externalorgapi.UpdateExternalOrgRequest) (externalorgapi.ExternalOrganization, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return externalorgapi.ExternalOrganization{}, err
	}
	o, err := s.app.UpdateOrg(ctx, orgID, domain.OrgUpdate{
		KindID:     req.KindId,
		Name:       req.Name,
		Code:       req.Code,
		CountryID:  req.CountryId,
		WikidataID: req.WikidataId,
		Status:     req.Status,
		Source:     req.Source,
		Confidence: req.Confidence,
		AsOf:       timeOr(req.AsOf),
	})
	if err != nil {
		return externalorgapi.ExternalOrganization{}, s.mapError(ctx, err)
	}
	return s.orgWithLabels(ctx, o)
}

func (s ExternalOrganizationService) DeleteExternalOrg(ctx context.Context, token bearertoken.Token, orgID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteOrg(ctx, orgID))
}

func (s ExternalOrganizationService) MergeExternalOrg(ctx context.Context, token bearertoken.Token, orgID string, req externalorgapi.MergeExternalOrgRequest) (externalorgapi.ExternalOrganization, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return externalorgapi.ExternalOrganization{}, err
	}
	o, err := s.app.MergeOrg(ctx, orgID, req.IntoOrgId, strOr(req.Confidence))
	if err != nil {
		return externalorgapi.ExternalOrganization{}, s.mapError(ctx, err)
	}
	return s.orgWithLabels(ctx, o)
}

// ============================ label assembly ============================

func (s ExternalOrganizationService) orgWithLabels(ctx context.Context, o domain.Organization) (externalorgapi.ExternalOrganization, error) {
	out, err := s.orgsWithLabels(ctx, []domain.Organization{o})
	if err != nil {
		return externalorgapi.ExternalOrganization{}, err
	}
	return out[0], nil
}

func (s ExternalOrganizationService) orgsWithLabels(ctx context.Context, rows []domain.Organization) ([]externalorgapi.ExternalOrganization, error) {
	kindIDs, countryIDs := make([]string, 0, len(rows)), make([]string, 0)
	nameDefaults := make(map[string]string, len(rows))
	for _, o := range rows {
		kindIDs = append(kindIDs, o.KindID)
		nameDefaults[o.ID] = o.Name
		if o.CountryID != "" {
			countryIDs = append(countryIDs, o.CountryID)
		}
	}
	kinds, err := s.app.KindNamesByIDs(ctx, kindIDs)
	if err != nil {
		return nil, err
	}
	countries, err := s.app.CountryNamesByIDs(ctx, countryIDs)
	if err != nil {
		return nil, err
	}
	names, err := s.loc.NamesByID(ctx, entOrg, nameDefaults)
	if err != nil {
		return nil, err
	}
	out := make([]externalorgapi.ExternalOrganization, 0, len(rows))
	for _, o := range rows {
		out = append(out, orgAPI(o, names[o.ID], kinds[o.KindID], countries[o.CountryID]))
	}
	return out, nil
}

// ============================ api mappers ============================

func kindAPI(k domain.Kind, name map[string]string) externalorgapi.ExternalOrgKind {
	return externalorgapi.ExternalOrgKind{Id: k.ID, Code: k.Code, Name: name, Status: k.Status, SortOrder: k.SortOrder}
}

func orgAPI(o domain.Organization, name map[string]string, kindLabel, countryLabel string) externalorgapi.ExternalOrganization {
	out := externalorgapi.ExternalOrganization{
		Id: o.ID, KindId: o.KindID, KindLabel: emptyToNil(kindLabel), Name: name,
		Code: emptyToNil(o.Code), CountryId: emptyToNil(o.CountryID), CountryLabel: emptyToNil(countryLabel),
		WikidataId: emptyToNil(o.WikidataID), Status: o.Status, Source: o.Source, Confidence: o.Confidence,
		CreatedAt: datetime.DateTime(o.CreatedAt), UpdatedAt: datetime.DateTime(o.UpdatedAt),
	}
	if o.AsOf != nil {
		t := datetime.DateTime(*o.AsOf)
		out.AsOf = &t
	}
	return out
}

// ============================ helpers ============================

func (s ExternalOrganizationService) nameMap(ctx context.Context, entityType, id, def string) (map[string]string, error) {
	m, err := s.loc.NamesByID(ctx, entityType, map[string]string{id: def})
	if err != nil {
		return nil, err
	}
	return m[id], nil
}

func (s ExternalOrganizationService) mapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return externalorgapi.NewExternalOrgNotFound("")
	case errors.Is(err, domain.ErrConflict):
		return externalorgapi.NewConflict("code or wikidata id already exists in scope")
	case errors.Is(err, domain.ErrInvalid):
		return externalorgapi.NewInvalid("invalid request or unknown reference")
	}
	return werror.WrapWithContextParams(ctx, err, "external-org operation failed")
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func timeOr(p *datetime.DateTime) *time.Time {
	if p == nil {
		return nil
	}
	t := time.Time(*p)
	return &t
}

// ---- pagination tokens (opaque base64 of the last id) ----

// pageSizePolicy mirrors the owning application service's clamp, applied at the wire edge over the
// optional Conjure arg (M56 / pkg/listing).
var pageSizePolicy = listing.PageSize{Default: 50, Max: 200}

func pageSizeOr(p *int) int { return pageSizePolicy.ResolvePtr(p) }

// decodeToken/encodeToken are the opaque keyset cursor over the last row's RID, delegated to the
// shared pkg/listing codec (M56). These endpoints previously emitted base64 StdEncoding, whose
// `+`, `/` and `=` are NOT URL-safe in a query parameter (a `+` decodes to a space, corrupting the
// cursor); listing.EncodeCursor emits RawURL, and its decode stays tolerant of the old alphabet so
// tokens issued before the upgrade keep working. An undecodable token still yields "" — restarting
// at the first page — preserving this transport's existing behaviour.
func decodeToken(p *string) string {
	id, err := listing.DecodeCursorPtr(p)
	if err != nil {
		return ""
	}
	return id
}

func encodeToken(id string) string { return listing.EncodeCursor(id) }
