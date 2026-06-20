// Package transport implements the generated religionapi.ReligionService (D-Religion, M22). It PEP-gates
// each op (taxonomy/catalog reads on religion.read and writes on religion.catalog.manage — instance
// reference data, satisfied anywhere; per-unit org ops on religionorg.manage, checked against the unit
// over the canonical graph), assembles translatable names as locale->text maps via the localization
// service, and maps domain sentinels to the Conjure Religion:* errors. Generated code is never hand-edited.
package transport

import (
	"context"
	"encoding/base64"
	"errors"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	religionapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/religion"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/religion/application"
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// i18n entity types the translatable names are stored under.
const (
	entTaxon           = "religion_taxon"
	entTaxonRank       = "religion_taxon_rank"
	entClassification  = "religion_classification"
	entOrgKind         = "religion_org_kind"
	entPolicyKind      = "religion_policy_kind"
	entGradeCategory   = "religion_grade_category"
	entClergyGrade     = "religion_clergy_grade"
	entOfficeType      = "religion_office_type"
	entAffiliationType = "religion_affiliation_type"
)

const (
	readPerm        = string(authzdomain.PermReligionRead)
	catalogPerm     = string(authzdomain.PermReligionCatalogManage)
	orgPerm         = string(authzdomain.PermReligionOrgManage)
	clergyPerm      = string(authzdomain.PermClergyManage)
	affiliationPerm = string(authzdomain.PermAffiliationManage)
)

// ReligionService adapts *application.Service to the generated religionapi.ReligionService interface.
type ReligionService struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the religion application service, localization, and PEP.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) ReligionService {
	return ReligionService{app: app, loc: loc, pep: enforcer}
}

var _ religionapi.ReligionService = ReligionService{}

// ============================ catalogs ============================

func (s ReligionService) ListTaxonRanks(ctx context.Context, token bearertoken.Token) (religionapi.TaxonRankList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.TaxonRankList{}, err
	}
	ranks, err := s.app.ListTaxonRanks(ctx)
	if err != nil {
		return religionapi.TaxonRankList{}, s.mapError(ctx, err)
	}
	names, err := s.names(ctx, entTaxonRank, defaultsTaxonRank(ranks))
	if err != nil {
		return religionapi.TaxonRankList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.TaxonRank, 0, len(ranks))
	for _, r := range ranks {
		out = append(out, religionapi.TaxonRank{Id: r.ID, Code: r.Code, Name: names[r.ID], Ordinal: r.Ordinal, Status: r.Status, SortOrder: r.SortOrder})
	}
	return religionapi.TaxonRankList{TaxonRanks: out}, nil
}

func (s ReligionService) UpsertTaxonRank(ctx context.Context, token bearertoken.Token, req religionapi.UpsertTaxonRankRequest) (religionapi.TaxonRank, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.TaxonRank{}, err
	}
	r, err := s.app.UpsertTaxonRank(ctx, req.Code, req.Name, req.Ordinal, req.SortOrder)
	if err != nil {
		return religionapi.TaxonRank{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entTaxonRank, r.ID, r.Name)
	if err != nil {
		return religionapi.TaxonRank{}, s.mapError(ctx, err)
	}
	return religionapi.TaxonRank{Id: r.ID, Code: r.Code, Name: name, Ordinal: r.Ordinal, Status: r.Status, SortOrder: r.SortOrder}, nil
}

func (s ReligionService) ListClassifications(ctx context.Context, token bearertoken.Token) (religionapi.ClassificationList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.ClassificationList{}, err
	}
	cls, err := s.app.ListClassifications(ctx)
	if err != nil {
		return religionapi.ClassificationList{}, s.mapError(ctx, err)
	}
	list, err := s.classificationList(ctx, cls)
	if err != nil {
		return religionapi.ClassificationList{}, s.mapError(ctx, err)
	}
	return list, nil
}

func (s ReligionService) UpsertClassification(ctx context.Context, token bearertoken.Token, req religionapi.UpsertClassificationRequest) (religionapi.Classification, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.Classification{}, err
	}
	c, err := s.app.UpsertClassification(ctx, req.Code, req.Name, req.Description, req.SortOrder)
	if err != nil {
		return religionapi.Classification{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entClassification, c.ID, c.Name)
	if err != nil {
		return religionapi.Classification{}, s.mapError(ctx, err)
	}
	return classificationAPI(c, name), nil
}

