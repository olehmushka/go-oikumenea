export interface IPrincipalGrantConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "PrincipalGrant:PrincipalGrantConflict";
    'parameters': {
        reason: string;
    };
}

export function isPrincipalGrantConflict(arg: any): arg is IPrincipalGrantConflict {
    return arg && arg.errorName === "PrincipalGrant:PrincipalGrantConflict";
}
