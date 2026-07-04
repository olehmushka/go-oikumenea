/** Add an external reference (idempotent by url), or replace one when id is supplied (M33). */
export interface IUpsertExternalReferenceRequest {
    'id'?: string | null;
    'kind'?: string | null;
    'url': string;
    'externalId'?: string | null;
    'categories'?: Array<string> | null;
    /** RFC-3339 timestamp of the last verification check. */
    'lastChecked'?: string | null;
    'disputed'?: boolean | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
