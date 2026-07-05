/** Update currency/accountTypeId/status and optionally re-key the IBAN; omitted fields are unchanged. */
export interface IUpdateAccountRequest {
    'iban'?: string | null;
    'currency'?: string | null;
    'accountTypeId'?: string | null;
    'status'?: string | null;
}
