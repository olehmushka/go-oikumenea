/** A per-tradition service/observance type (main/youth/prayer/jumua/shabbat/…). */
export interface IServiceType {
    'id': string;
    'traditionTaxonId'?: string | null;
    'code': string;
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
