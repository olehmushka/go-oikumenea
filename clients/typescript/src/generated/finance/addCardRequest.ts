/** Add a card to the account. The PAN is validated (Luhn), encrypted, and blind-indexed; bin/lastFour derive from it. */
export interface IAddCardRequest {
    'pan': string;
    'networkId'?: string | null;
    'cardType': string;
    'expiryMonth'?: number | null;
    'expiryYear'?: number | null;
    'cardholderPersonId'?: string | null;
}
