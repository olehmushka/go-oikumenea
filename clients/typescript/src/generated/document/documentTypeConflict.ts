export interface IDocumentTypeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Document:DocumentTypeConflict";
    'parameters': {
        reason: string;
    };
}

export function isDocumentTypeConflict(arg: any): arg is IDocumentTypeConflict {
    return arg && arg.errorName === "Document:DocumentTypeConflict";
}
