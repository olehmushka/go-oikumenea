export interface IIdentityConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Identity:IdentityConflict";
    'parameters': {
        reason: string;
    };
}

export function isIdentityConflict(arg: any): arg is IIdentityConflict {
    return arg && arg.errorName === "Identity:IdentityConflict";
}
