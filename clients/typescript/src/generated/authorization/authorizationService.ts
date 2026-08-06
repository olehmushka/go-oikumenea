import { IAssignment } from "./assignment";
import { IAssignmentPage } from "./assignmentPage";
import { IAssignmentStats } from "./assignmentStats";
import { IAuthorizeRequest } from "./authorizeRequest";
import { IAuthorizeResponse } from "./authorizeResponse";
import { IBatchAuthorizeRequest } from "./batchAuthorizeRequest";
import { IBatchAuthorizeResponse } from "./batchAuthorizeResponse";
import { ICreateRoleRequest } from "./createRoleRequest";
import { IGrantAssignmentRequest } from "./grantAssignmentRequest";
import { IGrantInstanceAdminRequest } from "./grantInstanceAdminRequest";
import { IGrantPrincipalPermissionRequest } from "./grantPrincipalPermissionRequest";
import { IInstanceAdmin } from "./instanceAdmin";
import { IMyCapabilities } from "./myCapabilities";
import { IPrincipalGrant } from "./principalGrant";
import { IPrincipalGrantPage } from "./principalGrantPage";
import { IRole } from "./role";
import { IRolePage } from "./rolePage";
import { IUpdateRoleRequest } from "./updateRoleRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The PDP + RBAC management. /authorize answers (person, action, unit) over the unit-graph
 * closure; roles/assignments/instance-admins are policy-as-data. /authorize requires
 * assignment.read (no self-exemption, OQ-5); role.* are instance-scope; assignment.grant/revoke
 * are unit-scoped on the target unit; instance.admin.manage is instance-scope. Writes audited
 * (D-Audit). Denied operations surface Authorization:PermissionDenied.
 *
 */
