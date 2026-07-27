/**
 * The CALLER's own effective authorization state (D-SelfCapabilities). `permissions` is the
 * flat, deduped, sorted union of the caller's active-grant permission codes; `isInstanceAdmin`
 * true means they hold everything and `permissions` is left empty (treat as show-all). A
 * machine/service subject receives an empty set with isInstanceAdmin false. Cosmetic UI gating
 * only — the PDP still re-decides every request.
 *
 */
export interface IMyCapabilities {
    'permissions': Array<string>;
    'isInstanceAdmin': boolean;
}