func (s ReligionService) ListOrgKinds(ctx context.Context, token bearertoken.Token) (religionapi.OrgKindList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.OrgKindList{}, err
	}
	kinds, err := s.app.ListOrgKinds(ctx)
	if err != nil {
		return religionapi.OrgKindList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(kinds))
	for _, k := range kinds {
		defaults[k.ID] = k.Name
	}
	names, err := s.names(ctx, entOrgKind, defaults)
	if err != nil {
		return religionapi.OrgKindList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.OrgKind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, religionapi.OrgKind{Id: k.ID, ReligionId: emptyToNil(k.ReligionID), Code: k.Code, Name: names[k.ID], Ordinal: k.Ordinal, Status: k.Status, SortOrder: k.SortOrder})
	}
	return religionapi.OrgKindList{OrgKinds: out}, nil
}

func (s ReligionService) UpsertOrgKind(ctx context.Context, token bearertoken.Token, req religionapi.UpsertOrgKindRequest) (religionapi.OrgKind, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.OrgKind{}, err
	}
	k, err := s.app.UpsertOrgKind(ctx, req.Code, req.Name, req.ReligionId, req.Ordinal, req.SortOrder)
	if err != nil {
		return religionapi.OrgKind{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entOrgKind, k.ID, k.Name)
	if err != nil {
		return religionapi.OrgKind{}, s.mapError(ctx, err)
	}
	return religionapi.OrgKind{Id: k.ID, ReligionId: emptyToNil(k.ReligionID), Code: k.Code, Name: name, Ordinal: k.Ordinal, Status: k.Status, SortOrder: k.SortOrder}, nil
}

func (s ReligionService) ListPolicyKinds(ctx context.Context, token bearertoken.Token) (religionapi.PolicyKindList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.PolicyKindList{}, err
	}
	kinds, err := s.app.ListPolicyKinds(ctx)
	if err != nil {
		return religionapi.PolicyKindList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(kinds))
	for _, k := range kinds {
		defaults[k.ID] = k.Name
	}
	names, err := s.names(ctx, entPolicyKind, defaults)
	if err != nil {
		return religionapi.PolicyKindList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.PolicyKind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, religionapi.PolicyKind{Id: k.ID, Code: k.Code, Name: names[k.ID], Description: emptyToNil(k.Description), Status: k.Status, SortOrder: k.SortOrder})
	}
	return religionapi.PolicyKindList{PolicyKinds: out}, nil
}

func (s ReligionService) UpsertPolicyKind(ctx context.Context, token bearertoken.Token, req religionapi.UpsertPolicyKindRequest) (religionapi.PolicyKind, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.PolicyKind{}, err
	}
	k, err := s.app.UpsertPolicyKind(ctx, req.Code, req.Name, req.Description, req.SortOrder)
	if err != nil {
		return religionapi.PolicyKind{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entPolicyKind, k.ID, k.Name)
	if err != nil {
		return religionapi.PolicyKind{}, s.mapError(ctx, err)
	}
	return religionapi.PolicyKind{Id: k.ID, Code: k.Code, Name: name, Description: emptyToNil(k.Description), Status: k.Status, SortOrder: k.SortOrder}, nil
}

// ============================ taxonomy ============================

func (s ReligionService) ListTaxa(ctx context.Context, token bearertoken.Token, rank, parent, religion, query *string, pageSize *int, pageToken *string) (religionapi.TaxonPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.TaxonPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListTaxa(ctx, strOr(rank), strOr(parent), strOr(religion), strOr(query), decodeToken(pageToken), limit)
	if err != nil {
		return religionapi.TaxonPage{}, s.mapError(ctx, err)
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		t := encodeToken(rows[len(rows)-1].ID)
		next = &t
	}
	apiRows, err := s.taxaAPI(ctx, rows)
	if err != nil {
		return religionapi.TaxonPage{}, s.mapError(ctx, err)
	}
	return religionapi.TaxonPage{Taxa: apiRows, NextPageToken: next}, nil
}

