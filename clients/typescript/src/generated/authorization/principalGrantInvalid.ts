export interface IPrincipalGrantInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "PrincipalGrant:PrincipalGrantInvalid";
    'parameters': {
        reason: string;
    };
}

export function isPrincipalGrantInvalid(arg: any): arg is IPrincipalGrantInvalid {
    return arg && arg.errorName === "PrincipalGrant:PrincipalGrantInvalid";
}
