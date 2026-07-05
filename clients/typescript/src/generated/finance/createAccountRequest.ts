/** Create a bank account. The IBAN is validated (ISO 13616 mod-97), encrypted, and blind-indexed. */
export interface ICreateAccountRequest {
    'institutionId': string;
    'iban': string;
    'currency'?: string | null;
    'accountTypeId'?: string | null;
}