func (s ReligionService) CreateTaxon(ctx context.Context, token bearertoken.Token, req religionapi.CreateTaxonRequest) (religionapi.Taxon, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.Taxon{}, err
	}
	t, err := s.app.CreateTaxon(ctx, domain.TaxonInput{
		Code: req.Code, Name: req.Name, RankID: req.RankId, ParentID: strOr(req.ParentId),
		Description: strOr(req.Description), WikidataID: strOr(req.WikidataId), SortOrder: req.SortOrder,
	})
	if err != nil {
		return religionapi.Taxon{}, s.mapError(ctx, err)
	}
	return s.taxonAPI(ctx, t)
}

func (s ReligionService) GetTaxon(ctx context.Context, token bearertoken.Token, id string) (religionapi.Taxon, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.Taxon{}, err
	}
	t, err := s.app.GetTaxon(ctx, id)
	if err != nil {
		return religionapi.Taxon{}, s.mapError(ctx, err)
	}
	return s.taxonAPI(ctx, t)
}

func (s ReligionService) UpdateTaxon(ctx context.Context, token bearertoken.Token, id string, req religionapi.UpdateTaxonRequest) (religionapi.Taxon, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.Taxon{}, err
	}
	t, err := s.app.UpdateTaxon(ctx, id, domain.TaxonUpdate{
		Name: req.Name, RankID: req.RankId, Description: req.Description, WikidataID: req.WikidataId, SortOrder: req.SortOrder,
	})
	if err != nil {
		return religionapi.Taxon{}, s.mapError(ctx, err)
	}
	return s.taxonAPI(ctx, t)
}

func (s ReligionService) DeleteTaxon(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteTaxon(ctx, id))
}

func (s ReligionService) ReparentTaxon(ctx context.Context, token bearertoken.Token, id string, req religionapi.ReparentTaxonRequest) (religionapi.Taxon, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.Taxon{}, err
	}
	t, err := s.app.ReparentTaxon(ctx, id, strOr(req.ParentId))
	if err != nil {
		return religionapi.Taxon{}, s.mapError(ctx, err)
	}
	return s.taxonAPI(ctx, t)
}

func (s ReligionService) RebuildClosure(ctx context.Context, token bearertoken.Token) (religionapi.ClosureReport, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.ClosureReport{}, err
	}
	rep, err := s.app.RebuildClosure(ctx)
	if err != nil {
		return religionapi.ClosureReport{}, s.mapError(ctx, err)
	}
	return religionapi.ClosureReport{MissingCount: 0, ExtraCount: 0, InDrift: rep.InDrift}, nil
}

func (s ReligionService) GetEffectiveClassifications(ctx context.Context, token bearertoken.Token, taxonID string) (religionapi.ClassificationList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return religionapi.ClassificationList{}, err
	}
	cls, err := s.app.EffectiveClassifications(ctx, taxonID)
	if err != nil {
		return religionapi.ClassificationList{}, s.mapError(ctx, err)
	}
	return s.classificationList(ctx, cls)
}

func (s ReligionService) SetTaxonClassifications(ctx context.Context, token bearertoken.Token, taxonID string, req religionapi.SetClassificationsRequest) (religionapi.ClassificationList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.ClassificationList{}, err
	}
	cls, err := s.app.SetTaxonClassifications(ctx, taxonID, req.ClassificationIds)
	if err != nil {
		return religionapi.ClassificationList{}, s.mapError(ctx, err)
	}
	return s.classificationList(ctx, cls)
}

// ============================ organization (per unit) ============================

func (s ReligionService) GetOrgProfile(ctx context.Context, token bearertoken.Token, unitID string) (religionapi.OrgProfile, error) {
	if err := s.pep.Require(ctx, token, readPerm, unitID); err != nil {
		return religionapi.OrgProfile{}, err
	}
	p, err := s.app.GetOrgProfile(ctx, unitID)
	if err != nil {
		return religionapi.OrgProfile{}, s.mapError(ctx, err)
	}
	return s.orgProfileAPI(ctx, p)
}

func (s ReligionService) SetOrgProfile(ctx context.Context, token bearertoken.Token, unitID string, req religionapi.SetOrgProfileRequest) (religionapi.OrgProfile, error) {
	if err := s.pep.Require(ctx, token, orgPerm, unitID); err != nil {
		return religionapi.OrgProfile{}, err
	}
	p, err := s.app.SetOrgProfile(ctx, unitID, req.OrgKindId, req.ShortCode)
	if err != nil {
		return religionapi.OrgProfile{}, s.mapError(ctx, err)
	}
	return s.orgProfileAPI(ctx, p)
}

