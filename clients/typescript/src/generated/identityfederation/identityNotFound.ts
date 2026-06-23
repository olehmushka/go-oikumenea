export interface IIdentityNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Identity:IdentityNotFound";
    'parameters': {
        identityId: string;
    };
}

export function isIdentityNotFound(arg: any): arg is IIdentityNotFound {
    return arg && arg.errorName === "Identity:IdentityNotFound";
}
