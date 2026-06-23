export interface IDocumentConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Document:DocumentConflict";
    'parameters': {
        reason: string;
    };
}

export function isDocumentConflict(arg: any): arg is IDocumentConflict {
    return arg && arg.errorName === "Document:DocumentConflict";
}