func (s ReligionService) AddOrgClassification(ctx context.Context, token bearertoken.Token, unitID string, req religionapi.AddOrgClassificationRequest) (religionapi.OrgClassification, error) {
	if err := s.pep.Require(ctx, token, orgPerm, unitID); err != nil {
		return religionapi.OrgClassification{}, err
	}
	c, err := s.app.AddOrgClassification(ctx, unitID, req.TaxonId, boolOr(req.IsPrimary), req.Source, req.Confidence)
	if err != nil {
		return religionapi.OrgClassification{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entTaxon, c.TaxonID, c.TaxonName)
	if err != nil {
		return religionapi.OrgClassification{}, s.mapError(ctx, err)
	}
	return orgClassificationAPI(c, name), nil
}

func (s ReligionService) RemoveOrgClassification(ctx context.Context, token bearertoken.Token, unitID, linkID string) error {
	if err := s.pep.Require(ctx, token, orgPerm, unitID); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveOrgClassification(ctx, unitID, linkID))
}

func (s ReligionService) SetUnitTypeOverride(ctx context.Context, token bearertoken.Token, unitID string, req religionapi.SetClassificationsRequest) (religionapi.ClassificationList, error) {
	if err := s.pep.Require(ctx, token, orgPerm, unitID); err != nil {
		return religionapi.ClassificationList{}, err
	}
	cls, err := s.app.SetUnitTypeOverride(ctx, unitID, req.ClassificationIds)
	if err != nil {
		return religionapi.ClassificationList{}, s.mapError(ctx, err)
	}
	return s.classificationList(ctx, cls)
}

func (s ReligionService) GetEffectiveType(ctx context.Context, token bearertoken.Token, unitID string) (religionapi.EffectiveType, error) {
	if err := s.pep.Require(ctx, token, readPerm, unitID); err != nil {
		return religionapi.EffectiveType{}, err
	}
	et, err := s.app.EffectiveType(ctx, unitID)
	if err != nil {
		return religionapi.EffectiveType{}, s.mapError(ctx, err)
	}
	list, err := s.classificationList(ctx, et.Classifications)
	if err != nil {
		return religionapi.EffectiveType{}, s.mapError(ctx, err)
	}
	return religionapi.EffectiveType{UnitId: et.UnitID, Classifications: list.Classifications, Source: et.Source}, nil
}

func (s ReligionService) ListOrgPolicies(ctx context.Context, token bearertoken.Token, unitID string) (religionapi.OrgPolicyList, error) {
	if err := s.pep.Require(ctx, token, readPerm, unitID); err != nil {
		return religionapi.OrgPolicyList{}, err
	}
	rows, err := s.app.ListOrgPolicies(ctx, unitID)
	if err != nil {
		return religionapi.OrgPolicyList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.OrgPolicy, 0, len(rows))
	for _, p := range rows {
		out = append(out, orgPolicyAPI(p))
	}
	return religionapi.OrgPolicyList{Policies: out}, nil
}

func (s ReligionService) AddOrgPolicy(ctx context.Context, token bearertoken.Token, unitID string, req religionapi.AddOrgPolicyRequest) (religionapi.OrgPolicy, error) {
	if err := s.pep.Require(ctx, token, orgPerm, unitID); err != nil {
		return religionapi.OrgPolicy{}, err
	}
	p, err := s.app.AddOrgPolicy(ctx, unitID, req.PolicyKindId, req.Reason, req.DecidedByPersonId)
	if err != nil {
		return religionapi.OrgPolicy{}, s.mapError(ctx, err)
	}
	return orgPolicyAPI(p), nil
}

func (s ReligionService) RemoveOrgPolicy(ctx context.Context, token bearertoken.Token, unitID, policyID string) error {
	if err := s.pep.Require(ctx, token, orgPerm, unitID); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveOrgPolicy(ctx, unitID, policyID))
}

