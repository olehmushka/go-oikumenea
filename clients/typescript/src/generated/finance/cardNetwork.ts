/** A payment-card network (visa/mastercard/amex…); instance-extensible. */
export interface ICardNetwork {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
