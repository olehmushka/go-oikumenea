/** Add an address, or replace one when id is supplied (D-PersonAddresses, M32). */
export interface IUpsertAddressRequest {
    /** The RID of an existing address row to replace; omit to add a new row. */
    'id'?: string | null;
    /** The location_locations RID this address resolves to. */
    'locationId': string;
    /** One of home | work | mailing | other. */
    'role': string;
    /** ISO-8601 date the address became effective; defaults to today. */
    'validFrom'?: string | null;
    'validTo'?: string | null;
    'isPrimary'?: boolean | null;
    'privacySeeking'?: boolean | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
