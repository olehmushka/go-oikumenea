/** A person's contact email (D-PersonContactChannels). pii:contact; distinct from the login email. */
export interface IEmail {
    'id': string;
    'personId': string;
    /** The email-type catalog code (personal | work | other ...). */
    'typeCode': string;
    /** The email address (stored case-insensitively). */
    'address': string;
    /** Provider derived from the address domain on write (gmail.com -> google); null when no mapping. */
    'provider'?: string | null;
    /** The person's primary email (at most one active). */
    'isPrimary': boolean;
}
