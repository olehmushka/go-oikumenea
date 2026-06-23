export interface ISiteTypeNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:SiteTypeNotFound";
    'parameters': {
        siteTypeId: string;
    };
}

export function isSiteTypeNotFound(arg: any): arg is ISiteTypeNotFound {
    return arg && arg.errorName === "Religion:SiteTypeNotFound";
}
