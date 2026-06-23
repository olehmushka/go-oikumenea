export interface ILocaleCodeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Localization:LocaleCodeConflict";
    'parameters': {
        localeCode: string;
    };
}

export function isLocaleCodeConflict(arg: any): arg is ILocaleCodeConflict {
    return arg && arg.errorName === "Localization:LocaleCodeConflict";
}
