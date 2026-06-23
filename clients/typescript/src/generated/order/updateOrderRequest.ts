import { IOrderItemInput } from "./orderItemInput";

/** Edit a draft order. Omitted scalar fields are unchanged; when `items` is present it REPLACES the draft's items. Rejected once issued. */
export interface IUpdateOrderRequest {
    'number'?: string | null;
    'issuedOn'?: string | null;
    'items'?: Array<IOrderItemInput> | null;
}
