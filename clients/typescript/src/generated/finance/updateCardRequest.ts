/** Update network/type/expiry/cardholder/status; omitted fields are unchanged. The PAN is not re-keyed here. */
export interface IUpdateCardRequest {
    'networkId'?: string | null;
    'cardType'?: string | null;
    'expiryMonth'?: number | null;
    'expiryYear'?: number | null;
    'cardholderPersonId'?: string | null;
    'status'?: string | null;
}
