export interface IOrderTypeNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Order:OrderTypeNotFound";
    'parameters': {
        typeId: string;
    };
}

export function isOrderTypeNotFound(arg: any): arg is IOrderTypeNotFound {
    return arg && arg.errorName === "Order:OrderTypeNotFound";
}
