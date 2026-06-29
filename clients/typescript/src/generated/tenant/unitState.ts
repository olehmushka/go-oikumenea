/**
 * A unit's (or organization's) lifecycle state. Transitions are reversible and recorded as
 * append-only events. Organizations reuse this enum (D-TenantOrganizations, M40).
 *
 */
export namespace UnitState {
    export type ACTIVE = "ACTIVE";
    export type SUSPENDED = "SUSPENDED";
    export type ARCHIVED = "ARCHIVED";

    export const ACTIVE = "ACTIVE" as "ACTIVE";
    export const SUSPENDED = "SUSPENDED" as "SUSPENDED";
    export const ARCHIVED = "ARCHIVED" as "ARCHIVED";
}

export type UnitState = keyof typeof UnitState;
