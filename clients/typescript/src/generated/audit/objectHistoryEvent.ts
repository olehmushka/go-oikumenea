import { AuditActorType } from "./auditActorType";
import { AuditOutcome } from "./auditOutcome";

/**
 * One dated change in an object's history (D-Temporal, review-2026-09 R-31): a projection of an
 * audit row keyed to this object's RID. `before`/`after` carry the change payload and are
 * **redacted (null) unless the caller holds the sensitive-reader capability** — the timeline
 * (when/what/who) is always visible under `audit.read`.
 *
 */
export interface IObjectHistoryEvent {
    /** When the change was recorded (the audit row's createdAt). */
    'at': string;
    /** The action code that produced the change, e.g. person.rank.assign, unit.transition. */
    'action': string;
    'actorType': AuditActorType;
    /** The person who acted; present iff actorType is PERSON. */
    'actorPersonId'?: string | null;
    /** Originating source for SYSTEM actions; present iff actorType is SYSTEM. */
    'subsystem'?: string | null;
    /** The acted-on entity kind (the audit target_type), e.g. person, unit. */
    'targetType': string;
    'outcome': AuditOutcome;
    /** Correlation key shared with logs, metrics, and traces. */
    'requestId': string;
    /** State before the change; null when redacted (caller lacks sensitive-reader) or absent. */
    'before'?: any | null;
    /** State after the change; null when redacted (caller lacks sensitive-reader) or absent. */
    'after'?: any | null;
}
