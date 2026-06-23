export interface IPrerequisiteCycle {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Education:PrerequisiteCycle";
    'parameters': {
        courseId: string;
    };
}

export function isPrerequisiteCycle(arg: any): arg is IPrerequisiteCycle {
    return arg && arg.errorName === "Education:PrerequisiteCycle";
}
