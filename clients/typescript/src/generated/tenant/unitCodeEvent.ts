/** One entry in a unit's append-only code-change ledger (D-UnitCodeLifecycle, M28). */
export interface IUnitCodeEvent {
    /** The event's URN RID. */
    'id': string;
    'unitId': string;
    /** The code before the change (absent = the unit was codeless). */
    'oldCode'?: string | null;
    /** The code after the change (absent = the code was cleared). */
    'newCode'?: string | null;
    'reason'?: string | null;
    'createdAt': string;
}
