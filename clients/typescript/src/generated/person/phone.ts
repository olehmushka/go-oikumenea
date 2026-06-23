/** A person's contact phone (D-PersonContactChannels). number is E.164-normalized; country is derived. pii:contact. */
export interface IPhone {
    'id': string;
    'personId': string;
    /** The phone-type catalog code (mobile | home | work | other ...). */
    'typeCode': string;
    /** The phone number, E.164-normalized on write. */
    'number': string;
    /** Country RID derived from the number (resolve codes via GET /geo/countries); null when underivable. */
    'country'?: string | null;
    /** The person's primary phone (at most one active). */
    'isPrimary': boolean;
}
