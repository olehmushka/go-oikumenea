import { IAddEdgeRequest } from "./addEdgeRequest";
import { IAddGraphRequest } from "./addGraphRequest";
import { IClosureReportList } from "./closureReportList";
import { ICreateDomainRequest } from "./createDomainRequest";
import { ICreateOrganizationRequest } from "./createOrganizationRequest";
import { ICreateUnitKindRequest } from "./createUnitKindRequest";
import { ICreateUnitRequest } from "./createUnitRequest";
import { IDomain } from "./domain";
import { IDomainList } from "./domainList";
import { IGraph } from "./graph";
import { IGraphList } from "./graphList";
import { IOrganization } from "./organization";
import { IOrganizationPage } from "./organizationPage";
import { ISetUnitCodeRequest } from "./setUnitCodeRequest";
import { ITransitionRequest } from "./transitionRequest";
import { IUnit } from "./unit";
import { IUnitCodeEventList } from "./unitCodeEventList";
import { IUnitEdge } from "./unitEdge";
import { IUnitKind } from "./unitKind";
import { IUnitKindList } from "./unitKindList";
import { IUnitLanguage } from "./unitLanguage";
import { IUnitPage } from "./unitPage";
import { IUnitRefList } from "./unitRefList";
import { IUnitRefPage } from "./unitRefPage";
import { IUpdateDomainRequest } from "./updateDomainRequest";
import { IUpdateGraphRequest } from "./updateGraphRequest";
import { IUpdateOrganizationRequest } from "./updateOrganizationRequest";
import { IUpdateUnitKindRequest } from "./updateUnitKindRequest";
import { IUpdateUnitRequest } from "./updateUnitRequest";
import { IUpsertUnitLanguageRequest } from "./upsertUnitLanguageRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The unit graph, its per-graph closure, and the graph registry (D-Graphs). Reads pass the
 * shadow-visibility gate and are gated by `unit.read`/`graph.read`; writes are unit-scoped
 * (`unit.create`/`unit.update`/`unit.edges.*`/`unit.lifecycle`) or instance-scoped
 * (`graph.manage`/`closure.rebuild`) — all enforced once authorization lands (M7).
 *
 */
