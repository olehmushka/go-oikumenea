export interface ILanguoidInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Language:LanguoidInvalid";
    'parameters': {
        reason: string;
    };
}

export function isLanguoidInvalid(arg: any): arg is ILanguoidInvalid {
    return arg && arg.errorName === "Language:LanguoidInvalid";
}
