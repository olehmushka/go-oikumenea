export interface IPersonalCodeDuplicate {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Document:PersonalCodeDuplicate";
    'parameters': {
        reason: string;
    };
}

export function isPersonalCodeDuplicate(arg: any): arg is IPersonalCodeDuplicate {
    return arg && arg.errorName === "Document:PersonalCodeDuplicate";
}
