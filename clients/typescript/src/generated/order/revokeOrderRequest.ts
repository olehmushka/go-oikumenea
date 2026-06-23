/** Revoke an issued order. The optional revoking order's RID is recorded as revoked_by_order_id. */
export interface IRevokeOrderRequest {
    'revokingOrderId'?: string | null;
}
