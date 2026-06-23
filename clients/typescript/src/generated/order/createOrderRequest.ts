import { IOrderItemInput } from "./orderItemInput";

/** Create a draft order for an issuing unit, with its items (≥1). */
export interface ICreateOrderRequest {
    'number'?: string | null;
    'issuedOn'?: string | null;
    'items': Array<IOrderItemInput>;
}
