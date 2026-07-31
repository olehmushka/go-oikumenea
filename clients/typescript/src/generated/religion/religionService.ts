import { IAddAffiliationRequest } from "./addAffiliationRequest";
import { IAddClergyCredentialRequest } from "./addClergyCredentialRequest";
import { IAddOrgClassificationRequest } from "./addOrgClassificationRequest";
import { IAddOrgPolicyRequest } from "./addOrgPolicyRequest";
import { IAffiliation } from "./affiliation";
import { IAffiliationList } from "./affiliationList";
import { IAffiliationType } from "./affiliationType";
import { IAffiliationTypeList } from "./affiliationTypeList";
import { IAlias } from "./alias";
import { IAliasList } from "./aliasList";
import { IClassification } from "./classification";
import { IClassificationList } from "./classificationList";
import { IClergyCredential } from "./clergyCredential";
import { IClergyCredentialList } from "./clergyCredentialList";
import { IClergyGrade } from "./clergyGrade";
import { IClergyGradeList } from "./clergyGradeList";
import { IClosureReport } from "./closureReport";
import { ICreateAliasRequest } from "./createAliasRequest";
import { ICreateChildOrgRequest } from "./createChildOrgRequest";
import { ICreateRootOrgRequest } from "./createRootOrgRequest";
import { ICreateScheduleRequest } from "./createScheduleRequest";
import { ICreateSiteRequest } from "./createSiteRequest";
import { ICreateTaxonRequest } from "./createTaxonRequest";
import { IDiscoverySitePage } from "./discoverySitePage";
import { IEffectiveType } from "./effectiveType";
import { IGradeCategory } from "./gradeCategory";
import { IGradeCategoryList } from "./gradeCategoryList";
import { IOfficeType } from "./officeType";
import { IOfficeTypeList } from "./officeTypeList";
import { IOrgClassification } from "./orgClassification";
import { IOrgKind } from "./orgKind";
import { IOrgKindList } from "./orgKindList";
import { IOrgPolicy } from "./orgPolicy";
import { IOrgPolicyList } from "./orgPolicyList";
import { IOrgProfile } from "./orgProfile";
import { IPolicyKind } from "./policyKind";
import { IPolicyKindList } from "./policyKindList";
import { IReparentTaxonRequest } from "./reparentTaxonRequest";
import { IServiceSchedule } from "./serviceSchedule";
import { IServiceScheduleList } from "./serviceScheduleList";
import { IServiceType } from "./serviceType";
import { IServiceTypeList } from "./serviceTypeList";
import { ISetClassificationsRequest } from "./setClassificationsRequest";
import { ISetOrgProfileRequest } from "./setOrgProfileRequest";
import { ISite } from "./site";
import { ISiteList } from "./siteList";
import { ISiteType } from "./siteType";
import { ISiteTypeList } from "./siteTypeList";
import { ITaxon } from "./taxon";
import { ITaxonPage } from "./taxonPage";
import { ITaxonRank } from "./taxonRank";
import { ITaxonRankList } from "./taxonRankList";
import { ITaxonStats } from "./taxonStats";
import { IUpdateAffiliationRequest } from "./updateAffiliationRequest";
import { IUpdateClergyCredentialRequest } from "./updateClergyCredentialRequest";
import { IUpdateSiteRequest } from "./updateSiteRequest";
import { IUpdateTaxonRequest } from "./updateTaxonRequest";
import { IUpsertAffiliationTypeRequest } from "./upsertAffiliationTypeRequest";
import { IUpsertClassificationRequest } from "./upsertClassificationRequest";
import { IUpsertClergyGradeRequest } from "./upsertClergyGradeRequest";
import { IUpsertGradeCategoryRequest } from "./upsertGradeCategoryRequest";
import { IUpsertOfficeTypeRequest } from "./upsertOfficeTypeRequest";
import { IUpsertOrgKindRequest } from "./upsertOrgKindRequest";
import { IUpsertPolicyKindRequest } from "./upsertPolicyKindRequest";
import { IUpsertServiceTypeRequest } from "./upsertServiceTypeRequest";
import { IUpsertSiteTypeRequest } from "./upsertSiteTypeRequest";
import { IUpsertTaxonRankRequest } from "./upsertTaxonRankRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The multi-faith taxonomy (recursive religion_taxa + closure) with a catalog-driven level marker,
 * the religion-type classifications, the per-faith catalogs, and the per-unit organization attributes.
 * Reads gate on `religion.read`; taxonomy/catalog writes on `religion.catalog.manage` (instance);
 * per-unit organization writes on `religionorg.manage` (checked over the canonical graph). Writes are
 * audited in-process (D-Audit).
 *
 */
