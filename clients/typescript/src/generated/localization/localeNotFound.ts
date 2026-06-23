export interface ILocaleNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Localization:LocaleNotFound";
    'parameters': {
        localeCode: string;
    };
}

export function isLocaleNotFound(arg: any): arg is ILocaleNotFound {
    return arg && arg.errorName === "Localization:LocaleNotFound";
}
