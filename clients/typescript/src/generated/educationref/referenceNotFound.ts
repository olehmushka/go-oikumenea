export interface IReferenceNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Education:ReferenceNotFound";
    'parameters': {
        entity: string;
        id: string;
    };
}

export function isReferenceNotFound(arg: any): arg is IReferenceNotFound {
    return arg && arg.errorName === "Education:ReferenceNotFound";
}
