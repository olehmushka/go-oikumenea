export interface IInstanceAdminConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Authorization:InstanceAdminConflict";
    'parameters': {
        reason: string;
    };
}

export function isInstanceAdminConflict(arg: any): arg is IInstanceAdminConflict {
    return arg && arg.errorName === "Authorization:InstanceAdminConflict";
}
