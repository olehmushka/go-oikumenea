/** The two actor kinds (D-Audit). An instance admin is a PERSON, never a separate kind. */
export namespace AuditActorType {
    export type PERSON = "PERSON";
    export type SYSTEM = "SYSTEM";

    export const PERSON = "PERSON" as "PERSON";
    export const SYSTEM = "SYSTEM" as "SYSTEM";
}

export type AuditActorType = keyof typeof AuditActorType;