func (s ReligionService) CreateChildOrg(ctx context.Context, token bearertoken.Token, unitID string, req religionapi.CreateChildOrgRequest) (religionapi.OrgProfile, error) {
	if err := s.pep.Require(ctx, token, orgPerm, unitID); err != nil {
		return religionapi.OrgProfile{}, err
	}
	p, err := s.app.CreateChildOrg(ctx, unitID, req.Code, req.Name, strOr(req.Visibility), strOr(req.OrgKindId), strOr(req.PrimaryTaxonId))
	if err != nil {
		return religionapi.OrgProfile{}, s.mapError(ctx, err)
	}
	return s.orgProfileAPI(ctx, p)
}

// ============================ mappers ============================

func (s ReligionService) taxonAPI(ctx context.Context, t domain.Taxon) (religionapi.Taxon, error) {
	name, err := s.nameMap(ctx, entTaxon, t.ID, t.Name)
	if err != nil {
		return religionapi.Taxon{}, s.mapError(ctx, err)
	}
	return taxonToAPI(t, name), nil
}

func (s ReligionService) taxaAPI(ctx context.Context, rows []domain.Taxon) ([]religionapi.Taxon, error) {
	defaults := make(map[string]string, len(rows))
	for _, t := range rows {
		defaults[t.ID] = t.Name
	}
	names, err := s.names(ctx, entTaxon, defaults)
	if err != nil {
		return nil, err
	}
	out := make([]religionapi.Taxon, 0, len(rows))
	for _, t := range rows {
		out = append(out, taxonToAPI(t, names[t.ID]))
	}
	return out, nil
}

func taxonToAPI(t domain.Taxon, name map[string]string) religionapi.Taxon {
	var depth *int
	if t.Depth > 0 {
		d := t.Depth
		depth = &d
	}
	return religionapi.Taxon{
		Id: t.ID, ParentId: emptyToNil(t.ParentID), RankId: t.RankID, RankCode: t.RankCode,
		ReligionId: emptyToNil(t.ReligionID), Code: t.Code, Name: name,
		Description: emptyToNil(t.Description), WikidataId: emptyToNil(t.WikidataID), SortOrder: t.SortOrder,
		Depth: depth, CreatedAt: datetime.DateTime(t.CreatedAt), UpdatedAt: datetime.DateTime(t.UpdatedAt),
	}
}

func (s ReligionService) classificationList(ctx context.Context, cls []domain.Classification) (religionapi.ClassificationList, error) {
	defaults := make(map[string]string, len(cls))
	for _, c := range cls {
		defaults[c.ID] = c.Name
	}
	names, err := s.names(ctx, entClassification, defaults)
	if err != nil {
		return religionapi.ClassificationList{}, err
	}
	out := make([]religionapi.Classification, 0, len(cls))
	for _, c := range cls {
		out = append(out, classificationAPI(c, names[c.ID]))
	}
	return religionapi.ClassificationList{Classifications: out}, nil
}

func classificationAPI(c domain.Classification, name map[string]string) religionapi.Classification {
	return religionapi.Classification{Id: c.ID, Code: c.Code, Name: name, Description: emptyToNil(c.Description), Status: c.Status, SortOrder: c.SortOrder}
}

func (s ReligionService) orgProfileAPI(ctx context.Context, p domain.OrgProfile) (religionapi.OrgProfile, error) {
	defaults := make(map[string]string, len(p.Classifications))
	for _, c := range p.Classifications {
		defaults[c.TaxonID] = c.TaxonName
	}
	names, err := s.names(ctx, entTaxon, defaults)
	if err != nil {
		return religionapi.OrgProfile{}, s.mapError(ctx, err)
	}
	cls := make([]religionapi.OrgClassification, 0, len(p.Classifications))
	for _, c := range p.Classifications {
		cls = append(cls, orgClassificationAPI(c, names[c.TaxonID]))
	}
	return religionapi.OrgProfile{
		UnitId: p.UnitID, OrgKindId: emptyToNil(p.OrgKindID), ShortCode: emptyToNil(p.ShortCode),
		Classifications: cls, CreatedAt: datetime.DateTime(p.CreatedAt), UpdatedAt: datetime.DateTime(p.UpdatedAt),
	}, nil
}

