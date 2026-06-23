export interface IOrderEffectFailed {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Order:OrderEffectFailed";
    'parameters': {
        reason: string;
    };
}

export function isOrderEffectFailed(arg: any): arg is IOrderEffectFailed {
    return arg && arg.errorName === "Order:OrderEffectFailed";
}
