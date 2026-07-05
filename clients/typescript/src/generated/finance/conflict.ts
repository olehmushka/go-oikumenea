export interface IConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Finance:Conflict";
    'parameters': {
        reason: string;
    };
}

export function isConflict(arg: any): arg is IConflict {
    return arg && arg.errorName === "Finance:Conflict";
}
