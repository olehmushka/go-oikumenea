export interface IPrincipalGrantNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "PrincipalGrant:PrincipalGrantNotFound";
    'parameters': {
        grantId: string;
    };
}

export function isPrincipalGrantNotFound(arg: any): arg is IPrincipalGrantNotFound {
    return arg && arg.errorName === "PrincipalGrant:PrincipalGrantNotFound";
}
