export interface IQueryWindowRequired {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Location:QueryWindowRequired";
    'parameters': {
    };
}

export function isQueryWindowRequired(arg: any): arg is IQueryWindowRequired {
    return arg && arg.errorName === "Location:QueryWindowRequired";
}
