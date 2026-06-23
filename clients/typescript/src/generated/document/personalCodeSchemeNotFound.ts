export interface IPersonalCodeSchemeNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Document:PersonalCodeSchemeNotFound";
    'parameters': {
        schemeCode: string;
    };
}

export function isPersonalCodeSchemeNotFound(arg: any): arg is IPersonalCodeSchemeNotFound {
    return arg && arg.errorName === "Document:PersonalCodeSchemeNotFound";
}
