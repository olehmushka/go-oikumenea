export interface IRefInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Education:RefInvalid";
    'parameters': {
        reason: string;
    };
}

export function isRefInvalid(arg: any): arg is IRefInvalid {
    return arg && arg.errorName === "Education:RefInvalid";
}
