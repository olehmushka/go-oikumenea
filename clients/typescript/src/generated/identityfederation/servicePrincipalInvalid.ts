export interface IServicePrincipalInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "ServicePrincipal:ServicePrincipalInvalid";
    'parameters': {
        reason: string;
    };
}

export function isServicePrincipalInvalid(arg: any): arg is IServicePrincipalInvalid {
    return arg && arg.errorName === "ServicePrincipal:ServicePrincipalInvalid";
}
