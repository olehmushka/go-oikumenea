export interface IAffiliationNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:AffiliationNotFound";
    'parameters': {
        affiliationId: string;
    };
}

export function isAffiliationNotFound(arg: any): arg is IAffiliationNotFound {
    return arg && arg.errorName === "Religion:AffiliationNotFound";
}
