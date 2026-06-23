import { AuditActorType } from "./auditActorType";
import { IAuditEntry } from "./auditEntry";
import { IAuditEntryPage } from "./auditEntryPage";
import { AuditOutcome } from "./auditOutcome";
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
    /** Read one entry by its Action RID. Returns Audit:AuditEntryNotFound when absent. */
    get(entryId: string): Promise<IAuditEntry>;
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
}
