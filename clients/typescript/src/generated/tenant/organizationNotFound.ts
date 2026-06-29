export interface IOrganizationNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Tenant:OrganizationNotFound";
    'parameters': {
        orgId: string;
    };
}

export function isOrganizationNotFound(arg: any): arg is IOrganizationNotFound {
    return arg && arg.errorName === "Tenant:OrganizationNotFound";
}
