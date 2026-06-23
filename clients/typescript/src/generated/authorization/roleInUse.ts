export interface IRoleInUse {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Role:RoleInUse";
    'parameters': {
        roleId: string;
    };
}

export function isRoleInUse(arg: any): arg is IRoleInUse {
    return arg && arg.errorName === "Role:RoleInUse";
}
