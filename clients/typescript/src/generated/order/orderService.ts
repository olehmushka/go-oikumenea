import { ICreateOrderRequest } from "./createOrderRequest";
import { ICreateOrderTypeRequest } from "./createOrderTypeRequest";
import { IOrder } from "./order";
import { IOrderPage } from "./orderPage";
import { IOrderType } from "./orderType";
import { IRevokeOrderRequest } from "./revokeOrderRequest";
import { IUpdateOrderRequest } from "./updateOrderRequest";
import { IUpdateOrderTypeRequest } from "./updateOrderTypeRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Administrative orders (наказ) — the legal basis for status changes (D-Orders). Orders are unit-
 * scoped on issuing_unit_id (+ shadow gate); the order-type catalog is instance-admin-managed.
 * Issuing a draft auto-applies its structural effects in the same transaction via domain events
 * (D-OrderApply): all-or-nothing, so a violated target invariant rolls the whole issue back and the
 * order stays draft. Writes are audited in-process (D-Audit).
 *
 */
export interface IOrderService {
    /** Create a draft order (+ items) for an issuing unit. Returns Order:OrderConflict on a duplicate number. */
    createOrder(unitId: string, request: ICreateOrderRequest): Promise<IOrder>;
    /** Read one order with its items. */
    getOrder(orderId: string): Promise<IOrder>;
    /** Edit a draft order/items. Returns Order:OrderAlreadyIssued once issued. */
    updateOrder(orderId: string, request: IUpdateOrderRequest): Promise<IOrder>;
    /**
     * Issue a draft order: lock it and AUTO-APPLY its structural effects in the same transaction
     * (D-OrderApply). All-or-nothing — a violated target invariant rolls the whole issue back (the
     * order stays draft) and surfaces as Order:OrderEffectFailed. Returns Order:OrderAlreadyIssued
     * if not draft.
     *
     */
    issueOrder(orderId: string): Promise<IOrder>;
    /** Revoke an issued order (legal-status flip; effects are not auto-reversed). Returns Order:OrderNotIssued if not issued. */
    revokeOrder(orderId: string, request: IRevokeOrderRequest): Promise<IOrder>;
    /** List an issuing unit's orders (headers only), token-paginated. */
    listUnitOrders(unitId: string, pageSize?: number | null, pageToken?: string | null): Promise<IOrderPage>;
    /** List orders affecting a person (via their items), token-paginated. */
    listPersonOrders(personId: string, pageSize?: number | null, pageToken?: string | null): Promise<IOrderPage>;
    /** List the order-type catalog. */
    listOrderTypes(): Promise<Array<IOrderType>>;
    /** Add an order type (instance-scope). Returns Order:OrderTypeConflict if the code is taken. */
    createOrderType(request: ICreateOrderTypeRequest): Promise<IOrderType>;
    /** Edit an order type (instance-scope). `code`, `category`, and `effect` are immutable by convention. */
    updateOrderType(typeId: string, request: IUpdateOrderTypeRequest): Promise<IOrderType>;
}

export class OrderService implements IOrderService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Create a draft order (+ items) for an issuing unit. Returns Order:OrderConflict on a duplicate number. */
    public createOrder(unitId: string, request: ICreateOrderRequest): Promise<IOrder> {
        return this.bridge.call<IOrder>(
            "OrderService",
            "createOrder",
            "POST",
            "/order/v1/units/{unitId}/orders",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Read one order with its items. */
    public getOrder(orderId: string): Promise<IOrder> {
        return this.bridge.call<IOrder>(
            "OrderService",
            "getOrder",
            "GET",
            "/order/v1/orders/{orderId}",
            __undefined,
            __undefined,
            __undefined,
            [
                orderId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Edit a draft order/items. Returns Order:OrderAlreadyIssued once issued. */
    public updateOrder(orderId: string, request: IUpdateOrderRequest): Promise<IOrder> {
        return this.bridge.call<IOrder>(
            "OrderService",
            "updateOrder",
            "PUT",
            "/order/v1/orders/{orderId}",
            request,
            __undefined,
            __undefined,
            [
                orderId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Issue a draft order: lock it and AUTO-APPLY its structural effects in the same transaction
     * (D-OrderApply). All-or-nothing — a violated target invariant rolls the whole issue back (the
     * order stays draft) and surfaces as Order:OrderEffectFailed. Returns Order:OrderAlreadyIssued
     * if not draft.
     *
     */
    public issueOrder(orderId: string): Promise<IOrder> {
        return this.bridge.call<IOrder>(
            "OrderService",
            "issueOrder",
            "POST",
            "/order/v1/orders/{orderId}/issue",
            __undefined,
            __undefined,
            __undefined,
            [
                orderId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Revoke an issued order (legal-status flip; effects are not auto-reversed). Returns Order:OrderNotIssued if not issued. */
    public revokeOrder(orderId: string, request: IRevokeOrderRequest): Promise<IOrder> {
        return this.bridge.call<IOrder>(
            "OrderService",
            "revokeOrder",
            "POST",
            "/order/v1/orders/{orderId}/revoke",
            request,
            __undefined,
            __undefined,
            [
                orderId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List an issuing unit's orders (headers only), token-paginated. */
    public listUnitOrders(unitId: string, pageSize?: number | null, pageToken?: string | null): Promise<IOrderPage> {
        return this.bridge.call<IOrderPage>(
            "OrderService",
            "listUnitOrders",
            "GET",
            "/order/v1/units/{unitId}/orders",
            __undefined,
            __undefined,
            {
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List orders affecting a person (via their items), token-paginated. */
    public listPersonOrders(personId: string, pageSize?: number | null, pageToken?: string | null): Promise<IOrderPage> {
        return this.bridge.call<IOrderPage>(
            "OrderService",
            "listPersonOrders",
            "GET",
            "/order/v1/persons/{personId}/orders",
            __undefined,
            __undefined,
            {
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the order-type catalog. */
    public listOrderTypes(): Promise<Array<IOrderType>> {
        return this.bridge.call<Array<IOrderType>>(
            "OrderService",
            "listOrderTypes",
            "GET",
            "/order/v1/order-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add an order type (instance-scope). Returns Order:OrderTypeConflict if the code is taken. */
    public createOrderType(request: ICreateOrderTypeRequest): Promise<IOrderType> {
        return this.bridge.call<IOrderType>(
            "OrderService",
            "createOrderType",
            "POST",
            "/order/v1/order-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Edit an order type (instance-scope). `code`, `category`, and `effect` are immutable by convention. */
    public updateOrderType(typeId: string, request: IUpdateOrderTypeRequest): Promise<IOrderType> {
        return this.bridge.call<IOrderType>(
            "OrderService",
            "updateOrderType",
            "PUT",
            "/order/v1/order-types/{typeId}",
            request,
            __undefined,
            __undefined,
            [
                typeId,
            ],
            __undefined,
            __undefined
        );
    }
}
