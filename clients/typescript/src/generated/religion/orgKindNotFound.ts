export interface IOrgKindNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:OrgKindNotFound";
    'parameters': {
        orgKindId: string;
    };
}

export function isOrgKindNotFound(arg: any): arg is IOrgKindNotFound {
    return arg && arg.errorName === "Religion:OrgKindNotFound";
}
