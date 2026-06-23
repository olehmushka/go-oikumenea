export interface IInUse {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Religion:InUse";
    'parameters': {
        reason: string;
    };
}

export function isInUse(arg: any): arg is IInUse {
    return arg && arg.errorName === "Religion:InUse";
}
