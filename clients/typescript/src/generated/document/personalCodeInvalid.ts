export interface IPersonalCodeInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Document:PersonalCodeInvalid";
    'parameters': {
        reason: string;
    };
}

export function isPersonalCodeInvalid(arg: any): arg is IPersonalCodeInvalid {
    return arg && arg.errorName === "Document:PersonalCodeInvalid";
}
