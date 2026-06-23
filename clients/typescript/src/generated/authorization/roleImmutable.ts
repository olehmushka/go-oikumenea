export interface IRoleImmutable {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Role:RoleImmutable";
    'parameters': {
        roleId: string;
    };
}

export function isRoleImmutable(arg: any): arg is IRoleImmutable {
    return arg && arg.errorName === "Role:RoleImmutable";
}
