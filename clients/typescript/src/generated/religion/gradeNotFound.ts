export interface IGradeNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:GradeNotFound";
    'parameters': {
        gradeId: string;
    };
}

export function isGradeNotFound(arg: any): arg is IGradeNotFound {
    return arg && arg.errorName === "Religion:GradeNotFound";
}
