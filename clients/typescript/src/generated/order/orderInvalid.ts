export interface IOrderInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Order:OrderInvalid";
    'parameters': {
        reason: string;
    };
}

export function isOrderInvalid(arg: any): arg is IOrderInvalid {
    return arg && arg.errorName === "Order:OrderInvalid";
}
