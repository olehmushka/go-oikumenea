export interface IOrganizationInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Tenant:OrganizationInvalid";
    'parameters': {
        reason: string;
    };
}

export function isOrganizationInvalid(arg: any): arg is IOrganizationInvalid {
    return arg && arg.errorName === "Tenant:OrganizationInvalid";
}
