import { ICreateMembershipRequest } from "./createMembershipRequest";
import { ICreatePositionRequest } from "./createPositionRequest";
import { IEndMembershipRequest } from "./endMembershipRequest";
import { IFillPositionRequest } from "./fillPositionRequest";
import { IMembership } from "./membership";
import { IMembershipPage } from "./membershipPage";
import { IPosition } from "./position";
import { IPositionPage } from "./positionPage";
import { IUpdatePositionRequest } from "./updatePositionRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Positions (unit-owned billets) and memberships (people belonging to / filling them). Reads gate
 * on `position.read`/`membership.read` + the shadow-visibility gate; writes on
 * `position.create`/`position.update`/`membership.create`/`membership.update` — all unit-scoped
 * and enforced once authorization (M7) lands. Position carries no authority (D-Position / D-Rank).
 * Writes are audited in-process (D-Audit).
 *
 */
export interface IMembershipService {
    /** Create a billet in a unit (vacant). Returns Position:PositionConflict if the code is taken in the unit. */
    createPosition(unitId: string, request: ICreatePositionRequest): Promise<IPosition>;
    /** List a unit's positions, token-paginated. Filter by state=vacant|filled. */
    listPositions(unitId: string, state?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IPositionPage>;
    /** Read one position with its current holder (if any). */
    getPosition(positionId: string): Promise<IPosition>;
    /** Update title / required rank / sort order. `code` and unit are immutable. */
    updatePosition(positionId: string, request: IUpdatePositionRequest): Promise<IPosition>;
    /** Abolish a billet (reversible status flip). Returns Position:PositionInUse if it has an active filling. */
    abolishPosition(positionId: string): Promise<IPosition>;
    /** Add a person's belonging to a unit, optionally filling a position. Returns Membership:PositionAlreadyFilled / Membership:MembershipConflict on a uniqueness clash. */
    createMembership(request: ICreateMembershipRequest): Promise<IMembership>;
    /** Fill a vacant position with a person. Returns Membership:PositionAlreadyFilled if already filled. */
    fillPosition(positionId: string, request: IFillPositionRequest): Promise<IMembership>;
    /** End a membership, vacating any filled billet. Returns Membership:MembershipLifecycleConflict if not active. */
    endMembership(membershipId: string, request: IEndMembershipRequest): Promise<IMembership>;
    /** Roster of a unit's active memberships, token-paginated. (The shadow gate applies once authz lands, M7.) */
    listMembers(unitId: string, pageSize?: number | null, pageToken?: string | null): Promise<IMembershipPage>;
    /** A person's active memberships across units, token-paginated. */
    listPersonMemberships(personId: string, pageSize?: number | null, pageToken?: string | null): Promise<IMembershipPage>;
}

export class MembershipService implements IMembershipService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Create a billet in a unit (vacant). Returns Position:PositionConflict if the code is taken in the unit. */
    public createPosition(unitId: string, request: ICreatePositionRequest): Promise<IPosition> {
        return this.bridge.call<IPosition>(
            "MembershipService",
            "createPosition",
            "POST",
            "/membership/v1/units/{unitId}/positions",
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

    /** List a unit's positions, token-paginated. Filter by state=vacant|filled. */
    public listPositions(unitId: string, state?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IPositionPage> {
        return this.bridge.call<IPositionPage>(
            "MembershipService",
            "listPositions",
            "GET",
            "/membership/v1/units/{unitId}/positions",
            __undefined,
            __undefined,
            {
                "state": state,
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

    /** Read one position with its current holder (if any). */
    public getPosition(positionId: string): Promise<IPosition> {
        return this.bridge.call<IPosition>(
            "MembershipService",
            "getPosition",
            "GET",
            "/membership/v1/positions/{positionId}",
            __undefined,
            __undefined,
            __undefined,
            [
                positionId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Update title / required rank / sort order. `code` and unit are immutable. */
    public updatePosition(positionId: string, request: IUpdatePositionRequest): Promise<IPosition> {
        return this.bridge.call<IPosition>(
            "MembershipService",
            "updatePosition",
            "PUT",
            "/membership/v1/positions/{positionId}",
            request,
            __undefined,
            __undefined,
            [
                positionId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Abolish a billet (reversible status flip). Returns Position:PositionInUse if it has an active filling. */
    public abolishPosition(positionId: string): Promise<IPosition> {
        return this.bridge.call<IPosition>(
            "MembershipService",
            "abolishPosition",
            "POST",
            "/membership/v1/positions/{positionId}/abolish",
            __undefined,
            __undefined,
            __undefined,
            [
                positionId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a person's belonging to a unit, optionally filling a position. Returns Membership:PositionAlreadyFilled / Membership:MembershipConflict on a uniqueness clash. */
    public createMembership(request: ICreateMembershipRequest): Promise<IMembership> {
        return this.bridge.call<IMembership>(
            "MembershipService",
            "createMembership",
            "POST",
            "/membership/v1/memberships",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Fill a vacant position with a person. Returns Membership:PositionAlreadyFilled if already filled. */
    public fillPosition(positionId: string, request: IFillPositionRequest): Promise<IMembership> {
        return this.bridge.call<IMembership>(
            "MembershipService",
            "fillPosition",
            "POST",
            "/membership/v1/positions/{positionId}/fill",
            request,
            __undefined,
            __undefined,
            [
                positionId,
            ],
            __undefined,
            __undefined
        );
    }

    /** End a membership, vacating any filled billet. Returns Membership:MembershipLifecycleConflict if not active. */
    public endMembership(membershipId: string, request: IEndMembershipRequest): Promise<IMembership> {
        return this.bridge.call<IMembership>(
            "MembershipService",
            "endMembership",
            "POST",
            "/membership/v1/memberships/{membershipId}/end",
            request,
            __undefined,
            __undefined,
            [
                membershipId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Roster of a unit's active memberships, token-paginated. (The shadow gate applies once authz lands, M7.) */
    public listMembers(unitId: string, pageSize?: number | null, pageToken?: string | null): Promise<IMembershipPage> {
        return this.bridge.call<IMembershipPage>(
            "MembershipService",
            "listMembers",
            "GET",
            "/membership/v1/units/{unitId}/members",
            __undefined,
            __undefined,
            {
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

    /** A person's active memberships across units, token-paginated. */
    public listPersonMemberships(personId: string, pageSize?: number | null, pageToken?: string | null): Promise<IMembershipPage> {
        return this.bridge.call<IMembershipPage>(
            "MembershipService",
            "listPersonMemberships",
            "GET",
            "/membership/v1/persons/{personId}/memberships",
            __undefined,
            __undefined,
            {
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }
}
