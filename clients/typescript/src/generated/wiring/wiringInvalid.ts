export interface IWiringInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Wiring:WiringInvalid";
    'parameters': {
        reason: string;
    };
}

export function isWiringInvalid(arg: any): arg is IWiringInvalid {
    return arg && arg.errorName === "Wiring:WiringInvalid";
}
