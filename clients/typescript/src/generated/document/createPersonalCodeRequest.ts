/** Attach a national-identifier code to a person. The value is validated against the scheme then encrypted. */
export interface ICreatePersonalCodeRequest {
    'schemeCode': string;
    /** The plaintext identifier (pii:sensitive). Validated against the scheme, then stored as ciphertext + blind index. */
    'value': string;
}
