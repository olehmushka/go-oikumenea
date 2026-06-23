export interface IDocumentTypeNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Document:DocumentTypeNotFound";
    'parameters': {
        typeId: string;
    };
}

export function isDocumentTypeNotFound(arg: any): arg is IDocumentTypeNotFound {
    return arg && arg.errorName === "Document:DocumentTypeNotFound";
}
