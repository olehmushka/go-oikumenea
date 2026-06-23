/** Flip status (active|suspended|revoked) and/or set effective-dating. Never a hard delete. */
export interface IUpdateClergyCredentialRequest {
    'status'?: string | null;
    'effectiveTo'?: string | null;
}
