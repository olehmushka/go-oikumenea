export interface IOrderNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Order:OrderNotFound";
    'parameters': {
        orderId: string;
    };
}

export function isOrderNotFound(arg: any): arg is IOrderNotFound {
    return arg && arg.errorName === "Order:OrderNotFound";
}
