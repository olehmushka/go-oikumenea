export interface IOrganizationCodeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Tenant:OrganizationCodeConflict";
    'parameters': {
        code: string;
    };
}

export function isOrganizationCodeConflict(arg: any): arg is IOrganizationCodeConflict {
    return arg && arg.errorName === "Tenant:OrganizationCodeConflict";
}
