// Package action is the action-type registry (review-2026-09 R-29 / D-ActionTypes): the machine
// catalog of every named write the system audits. Foundry's ontology has typed Action types with a
// parameter schema and required permission; this repo's Actions were, until this registry, free-text
// names in audit_log.action with no read-time contract (D-Audit uses the generic kind=action RID
// type_code 0). This package turns that free text into a checked, enumerable catalog: each entry is
// its stable dotted code + owning RID service + the object type it targets + the gating permission.
//
// The registry is the SOURCE OF TRUTH. audit's Service.Record rejects an unregistered action at write
// time (so a typo like assignment.granted fails a test rather than silently drifting), the
// ontology-mapping.md §3 catalog table is generated from it and coherence-checked, and the
// AuditService.listActionTypes endpoint serves it so the console's actions panel is registry-driven.
// It is expand-only: the audit RID stays kind=action/type 0 (history is not rewritten); a new milestone
// adds its action rows here, and the coherence + validation tests fail until it does.
//
// Permission is the module-granular gating write permission (the finding's "required permission");
// where an action has no dedicated per-action permission code it carries its module's write/manage
// permission — enough to make actions enumerable and auditable by required authority.
package action

import (
	"errors"
	"fmt"
	"sort"

	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

//go:generate sh -c "cd ../.. && scripts/gen-action-params.sh"

// ErrUnregistered is returned by Validate for an action code not in the catalog.
var ErrUnregistered = errors.New("action: unregistered action code")

// Validate is the write-time gate (called by audit.Service.Record): an unregistered action code is
// rejected so a typo or an un-catalogued new action fails a test rather than drifting silently.
func Validate(code string) error {
	if !IsRegistered(code) {
		return fmt.Errorf("%w %q (add it to pkg/action — D-ActionTypes)", ErrUnregistered, code)
	}
	return nil
}

// ActionType is one registered action: its stable dotted code (what audit_log.action holds), the RID
// service that owns it, the object type it targets (the audit target_type), and its gating permission.
//
// RequestType names the Conjure request type that carries the action's inputs, package-qualified
// (e.g. "oikumenea.authorization.GrantAssignmentRequest"), so its parameter schema is single-sourced
// from the contract and never hand-authored (review-2026-09 R-29 parameter-schema seam). It is empty
// for actions with no request body (deletes, lifecycle POSTs whose only input is a path RID, imports)
// or not yet annotated — the catalog is expand-only, and Params degrades to nil for those.
type ActionType struct {
	Code        string
	Service     int
	TargetType  string
	Permission  string
	RequestType string
}

// Param is one argument of an action, projected from a Conjure request field. It is DERIVED from the
// IR by tools/genactionparams (see params_gen.go) — never hand-written — so it cannot drift from the
// contract. Descriptive only (discoverability), not a write-time validation schema.
type Param struct {
	Name     string
	Type     string // display token derived from the Conjure field type: string, rid, datetime, enum, list<string>, …
	Required bool
	Docs     string
}

// Params returns the argument schema for an action code, single-sourced from its RequestType's Conjure
// fields (nil when the action has no request body or is not yet annotated). Backs the parameters field
// on AuditService.listActionTypes.
func Params(code string) []Param {
	a, ok := byCode[code]
	if !ok || a.RequestType == "" {
		return nil
	}
	return requestParams[a.RequestType]
}

// actionTypes is the full catalog, sorted by (service, code). Derived from the audit emit sites
// (review-2026-09 R-29) — keep it sorted and add a row when a module mints a new audited action.
var actionTypes = []ActionType{
	{Code: "color.upsert", Service: rid.SvcPlatform, TargetType: "color", Permission: "color.manage", RequestType: "oikumenea.platform.UpsertColorRequest"},
	{Code: "import.colors", Service: rid.SvcPlatform, TargetType: "colors", Permission: "import.manage"},
	{Code: "import.ethnicity-scheme", Service: rid.SvcPlatform, TargetType: "ethnicity-scheme", Permission: "import.manage"},
	{Code: "import.external-organizations", Service: rid.SvcPlatform, TargetType: "external-organizations", Permission: "import.manage"},
	{Code: "import.geo-countries", Service: rid.SvcPlatform, TargetType: "geo-countries", Permission: "import.manage"},
	{Code: "import.geo-places", Service: rid.SvcPlatform, TargetType: "geo-places", Permission: "import.manage"},
	{Code: "import.language-scheme", Service: rid.SvcPlatform, TargetType: "language-scheme", Permission: "import.manage"},
	{Code: "import.language-scripts", Service: rid.SvcPlatform, TargetType: "language-scripts", Permission: "import.manage"},
	{Code: "import.person-regulatory-sanctions", Service: rid.SvcPlatform, TargetType: "person-regulatory-sanctions", Permission: "import.manage"},
	{Code: "import.religion-scheme", Service: rid.SvcPlatform, TargetType: "religion-scheme", Permission: "import.manage"},
	{Code: "import.translations", Service: rid.SvcPlatform, TargetType: "translations", Permission: "import.manage"},
	{Code: "legal-basis.upsert", Service: rid.SvcPlatform, TargetType: "color", Permission: "legal-basis.manage", RequestType: "oikumenea.platform.UpsertLegalBasisKindRequest"},
	{Code: "locale.add", Service: rid.SvcI18n, TargetType: "locale", Permission: "locale.manage", RequestType: "oikumenea.localization.AddLocaleRequest"},
	{Code: "locale.update", Service: rid.SvcI18n, TargetType: "locale", Permission: "locale.manage", RequestType: "oikumenea.localization.UpdateLocaleRequest"},
	{Code: "translation.upsert", Service: rid.SvcI18n, TargetType: "languoid", Permission: "translation.manage"},
	{Code: "closure.rebuild", Service: rid.SvcTenant, TargetType: "graph", Permission: "closure.rebuild"},
	{Code: "closure.verify", Service: rid.SvcTenant, TargetType: "graph", Permission: "unit.edges.manage"},
	{Code: "domain.create", Service: rid.SvcTenant, TargetType: "domain", Permission: "domain.manage", RequestType: "oikumenea.tenant.CreateDomainRequest"},
	{Code: "domain.update", Service: rid.SvcTenant, TargetType: "domain", Permission: "domain.manage", RequestType: "oikumenea.tenant.UpdateDomainRequest"},
	{Code: "graph.create", Service: rid.SvcTenant, TargetType: "graph", Permission: "graph.manage", RequestType: "oikumenea.tenant.AddGraphRequest"},
	{Code: "graph.delete", Service: rid.SvcTenant, TargetType: "graph", Permission: "graph.manage"},
	{Code: "graph.update", Service: rid.SvcTenant, TargetType: "graph", Permission: "graph.manage", RequestType: "oikumenea.tenant.UpdateGraphRequest"},
	{Code: "organization.create", Service: rid.SvcTenant, TargetType: "organization", Permission: "organization.create", RequestType: "oikumenea.tenant.CreateOrganizationRequest"},
	{Code: "organization.transition", Service: rid.SvcTenant, TargetType: "organization", Permission: "organization.lifecycle", RequestType: "oikumenea.tenant.TransitionRequest"},
	{Code: "organization.update", Service: rid.SvcTenant, TargetType: "organization", Permission: "organization.update", RequestType: "oikumenea.tenant.UpdateOrganizationRequest"},
	{Code: "unit-kind.create", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit-kind.manage", RequestType: "oikumenea.tenant.CreateUnitKindRequest"},
	{Code: "unit-kind.update", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit-kind.manage", RequestType: "oikumenea.tenant.UpdateUnitKindRequest"},
	{Code: "unit.create", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit.create", RequestType: "oikumenea.tenant.CreateUnitRequest"},
	{Code: "unit.edge.add", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit.edges.manage", RequestType: "oikumenea.tenant.AddEdgeRequest"},
	{Code: "unit.edge.remove", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit.edges.manage"},
	{Code: "unit.language.delete", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit.update"},
	{Code: "unit.language.upsert", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit.update", RequestType: "oikumenea.tenant.UpsertUnitLanguageRequest"},
	{Code: "unit.recode", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit.recode", RequestType: "oikumenea.tenant.SetUnitCodeRequest"},
	{Code: "unit.transition", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit.lifecycle", RequestType: "oikumenea.tenant.TransitionRequest"},
	{Code: "unit.update", Service: rid.SvcTenant, TargetType: "unit", Permission: "unit.update", RequestType: "oikumenea.tenant.UpdateUnitRequest"},
	{Code: "rank.category.create", Service: rid.SvcRank, TargetType: "rank_category", Permission: "rank.scheme.manage", RequestType: "oikumenea.rank.AddCategoryRequest"},
	{Code: "rank.category.delete", Service: rid.SvcRank, TargetType: "rank_category", Permission: "rank.scheme.manage"},
	{Code: "rank.category.update", Service: rid.SvcRank, TargetType: "rank_category", Permission: "rank.scheme.manage", RequestType: "oikumenea.rank.UpdateCategoryRequest"},
	{Code: "rank.rank.create", Service: rid.SvcRank, TargetType: "rank", Permission: "rank.scheme.manage", RequestType: "oikumenea.rank.AddRankRequest"},
	{Code: "rank.rank.delete", Service: rid.SvcRank, TargetType: "rank", Permission: "rank.scheme.manage"},
	{Code: "rank.rank.update", Service: rid.SvcRank, TargetType: "rank", Permission: "rank.scheme.manage", RequestType: "oikumenea.rank.UpdateRankRequest"},
	{Code: "rank.scheme.import", Service: rid.SvcRank, TargetType: "rank_system", Permission: "rank.scheme.manage", RequestType: "oikumenea.rank.ImportRankSchemeRequest"},
	{Code: "rank.system.create", Service: rid.SvcRank, TargetType: "rank_system", Permission: "rank.scheme.manage", RequestType: "oikumenea.rank.AddSystemRequest"},
	{Code: "rank.system.delete", Service: rid.SvcRank, TargetType: "rank_system", Permission: "rank.scheme.manage"},
	{Code: "rank.system.update", Service: rid.SvcRank, TargetType: "rank_system", Permission: "rank.scheme.manage", RequestType: "oikumenea.rank.UpdateSystemRequest"},
	{Code: "rank.type.create", Service: rid.SvcRank, TargetType: "rank_type", Permission: "rank.scheme.manage", RequestType: "oikumenea.rank.AddTypeRequest"},
	{Code: "rank.type.delete", Service: rid.SvcRank, TargetType: "rank_type", Permission: "rank.scheme.manage"},
	{Code: "rank.type.update", Service: rid.SvcRank, TargetType: "rank_type", Permission: "rank.scheme.manage", RequestType: "oikumenea.rank.UpdateTypeRequest"},
	{Code: "person.address.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.address.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertAddressRequest"},
	{Code: "person.association.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertAssociationRequest"},
	{Code: "person.call-sign.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.call-sign.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertCallSignRequest"},
	{Code: "person.citizenship.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.citizenship.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertCitizenshipRequest"},
	{Code: "person.crypto_wallet.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.crypto_wallet.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertCryptoWalletRequest"},
	{Code: "person.deactivate", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.DeactivateRequest"},
	{Code: "person.distinguishing_mark.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.distinguishing_mark.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertDistinguishingMarkRequest"},
	{Code: "person.email.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.email.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertEmailRequest"},
	{Code: "person.ethnicity.add", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.AddEthnicityRequest"},
	{Code: "person.ethnicity.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.ethnicity.erase", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.ethnicity.update", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpdateEthnicityRequest"},
	{Code: "person.ethnicity_type.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertEthnicityTypeRequest"},
	{Code: "person.external_reference.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.external_reference.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertExternalReferenceRequest"},
	{Code: "person.government_position.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.government_position.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertGovernmentPositionRequest"},
	{Code: "person.guardianship.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertGuardianshipRequest"},
	{Code: "person.kinship.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertKinshipRequest"},
	{Code: "person.language.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.language.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertPersonLanguageRequest"},
	{Code: "person.lobbying.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.lobbying.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertLobbyingRelationshipRequest"},
	{Code: "person.merge", Service: rid.SvcPerson, TargetType: "person", Permission: "person.merge", RequestType: "oikumenea.person.MergePersonRequest"},
	{Code: "person.messenger-link.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.messenger-link.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertMessengerLinkRequest"},
	{Code: "person.name_alias.add", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.AddNameAliasRequest"},
	{Code: "person.name_alias.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.name_variant.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.name_variant.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertNameVariantRequest"},
	{Code: "person.next-of-kin.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertNextOfKinRequest"},
	{Code: "person.partnership.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertPartnershipRequest"},
	{Code: "person.party_membership.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.party_membership.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertPartyMembershipRequest"},
	{Code: "person.personality.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.personality.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertPersonalityRequest"},
	{Code: "person.phone.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.phone.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertPhoneRequest"},
	{Code: "person.physical_description.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.physical_description.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertPhysicalDescriptionRequest"},
	{Code: "person.political_leaning.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.political_leaning.set", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertPoliticalLeaningRequest"},
	{Code: "person.provisional.create", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.CreateProvisionalPersonRequest"},
	{Code: "person.purge", Service: rid.SvcPerson, TargetType: "person", Permission: "person.purge"},
	{Code: "person.rank.assign", Service: rid.SvcPerson, TargetType: "person", Permission: "person.rank.assign", RequestType: "oikumenea.person.SetRankRequest"},
	{Code: "person.reactivate", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.regulatory_sanction.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.regulatory_sanction.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertRegulatorySanctionRequest"},
	{Code: "person.relationship.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.residence.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.residence.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertResidenceRequest"},
	{Code: "person.social-account.delete", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update"},
	{Code: "person.social-account.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertSocialAccountRequest"},
	{Code: "person.sponsorship.upsert", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpsertSponsorshipRequest"},
	{Code: "person.update", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.person.UpdatePersonRequest"},
	{Code: "person.watchlist.check", Service: rid.SvcPerson, TargetType: "person", Permission: "person.update", RequestType: "oikumenea.hermenea.WatchlistQuery"},
	{Code: "membership.create", Service: rid.SvcMembership, TargetType: "membership", Permission: "membership.create", RequestType: "oikumenea.membership.CreateMembershipRequest"},
	{Code: "membership.end", Service: rid.SvcMembership, TargetType: "membership", Permission: "membership.update", RequestType: "oikumenea.membership.EndMembershipRequest"},
	{Code: "membership.fill", Service: rid.SvcMembership, TargetType: "membership", Permission: "membership.create", RequestType: "oikumenea.membership.FillPositionRequest"},
	{Code: "position.abolish", Service: rid.SvcMembership, TargetType: "position", Permission: "position.update"},
	{Code: "position.create", Service: rid.SvcMembership, TargetType: "position", Permission: "position.create", RequestType: "oikumenea.membership.CreatePositionRequest"},
	{Code: "position.update", Service: rid.SvcMembership, TargetType: "position", Permission: "position.update", RequestType: "oikumenea.membership.UpdatePositionRequest"},
	{Code: "assignment.grant", Service: rid.SvcAuthz, TargetType: "assignment", Permission: "assignment.grant", RequestType: "oikumenea.authorization.GrantAssignmentRequest"},
	{Code: "assignment.revoke", Service: rid.SvcAuthz, TargetType: "assignment", Permission: "assignment.revoke"},
	{Code: "instance.admin.grant", Service: rid.SvcAuthz, TargetType: "instance_admin", Permission: "instance.admin.manage", RequestType: "oikumenea.authorization.GrantInstanceAdminRequest"},
	{Code: "instance.admin.revoke", Service: rid.SvcAuthz, TargetType: "instance_admin", Permission: "instance.admin.manage"},
	{Code: "role.create", Service: rid.SvcAuthz, TargetType: "role", Permission: "role.create", RequestType: "oikumenea.authorization.CreateRoleRequest"},
	{Code: "role.delete", Service: rid.SvcAuthz, TargetType: "role", Permission: "role.delete"},
	{Code: "role.update", Service: rid.SvcAuthz, TargetType: "role", Permission: "role.update", RequestType: "oikumenea.authorization.UpdateRoleRequest"},
	{Code: "account.create", Service: rid.SvcAccount, TargetType: "account", Permission: "instance.admin.manage", RequestType: "oikumenea.identityfederation.CreateAccountRequest"},
	{Code: "account.disable", Service: rid.SvcAccount, TargetType: "account", Permission: "instance.admin.manage"},
	{Code: "identity.link", Service: rid.SvcAccount, TargetType: "external_identity", Permission: "instance.admin.manage", RequestType: "oikumenea.identityfederation.LinkIdentityRequest"},
	{Code: "identity.unlink", Service: rid.SvcAccount, TargetType: "external_identity", Permission: "instance.admin.manage"},
	{Code: "person.create", Service: rid.SvcAccount, TargetType: "person", Permission: "person.create", RequestType: "oikumenea.person.CreatePersonRequest"},
	{Code: "document.create", Service: rid.SvcDocument, TargetType: "document", Permission: "document.create", RequestType: "oikumenea.document.CreateDocumentRequest"},
	{Code: "document.delete", Service: rid.SvcDocument, TargetType: "document", Permission: "document.delete"},
	{Code: "document.person.erase", Service: rid.SvcDocument, TargetType: "person", Permission: "person.purge"},
	{Code: "document.type.create", Service: rid.SvcDocument, TargetType: "document_type", Permission: "document.type.manage", RequestType: "oikumenea.document.CreateDocumentTypeRequest"},
	{Code: "document.type.update", Service: rid.SvcDocument, TargetType: "document_type", Permission: "document.type.manage", RequestType: "oikumenea.document.UpdateDocumentTypeRequest"},
	{Code: "document.update", Service: rid.SvcDocument, TargetType: "document", Permission: "document.update", RequestType: "oikumenea.document.UpdateDocumentRequest"},
	{Code: "personal-code-scheme.create", Service: rid.SvcDocument, TargetType: "document", Permission: "personal-code-scheme.manage", RequestType: "oikumenea.document.CreatePersonalCodeSchemeRequest"},
	{Code: "personal-code-scheme.update", Service: rid.SvcDocument, TargetType: "document", Permission: "personal-code-scheme.manage", RequestType: "oikumenea.document.UpdatePersonalCodeSchemeRequest"},
	{Code: "personal-code.create", Service: rid.SvcDocument, TargetType: "personal_code", Permission: "personal-code.create", RequestType: "oikumenea.document.CreatePersonalCodeRequest"},
	{Code: "personal-code.delete", Service: rid.SvcDocument, TargetType: "personal_code", Permission: "personal-code.delete"},
	{Code: "personal-code.update", Service: rid.SvcDocument, TargetType: "personal_code", Permission: "personal-code.update", RequestType: "oikumenea.document.UpdatePersonalCodeRequest"},
	{Code: "order.create", Service: rid.SvcOrder, TargetType: "order", Permission: "order.create", RequestType: "oikumenea.order.CreateOrderRequest"},
	{Code: "order.issue", Service: rid.SvcOrder, TargetType: "order", Permission: "order.issue"},
	{Code: "order.revoke", Service: rid.SvcOrder, TargetType: "order", Permission: "order.revoke", RequestType: "oikumenea.order.RevokeOrderRequest"},
	{Code: "order.type.create", Service: rid.SvcOrder, TargetType: "order_type", Permission: "order.type.manage", RequestType: "oikumenea.order.CreateOrderTypeRequest"},
	{Code: "order.type.update", Service: rid.SvcOrder, TargetType: "order_type", Permission: "order.type.manage", RequestType: "oikumenea.order.UpdateOrderTypeRequest"},
	{Code: "order.update", Service: rid.SvcOrder, TargetType: "order", Permission: "order.create", RequestType: "oikumenea.order.UpdateOrderRequest"},
	{Code: "location.create", Service: rid.SvcLocation, TargetType: "location", Permission: "location.create", RequestType: "oikumenea.location.LocationWrite"},
	{Code: "location.delete", Service: rid.SvcLocation, TargetType: "location", Permission: "location.update"},
	{Code: "location.update", Service: rid.SvcLocation, TargetType: "location", Permission: "location.update", RequestType: "oikumenea.location.LocationWrite"},
	{Code: "education.accreditation-event.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.catalog.manage", RequestType: "oikumenea.educationref.UpsertAccreditationEventRequest"},
	{Code: "education.accreditation-event.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.catalog.manage", RequestType: "oikumenea.educationref.UpsertAccreditationEventRequest"},
	{Code: "education.appointment.end", Service: rid.SvcEducation, TargetType: "education", Permission: "education.position.manage", RequestType: "oikumenea.education.EndAppointmentRequest"},
	{Code: "education.appointment.fill", Service: rid.SvcEducation, TargetType: "education", Permission: "education.position.manage", RequestType: "oikumenea.education.FillPositionRequest"},
	{Code: "education.building.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.education.CreateBuildingRequest"},
	{Code: "education.building.delete", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage"},
	{Code: "education.building.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.education.UpdateBuildingRequest"},
	{Code: "education.course-prerequisite.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.CreateCoursePrerequisiteRequest"},
	{Code: "education.course.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertCourseRequest"},
	{Code: "education.course.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertCourseRequest"},
	{Code: "education.curriculum-item.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertCurriculumItemRequest"},
	{Code: "education.curriculum-item.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertCurriculumItemRequest"},
	{Code: "education.curriculum-version.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertCurriculumVersionRequest"},
	{Code: "education.curriculum-version.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertCurriculumVersionRequest"},
	{Code: "education.dormitory-stay.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.education.UpsertDormitoryStayRequest"},
	{Code: "education.dormitory-stay.delete", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage"},
	{Code: "education.dormitory-stay.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.education.UpsertDormitoryStayRequest"},
	{Code: "education.enrollment.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.enrollment.manage", RequestType: "oikumenea.education.UpsertEnrollmentRequest"},
	{Code: "education.enrollment.delete", Service: rid.SvcEducation, TargetType: "education", Permission: "education.enrollment.manage"},
	{Code: "education.enrollment.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.enrollment.manage", RequestType: "oikumenea.education.UpsertEnrollmentRequest"},
	{Code: "education.governance-body.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertGovernanceBodyRequest"},
	{Code: "education.governance-body.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertGovernanceBodyRequest"},
	{Code: "education.governance-membership.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertGovernanceMembershipRequest"},
	{Code: "education.governance-membership.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertGovernanceMembershipRequest"},
	{Code: "education.grant-holding.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertGrantHoldingRequest"},
	{Code: "education.grant-holding.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertGrantHoldingRequest"},
	{Code: "education.grant.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertGrantRequest"},
	{Code: "education.grant.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertGrantRequest"},
	{Code: "education.group.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.education.CreateGroupRequest"},
	{Code: "education.group.delete", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage"},
	{Code: "education.group.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.education.UpdateGroupRequest"},
	{Code: "education.institution-kind.upsert", Service: rid.SvcEducation, TargetType: "education", Permission: "education.catalog.manage", RequestType: "oikumenea.education.UpsertCatalogKindRequest"},
	{Code: "education.institution.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.education.CreateInstitutionRequest"},
	{Code: "education.institution.delete", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage"},
	{Code: "education.institution.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.education.UpdateInstitutionRequest"},
	{Code: "education.policy.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertPolicyRequest"},
	{Code: "education.policy.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertPolicyRequest"},
	{Code: "education.position.abolish", Service: rid.SvcEducation, TargetType: "education", Permission: "education.position.manage"},
	{Code: "education.position.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.position.manage", RequestType: "oikumenea.education.CreatePositionRequest"},
	{Code: "education.position.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.position.manage", RequestType: "oikumenea.education.UpdatePositionRequest"},
	{Code: "education.program.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertProgramRequest"},
	{Code: "education.program.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertProgramRequest"},
	{Code: "education.publication-authorship.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertPublicationAuthorshipRequest"},
	{Code: "education.publication-authorship.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertPublicationAuthorshipRequest"},
	{Code: "education.publication.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertPublicationRequest"},
	{Code: "education.publication.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertPublicationRequest"},
	{Code: "education.qualification-award.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertQualificationAwardRequest"},
	{Code: "education.qualification-award.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertQualificationAwardRequest"},
	{Code: "education.qualification.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertQualificationRequest"},
	{Code: "education.qualification.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertQualificationRequest"},
	{Code: "education.research-centre.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertResearchCentreRequest"},
	{Code: "education.research-centre.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertResearchCentreRequest"},
	{Code: "education.research-group.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertResearchGroupRequest"},
	{Code: "education.research-group.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertResearchGroupRequest"},
	{Code: "education.research-membership.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertResearchMembershipRequest"},
	{Code: "education.research-membership.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertResearchMembershipRequest"},
	{Code: "education.scholarship-award.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertScholarshipAwardRequest"},
	{Code: "education.scholarship-award.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertScholarshipAwardRequest"},
	{Code: "education.scholarship.create", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertScholarshipRequest"},
	{Code: "education.scholarship.update", Service: rid.SvcEducation, TargetType: "education", Permission: "education.manage", RequestType: "oikumenea.educationref.UpsertScholarshipRequest"},
	{Code: "company.appointment.end", Service: rid.SvcCompany, TargetType: "company", Permission: "company.position.manage", RequestType: "oikumenea.company.EndAppointmentRequest"},
	{Code: "company.appointment.fill", Service: rid.SvcCompany, TargetType: "company", Permission: "company.position.manage", RequestType: "oikumenea.company.FillPositionRequest"},
	{Code: "company.beneficiary.record", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.RecordBeneficiaryRequest"},
	{Code: "company.branch.record", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.RecordBranchRequest"},
	{Code: "company.create", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.CreateCompanyRequest"},
	{Code: "company.delete", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage"},
	{Code: "company.founding.record", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.RecordFoundingRequest"},
	{Code: "company.industry-class.upsert", Service: rid.SvcCompany, TargetType: "company", Permission: "company.catalog.manage", RequestType: "oikumenea.company.UpsertIndustryClassRequest"},
	{Code: "company.industry.assign", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.AssignIndustryRequest"},
	{Code: "company.legal-form.upsert", Service: rid.SvcCompany, TargetType: "company", Permission: "company.catalog.manage", RequestType: "oikumenea.company.UpsertLegalFormRequest"},
	{Code: "company.location.add", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.AddCompanyLocationRequest"},
	{Code: "company.position.abolish", Service: rid.SvcCompany, TargetType: "company", Permission: "company.position.manage"},
	{Code: "company.position.create", Service: rid.SvcCompany, TargetType: "company", Permission: "company.position.manage", RequestType: "oikumenea.company.CreatePositionRequest"},
	{Code: "company.position.update", Service: rid.SvcCompany, TargetType: "company", Permission: "company.position.manage", RequestType: "oikumenea.company.UpdatePositionRequest"},
	{Code: "company.registration-scheme.upsert", Service: rid.SvcCompany, TargetType: "company", Permission: "company.catalog.manage", RequestType: "oikumenea.company.UpsertSchemeRequest"},
	{Code: "company.registration.add", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.UpsertRegistrationRequest"},
	{Code: "company.registration.update", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.UpsertRegistrationRequest"},
	{Code: "company.shareholding.record", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.RecordShareholdingRequest"},
	{Code: "company.succession.record", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.RecordSuccessionRequest"},
	{Code: "company.update", Service: rid.SvcCompany, TargetType: "company", Permission: "company.manage", RequestType: "oikumenea.company.UpdateCompanyRequest"},
	{Code: "religion.affiliation-type.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "affiliation.manage", RequestType: "oikumenea.religion.UpsertAffiliationTypeRequest"},
	{Code: "religion.affiliation.add", Service: rid.SvcReligion, TargetType: "religion", Permission: "affiliation.manage", RequestType: "oikumenea.religion.AddAffiliationRequest"},
	{Code: "religion.affiliation.delete", Service: rid.SvcReligion, TargetType: "religion", Permission: "affiliation.manage"},
	{Code: "religion.affiliation.erase", Service: rid.SvcReligion, TargetType: "religion", Permission: "affiliation.manage"},
	{Code: "religion.affiliation.update", Service: rid.SvcReligion, TargetType: "religion", Permission: "affiliation.manage", RequestType: "oikumenea.religion.UpdateAffiliationRequest"},
	{Code: "religion.alias.add", Service: rid.SvcReligion, TargetType: "religion", Permission: "site.manage", RequestType: "oikumenea.religion.CreateAliasRequest"},
	{Code: "religion.alias.delete", Service: rid.SvcReligion, TargetType: "religion", Permission: "site.manage"},
	{Code: "religion.classification.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.UpsertClassificationRequest"},
	{Code: "religion.clergy-credential.add", Service: rid.SvcReligion, TargetType: "religion", Permission: "clergy.manage", RequestType: "oikumenea.religion.AddClergyCredentialRequest"},
	{Code: "religion.clergy-credential.update", Service: rid.SvcReligion, TargetType: "religion", Permission: "clergy.manage", RequestType: "oikumenea.religion.UpdateClergyCredentialRequest"},
	{Code: "religion.clergy-grade.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "clergy.manage", RequestType: "oikumenea.religion.UpsertClergyGradeRequest"},
	{Code: "religion.grade-category.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.UpsertGradeCategoryRequest"},
	{Code: "religion.office-type.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.UpsertOfficeTypeRequest"},
	{Code: "religion.org-classification.add", Service: rid.SvcReligion, TargetType: "religion", Permission: "religionorg.manage", RequestType: "oikumenea.religion.AddOrgClassificationRequest"},
	{Code: "religion.org-classification.remove", Service: rid.SvcReligion, TargetType: "religion", Permission: "religionorg.manage"},
	{Code: "religion.org-kind.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.UpsertOrgKindRequest"},
	{Code: "religion.org-policy.add", Service: rid.SvcReligion, TargetType: "religion", Permission: "religionorg.manage", RequestType: "oikumenea.religion.AddOrgPolicyRequest"},
	{Code: "religion.org-policy.remove", Service: rid.SvcReligion, TargetType: "religion", Permission: "religionorg.manage"},
	{Code: "religion.org-profile.set", Service: rid.SvcReligion, TargetType: "religion", Permission: "religionorg.manage", RequestType: "oikumenea.religion.SetOrgProfileRequest"},
	{Code: "religion.policy-kind.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.UpsertPolicyKindRequest"},
	{Code: "religion.schedule.add", Service: rid.SvcReligion, TargetType: "religion", Permission: "schedule.manage", RequestType: "oikumenea.religion.CreateScheduleRequest"},
	{Code: "religion.schedule.delete", Service: rid.SvcReligion, TargetType: "religion", Permission: "schedule.manage"},
	{Code: "religion.service-type.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.UpsertServiceTypeRequest"},
	{Code: "religion.site-type.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "site.manage", RequestType: "oikumenea.religion.UpsertSiteTypeRequest"},
	{Code: "religion.site.add", Service: rid.SvcReligion, TargetType: "religion", Permission: "site.manage", RequestType: "oikumenea.religion.CreateSiteRequest"},
	{Code: "religion.site.delete", Service: rid.SvcReligion, TargetType: "religion", Permission: "site.manage"},
	{Code: "religion.site.update", Service: rid.SvcReligion, TargetType: "religion", Permission: "site.manage", RequestType: "oikumenea.religion.UpdateSiteRequest"},
	{Code: "religion.taxon-rank.upsert", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.UpsertTaxonRankRequest"},
	{Code: "religion.taxon.create", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.CreateTaxonRequest"},
	{Code: "religion.taxon.delete", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage"},
	{Code: "religion.taxon.reparent", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.ReparentTaxonRequest"},
	{Code: "religion.taxon.set-classifications", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.SetClassificationsRequest"},
	{Code: "religion.taxon.update", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage", RequestType: "oikumenea.religion.UpdateTaxonRequest"},
	{Code: "religion.taxonomy.rebuild-closure", Service: rid.SvcReligion, TargetType: "religion", Permission: "religion.catalog.manage"},
	{Code: "religion.unit-type-override.set", Service: rid.SvcReligion, TargetType: "religion", Permission: "religionorg.manage", RequestType: "oikumenea.religion.SetClassificationsRequest"},
	{Code: "vehicle.brand.upsert", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.catalog.manage", RequestType: "oikumenea.vehicle.UpsertBrandRequest"},
	{Code: "vehicle.create", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.manage", RequestType: "oikumenea.vehicle.CreateVehicleRequest"},
	{Code: "vehicle.delete", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.manage"},
	{Code: "vehicle.manufacturer.add", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.catalog.manage", RequestType: "oikumenea.vehicle.AddManufacturerRequest"},
	{Code: "vehicle.manufacturer.remove", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.catalog.manage"},
	{Code: "vehicle.model.upsert", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.catalog.manage", RequestType: "oikumenea.vehicle.UpsertModelRequest"},
	{Code: "vehicle.number-type.upsert", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.catalog.manage", RequestType: "oikumenea.vehicle.UpsertNumberTypeRequest"},
	{Code: "vehicle.register", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.manage", RequestType: "oikumenea.vehicle.RegisterVehicleRequest"},
	{Code: "vehicle.registration.close", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.manage"},
	{Code: "vehicle.registrations.erase", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.manage"},
	{Code: "vehicle.type.upsert", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.catalog.manage", RequestType: "oikumenea.vehicle.UpsertVehicleTypeRequest"},
	{Code: "vehicle.update", Service: rid.SvcVehicle, TargetType: "vehicle", Permission: "vehicle.manage", RequestType: "oikumenea.vehicle.UpdateVehicleRequest"},
	{Code: "external-org.create", Service: rid.SvcExternalOrg, TargetType: "external_organization", Permission: "externalorg.manage", RequestType: "oikumenea.externalorg.CreateExternalOrgRequest"},
	{Code: "external-org.delete", Service: rid.SvcExternalOrg, TargetType: "external_organization", Permission: "externalorg.manage"},
	{Code: "external-org.kind.upsert", Service: rid.SvcExternalOrg, TargetType: "external_organization", Permission: "externalorg.manage", RequestType: "oikumenea.externalorg.UpsertExternalOrgKindRequest"},
	{Code: "external-org.merge", Service: rid.SvcExternalOrg, TargetType: "external_organization", Permission: "externalorg.manage", RequestType: "oikumenea.externalorg.MergeExternalOrgRequest"},
	{Code: "external-org.update", Service: rid.SvcExternalOrg, TargetType: "external_organization", Permission: "externalorg.manage", RequestType: "oikumenea.externalorg.UpdateExternalOrgRequest"},
	{Code: "finance.account-type.upsert", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.catalog.manage", RequestType: "oikumenea.finance.UpsertAccountTypeRequest"},
	{Code: "finance.account.create", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.manage", RequestType: "oikumenea.finance.CreateAccountRequest"},
	{Code: "finance.account.delete", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.manage"},
	{Code: "finance.account.update", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.manage", RequestType: "oikumenea.finance.UpdateAccountRequest"},
	{Code: "finance.card-network.upsert", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.catalog.manage", RequestType: "oikumenea.finance.UpsertCardNetworkRequest"},
	{Code: "finance.card.add", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.manage", RequestType: "oikumenea.finance.AddCardRequest"},
	{Code: "finance.card.delete", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.manage"},
	{Code: "finance.card.update", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.manage", RequestType: "oikumenea.finance.UpdateCardRequest"},
	{Code: "finance.holder.add", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.manage", RequestType: "oikumenea.finance.AddAccountHolderRequest"},
	{Code: "finance.holder.end", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.manage"},
	{Code: "finance.holdings.erase", Service: rid.SvcFinance, TargetType: "finance", Permission: "finance.manage"}}

var byCode = func() map[string]ActionType {
	m := make(map[string]ActionType, len(actionTypes))
	for _, a := range actionTypes {
		m[a.Code] = a
	}
	return m
}()

// IsRegistered reports whether code is a known action type (the audit write-time gate).
func IsRegistered(code string) bool { _, ok := byCode[code]; return ok }

// Lookup returns the action type for code, or (zero, false).
func Lookup(code string) (ActionType, bool) { a, ok := byCode[code]; return a, ok }

// All returns the catalog sorted by (service, code) — a defensive copy.
func All() []ActionType {
	out := make([]ActionType, len(actionTypes))
	copy(out, actionTypes)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// Count is the number of registered action types.
func Count() int { return len(actionTypes) }
