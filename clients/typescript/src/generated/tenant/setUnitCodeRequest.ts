/**
 * Set, correct, or clear a unit's code (D-UnitCodeLifecycle, M28). An omitted `code` CLEARS the
 * code (the unit becomes a non-separate sub-unit). `reason` is recorded on the append-only ledger.
 *
 */
export interface ISetUnitCodeRequest {
    /** The new code; omit to clear the code (NULL). Must be unique among active coded units. */
    'code'?: string | null;
    /** Optional free-text reason recorded in the code-change ledger. */
    'reason'?: string | null;
}
