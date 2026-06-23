export interface IDocumentInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Document:DocumentInvalid";
    'parameters': {
        reason: string;
    };
}

export function isDocumentInvalid(arg: any): arg is IDocumentInvalid {
    return arg && arg.errorName === "Document:DocumentInvalid";
}
