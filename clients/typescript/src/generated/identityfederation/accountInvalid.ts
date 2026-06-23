export interface IAccountInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Account:AccountInvalid";
    'parameters': {
        reason: string;
    };
}

export function isAccountInvalid(arg: any): arg is IAccountInvalid {
    return arg && arg.errorName === "Account:AccountInvalid";
}
