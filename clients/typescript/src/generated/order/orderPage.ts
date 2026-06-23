import { IOrder } from "./order";

/** A page of orders plus the opaque token for the next page (empty when exhausted). Items are omitted on list rows. */
export interface IOrderPage {
    'orders': Array<IOrder>;
    'nextPageToken'?: string | null;
}
