/** Update a personal code's value (re-validated + re-encrypted) and/or status. Omitted fields are unchanged. */
export interface IUpdatePersonalCodeRequest {
    'value'?: string | null;
    'status'?: string | null;
}
