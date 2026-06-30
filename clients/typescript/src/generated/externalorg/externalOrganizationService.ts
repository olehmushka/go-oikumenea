import { ICreateExternalOrgRequest } from "./createExternalOrgRequest";
import { IExternalOrgKind } from "./externalOrgKind";
import { IExternalOrgKindList } from "./externalOrgKindList";
import { IExternalOrgPage } from "./externalOrgPage";
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
    listExternalOrgs(query?: string | null, kind?: string | null, country?: string | null, status?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IExternalOrgPage>;
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

    public listExternalOrgs(query?: string | null, kind?: string | null, country?: string | null, status?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IExternalOrgPage> {
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
                "pageSize": pageSize,
                "pageToken": pageToken,
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
