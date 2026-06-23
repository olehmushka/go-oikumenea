export interface IInstitutionNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Education:InstitutionNotFound";
    'parameters': {
        institutionId: string;
    };
}

export function isInstitutionNotFound(arg: any): arg is IInstitutionNotFound {
    return arg && arg.errorName === "Education:InstitutionNotFound";
}
