/** A vehicle taxonomy node (car/truck/motorcycle…); a shallow tree (parentId + denormalized rootId). */
export interface IVehicleType {
    'id': string;
    'code': string;
    /** Default-locale fallback + i18n translations. */
    'name': { [key: string]: string };
    'parentId'?: string | null;
    'rootId'?: string | null;
    'status': string;
    'sortOrder'?: number | null;
}
