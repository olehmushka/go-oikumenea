export interface IIdentityInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Identity:IdentityInvalid";
    'parameters': {
        reason: string;
    };
}

export function isIdentityInvalid(arg: any): arg is IIdentityInvalid {
    return arg && arg.errorName === "Identity:IdentityInvalid";
}