export interface IReligionService {
    listTaxonRanks(): Promise<ITaxonRankList>;
    upsertTaxonRank(request: IUpsertTaxonRankRequest): Promise<ITaxonRank>;
    listClassifications(): Promise<IClassificationList>;
    upsertClassification(request: IUpsertClassificationRequest): Promise<IClassification>;
    listOrgKinds(): Promise<IOrgKindList>;
    upsertOrgKind(request: IUpsertOrgKindRequest): Promise<IOrgKind>;
    listPolicyKinds(): Promise<IPolicyKindList>;
    upsertPolicyKind(request: IUpsertPolicyKindRequest): Promise<IPolicyKind>;
    /** Search/filter the taxonomy. Filters compose; results carry closure depth where a parent/root is given. */
    listTaxa(rank?: string | null, parent?: string | null, religion?: string | null, classification?: string | null, query?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ITaxonPage>;
    /**
     * Facet distributions over the taxonomy — the dashboard half of the facet vocabulary (M58 /
     * D-ObjectFacets). Takes exactly the filter args `listTaxa` takes (minus paging) plus an
     * optional `facets` CSV, so a dashboard and a list are two renderings of ONE request state
     * and a chart segment is a link to the same URL with one more filter applied.
     *
     * `totalCount` equals the number of rows exhaustively paging `listTaxa` with these same
     * filters would return. One round-trip serves the whole dashboard.
     *
     * ONE aggregate arm, with no subject and no scoped twin — but for the OPPOSITE reason to the
     * audit ledger's single arm. The taxonomy is flat instance-global reference data with no
     * row-level security and no unit reach (the RLS in this module is on the unit-scoped
     * `religion_org_*` tables, not here), so `religion.read` held anywhere is the whole
     * visibility decision and there is nothing for a second arm to narrow.
     *
     * `subtree` and `classification` do not partition the result set — see TaxonStats.
     *
     * The path is `/stats/taxa` rather than `/taxa/stats` because the server's router rejects a
     * literal path segment that is a sibling of `{taxonId}`.
     *
     */
    taxonStats(facets?: string | null, rank?: string | null, parent?: string | null, religion?: string | null, classification?: string | null, query?: string | null): Promise<ITaxonStats>;
    createTaxon(request: ICreateTaxonRequest): Promise<ITaxon>;
    getTaxon(taxonId: string): Promise<ITaxon>;
    updateTaxon(taxonId: string, request: IUpdateTaxonRequest): Promise<ITaxon>;
    deleteTaxon(taxonId: string): Promise<void>;
    /** Move a taxon under a new parent, recomputing the closure. Returns Religion:TaxonCycleDetected on a cycle. */
    reparentTaxon(taxonId: string, request: IReparentTaxonRequest): Promise<ITaxon>;
    /**
     * Recompute the whole-tree taxonomy closure (admin maintenance). Pathed under /taxonomy (not
     * /taxa) so the static segment does not collide with the /taxa/{taxonId} wildcard route.
     *
     */
    rebuildClosure(): Promise<IClosureReport>;
    /** The taxon's resolved theism classifications (nearest declaring ancestor wins). */
    getEffectiveClassifications(taxonId: string): Promise<IClassificationList>;
    /** Replace the theism tags declared directly on this taxon. */
    setTaxonClassifications(taxonId: string, request: ISetClassificationsRequest): Promise<IClassificationList>;
    getOrgProfile(unitId: string): Promise<IOrgProfile>;
    setOrgProfile(unitId: string, request: ISetOrgProfileRequest): Promise<IOrgProfile>;
    addOrgClassification(unitId: string, request: IAddOrgClassificationRequest): Promise<IOrgClassification>;
    removeOrgClassification(unitId: string, linkId: string): Promise<void>;
    /** Replace the unit's theism override set (empty list clears it, restoring inheritance). */
    setUnitTypeOverride(unitId: string, request: ISetClassificationsRequest): Promise<IClassificationList>;
    getEffectiveType(unitId: string): Promise<IEffectiveType>;
    listOrgPolicies(unitId: string): Promise<IOrgPolicyList>;
    addOrgPolicy(unitId: string, request: IAddOrgPolicyRequest): Promise<IOrgPolicy>;
    removeOrgPolicy(unitId: string, policyId: string): Promise<void>;
    /**
     * Create a child religious-body unit under this parent in the canonical graph. Rejected with
     * Religion:ChildCreationExcluded if the parent carries an active excludes_child_creation policy.
     *
     */
    createChildOrg(unitId: string, request: ICreateChildOrgRequest): Promise<IOrgProfile>;
    /**
     * Create a top-level religious body (M41 / D-UnifiedOrgGraph): a `church`-domain tenant
     * organization + its root religious-body unit + profile. Instance-level (religion.catalog.manage).
     *
     */
    createRootOrg(request: ICreateRootOrgRequest): Promise<IOrgProfile>;
    listGradeCategories(): Promise<IGradeCategoryList>;
    upsertGradeCategory(request: IUpsertGradeCategoryRequest): Promise<IGradeCategory>;
    /** List clergy grades, optionally filtered to a tradition taxon (RID or code), ordered by tradition then ordinal. */
    listClergyGrades(tradition?: string | null): Promise<IClergyGradeList>;
    upsertClergyGrade(request: IUpsertClergyGradeRequest): Promise<IClergyGrade>;
    listOfficeTypes(): Promise<IOfficeTypeList>;
    upsertOfficeType(request: IUpsertOfficeTypeRequest): Promise<IOfficeType>;
    listPersonClergyCredentials(personId: string): Promise<IClergyCredentialList>;
    /** The clergy roster of an organization unit (people holding a credential conferred by it). */
    listUnitClergyCredentials(unitId: string): Promise<IClergyCredentialList>;
    addClergyCredential(personId: string, request: IAddClergyCredentialRequest): Promise<IClergyCredential>;
    /** Flip a credential's status (e.g. suspend/revoke) and/or set effective-dating. Never deletes. */
    updateClergyCredential(credentialId: string, request: IUpdateClergyCredentialRequest): Promise<IClergyCredential>;
    listAffiliationTypes(): Promise<IAffiliationTypeList>;
    upsertAffiliationType(request: IUpsertAffiliationTypeRequest): Promise<IAffiliationType>;
    /** List a person's lay religious affiliations. pii:special — gated on affiliation.manage; the belief value is decrypted. */
    listPersonAffiliations(personId: string): Promise<IAffiliationList>;
    addAffiliation(personId: string, request: IAddAffiliationRequest): Promise<IAffiliation>;
    updateAffiliation(affiliationId: string, request: IUpdateAffiliationRequest): Promise<IAffiliation>;
    deleteAffiliation(affiliationId: string): Promise<void>;
    listSiteTypes(): Promise<ISiteTypeList>;
    upsertSiteType(request: IUpsertSiteTypeRequest): Promise<ISiteType>;
    listServiceTypes(): Promise<IServiceTypeList>;
    upsertServiceType(request: IUpsertServiceTypeRequest): Promise<IServiceType>;
    /** List an org unit's sites (exact coordinates, for authorized owners). */
    listUnitSites(unitId: string): Promise<ISiteList>;
    createSite(unitId: string, request: ICreateSiteRequest): Promise<ISite>;
    updateSite(siteId: string, request: IUpdateSiteRequest): Promise<ISite>;
    deleteSite(siteId: string): Promise<void>;
    listSiteSchedules(siteId: string): Promise<IServiceScheduleList>;
    createSchedule(siteId: string, request: ICreateScheduleRequest): Promise<IServiceSchedule>;
    deleteSchedule(scheduleId: string): Promise<void>;
    listUnitAliases(unitId: string): Promise<IAliasList>;
    createAlias(unitId: string, request: ICreateAliasRequest): Promise<IAlias>;
    deleteAlias(unitId: string, aliasId: string): Promise<void>;
    /**
     * Closure-aware PostGIS discovery search over PUBLIC sites. Supply a radius (lat+lng+radiusM via
     * ST_DWithin) or a bounding box (minLat/minLng/maxLat/maxLng via ST_Intersects); optionally filter
     * by a religion taxon (org units classified under it via the taxonomy closure), a service language/
     * weekday/online toggle, and a fuzzy match on the unit code/name or an alias. Coordinates are
     * coarsened per each site's publicPrecision.
     *
     */
    searchSites(lat?: number | "NaN" | null, lng?: number | "NaN" | null, radiusM?: number | "NaN" | null, minLat?: number | "NaN" | null, minLng?: number | "NaN" | null, maxLat?: number | "NaN" | null, maxLng?: number | "NaN" | null, religion?: string | null, language?: string | null, dayOfWeek?: number | null, onlineOnly?: boolean | null, query?: string | null, pageSize?: number | null): Promise<IDiscoverySitePage>;
}

export class ReligionService implements IReligionService {
    constructor(private bridge: IHttpApiBridge) {
    }