export interface IAuthorizationService {
    /** Decide (subject, action, unit). Requires assignment.read reaching the unit. */
    authorize(request: IAuthorizeRequest): Promise<IAuthorizeResponse>;
    /** Batch decisions for one subject, with optional decision-explain (DS-16). */
    authorizeBatch(request: IBatchAuthorizeRequest): Promise<IBatchAuthorizeResponse>;
    /**
     * The caller's OWN effective permission codes + instance-admin flag (D-SelfCapabilities).
     * Self-only: subject taken from the request context, NOT gated on assignment.read. Machine
     * subjects get an empty set. Lets an unprivileged console hide modules it cannot read in one
     * round-trip; cosmetic only.
     *
     */
    myCapabilities(): Promise<IMyCapabilities>;
    /** Create a custom role (instance-scope role.create). Returns Role:RoleConflict if the code is taken. */
    createRole(request: ICreateRoleRequest): Promise<IRole>;
    /** List roles, token-paginated (role.read). */
    listRoles(pageSize?: number | null, pageToken?: string | null): Promise<IRolePage>;
    /** Read one role with its permission set (role.read). */
    getRole(roleId: string): Promise<IRole>;
    /** Edit a custom role (instance-scope role.update). Base roles return Role:RoleImmutable. */
    updateRole(roleId: string, request: IUpdateRoleRequest): Promise<IRole>;
    /** Soft-delete a custom role (instance-scope role.delete). Blocked with Role:RoleInUse if assigned. */
    deleteRole(roleId: string): Promise<void>;
    /** Grant (subject, role, unit, scope, graph) + optional expiry (assignment.grant on the target unit). */
    grantAssignment(request: IGrantAssignmentRequest): Promise<IAssignment>;
    /** Revoke an assignment (reversible flip; assignment.revoke). */
    revokeAssignment(assignmentId: string): Promise<IAssignment>;
    /**
     * List ACTIVE assignments, token-paginated. Gated on `assignment.read` held anywhere, and
     * reach-trimmed: an instance admin sees every grant, anyone else sees only grants whose
     * target unit is within their `assignment.read` reach.
     *
     * Every argument is an optional FACET filter (M58 ticket 6 / D-ObjectFacets). Until then this
     * endpoint required exactly one of `subjectPersonId`/`targetUnitId` and there was no way to
     * ask for "the grants"; Assignment:AssignmentInvalid is no longer returned for that reason.
     *
     * Two things this changed rather than added, both deliberate:
     * - the `subjectPersonId` arm used to be gated on `assignment.read` held ANYWHERE with no
     *   trim, so one grant anywhere enumerated any person's authority everywhere. It is now
     *   trimmed like every other arm, so it can only narrow.
     * - the `targetUnitId` arm used to 403 when the caller could not reach the named unit; it now
     *   returns an empty page. The row set is identical.
     *
     * ACTIVE ONLY, and that default stands: `revokedAt` rows are never returned, so there is no
     * `active` or `expiresAt` filter (a distribution whose every row is active is a chart with
     * one bar).
     *
     */
    listAssignments(subjectPersonId?: string | null, targetUnitId?: string | null, roleId?: string | null, scope?: string | null, graphId?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IAssignmentPage>;
    /**
     * Facet distributions over the active grant population — the dashboard half of the assignment
     * facet vocabulary (M58 ticket 6 / D-ObjectFacets). Takes exactly the filter args
     * `listAssignments` takes, minus paging, so a dashboard and a list are two renderings of one
     * request state.
     *
     * The path is `/stats/assignments` rather than `/assignments/stats` because the server's
     * router rejects a literal path segment that is a sibling of `{assignmentId}`.
     *
     */
    assignmentStats(facets?: string | null, subjectPersonId?: string | null, targetUnitId?: string | null, roleId?: string | null, scope?: string | null, graphId?: string | null): Promise<IAssignmentStats>;
    /** Grant instance-admin (instance.admin.manage). Returns Authorization:InstanceAdminConflict if already active. */
    grantInstanceAdmin(request: IGrantInstanceAdminRequest): Promise<IInstanceAdmin>;
    /** Revoke instance-admin (instance.admin.manage; reversible flip). */
    revokeInstanceAdmin(instanceAdminId: string): Promise<IInstanceAdmin>;
    /**
     * Grant a permission code to a machine subject (M51 / D-ServiceIdentities), optionally confined
     * to one organization. Instance-plane act gated on `service-principal.manage`.
     *
     * A principal never holds a ROLE: instance-scope codes such as `import.manage` are satisfiable
     * only on the instance plane, so a role could not carry them (see the PDP). Audited.
     *
     */
    grantPrincipalPermission(request: IGrantPrincipalPermissionRequest): Promise<IPrincipalGrant>;
    /**
     * Revoke-flip a principal grant (the row survives; the history stays readable). Takes effect
     * immediately — principal grants are read per request, not cached. Gates on
     * `service-principal.manage`. Audited.
     *
     */
    revokePrincipalPermission(grantId: string): Promise<IPrincipalGrant>;
    /** List a machine subject's active grants. Gates on `service-principal.read`. */
    listPrincipalGrants(principalId: string): Promise<IPrincipalGrantPage>;
}

export class AuthorizationService implements IAuthorizationService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Decide (subject, action, unit). Requires assignment.read reaching the unit. */
    public authorize(request: IAuthorizeRequest): Promise<IAuthorizeResponse> {
        return this.bridge.call<IAuthorizeResponse>(
            "AuthorizationService",
            "authorize",
            "POST",
            "/authorization/v1/authorize",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Batch decisions for one subject, with optional decision-explain (DS-16). */
    public authorizeBatch(request: IBatchAuthorizeRequest): Promise<IBatchAuthorizeResponse> {
        return this.bridge.call<IBatchAuthorizeResponse>(
            "AuthorizationService",
            "authorizeBatch",
            "POST",
            "/authorization/v1/authorize/batch",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * The caller's OWN effective permission codes + instance-admin flag (D-SelfCapabilities).
     * Self-only: subject taken from the request context, NOT gated on assignment.read. Machine
     * subjects get an empty set. Lets an unprivileged console hide modules it cannot read in one
     * round-trip; cosmetic only.
     *
     */
    public myCapabilities(): Promise<IMyCapabilities> {
        return this.bridge.call<IMyCapabilities>(
            "AuthorizationService",
            "myCapabilities",
            "GET",
            "/authorization/v1/me/capabilities",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Create a custom role (instance-scope role.create). Returns Role:RoleConflict if the code is taken. */
    public createRole(request: ICreateRoleRequest): Promise<IRole> {
        return this.bridge.call<IRole>(
            "AuthorizationService",
            "createRole",
            "POST",
            "/authorization/v1/roles",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List roles, token-paginated (role.read). */
    public listRoles(pageSize?: number | null, pageToken?: string | null): Promise<IRolePage> {
        return this.bridge.call<IRolePage>(
            "AuthorizationService",
            "listRoles",
            "GET",
            "/authorization/v1/roles",
            __undefined,
            __undefined,
            {
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Read one role with its permission set (role.read). */
    public getRole(roleId: string): Promise<IRole> {
        return this.bridge.call<IRole>(
            "AuthorizationService",
            "getRole",
            "GET",
            "/authorization/v1/roles/{roleId}",
            __undefined,
            __undefined,
            __undefined,
            [
                roleId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Edit a custom role (instance-scope role.update). Base roles return Role:RoleImmutable. */
    public updateRole(roleId: string, request: IUpdateRoleRequest): Promise<IRole> {
        return this.bridge.call<IRole>(
            "AuthorizationService",
            "updateRole",
            "PUT",
            "/authorization/v1/roles/{roleId}",
            request,
            __undefined,
            __undefined,
            [
                roleId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Soft-delete a custom role (instance-scope role.delete). Blocked with Role:RoleInUse if assigned. */
    public deleteRole(roleId: string): Promise<void> {
        return this.bridge.call<void>(
            "AuthorizationService",
            "deleteRole",
            "DELETE",
            "/authorization/v1/roles/{roleId}",
            __undefined,
            __undefined,
            __undefined,
            [
                roleId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Grant (subject, role, unit, scope, graph) + optional expiry (assignment.grant on the target unit). */
    public grantAssignment(request: IGrantAssignmentRequest): Promise<IAssignment> {
        return this.bridge.call<IAssignment>(
            "AuthorizationService",
            "grantAssignment",
            "POST",
            "/authorization/v1/assignments",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Revoke an assignment (reversible flip; assignment.revoke). */
    public revokeAssignment(assignmentId: string): Promise<IAssignment> {
        return this.bridge.call<IAssignment>(
            "AuthorizationService",
            "revokeAssignment",
            "DELETE",
            "/authorization/v1/assignments/{assignmentId}",
            __undefined,
            __undefined,
            __undefined,
            [
                assignmentId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * List ACTIVE assignments, token-paginated. Gated on `assignment.read` held anywhere, and
     * reach-trimmed: an instance admin sees every grant, anyone else sees only grants whose
     * target unit is within their `assignment.read` reach.
     *
     * Every argument is an optional FACET filter (M58 ticket 6 / D-ObjectFacets). Until then this
     * endpoint required exactly one of `subjectPersonId`/`targetUnitId` and there was no way to
     * ask for "the grants"; Assignment:AssignmentInvalid is no longer returned for that reason.
     *
     * Two things this changed rather than added, both deliberate:
     * - the `subjectPersonId` arm used to be gated on `assignment.read` held ANYWHERE with no
     *   trim, so one grant anywhere enumerated any person's authority everywhere. It is now
     *   trimmed like every other arm, so it can only narrow.
     * - the `targetUnitId` arm used to 403 when the caller could not reach the named unit; it now
     *   returns an empty page. The row set is identical.
     *
     * ACTIVE ONLY, and that default stands: `revokedAt` rows are never returned, so there is no
     * `active` or `expiresAt` filter (a distribution whose every row is active is a chart with
     * one bar).
     *
     */
    public listAssignments(subjectPersonId?: string | null, targetUnitId?: string | null, roleId?: string | null, scope?: string | null, graphId?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IAssignmentPage> {
        return this.bridge.call<IAssignmentPage>(
            "AuthorizationService",
            "listAssignments",
            "GET",
            "/authorization/v1/assignments",
            __undefined,
            __undefined,
            {
                "subjectPersonId": subjectPersonId,
                "targetUnitId": targetUnitId,
                "roleId": roleId,
                "scope": scope,
                "graphId": graphId,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the active grant population — the dashboard half of the assignment
     * facet vocabulary (M58 ticket 6 / D-ObjectFacets). Takes exactly the filter args
     * `listAssignments` takes, minus paging, so a dashboard and a list are two renderings of one
     * request state.
     *
     * The path is `/stats/assignments` rather than `/assignments/stats` because the server's
     * router rejects a literal path segment that is a sibling of `{assignmentId}`.
     *
     */
    public assignmentStats(facets?: string | null, subjectPersonId?: string | null, targetUnitId?: string | null, roleId?: string | null, scope?: string | null, graphId?: string | null): Promise<IAssignmentStats> {
        return this.bridge.call<IAssignmentStats>(
            "AuthorizationService",
            "assignmentStats",
            "GET",
            "/authorization/v1/stats/assignments",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "subjectPersonId": subjectPersonId,
                "targetUnitId": targetUnitId,
                "roleId": roleId,
                "scope": scope,
                "graphId": graphId,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Grant instance-admin (instance.admin.manage). Returns Authorization:InstanceAdminConflict if already active. */
    public grantInstanceAdmin(request: IGrantInstanceAdminRequest): Promise<IInstanceAdmin> {
        return this.bridge.call<IInstanceAdmin>(
            "AuthorizationService",
            "grantInstanceAdmin",
            "POST",
            "/authorization/v1/instance-admins",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Revoke instance-admin (instance.admin.manage; reversible flip). */
    public revokeInstanceAdmin(instanceAdminId: string): Promise<IInstanceAdmin> {
        return this.bridge.call<IInstanceAdmin>(
            "AuthorizationService",
            "revokeInstanceAdmin",
            "DELETE",
            "/authorization/v1/instance-admins/{instanceAdminId}",
            __undefined,
            __undefined,
            __undefined,
            [
                instanceAdminId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Grant a permission code to a machine subject (M51 / D-ServiceIdentities), optionally confined
     * to one organization. Instance-plane act gated on `service-principal.manage`.
     *
     * A principal never holds a ROLE: instance-scope codes such as `import.manage` are satisfiable
     * only on the instance plane, so a role could not carry them (see the PDP). Audited.
     *
     */
    public grantPrincipalPermission(request: IGrantPrincipalPermissionRequest): Promise<IPrincipalGrant> {
        return this.bridge.call<IPrincipalGrant>(
            "AuthorizationService",
            "grantPrincipalPermission",
            "POST",
            "/authorization/v1/principal-grants",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Revoke-flip a principal grant (the row survives; the history stays readable). Takes effect
     * immediately — principal grants are read per request, not cached. Gates on
     * `service-principal.manage`. Audited.
     *
     */
    public revokePrincipalPermission(grantId: string): Promise<IPrincipalGrant> {
        return this.bridge.call<IPrincipalGrant>(
            "AuthorizationService",
            "revokePrincipalPermission",
            "DELETE",
            "/authorization/v1/principal-grants/{grantId}",
            __undefined,
            __undefined,
            __undefined,
            [
                grantId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a machine subject's active grants. Gates on `service-principal.read`. */
    public listPrincipalGrants(principalId: string): Promise<IPrincipalGrantPage> {
        return this.bridge.call<IPrincipalGrantPage>(
            "AuthorizationService",
            "listPrincipalGrants",
            "GET",
            "/authorization/v1/principal-grants",
            __undefined,
            __undefined,
            {
                "principalId": principalId,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }
}
