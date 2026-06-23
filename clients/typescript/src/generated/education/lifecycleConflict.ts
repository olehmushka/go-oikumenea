export interface ILifecycleConflict {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Education:LifecycleConflict";
    'parameters': {
        reason: string;
    };
}

export function isLifecycleConflict(arg: any): arg is ILifecycleConflict {
    return arg && arg.errorName === "Education:LifecycleConflict";
}
