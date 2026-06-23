export interface IOrderTypeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Order:OrderTypeConflict";
    'parameters': {
        reason: string;
    };
}

export function isOrderTypeConflict(arg: any): arg is IOrderTypeConflict {
    return arg && arg.errorName === "Order:OrderTypeConflict";
}
