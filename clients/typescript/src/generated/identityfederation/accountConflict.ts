export interface IAccountConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Account:AccountConflict";
    'parameters': {
        reason: string;
    };
}

export function isAccountConflict(arg: any): arg is IAccountConflict {
    return arg && arg.errorName === "Account:AccountConflict";
}
