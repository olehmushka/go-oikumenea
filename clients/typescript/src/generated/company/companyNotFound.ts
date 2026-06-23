export interface ICompanyNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Company:CompanyNotFound";
    'parameters': {
        companyId: string;
    };
}

export function isCompanyNotFound(arg: any): arg is ICompanyNotFound {
    return arg && arg.errorName === "Company:CompanyNotFound";
}
