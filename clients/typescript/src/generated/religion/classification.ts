/** A religion-type ("theism") classification (monotheistic/polytheistic/…), tagged onto taxa/units. */
export interface IClassification {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'description'?: string | null;
    'status': string;
    'sortOrder'?: number | null;
}
