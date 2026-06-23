export interface IRefConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Education:RefConflict";
    'parameters': {
        reason: string;
    };
}

export function isRefConflict(arg: any): arg is IRefConflict {
    return arg && arg.errorName === "Education:RefConflict";
}
