export interface IConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Religion:Conflict";
    'parameters': {
        reason: string;
    };
}

export function isConflict(arg: any): arg is IConflict {
    return arg && arg.errorName === "Religion:Conflict";
}
