/** A model under a brand (containment); generation + the manufacture window are structural specs. */
export interface IModel {
    'id': string;
    'brandId': string;
    'code': string;
    'name': { [key: string]: string };
    'generation'?: string | null;
    'manufactureStart'?: string | null;
    'manufactureEnd'?: string | null;
    'status': string;
    'sortOrder'?: number | null;
}
