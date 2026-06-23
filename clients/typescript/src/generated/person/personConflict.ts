export interface IPersonConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Person:PersonConflict";
    'parameters': {
        reason: string;
    };
}

export function isPersonConflict(arg: any): arg is IPersonConflict {
    return arg && arg.errorName === "Person:PersonConflict";
}
