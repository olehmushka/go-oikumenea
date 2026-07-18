export interface IServicePrincipalConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "ServicePrincipal:ServicePrincipalConflict";
    'parameters': {
        reason: string;
    };
}

export function isServicePrincipalConflict(arg: any): arg is IServicePrincipalConflict {
    return arg && arg.errorName === "ServicePrincipal:ServicePrincipalConflict";
}
