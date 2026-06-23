export interface IRoleConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Role:RoleConflict";
    'parameters': {
        reason: string;
    };
}

export function isRoleConflict(arg: any): arg is IRoleConflict {
    return arg && arg.errorName === "Role:RoleConflict";
}
