export interface IServicePrincipalNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "ServicePrincipal:ServicePrincipalNotFound";
    'parameters': {
        principalId: string;
    };
}

export function isServicePrincipalNotFound(arg: any): arg is IServicePrincipalNotFound {
    return arg && arg.errorName === "ServicePrincipal:ServicePrincipalNotFound";
}
