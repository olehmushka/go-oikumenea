/**
 * Register an external organization. status defaults to resolved; pass provisional to create a
 * stub awaiting merge. source/confidence default to operator_verified/possible when omitted.
 *
 */
export interface ICreateExternalOrgRequest {
    'kindId': string;
    'name': string;
    'code'?: string | null;
    'countryId'?: string | null;
    'wikidataId'?: string | null;
    'status'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
    'asOf'?: string | null;
}
