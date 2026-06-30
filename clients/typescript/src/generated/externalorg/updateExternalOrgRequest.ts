/** Update kind/name/code/country/wikidata/status/attribution; omitted fields are unchanged. */
export interface IUpdateExternalOrgRequest {
    'kindId'?: string | null;
    'name'?: string | null;
    'code'?: string | null;
    'countryId'?: string | null;
    'wikidataId'?: string | null;
    'status'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
    'asOf'?: string | null;
}