    public listTaxonRanks(): Promise<ITaxonRankList> {
        return this.bridge.call<ITaxonRankList>(
            "ReligionService",
            "listTaxonRanks",
            "GET",
            "/religion/v1/taxon-ranks",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertTaxonRank(request: IUpsertTaxonRankRequest): Promise<ITaxonRank> {
        return this.bridge.call<ITaxonRank>(
            "ReligionService",
            "upsertTaxonRank",
            "PUT",
            "/religion/v1/taxon-ranks",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listClassifications(): Promise<IClassificationList> {
        return this.bridge.call<IClassificationList>(
            "ReligionService",
            "listClassifications",
            "GET",
            "/religion/v1/classifications",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertClassification(request: IUpsertClassificationRequest): Promise<IClassification> {
        return this.bridge.call<IClassification>(
            "ReligionService",
            "upsertClassification",
            "PUT",
            "/religion/v1/classifications",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listOrgKinds(): Promise<IOrgKindList> {
        return this.bridge.call<IOrgKindList>(
            "ReligionService",
            "listOrgKinds",
            "GET",
            "/religion/v1/org-kinds",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertOrgKind(request: IUpsertOrgKindRequest): Promise<IOrgKind> {
        return this.bridge.call<IOrgKind>(
            "ReligionService",
            "upsertOrgKind",
            "PUT",
            "/religion/v1/org-kinds",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listPolicyKinds(): Promise<IPolicyKindList> {
        return this.bridge.call<IPolicyKindList>(
            "ReligionService",
            "listPolicyKinds",
            "GET",
            "/religion/v1/policy-kinds",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertPolicyKind(request: IUpsertPolicyKindRequest): Promise<IPolicyKind> {
        return this.bridge.call<IPolicyKind>(
            "ReligionService",
            "upsertPolicyKind",
            "PUT",
            "/religion/v1/policy-kinds",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Search/filter the taxonomy. Filters compose; results carry closure depth where a parent/root is given. */
    public listTaxa(rank?: string | null, parent?: string | null, religion?: string | null, classification?: string | null, query?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ITaxonPage> {
        return this.bridge.call<ITaxonPage>(
            "ReligionService",
            "listTaxa",
            "GET",
            "/religion/v1/taxa",
            __undefined,
            __undefined,
            {
                "rank": rank,
                "parent": parent,
                "religion": religion,
                "classification": classification,
                "query": query,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the taxonomy — the dashboard half of the facet vocabulary (M58 /
     * D-ObjectFacets). Takes exactly the filter args `listTaxa` takes (minus paging) plus an
     * optional `facets` CSV, so a dashboard and a list are two renderings of ONE request state
     * and a chart segment is a link to the same URL with one more filter applied.
     *
     * `totalCount` equals the number of rows exhaustively paging `listTaxa` with these same
     * filters would return. One round-trip serves the whole dashboard.
     *
     * ONE aggregate arm, with no subject and no scoped twin — but for the OPPOSITE reason to the
     * audit ledger's single arm. The taxonomy is flat instance-global reference data with no
     * row-level security and no unit reach (the RLS in this module is on the unit-scoped
     * `religion_org_*` tables, not here), so `religion.read` held anywhere is the whole
     * visibility decision and there is nothing for a second arm to narrow.
     *
     * `subtree` and `classification` do not partition the result set — see TaxonStats.
     *
     * The path is `/stats/taxa` rather than `/taxa/stats` because the server's router rejects a
     * literal path segment that is a sibling of `{taxonId}`.
     *
     */
    public taxonStats(facets?: string | null, rank?: string | null, parent?: string | null, religion?: string | null, classification?: string | null, query?: string | null): Promise<ITaxonStats> {
        return this.bridge.call<ITaxonStats>(
            "ReligionService",
            "taxonStats",
            "GET",
            "/religion/v1/stats/taxa",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "rank": rank,
                "parent": parent,
                "religion": religion,
                "classification": classification,
                "query": query,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public createTaxon(request: ICreateTaxonRequest): Promise<ITaxon> {
        return this.bridge.call<ITaxon>(
            "ReligionService",
            "createTaxon",
            "POST",
            "/religion/v1/taxa",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public getTaxon(taxonId: string): Promise<ITaxon> {
        return this.bridge.call<ITaxon>(
            "ReligionService",
            "getTaxon",
            "GET",
            "/religion/v1/taxa/{taxonId}",
            __undefined,
            __undefined,
            __undefined,
            [
                taxonId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateTaxon(taxonId: string, request: IUpdateTaxonRequest): Promise<ITaxon> {
        return this.bridge.call<ITaxon>(
            "ReligionService",
            "updateTaxon",
            "PUT",
            "/religion/v1/taxa/{taxonId}",
            request,
            __undefined,
            __undefined,
            [
                taxonId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteTaxon(taxonId: string): Promise<void> {
        return this.bridge.call<void>(
            "ReligionService",
            "deleteTaxon",
            "DELETE",
            "/religion/v1/taxa/{taxonId}",
            __undefined,
            __undefined,
            __undefined,
            [
                taxonId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Move a taxon under a new parent, recomputing the closure. Returns Religion:TaxonCycleDetected on a cycle. */
    public reparentTaxon(taxonId: string, request: IReparentTaxonRequest): Promise<ITaxon> {
        return this.bridge.call<ITaxon>(
            "ReligionService",
            "reparentTaxon",
            "POST",
            "/religion/v1/taxa/{taxonId}/reparent",
            request,
            __undefined,
            __undefined,
            [
                taxonId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Recompute the whole-tree taxonomy closure (admin maintenance). Pathed under /taxonomy (not
     * /taxa) so the static segment does not collide with the /taxa/{taxonId} wildcard route.
     *
     */
    public rebuildClosure(): Promise<IClosureReport> {
        return this.bridge.call<IClosureReport>(
            "ReligionService",
            "rebuildClosure",
            "POST",
            "/religion/v1/taxonomy/rebuild-closure",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** The taxon's resolved theism classifications (nearest declaring ancestor wins). */
    public getEffectiveClassifications(taxonId: string): Promise<IClassificationList> {
        return this.bridge.call<IClassificationList>(
            "ReligionService",
            "getEffectiveClassifications",
            "GET",
            "/religion/v1/taxa/{taxonId}/effective-classifications",
            __undefined,
            __undefined,
            __undefined,
            [
                taxonId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Replace the theism tags declared directly on this taxon. */
    public setTaxonClassifications(taxonId: string, request: ISetClassificationsRequest): Promise<IClassificationList> {
        return this.bridge.call<IClassificationList>(
            "ReligionService",
            "setTaxonClassifications",
            "PUT",
            "/religion/v1/taxa/{taxonId}/classifications",
            request,
            __undefined,
            __undefined,
            [
                taxonId,
            ],
            __undefined,
            __undefined
        );
    }

    public getOrgProfile(unitId: string): Promise<IOrgProfile> {
        return this.bridge.call<IOrgProfile>(
            "ReligionService",
            "getOrgProfile",
            "GET",
            "/religion/v1/units/{unitId}/religion-profile",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public setOrgProfile(unitId: string, request: ISetOrgProfileRequest): Promise<IOrgProfile> {
        return this.bridge.call<IOrgProfile>(
            "ReligionService",
            "setOrgProfile",
            "PUT",
            "/religion/v1/units/{unitId}/religion-profile",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public addOrgClassification(unitId: string, request: IAddOrgClassificationRequest): Promise<IOrgClassification> {
        return this.bridge.call<IOrgClassification>(
            "ReligionService",
            "addOrgClassification",
            "POST",
            "/religion/v1/units/{unitId}/classifications",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeOrgClassification(unitId: string, linkId: string): Promise<void> {
        return this.bridge.call<void>(
            "ReligionService",
            "removeOrgClassification",
            "DELETE",
            "/religion/v1/units/{unitId}/classifications/{linkId}",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Replace the unit's theism override set (empty list clears it, restoring inheritance). */
    public setUnitTypeOverride(unitId: string, request: ISetClassificationsRequest): Promise<IClassificationList> {
        return this.bridge.call<IClassificationList>(
            "ReligionService",
            "setUnitTypeOverride",
            "PUT",
            "/religion/v1/units/{unitId}/type-overrides",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public getEffectiveType(unitId: string): Promise<IEffectiveType> {
        return this.bridge.call<IEffectiveType>(
            "ReligionService",
            "getEffectiveType",
            "GET",
            "/religion/v1/units/{unitId}/effective-type",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public listOrgPolicies(unitId: string): Promise<IOrgPolicyList> {
        return this.bridge.call<IOrgPolicyList>(
            "ReligionService",
            "listOrgPolicies",
            "GET",
            "/religion/v1/units/{unitId}/religion-policies",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public addOrgPolicy(unitId: string, request: IAddOrgPolicyRequest): Promise<IOrgPolicy> {
        return this.bridge.call<IOrgPolicy>(
            "ReligionService",
            "addOrgPolicy",
            "POST",
            "/religion/v1/units/{unitId}/religion-policies",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeOrgPolicy(unitId: string, policyId: string): Promise<void> {
        return this.bridge.call<void>(
            "ReligionService",
            "removeOrgPolicy",
            "DELETE",
            "/religion/v1/units/{unitId}/religion-policies/{policyId}",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
                policyId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Create a child religious-body unit under this parent in the canonical graph. Rejected with
     * Religion:ChildCreationExcluded if the parent carries an active excludes_child_creation policy.
     *
     */
    public createChildOrg(unitId: string, request: ICreateChildOrgRequest): Promise<IOrgProfile> {
        return this.bridge.call<IOrgProfile>(
            "ReligionService",
            "createChildOrg",
            "POST",
            "/religion/v1/units/{unitId}/child-orgs",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Create a top-level religious body (M41 / D-UnifiedOrgGraph): a `church`-domain tenant
     * organization + its root religious-body unit + profile. Instance-level (religion.catalog.manage).
     *
     */
    public createRootOrg(request: ICreateRootOrgRequest): Promise<IOrgProfile> {
        return this.bridge.call<IOrgProfile>(
            "ReligionService",
            "createRootOrg",
            "POST",
            "/religion/v1/religion-orgs",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listGradeCategories(): Promise<IGradeCategoryList> {
        return this.bridge.call<IGradeCategoryList>(
            "ReligionService",
            "listGradeCategories",
            "GET",
            "/religion/v1/grade-categories",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertGradeCategory(request: IUpsertGradeCategoryRequest): Promise<IGradeCategory> {
        return this.bridge.call<IGradeCategory>(
            "ReligionService",
            "upsertGradeCategory",
            "PUT",
            "/religion/v1/grade-categories",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List clergy grades, optionally filtered to a tradition taxon (RID or code), ordered by tradition then ordinal. */
    public listClergyGrades(tradition?: string | null): Promise<IClergyGradeList> {
        return this.bridge.call<IClergyGradeList>(
            "ReligionService",
            "listClergyGrades",
            "GET",
            "/religion/v1/clergy-grades",
            __undefined,
            __undefined,
            {
                "tradition": tradition,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertClergyGrade(request: IUpsertClergyGradeRequest): Promise<IClergyGrade> {
        return this.bridge.call<IClergyGrade>(
            "ReligionService",
            "upsertClergyGrade",
            "PUT",
            "/religion/v1/clergy-grades",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listOfficeTypes(): Promise<IOfficeTypeList> {
        return this.bridge.call<IOfficeTypeList>(
            "ReligionService",
            "listOfficeTypes",
            "GET",
            "/religion/v1/office-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertOfficeType(request: IUpsertOfficeTypeRequest): Promise<IOfficeType> {
        return this.bridge.call<IOfficeType>(
            "ReligionService",
            "upsertOfficeType",
            "PUT",
            "/religion/v1/office-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listPersonClergyCredentials(personId: string): Promise<IClergyCredentialList> {
        return this.bridge.call<IClergyCredentialList>(
            "ReligionService",
            "listPersonClergyCredentials",
            "GET",
            "/religion/v1/persons/{personId}/clergy-credentials",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** The clergy roster of an organization unit (people holding a credential conferred by it). */
    public listUnitClergyCredentials(unitId: string): Promise<IClergyCredentialList> {
        return this.bridge.call<IClergyCredentialList>(
            "ReligionService",
            "listUnitClergyCredentials",
            "GET",
            "/religion/v1/units/{unitId}/clergy-credentials",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public addClergyCredential(personId: string, request: IAddClergyCredentialRequest): Promise<IClergyCredential> {
        return this.bridge.call<IClergyCredential>(
            "ReligionService",
            "addClergyCredential",
            "POST",
            "/religion/v1/persons/{personId}/clergy-credentials",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Flip a credential's status (e.g. suspend/revoke) and/or set effective-dating. Never deletes. */
    public updateClergyCredential(credentialId: string, request: IUpdateClergyCredentialRequest): Promise<IClergyCredential> {
        return this.bridge.call<IClergyCredential>(
            "ReligionService",
            "updateClergyCredential",
            "PUT",
            "/religion/v1/clergy-credentials/{credentialId}",
            request,
            __undefined,
            __undefined,
            [
                credentialId,
            ],
            __undefined,
            __undefined
        );
    }

    public listAffiliationTypes(): Promise<IAffiliationTypeList> {
        return this.bridge.call<IAffiliationTypeList>(
            "ReligionService",
            "listAffiliationTypes",
            "GET",
            "/religion/v1/affiliation-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertAffiliationType(request: IUpsertAffiliationTypeRequest): Promise<IAffiliationType> {
        return this.bridge.call<IAffiliationType>(
            "ReligionService",
            "upsertAffiliationType",
            "PUT",
            "/religion/v1/affiliation-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List a person's lay religious affiliations. pii:special — gated on affiliation.manage; the belief value is decrypted. */
    public listPersonAffiliations(personId: string): Promise<IAffiliationList> {
        return this.bridge.call<IAffiliationList>(
            "ReligionService",
            "listPersonAffiliations",
            "GET",
            "/religion/v1/persons/{personId}/affiliations",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    public addAffiliation(personId: string, request: IAddAffiliationRequest): Promise<IAffiliation> {
        return this.bridge.call<IAffiliation>(
            "ReligionService",
            "addAffiliation",
            "POST",
            "/religion/v1/persons/{personId}/affiliations",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateAffiliation(affiliationId: string, request: IUpdateAffiliationRequest): Promise<IAffiliation> {
        return this.bridge.call<IAffiliation>(
            "ReligionService",
            "updateAffiliation",
            "PUT",
            "/religion/v1/affiliations/{affiliationId}",
            request,
            __undefined,
            __undefined,
            [
                affiliationId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteAffiliation(affiliationId: string): Promise<void> {
        return this.bridge.call<void>(
            "ReligionService",
            "deleteAffiliation",
            "DELETE",
            "/religion/v1/affiliations/{affiliationId}",
            __undefined,
            __undefined,
            __undefined,
            [
                affiliationId,
            ],
            __undefined,
            __undefined
        );
    }

    public listSiteTypes(): Promise<ISiteTypeList> {
        return this.bridge.call<ISiteTypeList>(
            "ReligionService",
            "listSiteTypes",
            "GET",
            "/religion/v1/site-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertSiteType(request: IUpsertSiteTypeRequest): Promise<ISiteType> {
        return this.bridge.call<ISiteType>(
            "ReligionService",
            "upsertSiteType",
            "PUT",
            "/religion/v1/site-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listServiceTypes(): Promise<IServiceTypeList> {
        return this.bridge.call<IServiceTypeList>(
            "ReligionService",
            "listServiceTypes",
            "GET",
            "/religion/v1/service-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertServiceType(request: IUpsertServiceTypeRequest): Promise<IServiceType> {
        return this.bridge.call<IServiceType>(
            "ReligionService",
            "upsertServiceType",
            "PUT",
            "/religion/v1/service-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List an org unit's sites (exact coordinates, for authorized owners). */
    public listUnitSites(unitId: string): Promise<ISiteList> {
        return this.bridge.call<ISiteList>(
            "ReligionService",
            "listUnitSites",
            "GET",
            "/religion/v1/units/{unitId}/sites",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public createSite(unitId: string, request: ICreateSiteRequest): Promise<ISite> {
        return this.bridge.call<ISite>(
            "ReligionService",
            "createSite",
            "POST",
            "/religion/v1/units/{unitId}/sites",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateSite(siteId: string, request: IUpdateSiteRequest): Promise<ISite> {
        return this.bridge.call<ISite>(
            "ReligionService",
            "updateSite",
            "PUT",
            "/religion/v1/sites/{siteId}",
            request,
            __undefined,
            __undefined,
            [
                siteId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteSite(siteId: string): Promise<void> {
        return this.bridge.call<void>(
            "ReligionService",
            "deleteSite",
            "DELETE",
            "/religion/v1/sites/{siteId}",
            __undefined,
            __undefined,
            __undefined,
            [
                siteId,
            ],
            __undefined,
            __undefined
        );
    }

    public listSiteSchedules(siteId: string): Promise<IServiceScheduleList> {
        return this.bridge.call<IServiceScheduleList>(
            "ReligionService",
            "listSiteSchedules",
            "GET",
            "/religion/v1/sites/{siteId}/schedules",
            __undefined,
            __undefined,
            __undefined,
            [
                siteId,
            ],
            __undefined,
            __undefined
        );
    }

    public createSchedule(siteId: string, request: ICreateScheduleRequest): Promise<IServiceSchedule> {
        return this.bridge.call<IServiceSchedule>(
            "ReligionService",
            "createSchedule",
            "POST",
            "/religion/v1/sites/{siteId}/schedules",
            request,
            __undefined,
            __undefined,
            [
                siteId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteSchedule(scheduleId: string): Promise<void> {
        return this.bridge.call<void>(
            "ReligionService",
            "deleteSchedule",
            "DELETE",
            "/religion/v1/schedules/{scheduleId}",
            __undefined,
            __undefined,
            __undefined,
            [
                scheduleId,
            ],
            __undefined,
            __undefined
        );
    }

    public listUnitAliases(unitId: string): Promise<IAliasList> {
        return this.bridge.call<IAliasList>(
            "ReligionService",
            "listUnitAliases",
            "GET",
            "/religion/v1/units/{unitId}/aliases",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public createAlias(unitId: string, request: ICreateAliasRequest): Promise<IAlias> {
        return this.bridge.call<IAlias>(
            "ReligionService",
            "createAlias",
            "POST",
            "/religion/v1/units/{unitId}/aliases",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteAlias(unitId: string, aliasId: string): Promise<void> {
        return this.bridge.call<void>(
            "ReligionService",
            "deleteAlias",
            "DELETE",
            "/religion/v1/units/{unitId}/aliases/{aliasId}",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
                aliasId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Closure-aware PostGIS discovery search over PUBLIC sites. Supply a radius (lat+lng+radiusM via
     * ST_DWithin) or a bounding box (minLat/minLng/maxLat/maxLng via ST_Intersects); optionally filter
     * by a religion taxon (org units classified under it via the taxonomy closure), a service language/
     * weekday/online toggle, and a fuzzy match on the unit code/name or an alias. Coordinates are
     * coarsened per each site's publicPrecision.
     *
     */
    public searchSites(lat?: number | "NaN" | null, lng?: number | "NaN" | null, radiusM?: number | "NaN" | null, minLat?: number | "NaN" | null, minLng?: number | "NaN" | null, maxLat?: number | "NaN" | null, maxLng?: number | "NaN" | null, religion?: string | null, language?: string | null, dayOfWeek?: number | null, onlineOnly?: boolean | null, query?: string | null, pageSize?: number | null): Promise<IDiscoverySitePage> {
        return this.bridge.call<IDiscoverySitePage>(
            "ReligionService",
            "searchSites",
            "GET",
            "/religion/v1/discovery/sites",
            __undefined,
            __undefined,
            {
                "lat": lat,
                "lng": lng,
                "radiusM": radiusM,
                "minLat": minLat,
                "minLng": minLng,
                "maxLat": maxLat,
                "maxLng": maxLng,
                "religion": religion,
                "language": language,
                "dayOfWeek": dayOfWeek,
                "onlineOnly": onlineOnly,
                "query": query,
                "pageSize": pageSize,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }
}
