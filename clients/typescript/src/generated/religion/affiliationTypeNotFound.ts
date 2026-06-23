export interface IAffiliationTypeNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:AffiliationTypeNotFound";
    'parameters': {
        affiliationTypeId: string;
    };
}

export function isAffiliationTypeNotFound(arg: any): arg is IAffiliationTypeNotFound {
    return arg && arg.errorName === "Religion:AffiliationTypeNotFound";
}
