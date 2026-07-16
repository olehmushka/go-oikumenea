import { IActionParam } from "./actionParam";

/**
 * One registered action type (D-ActionTypes, review-2026-09 R-29): the machine catalog behind
 * the free-text `audit_log.action`. `code` is what an audit entry's `action` carries.
 *
 */
export interface IActionType {
    /** Stable dotted action code, e.g. assignment.grant, unit.transition. */
    'code': string;
    /** The owning RID service (module) name, e.g. authz, tenant, person. */
    'service': string;
    /** The object type the action targets (the audit target_type), e.g. unit, person, education. */
    'targetType': string;
    /** The gating write permission (module-granular), e.g. assignment.grant, education.manage. */
    'permission': string;
    /**
     * The action's argument schema (D-ActionTypes, review-2026-09 R-29 parameter-schema seam),
     * single-sourced from the Conjure request type that carries the action's inputs and derived
     * from the IR — never hand-authored. Empty for actions with no request body (deletes,
     * lifecycle POSTs, imports) or not yet annotated (the catalog is expand-only).
     *
     */
    'parameters': Array<IActionParam>;
}
