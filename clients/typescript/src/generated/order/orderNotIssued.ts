export interface IOrderNotIssued {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Order:OrderNotIssued";
    'parameters': {
        orderId: string;
    };
}

export function isOrderNotIssued(arg: any): arg is IOrderNotIssued {
    return arg && arg.errorName === "Order:OrderNotIssued";
}
