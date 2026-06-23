/** Outcome of the recorded action; denied write attempts are recorded too (D-Audit). */
export namespace AuditOutcome {
    export type SUCCESS = "SUCCESS";
    export type DENIED = "DENIED";
    export type ERROR = "ERROR";

    export const SUCCESS = "SUCCESS" as "SUCCESS";
    export const DENIED = "DENIED" as "DENIED";
    export const ERROR = "ERROR" as "ERROR";
}

export type AuditOutcome = keyof typeof AuditOutcome;
