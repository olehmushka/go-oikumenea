export interface ILocaleInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Localization:LocaleInvalid";
    'parameters': {
        reason: string;
    };
}

export function isLocaleInvalid(arg: any): arg is ILocaleInvalid {
    return arg && arg.errorName === "Localization:LocaleInvalid";
}
