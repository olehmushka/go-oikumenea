export interface ILocaleConstraintViolation {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Localization:LocaleConstraintViolation";
    'parameters': {
        reason: string;
    };
}

export function isLocaleConstraintViolation(arg: any): arg is ILocaleConstraintViolation {
    return arg && arg.errorName === "Localization:LocaleConstraintViolation";
}
