/**
 * A person's precise, effective-dated address (D-PersonAddresses, M32) — a reified link to a
 * shared location_locations row (M19 Location). Distinct from Residence (country-grade legal
 * residence); an Address is the geocoded overlay. pii:contact; purge-erased.
 *
 */
export interface IAddress {
    'id': string;
    'personId': string;
    /** The location_locations RID this address resolves to (create/resolve via the location service). */
    'locationId': string;
    /** One of home | work | mailing | other. */
    'role': string;
    /** ISO-8601 date the address became effective. */
    'validFrom': string;
    /** ISO-8601 date the address ended; null = current. */
    'validTo'?: string | null;
    /** The person's primary address (at most one active primary per person). */
    'isPrimary': boolean;
    /** A mailing address that deliberately differs from home (itself a signal). */
    'privacySeeking': boolean;
    'source'?: string | null;
    'confidence'?: string | null;
}
