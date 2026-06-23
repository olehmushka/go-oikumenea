export interface IRoleInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Role:RoleInvalid";
    'parameters': {
        reason: string;
    };
}

export function isRoleInvalid(arg: any): arg is IRoleInvalid {
    return arg && arg.errorName === "Role:RoleInvalid";
}
