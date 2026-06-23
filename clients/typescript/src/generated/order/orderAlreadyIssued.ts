export interface IOrderAlreadyIssued {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Order:OrderAlreadyIssued";
    'parameters': {
        orderId: string;
    };
}

export function isOrderAlreadyIssued(arg: any): arg is IOrderAlreadyIssued {
    return arg && arg.errorName === "Order:OrderAlreadyIssued";
}
