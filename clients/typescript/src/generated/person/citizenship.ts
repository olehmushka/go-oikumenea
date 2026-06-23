/** A person's effective-dated nationality in a country (D-Geo). A person may hold several; at most one active per country. */
export interface ICitizenship {
    'id': string;
    'personId': string;
    /** Country RID (resolve via GET /geo/countries). */
    'country': string;
    /** How the citizenship was acquired — one of birth | descent | naturalization | other. */
    'basis': string;
    /** ISO-8601 date the citizenship was acquired. */
    'acquiredOn'?: string | null;
    /** ISO-8601 date the citizenship was lost; null = currently held. */
    'lostOn'?: string | null;
    /** The person's primary nationality (at most one active). */
    'isPrimary': boolean;
}
