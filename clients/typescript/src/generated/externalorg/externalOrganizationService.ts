import { ICreateExternalOrgRequest } from "./createExternalOrgRequest";
import { IExternalOrgKind } from "./externalOrgKind";
import { IExternalOrgKindList } from "./externalOrgKindList";
import { IExternalOrgPage } from "./externalOrgPage";
import { IExternalOrgStats } from "./externalOrgStats";
import { IExternalOrganization } from "./externalOrganization";
import { IMergeExternalOrgRequest } from "./mergeExternalOrgRequest";
import { IUpdateExternalOrgRequest } from "./updateExternalOrgRequest";
import { IUpsertExternalOrgKindRequest } from "./upsertExternalOrgKindRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * An external-organizations registry: the kind catalog + the external-organization node-space, with
 * provisional/resolved resolution. Reads gate on `externalorg.read`; catalog + org writes on the
 * instance-scope `externalorg.manage`. Writes are audited in-process (D-Audit).
 *
 */
export interface IExternalOrganizationService {
    listExternalOrgKinds(): Promise<IExternalOrgKindList>;
    upsertExternalOrgKind(request: IUpsertExternalOrgKindRequest): Promise<IExternalOrgKind>;
    listExternalOrgs(query?: string | null, kind?: string | null, country?: string | null, status?: string | null, source?: string | null, confidence?: string | null, asOfFrom?: string | null, asOfTo?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IExternalOrgPage>;
    /**
     * Facet distributions over the registry — the dashboard half of the facet vocabulary (M58 /
     * D-ObjectFacets). Takes exactly the filter args `listExternalOrgs` takes (minus paging) plus
     * an optional `facets` CSV, so a dashboard and a list are two renderings of ONE request state
     * and a chart segment is a link to the same URL with one more filter applied.
     *
     * `totalCount` equals the number of rows exhaustively paging `listExternalOrgs` with these
     * same filters would return. One round-trip serves the whole dashboard.
     *
     * ONE aggregate arm, with no subject and no scoped twin — but for the OPPOSITE reason to the
     * audit ledger's single arm. `external_organizations` is a flat instance-global reference
     * table with no row-level security and no unit reach: `externalorg.read` held anywhere is the
     * whole visibility decision, so there is nothing for a second arm to narrow.
     *
     * The path is `/stats/external-orgs` rather than `/external-orgs/stats` because the server's
     * router rejects a literal path segment that is a sibling of `{orgId}`.
     *
     */
    externalOrgStats(facets?: string | null, query?: string | null, kind?: string | null, country?: string | null, status?: string | null, source?: string | null, confidence?: string | null, asOfFrom?: string | null, asOfTo?: string | null): Promise<IExternalOrgStats>;
    createExternalOrg(request: ICreateExternalOrgRequest): Promise<IExternalOrganization>;
    getExternalOrg(orgId: string): Promise<IExternalOrganization>;
    updateExternalOrg(orgId: string, request: IUpdateExternalOrgRequest): Promise<IExternalOrganization>;
    deleteExternalOrg(orgId: string): Promise<void>;
    /** Resolve a provisional stub (orgId) into a canonical organization (intoOrgId); tombstones the stub. */
    mergeExternalOrg(orgId: string, request: IMergeExternalOrgRequest): Promise<IExternalOrganization>;
}

export class ExternalOrganizationService implements IExternalOrganizationService {
    constructor(private bridge: IHttpApiBridge) {
    }

