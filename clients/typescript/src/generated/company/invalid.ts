export interface IInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Company:Invalid";
    'parameters': {
        reason: string;
    };
}

export function isInvalid(arg: any): arg is IInvalid {
    return arg && arg.errorName === "Company:Invalid";
}
