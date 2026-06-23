export interface IOrderConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Order:OrderConflict";
    'parameters': {
        reason: string;
    };
}

export function isOrderConflict(arg: any): arg is IOrderConflict {
    return arg && arg.errorName === "Order:OrderConflict";
}
