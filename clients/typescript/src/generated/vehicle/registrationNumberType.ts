/** A plate-type catalog entry (regular/temporary/transit/diplomatic/military/old…). */
export interface IRegistrationNumberType {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
