export interface IQueryTooShort {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Search:QueryTooShort";
    'parameters': {
        minLength: number;
    };
}

export function isQueryTooShort(arg: any): arg is IQueryTooShort {
    return arg && arg.errorName === "Search:QueryTooShort";
}
