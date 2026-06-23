export interface IPermissionDenied {
    'errorCode': "PERMISSION_DENIED";
    'errorInstanceId': string;
    'errorName': "Authorization:PermissionDenied";
    'parameters': {
        action: string;
    };
}

export function isPermissionDenied(arg: any): arg is IPermissionDenied {
    return arg && arg.errorName === "Authorization:PermissionDenied";
}
