/**
 * A registered connector — a deployable agent beside the core. `principalId` is the M51 service
 * principal it authenticates as; disabling that principal stops its reports and wiring reads at
 * once, so there is no separate kill switch here.
 *
 */
export interface IConnector {
    'id': string;
    /** Stable, locale-agnostic handle (D-Code) — what operators and audit rows reference. */
    'code': string;
    'name': string;
    'description'?: string | null;
    /** The service principal this agent authenticates as. Bound by the core from the caller. */
    'principalId'?: string | null;
    /** active | disabled. */
    'status': string;
    /**
     * When the core last heard from this agent (registration or a run report). A liveness HINT,
     * not a health verdict — the core never probes connectors.
     *
     */
    'lastSeenAt'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
