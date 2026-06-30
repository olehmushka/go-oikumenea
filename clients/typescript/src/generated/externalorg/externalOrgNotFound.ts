export interface IExternalOrgNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "ExternalOrg:ExternalOrgNotFound";
    'parameters': {
        orgId: string;
    };
}

export function isExternalOrgNotFound(arg: any): arg is IExternalOrgNotFound {
    return arg && arg.errorName === "ExternalOrg:ExternalOrgNotFound";
}
