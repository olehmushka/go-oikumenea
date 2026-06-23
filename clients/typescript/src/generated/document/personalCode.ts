/** A person-held government identifier of some scheme. Its value is pii:sensitive — stored encrypted, returned decrypted here. */
export interface IPersonalCode {
    /** The personal code's URN RID. */
    'id': string;
    /** The holder's person URN RID. */
    'personId': string;
    /** The scheme this code belongs to (its country derives from the scheme). */
    'schemeCode': string;
    /** The decrypted national-identifier value (pii:sensitive). Reading it is a sensitive action gated by personal-code.read. */
    'value': string;
    /** One of active | superseded | revoked. */
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
