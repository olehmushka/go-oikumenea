export interface ILanguoidNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Language:LanguoidNotFound";
    'parameters': {
        languoidId: string;
    };
}

export function isLanguoidNotFound(arg: any): arg is ILanguoidNotFound {
    return arg && arg.errorName === "Language:LanguoidNotFound";
}
