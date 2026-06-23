export interface IEnvelopeMismatch {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "DataImport:EnvelopeMismatch";
    'parameters': {
        pathObjectType: string;
        bodyObjectType: string;
    };
}

export function isEnvelopeMismatch(arg: any): arg is IEnvelopeMismatch {
    return arg && arg.errorName === "DataImport:EnvelopeMismatch";
}
