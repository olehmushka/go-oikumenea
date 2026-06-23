export interface IRefInUse {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Education:RefInUse";
    'parameters': {
        reason: string;
    };
}

export function isRefInUse(arg: any): arg is IRefInUse {
    return arg && arg.errorName === "Education:RefInUse";
}
