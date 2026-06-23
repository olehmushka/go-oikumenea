export interface IUnknownLocale {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Localization:UnknownLocale";
    'parameters': {
        localeCode: string;
    };
}

export function isUnknownLocale(arg: any): arg is IUnknownLocale {
    return arg && arg.errorName === "Localization:UnknownLocale";
}
