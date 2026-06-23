export interface IPersonalCodeSchemeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Document:PersonalCodeSchemeConflict";
    'parameters': {
        reason: string;
    };
}

export function isPersonalCodeSchemeConflict(arg: any): arg is IPersonalCodeSchemeConflict {
    return arg && arg.errorName === "Document:PersonalCodeSchemeConflict";
}