func orgClassificationAPI(c domain.OrgClassification, taxonName map[string]string) religionapi.OrgClassification {
	return religionapi.OrgClassification{
		Id: c.ID, UnitId: c.UnitID, TaxonId: c.TaxonID, TaxonCode: c.TaxonCode, TaxonName: taxonName,
		IsPrimary: c.IsPrimary, Source: emptyToNil(c.Source), Confidence: emptyToNil(c.Confidence),
		CreatedAt: datetime.DateTime(c.CreatedAt), UpdatedAt: datetime.DateTime(c.UpdatedAt),
	}
}

func orgPolicyAPI(p domain.OrgPolicy) religionapi.OrgPolicy {
	var decidedAt *datetime.DateTime
	if p.DecidedAt != nil {
		d := datetime.DateTime(*p.DecidedAt)
		decidedAt = &d
	}
	return religionapi.OrgPolicy{
		Id: p.ID, UnitId: p.UnitID, PolicyKindId: p.PolicyKindID, PolicyKindCode: p.PolicyKindCode,
		Reason: emptyToNil(p.Reason), DecidedByPersonId: emptyToNil(p.DecidedByPersonID), DecidedAt: decidedAt,
		CreatedAt: datetime.DateTime(p.CreatedAt), UpdatedAt: datetime.DateTime(p.UpdatedAt),
	}
}

func defaultsTaxonRank(ranks []domain.TaxonRank) map[string]string {
	m := make(map[string]string, len(ranks))
	for _, r := range ranks {
		m[r.ID] = r.Name
	}
	return m
}

// ============================ helpers ============================

// names assembles translatable names as locale->text maps (default + i18n overlay).
func (s ReligionService) names(ctx context.Context, entityType string, defaults map[string]string) (map[string]map[string]string, error) {
	return s.loc.NamesByID(ctx, entityType, defaults)
}

func (s ReligionService) nameMap(ctx context.Context, entityType, id, def string) (map[string]string, error) {
	m, err := s.loc.NamesByID(ctx, entityType, map[string]string{id: def})
	if err != nil {
		return nil, err
	}
	return m[id], nil
}

// mapError translates religion domain sentinels into the Conjure Religion:* errors.
func (s ReligionService) mapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrTaxonNotFound):
		return religionapi.NewTaxonNotFound("")
	case errors.Is(err, domain.ErrClassificationNotFound):
		return religionapi.NewClassificationNotFound("")
	case errors.Is(err, domain.ErrOrgKindNotFound):
		return religionapi.NewOrgKindNotFound("")
	case errors.Is(err, domain.ErrPolicyKindNotFound):
		return religionapi.NewPolicyKindNotFound("")
	case errors.Is(err, domain.ErrProfileNotFound):
		return religionapi.NewProfileNotFound("")
	case errors.Is(err, domain.ErrPolicyNotFound):
		return religionapi.NewPolicyNotFound("")
	case errors.Is(err, domain.ErrConflict):
		return religionapi.NewConflict("code already exists in scope")
	case errors.Is(err, domain.ErrTaxonCycle):
		return religionapi.NewTaxonCycleDetected("")
	case errors.Is(err, domain.ErrInUse):
		return religionapi.NewInUse("entity still referenced")
	case errors.Is(err, domain.ErrChildCreationExcluded):
		return religionapi.NewChildCreationExcluded("")
	case errors.Is(err, domain.ErrGradeNotFound):
		return religionapi.NewGradeNotFound("")
	case errors.Is(err, domain.ErrCredentialNotFound):
		return religionapi.NewCredentialNotFound("")
	case errors.Is(err, domain.ErrAffiliationTypeNotFound):
		return religionapi.NewAffiliationTypeNotFound("")
	case errors.Is(err, domain.ErrAffiliationNotFound):
		return religionapi.NewAffiliationNotFound("")
	case errors.Is(err, domain.ErrInvalid):
		return religionapi.NewInvalid("invalid request or unknown reference")
	}
	return werror.WrapWithContextParams(ctx, err, "religion operation failed")
}

// ---- pagination tokens (opaque base64 of the last id) ----

func decodeToken(p *string) string {
	if p == nil || *p == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(*p)
	if err != nil {
		return ""
	}
	return string(raw)
}

func encodeToken(id string) string { return base64.StdEncoding.EncodeToString([]byte(id)) }

func pageSizeOr(p *int) int {
	if p == nil || *p <= 0 {
		return 50
	}
	if *p > 200 {
		return 200
	}
	return *p
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func boolOr(p *bool) bool { return p != nil && *p }

func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
