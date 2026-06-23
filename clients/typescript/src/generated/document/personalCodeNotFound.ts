export interface IPersonalCodeNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Document:PersonalCodeNotFound";
    'parameters': {
        codeId: string;
    };
}

export function isPersonalCodeNotFound(arg: any): arg is IPersonalCodeNotFound {
    return arg && arg.errorName === "Document:PersonalCodeNotFound";
}