    public listExternalOrgKinds(): Promise<IExternalOrgKindList> {
        return this.bridge.call<IExternalOrgKindList>(
            "ExternalOrganizationService",
            "listExternalOrgKinds",
            "GET",
            "/external-orgs/v1/external-org-kinds",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertExternalOrgKind(request: IUpsertExternalOrgKindRequest): Promise<IExternalOrgKind> {
        return this.bridge.call<IExternalOrgKind>(
            "ExternalOrganizationService",
            "upsertExternalOrgKind",
            "PUT",
            "/external-orgs/v1/external-org-kinds",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listExternalOrgs(query?: string | null, kind?: string | null, country?: string | null, status?: string | null, source?: string | null, confidence?: string | null, asOfFrom?: string | null, asOfTo?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IExternalOrgPage> {
        return this.bridge.call<IExternalOrgPage>(
            "ExternalOrganizationService",
            "listExternalOrgs",
            "GET",
            "/external-orgs/v1/external-orgs",
            __undefined,
            __undefined,
            {
                "query": query,
                "kind": kind,
                "country": country,
                "status": status,
                "source": source,
                "confidence": confidence,
                "asOfFrom": asOfFrom,
                "asOfTo": asOfTo,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the registry — the dashboard half of the facet vocabulary (M58 /
     * D-ObjectFacets). Takes exactly the filter args `listExternalOrgs` takes (minus paging) plus
     * an optional `facets` CSV, so a dashboard and a list are two renderings of ONE request state
     * and a chart segment is a link to the same URL with one more filter applied.
     *
     * `totalCount` equals the number of rows exhaustively paging `listExternalOrgs` with these
     * same filters would return. One round-trip serves the whole dashboard.
     *
     * ONE aggregate arm, with no subject and no scoped twin — but for the OPPOSITE reason to the
     * audit ledger's single arm. `external_organizations` is a flat instance-global reference
     * table with no row-level security and no unit reach: `externalorg.read` held anywhere is the
     * whole visibility decision, so there is nothing for a second arm to narrow.
     *
     * The path is `/stats/external-orgs` rather than `/external-orgs/stats` because the server's
     * router rejects a literal path segment that is a sibling of `{orgId}`.
     *
     */
    public externalOrgStats(facets?: string | null, query?: string | null, kind?: string | null, country?: string | null, status?: string | null, source?: string | null, confidence?: string | null, asOfFrom?: string | null, asOfTo?: string | null): Promise<IExternalOrgStats> {
        return this.bridge.call<IExternalOrgStats>(
            "ExternalOrganizationService",
            "externalOrgStats",
            "GET",
            "/external-orgs/v1/stats/external-orgs",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "query": query,
                "kind": kind,
                "country": country,
                "status": status,
                "source": source,
                "confidence": confidence,
                "asOfFrom": asOfFrom,
                "asOfTo": asOfTo,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public createExternalOrg(request: ICreateExternalOrgRequest): Promise<IExternalOrganization> {
        return this.bridge.call<IExternalOrganization>(
            "ExternalOrganizationService",
            "createExternalOrg",
            "POST",
            "/external-orgs/v1/external-orgs",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public getExternalOrg(orgId: string): Promise<IExternalOrganization> {
        return this.bridge.call<IExternalOrganization>(
            "ExternalOrganizationService",
            "getExternalOrg",
            "GET",
            "/external-orgs/v1/external-orgs/{orgId}",
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

    public updateExternalOrg(orgId: string, request: IUpdateExternalOrgRequest): Promise<IExternalOrganization> {
        return this.bridge.call<IExternalOrganization>(
            "ExternalOrganizationService",
            "updateExternalOrg",
            "PUT",
            "/external-orgs/v1/external-orgs/{orgId}",
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

    public deleteExternalOrg(orgId: string): Promise<void> {
        return this.bridge.call<void>(
            "ExternalOrganizationService",
            "deleteExternalOrg",
            "DELETE",
            "/external-orgs/v1/external-orgs/{orgId}",
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

    /** Resolve a provisional stub (orgId) into a canonical organization (intoOrgId); tombstones the stub. */
    public mergeExternalOrg(orgId: string, request: IMergeExternalOrgRequest): Promise<IExternalOrganization> {
        return this.bridge.call<IExternalOrganization>(
            "ExternalOrganizationService",
            "mergeExternalOrg",
            "POST",
            "/external-orgs/v1/external-orgs/{orgId}/merge",
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
}
