export interface IClassificationNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:ClassificationNotFound";
    'parameters': {
        classificationId: string;
    };
}

export function isClassificationNotFound(arg: any): arg is IClassificationNotFound {
    return arg && arg.errorName === "Religion:ClassificationNotFound";
}
