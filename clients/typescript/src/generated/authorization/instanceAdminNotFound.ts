export interface IInstanceAdminNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Authorization:InstanceAdminNotFound";
    'parameters': {
        instanceAdminId: string;
    };
}

export function isInstanceAdminNotFound(arg: any): arg is IInstanceAdminNotFound {
    return arg && arg.errorName === "Authorization:InstanceAdminNotFound";
}
