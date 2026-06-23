/** Add a person's belonging to a unit, optionally filling a position. The position (if given) must belong to the unit. */
export interface ICreateMembershipRequest {
    'personId': string;
    'unitId': string;
    /** The position to fill; omit for plain belonging. */
    'positionId'?: string | null;
    /** Optional provenance pointer to the authorizing order item (D-Orders). */
    'orderItemId'?: string | null;
    /** When the membership takes effect; defaults to now. */
    'effectiveFrom'?: string | null;
}
