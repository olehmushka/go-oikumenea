/**
 * A person's belonging to a unit (the reified link__member_of), effective-dated, optionally
 * filling a position. A position-less membership is plain belonging. Reversible: ending flips
 * status and sets effectiveTo rather than deleting, vacating any filled billet.
 *
 */
export interface IMembership {
    /** The membership's URN RID (carried as a plain string). */
    'id': string;
    /** The URN RID of the person who belongs / fills. */
    'personId': string;
    /** The URN RID of the unit belonged to. */
    'unitId': string;
    /** The URN RID of the position this membership fills; null for plain belonging. */
    'positionId'?: string | null;
    /** Provenance — the order (наказ) item that authorized this fill/belonging (D-Orders); null when created without an order. */
    'orderItemId'?: string | null;
    /** One of active | ended. */
    'status': string;
    'effectiveFrom': string;
    /** When the membership ended; null while active. */
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
