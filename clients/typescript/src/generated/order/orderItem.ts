/** One affected person/act within an order. Parent-scoped — no lifecycle of its own. */
export interface IOrderItem {
    /** The order item's URN RID. */
    'id': string;
    /** The parent order's URN RID. */
    'orderId': string;
    /** The order type's URN RID (its effect drives the required target columns). */
    'typeId': string;
    /** The affected person's URN RID. */
    'personId': string;
    /** Target unit RID (arrival/transfer/appointment); per the type's effect. */
    'unitId'?: string | null;
    /** Target billet RID (appoint/dismiss); per the type's effect. */
    'positionId'?: string | null;
    /** Target rank RID (rank-change); per the type's effect. */
    'rankId'?: string | null;
    /** ISO-8601 date the act takes effect (legal metadata, not a scheduler trigger). */
    'effectiveFrom'?: string | null;
    /** ISO-8601 date the act ceases effect. */
    'effectiveTo'?: string | null;
    /** Free-text detail (reason, reference). pii:basic — minimized, no secrets. */
    'note'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
