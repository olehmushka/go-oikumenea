export interface ISiteNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:SiteNotFound";
    'parameters': {
        siteId: string;
    };
}

export function isSiteNotFound(arg: any): arg is ISiteNotFound {
    return arg && arg.errorName === "Religion:SiteNotFound";
}
