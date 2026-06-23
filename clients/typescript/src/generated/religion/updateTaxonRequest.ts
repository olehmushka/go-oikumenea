/** Update name/rank/description/wikidata/sort. code and parent are changed via reparent only. */
export interface IUpdateTaxonRequest {
    'name'?: string | null;
    'rankId'?: string | null;
    'description'?: string | null;
    'wikidataId'?: string | null;
    'sortOrder'?: number | null;
}
