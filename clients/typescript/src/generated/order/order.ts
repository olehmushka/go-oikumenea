import { IOrderItem } from "./orderItem";

/** An order header (наказ) plus its items. Mutable while draft; locked on issue. */
export interface IOrder {
    /** The order's URN RID. */
    'id': string;
    /** The order number (unique within its issuing unit). */
    'number'?: string | null;
    /** ISO-8601 date of the order. */
    'issuedOn'?: string | null;
    /** The unit that issues the order (NOT NULL — no instance-level orders); anchors unit-scope authz. */
    'issuingUnitId': string;
    /** One of draft | issued | revoked. */
    'status': string;
    /** The later order that revoked this one (set on revoke). */
    'revokedByOrderId'?: string | null;
    'revokedAt'?: string | null;
    /** The order's items (≥1). Returned on get/create/issue/revoke; empty on list pages. */
    'items': Array<IOrderItem>;
    'createdAt': string;
    'updatedAt': string;
}
