/**
 * A payment card contained by exactly one account. The full PAN is envelope-encrypted; `pan` is
 * populated (decrypted) only on getCard for authorized callers. bin (first 6) + lastFour are the
 * display-only clear digits, always returned. No CVV exists.
 *
 */
export interface ICard {
    'id': string;
    'accountId': string;
    /** The decrypted PAN; present only on getCard for authorized callers. */
    'pan'?: string | null;
    'bin'?: string | null;
    'lastFour'?: string | null;
    'networkId'?: string | null;
    'networkLabel'?: string | null;
    /** One of debit | credit. */
    'cardType': string;
    'expiryMonth'?: number | null;
    'expiryYear'?: number | null;
    /** An optional named cardholder (a person RID), independent of the account's holders. */
    'cardholderPersonId'?: string | null;
    /** One of active | blocked | expired. */
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
