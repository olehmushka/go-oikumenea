export interface IInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Location:Invalid";
    'parameters': {
        reason: string;
    };
}

export function isInvalid(arg: any): arg is IInvalid {
    return arg && arg.errorName === "Location:Invalid";
}
