import { IActionType } from "./actionType";
import { AuditActorType } from "./auditActorType";
import { IAuditEntry } from "./auditEntry";
import { IAuditEntryPage } from "./auditEntryPage";
import { AuditOutcome } from "./auditOutcome";
import { IAuditStats } from "./auditStats";
import { IObjectHistory } from "./objectHistory";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Read-only access to the append-only audit log (D-Audit). Gated by `audit.read`, unit-scoped
 * exactly like `person.read` (PDP over the closure + shadow gate) once authorization lands (M7).
 *
 */
export interface IAuditService {
    /** Query the log, filterable by actor/target/unit/action/outcome/time, token-paginated. */
    query(actorPersonId?: string | null, actorType?: AuditActorType | null, targetType?: string | null, targetId?: string | null, unitId?: string | null, action?: string | null, outcome?: AuditOutcome | null, since?: string | null, until?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IAuditEntryPage>;
    /**
     * Facet distributions over the ledger — the dashboard half of the facet vocabulary (M58 /
     * D-ObjectFacets). Takes exactly the filter args `query` takes (minus paging) plus an optional
     * `facets` CSV, so a dashboard and a list are two renderings of ONE request state and a chart
     * segment is a link to the same URL with one more filter applied.
     *
     * Counted INSIDE the row-level security policy that governs `query` itself: `totalCount`
     * equals the number of rows exhaustively paging `query` with these same filters would return.
     * One round-trip serves the whole dashboard.
     *
     * `since`/`until` prune the month partitions, and are the only thing that bounds how much of
     * an ever-growing ledger this reads. Nothing is defaulted here — a hidden window would make
     * `totalCount` disagree with the caller's own filters — so the console sends its default
     * window as a real, visible filter.
     *
     * The path is `/stats/audit` rather than `/audit/stats` because the server's router rejects a
     * literal path segment that is a sibling of `{entryId}` — see the route-conflict guard in
     * `internal/platform/transport`.
     *
     */
    auditStats(facets?: string | null, actorPersonId?: string | null, actorType?: AuditActorType | null, targetType?: string | null, targetId?: string | null, unitId?: string | null, action?: string | null, outcome?: AuditOutcome | null, since?: string | null, until?: string | null): Promise<IAuditStats>;
    /** Read one entry by its Action RID. Returns Audit:AuditEntryNotFound when absent. */
    get(entryId: string): Promise<IAuditEntry>;
    /**
     * The full action-type catalog (D-ActionTypes, R-29), sorted by (service, code). Lets a client
     * — e.g. the console's object-actions panel — discover what actions exist and their gating
     * permission, instead of hard-coding them. Static registry read; requires only an
     * authenticated subject.
     *
     */
    listActionTypes(): Promise<Array<IActionType>>;
    /**
     * The reverse-chronological audit history of one object (D-Temporal tier b, R-31): every
     * recorded change to the object with `target_id = {rid}`, token-paginated. Gated by
     * `audit.read`; the `before`/`after` change payloads are **redacted** (null, `redacted=true`)
     * unless the caller also holds the sensitive-reader capability, because a folded per-object
     * timeline can surface pii up to the D-DataScope special-category ceiling.
     *
     */
    getObjectHistory(rid: string, pageSize?: number | null, pageToken?: string | null): Promise<IObjectHistory>;
}

export class AuditService implements IAuditService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Query the log, filterable by actor/target/unit/action/outcome/time, token-paginated. */
    public query(actorPersonId?: string | null, actorType?: AuditActorType | null, targetType?: string | null, targetId?: string | null, unitId?: string | null, action?: string | null, outcome?: AuditOutcome | null, since?: string | null, until?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IAuditEntryPage> {
        return this.bridge.call<IAuditEntryPage>(
            "AuditService",
            "query",
            "GET",
            "/audit/v1/audit",
            __undefined,
            __undefined,
            {
                "actorPersonId": actorPersonId,
                "actorType": actorType,
                "targetType": targetType,
                "targetId": targetId,
                "unitId": unitId,
                "action": action,
                "outcome": outcome,
                "since": since,
                "until": until,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the ledger — the dashboard half of the facet vocabulary (M58 /
     * D-ObjectFacets). Takes exactly the filter args `query` takes (minus paging) plus an optional
     * `facets` CSV, so a dashboard and a list are two renderings of ONE request state and a chart
     * segment is a link to the same URL with one more filter applied.
     *
     * Counted INSIDE the row-level security policy that governs `query` itself: `totalCount`
     * equals the number of rows exhaustively paging `query` with these same filters would return.
     * One round-trip serves the whole dashboard.
     *
     * `since`/`until` prune the month partitions, and are the only thing that bounds how much of
     * an ever-growing ledger this reads. Nothing is defaulted here — a hidden window would make
     * `totalCount` disagree with the caller's own filters — so the console sends its default
     * window as a real, visible filter.
     *
     * The path is `/stats/audit` rather than `/audit/stats` because the server's router rejects a
     * literal path segment that is a sibling of `{entryId}` — see the route-conflict guard in
     * `internal/platform/transport`.
     *
     */
    public auditStats(facets?: string | null, actorPersonId?: string | null, actorType?: AuditActorType | null, targetType?: string | null, targetId?: string | null, unitId?: string | null, action?: string | null, outcome?: AuditOutcome | null, since?: string | null, until?: string | null): Promise<IAuditStats> {
        return this.bridge.call<IAuditStats>(
            "AuditService",
            "auditStats",
            "GET",
            "/audit/v1/stats/audit",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "actorPersonId": actorPersonId,
                "actorType": actorType,
                "targetType": targetType,
                "targetId": targetId,
                "unitId": unitId,
                "action": action,
                "outcome": outcome,
                "since": since,
                "until": until,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Read one entry by its Action RID. Returns Audit:AuditEntryNotFound when absent. */
    public get(entryId: string): Promise<IAuditEntry> {
        return this.bridge.call<IAuditEntry>(
            "AuditService",
            "get",
            "GET",
            "/audit/v1/audit/{entryId}",
            __undefined,
            __undefined,
            __undefined,
            [
                entryId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * The full action-type catalog (D-ActionTypes, R-29), sorted by (service, code). Lets a client
     * — e.g. the console's object-actions panel — discover what actions exist and their gating
     * permission, instead of hard-coding them. Static registry read; requires only an
     * authenticated subject.
     *
     */
    public listActionTypes(): Promise<Array<IActionType>> {
        return this.bridge.call<Array<IActionType>>(
            "AuditService",
            "listActionTypes",
            "GET",
            "/audit/v1/action-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * The reverse-chronological audit history of one object (D-Temporal tier b, R-31): every
     * recorded change to the object with `target_id = {rid}`, token-paginated. Gated by
     * `audit.read`; the `before`/`after` change payloads are **redacted** (null, `redacted=true`)
     * unless the caller also holds the sensitive-reader capability, because a folded per-object
     * timeline can surface pii up to the D-DataScope special-category ceiling.
     *
     */
    public getObjectHistory(rid: string, pageSize?: number | null, pageToken?: string | null): Promise<IObjectHistory> {
        return this.bridge.call<IObjectHistory>(
            "AuditService",
            "getObjectHistory",
            "GET",
            "/audit/v1/objects/{rid}/history",
            __undefined,
            __undefined,
            {
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                rid,
            ],
            __undefined,
            __undefined
        );
    }
}
