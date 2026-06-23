/** End a membership, vacating any filled billet (reversible status flip + effectiveTo). */
export interface IEndMembershipRequest {
    /** When the membership ended; defaults to now. */
    'effectiveTo'?: string | null;
    /** Optional provenance pointer to the authorizing order item (D-Orders). */
    'orderItemId'?: string | null;
}