export interface ITenantService {
    /** Create a unit. Returns Tenant:UnitCodeConflict if the code is taken among active units. */
    createUnit(request: ICreateUnitRequest): Promise<IUnit>;
    /** Read one unit by RID (shadow-gated once authz lands). */
    getUnit(unitId: string): Promise<IUnit>;
    /** Update name/kind/level/metadata/visibility. `code` is excluded — use setUnitCode. */
    updateUnit(unitId: string, request: IUpdateUnitRequest): Promise<IUnit>;
    /**
     * Set, correct, or clear the unit's code (audited; appends a tenant_unit_code_events row).
     * Returns Tenant:UnitCodeConflict if the code is taken among active units. Requires unit.recode.
     *
     */
    setUnitCode(unitId: string, request: ISetUnitCodeRequest): Promise<IUnit>;
    /** A unit's code-change history, newest first (D-UnitCodeLifecycle, M28). */
    listUnitCodeEvents(unitId: string): Promise<IUnitCodeEventList>;
    /**
     * List/search units within an organization (D-TenantOrganizations, M40). `org` is REQUIRED —
     * a fully-unscoped listing is rejected with Tenant:UnitInvalid. Optionally narrowed by the
     * unit facet set (D-ObjectFacets, M56): `domain` (cross-cut within the org, for mixed trees),
     * `unitKind`, `level`, `visibility`, `state` and `pdpScoped`. Token-paginated.
     *
     * The shadow-visibility gate still trims the page AFTER it is cut, so `visibility` NARROWS and
     * never widens: asking for `visibility=shadow` without shadow reach yields an empty page, not
     * an error and not a leak.
     *
     * For hierarchical (expand-on-click) browsing in graph `graph` (default `command`): pass
     * `rootsOnly=true` to list only the org's top-level units (those with no parent in the graph),
     * or `parent=<unitRid>` to list a unit's DIRECT children in the graph. The two are mutually
     * exclusive, and each ignores the flat-listing filters
     * (`domain`/`unitKind`/`level`/`visibility`/`state`/`pdpScoped`). When neither is set the
     * listing is the flat, filtered org listing.
     *
     */
    listUnits(org: string, domain?: string | null, unitKind?: string | null, level?: number | null, visibility?: string | null, state?: string | null, pdpScoped?: boolean | null, graph?: string | null, parent?: string | null, rootsOnly?: boolean | null, pageSize?: number | null, pageToken?: string | null): Promise<IUnitPage>;
    /** Attach the path unit as a child of parentId within a graph (default command). Returns Tenant:UnitCycleDetected on a cycle. */
    addEdge(unitId: string, request: IAddEdgeRequest): Promise<IUnitEdge>;
    /** Detach the path unit from a parent within a graph. */
    removeEdge(unitId: string, parentId: string, graph?: string | null): Promise<void>;
    /** Ancestors of the unit in graph `graph` (default command), nearest first. */
    unitAncestors(unitId: string, graph?: string | null): Promise<IUnitRefList>;
    /** The unit's subtree in graph `graph` (default command), token-paginated. */
    unitDescendants(unitId: string, graph?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IUnitRefPage>;
    /** Transition the unit's lifecycle state. Returns Tenant:TransitionInvalid for an illegal transition. */
    transitionUnit(unitId: string, request: ITransitionRequest): Promise<IUnit>;
    /** List a unit's official/working languages (D-Languages, M18). */
    listUnitLanguages(unitId: string): Promise<Array<IUnitLanguage>>;
    /**
     * Add or update a unit's official/working language (keyed on languageId). Returns
     * Tenant:UnitInvalid when languageId does not resolve to a languoid.
     *
     */
    upsertUnitLanguage(unitId: string, request: IUpsertUnitLanguageRequest): Promise<IUnitLanguage>;
    /** Remove a unit's language by languoid id. Idempotent within the active set. */
    deleteUnitLanguage(unitId: string, languageId: string): Promise<void>;
    /** Diff the stored closure vs. the edges and upsert the per-graph drift status (default all graphs). */
    verifyClosure(graph?: string | null): Promise<IClosureReportList>;
    /** Truncate + recompute the closure, one transaction per graph (default all graphs). */
    rebuildClosure(graph?: string | null): Promise<IClosureReportList>;
    /**
     * List graphs in display order (M40). With `org`, returns that organization's graphs plus the
     * instance-global graphs; without `org`, returns only the instance-global graphs.
     *
     */
    listGraphs(org?: string | null): Promise<IGraphList>;
    /** Add a graph. Returns Tenant:GraphCodeConflict if the code exists. */
    addGraph(request: IAddGraphRequest): Promise<IGraph>;
    /** Rename / set default / flip isAuthorityBearing (guarded; command is locked authority-bearing). */
    updateGraph(graphId: string, request: IUpdateGraphRequest): Promise<IGraph>;
    /** Delete a graph (blocked for command, or while it has live edges). */
    deleteGraph(graphId: string): Promise<void>;
    /** List the org-kind domain catalog in display order (D-TenantOrganizations, M40). Gated by domain.read. */
    listDomains(): Promise<IDomainList>;
    /** Add a domain (instance-admin; domain.manage). Returns Tenant:DomainCodeConflict if the code exists. */
    createDomain(request: ICreateDomainRequest): Promise<IDomain>;
    /** Rename / retire a domain. Returns Tenant:DomainNotFound. */
    updateDomain(domainId: string, request: IUpdateDomainRequest): Promise<IDomain>;
    /** List the unit-kind catalog for a domain (D-TenantOrganizations, M40). Gated by unit-kind.read. */
    listUnitKinds(domain: string): Promise<IUnitKindList>;
    /** Add a domain-scoped unit kind (instance-admin; unit-kind.manage). Returns Tenant:UnitKindCodeConflict. */
    createUnitKind(request: ICreateUnitKindRequest): Promise<IUnitKind>;
    /** Rename / retire a unit kind or adjust its attr schema. Returns Tenant:UnitKindNotFound. */
    updateUnitKind(unitKindId: string, request: IUpdateUnitKindRequest): Promise<IUnitKind>;
    /**
     * List organizations, token-paginated, optionally filtered by domain (D-TenantOrganizations,
     * M40). Shadow-gated. Gated by organization.read.
     *
     */
    listOrganizations(domain?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IOrganizationPage>;
    /**
     * Create an organization and seed its command + operational graphs in one transaction
     * (organization.create). Returns Tenant:OrganizationCodeConflict if the code exists.
     *
     */
    createOrganization(request: ICreateOrganizationRequest): Promise<IOrganization>;
    /** Read one organization by RID (shadow-gated). Returns Tenant:OrganizationNotFound. */
    getOrganization(orgId: string): Promise<IOrganization>;
    /** Update an organization's name/domain/metadata/visibility (organization.update). */
    updateOrganization(orgId: string, request: IUpdateOrganizationRequest): Promise<IOrganization>;
    /** Transition an organization's lifecycle state (organization.lifecycle). Returns Tenant:TransitionInvalid for an illegal transition. */
    transitionOrganization(orgId: string, request: ITransitionRequest): Promise<IOrganization>;
    /** List an organization's graph registry (alias of GET /graphs?org=, path-scoped). */
    listOrganizationGraphs(orgId: string): Promise<IGraphList>;
}

export class TenantService implements ITenantService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Create a unit. Returns Tenant:UnitCodeConflict if the code is taken among active units. */
    public createUnit(request: ICreateUnitRequest): Promise<IUnit> {
        return this.bridge.call<IUnit>(
            "TenantService",
            "createUnit",
            "POST",
            "/tenant/v1/units",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Read one unit by RID (shadow-gated once authz lands). */
    public getUnit(unitId: string): Promise<IUnit> {
        return this.bridge.call<IUnit>(
            "TenantService",
            "getUnit",
            "GET",
            "/tenant/v1/units/{unitId}",
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

    /** Update name/kind/level/metadata/visibility. `code` is excluded — use setUnitCode. */
    public updateUnit(unitId: string, request: IUpdateUnitRequest): Promise<IUnit> {
        return this.bridge.call<IUnit>(
            "TenantService",
            "updateUnit",
            "PUT",
            "/tenant/v1/units/{unitId}",
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
     * Set, correct, or clear the unit's code (audited; appends a tenant_unit_code_events row).
     * Returns Tenant:UnitCodeConflict if the code is taken among active units. Requires unit.recode.
     *
     */
    public setUnitCode(unitId: string, request: ISetUnitCodeRequest): Promise<IUnit> {
        return this.bridge.call<IUnit>(
            "TenantService",
            "setUnitCode",
            "PUT",
            "/tenant/v1/units/{unitId}/code",
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

    /** A unit's code-change history, newest first (D-UnitCodeLifecycle, M28). */
    public listUnitCodeEvents(unitId: string): Promise<IUnitCodeEventList> {
        return this.bridge.call<IUnitCodeEventList>(
            "TenantService",
            "listUnitCodeEvents",
            "GET",
            "/tenant/v1/units/{unitId}/code-events",
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

    /**
     * List/search units within an organization (D-TenantOrganizations, M40). `org` is REQUIRED —
     * a fully-unscoped listing is rejected with Tenant:UnitInvalid. Optionally narrowed by the
     * unit facet set (D-ObjectFacets, M56): `domain` (cross-cut within the org, for mixed trees),
     * `unitKind`, `level`, `visibility`, `state` and `pdpScoped`. Token-paginated.
     *
     * The shadow-visibility gate still trims the page AFTER it is cut, so `visibility` NARROWS and
     * never widens: asking for `visibility=shadow` without shadow reach yields an empty page, not
     * an error and not a leak.
     *
     * For hierarchical (expand-on-click) browsing in graph `graph` (default `command`): pass
     * `rootsOnly=true` to list only the org's top-level units (those with no parent in the graph),
     * or `parent=<unitRid>` to list a unit's DIRECT children in the graph. The two are mutually
     * exclusive, and each ignores the flat-listing filters
     * (`domain`/`unitKind`/`level`/`visibility`/`state`/`pdpScoped`). When neither is set the
     * listing is the flat, filtered org listing.
     *
     */
    public listUnits(org: string, domain?: string | null, unitKind?: string | null, level?: number | null, visibility?: string | null, state?: string | null, pdpScoped?: boolean | null, graph?: string | null, parent?: string | null, rootsOnly?: boolean | null, pageSize?: number | null, pageToken?: string | null): Promise<IUnitPage> {
        return this.bridge.call<IUnitPage>(
            "TenantService",
            "listUnits",
            "GET",
            "/tenant/v1/units",
            __undefined,
            __undefined,
            {
                "org": org,
                "domain": domain,
                "unitKind": unitKind,
                "level": level,
                "visibility": visibility,
                "state": state,
                "pdpScoped": pdpScoped,
                "graph": graph,
                "parent": parent,
                "rootsOnly": rootsOnly,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Attach the path unit as a child of parentId within a graph (default command). Returns Tenant:UnitCycleDetected on a cycle. */
    public addEdge(unitId: string, request: IAddEdgeRequest): Promise<IUnitEdge> {
        return this.bridge.call<IUnitEdge>(
            "TenantService",
            "addEdge",
            "POST",
            "/tenant/v1/units/{unitId}/edges",
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

    /** Detach the path unit from a parent within a graph. */
    public removeEdge(unitId: string, parentId: string, graph?: string | null): Promise<void> {
        return this.bridge.call<void>(
            "TenantService",
            "removeEdge",
            "DELETE",
            "/tenant/v1/units/{unitId}/edges",
            __undefined,
            __undefined,
            {
                "parentId": parentId,
                "graph": graph,
            },
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Ancestors of the unit in graph `graph` (default command), nearest first. */
    public unitAncestors(unitId: string, graph?: string | null): Promise<IUnitRefList> {
        return this.bridge.call<IUnitRefList>(
            "TenantService",
            "unitAncestors",
            "GET",
            "/tenant/v1/units/{unitId}/ancestors",
            __undefined,
            __undefined,
            {
                "graph": graph,
            },
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    /** The unit's subtree in graph `graph` (default command), token-paginated. */
    public unitDescendants(unitId: string, graph?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IUnitRefPage> {
        return this.bridge.call<IUnitRefPage>(
            "TenantService",
            "unitDescendants",
            "GET",
            "/tenant/v1/units/{unitId}/descendants",
            __undefined,
            __undefined,
            {
                "graph": graph,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Transition the unit's lifecycle state. Returns Tenant:TransitionInvalid for an illegal transition. */
    public transitionUnit(unitId: string, request: ITransitionRequest): Promise<IUnit> {
        return this.bridge.call<IUnit>(
            "TenantService",
            "transitionUnit",
            "POST",
            "/tenant/v1/units/{unitId}/transition",
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

    /** List a unit's official/working languages (D-Languages, M18). */
    public listUnitLanguages(unitId: string): Promise<Array<IUnitLanguage>> {
        return this.bridge.call<Array<IUnitLanguage>>(
            "TenantService",
            "listUnitLanguages",
            "GET",
            "/tenant/v1/units/{unitId}/languages",
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

    /**
     * Add or update a unit's official/working language (keyed on languageId). Returns
     * Tenant:UnitInvalid when languageId does not resolve to a languoid.
     *
     */
    public upsertUnitLanguage(unitId: string, request: IUpsertUnitLanguageRequest): Promise<IUnitLanguage> {
        return this.bridge.call<IUnitLanguage>(
            "TenantService",
            "upsertUnitLanguage",
            "PUT",
            "/tenant/v1/units/{unitId}/languages",
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

    /** Remove a unit's language by languoid id. Idempotent within the active set. */
    public deleteUnitLanguage(unitId: string, languageId: string): Promise<void> {
        return this.bridge.call<void>(
            "TenantService",
            "deleteUnitLanguage",
            "DELETE",
            "/tenant/v1/units/{unitId}/languages/{languageId}",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
                languageId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Diff the stored closure vs. the edges and upsert the per-graph drift status (default all graphs). */
    public verifyClosure(graph?: string | null): Promise<IClosureReportList> {
        return this.bridge.call<IClosureReportList>(
            "TenantService",
            "verifyClosure",
            "POST",
            "/tenant/v1/closure/verify",
            __undefined,
            __undefined,
            {
                "graph": graph,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Truncate + recompute the closure, one transaction per graph (default all graphs). */
    public rebuildClosure(graph?: string | null): Promise<IClosureReportList> {
        return this.bridge.call<IClosureReportList>(
            "TenantService",
            "rebuildClosure",
            "POST",
            "/tenant/v1/closure/rebuild",
            __undefined,
            __undefined,
            {
                "graph": graph,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * List graphs in display order (M40). With `org`, returns that organization's graphs plus the
     * instance-global graphs; without `org`, returns only the instance-global graphs.
     *
     */
    public listGraphs(org?: string | null): Promise<IGraphList> {
        return this.bridge.call<IGraphList>(
            "TenantService",
            "listGraphs",
            "GET",
            "/tenant/v1/graphs",
            __undefined,
            __undefined,
            {
                "org": org,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add a graph. Returns Tenant:GraphCodeConflict if the code exists. */
    public addGraph(request: IAddGraphRequest): Promise<IGraph> {
        return this.bridge.call<IGraph>(
            "TenantService",
            "addGraph",
            "POST",
            "/tenant/v1/graphs",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Rename / set default / flip isAuthorityBearing (guarded; command is locked authority-bearing). */
    public updateGraph(graphId: string, request: IUpdateGraphRequest): Promise<IGraph> {
        return this.bridge.call<IGraph>(
            "TenantService",
            "updateGraph",
            "PUT",
            "/tenant/v1/graphs/{graphId}",
            request,
            __undefined,
            __undefined,
            [
                graphId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Delete a graph (blocked for command, or while it has live edges). */
    public deleteGraph(graphId: string): Promise<void> {
        return this.bridge.call<void>(
            "TenantService",
            "deleteGraph",
            "DELETE",
            "/tenant/v1/graphs/{graphId}",
            __undefined,
            __undefined,
            __undefined,
            [
                graphId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the org-kind domain catalog in display order (D-TenantOrganizations, M40). Gated by domain.read. */
    public listDomains(): Promise<IDomainList> {
        return this.bridge.call<IDomainList>(
            "TenantService",
            "listDomains",
            "GET",
            "/tenant/v1/domains",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add a domain (instance-admin; domain.manage). Returns Tenant:DomainCodeConflict if the code exists. */
    public createDomain(request: ICreateDomainRequest): Promise<IDomain> {
        return this.bridge.call<IDomain>(
            "TenantService",
            "createDomain",
            "POST",
            "/tenant/v1/domains",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Rename / retire a domain. Returns Tenant:DomainNotFound. */
    public updateDomain(domainId: string, request: IUpdateDomainRequest): Promise<IDomain> {
        return this.bridge.call<IDomain>(
            "TenantService",
            "updateDomain",
            "PUT",
            "/tenant/v1/domains/{domainId}",
            request,
            __undefined,
            __undefined,
            [
                domainId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the unit-kind catalog for a domain (D-TenantOrganizations, M40). Gated by unit-kind.read. */
    public listUnitKinds(domain: string): Promise<IUnitKindList> {
        return this.bridge.call<IUnitKindList>(
            "TenantService",
            "listUnitKinds",
            "GET",
            "/tenant/v1/unit-kinds",
            __undefined,
            __undefined,
            {
                "domain": domain,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add a domain-scoped unit kind (instance-admin; unit-kind.manage). Returns Tenant:UnitKindCodeConflict. */
    public createUnitKind(request: ICreateUnitKindRequest): Promise<IUnitKind> {
        return this.bridge.call<IUnitKind>(
            "TenantService",
            "createUnitKind",
            "POST",
            "/tenant/v1/unit-kinds",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Rename / retire a unit kind or adjust its attr schema. Returns Tenant:UnitKindNotFound. */
    public updateUnitKind(unitKindId: string, request: IUpdateUnitKindRequest): Promise<IUnitKind> {
        return this.bridge.call<IUnitKind>(
            "TenantService",
            "updateUnitKind",
            "PUT",
            "/tenant/v1/unit-kinds/{unitKindId}",
            request,
            __undefined,
            __undefined,
            [
                unitKindId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * List organizations, token-paginated, optionally filtered by domain (D-TenantOrganizations,
     * M40). Shadow-gated. Gated by organization.read.
     *
     */
    public listOrganizations(domain?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IOrganizationPage> {
        return this.bridge.call<IOrganizationPage>(
            "TenantService",
            "listOrganizations",
            "GET",
            "/tenant/v1/organizations",
            __undefined,
            __undefined,
            {
                "domain": domain,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Create an organization and seed its command + operational graphs in one transaction
     * (organization.create). Returns Tenant:OrganizationCodeConflict if the code exists.
     *
     */
    public createOrganization(request: ICreateOrganizationRequest): Promise<IOrganization> {
        return this.bridge.call<IOrganization>(
            "TenantService",
            "createOrganization",
            "POST",
            "/tenant/v1/organizations",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Read one organization by RID (shadow-gated). Returns Tenant:OrganizationNotFound. */
    public getOrganization(orgId: string): Promise<IOrganization> {
        return this.bridge.call<IOrganization>(
            "TenantService",
            "getOrganization",
            "GET",
            "/tenant/v1/organizations/{orgId}",
            __undefined,
            __undefined,
            __undefined,
            [
                orgId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Update an organization's name/domain/metadata/visibility (organization.update). */
    public updateOrganization(orgId: string, request: IUpdateOrganizationRequest): Promise<IOrganization> {
        return this.bridge.call<IOrganization>(
            "TenantService",
            "updateOrganization",
            "PUT",
            "/tenant/v1/organizations/{orgId}",
            request,
            __undefined,
            __undefined,
            [
                orgId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Transition an organization's lifecycle state (organization.lifecycle). Returns Tenant:TransitionInvalid for an illegal transition. */
    public transitionOrganization(orgId: string, request: ITransitionRequest): Promise<IOrganization> {
        return this.bridge.call<IOrganization>(
            "TenantService",
            "transitionOrganization",
            "PUT",
            "/tenant/v1/organizations/{orgId}/state",
            request,
            __undefined,
            __undefined,
            [
                orgId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List an organization's graph registry (alias of GET /graphs?org=, path-scoped). */
    public listOrganizationGraphs(orgId: string): Promise<IGraphList> {
        return this.bridge.call<IGraphList>(
            "TenantService",
            "listOrganizationGraphs",
            "GET",
            "/tenant/v1/organizations/{orgId}/graphs",
            __undefined,
            __undefined,
            __undefined,
            [
                orgId,
            ],
            __undefined,
            __undefined
        );
    }
}
