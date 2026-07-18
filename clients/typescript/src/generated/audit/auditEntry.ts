import { AuditActorType } from "./auditActorType";
import { AuditOutcome } from "./auditOutcome";

/** One immutable audit record — who did what to which target, when, in which request. */
export interface IAuditEntry {
    /** The Action RID (action__<type>) this entry records; the audit log is the action ledger. */
    'id': string;
    'createdAt': string;
    'actorType': AuditActorType;
    /** The person who acted; present iff actorType is PERSON. */
    'actorPersonId'?: string | null;
    /** Originating source for SYSTEM actions (bootstrap, event-subscriber, …); present iff actorType is SYSTEM. */
    'subsystem'?: string | null;
    /**
     * The SERVICE PRINCIPAL that acted (M51 / D-ServiceIdentities) — a machine subject naming
     * itself. Only ever present on SYSTEM entries: a principal is a `system` actor, not a third
     * actor kind (D-Audit's two kinds are binding). Absent for system actions with no machine
     * caller (bootstrap, event subscribers).
     *
     */
    'actorPrincipalId'?: string | null;
    /** The action code, e.g. assignment.grant, unit.transition, rank.scheme.update. */
    'action': string;
    /** The acted-on entity kind, e.g. unit, person, role_assignment, account, graph. */
    'targetType': string;
    'targetId'?: string | null;
    /** The unit context of the action where applicable (drives scoped audit reads). */
    'unitId'?: string | null;
    /** Correlation key shared with logs, metrics, and traces. */
    'requestId': string;
    /** State snapshot before the change (no secrets; PII minimized). */
    'before'?: any | null;
    /** State snapshot / change payload after the change (no secrets; PII minimized). */
    'after'?: any | null;
    'outcome': AuditOutcome;
}
