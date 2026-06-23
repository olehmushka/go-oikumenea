export interface IRoleNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Role:RoleNotFound";
    'parameters': {
        roleId: string;
    };
}

export function isRoleNotFound(arg: any): arg is IRoleNotFound {
    return arg && arg.errorName === "Role:RoleNotFound";
}
