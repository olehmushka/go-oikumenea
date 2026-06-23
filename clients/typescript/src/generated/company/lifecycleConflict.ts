export interface ILifecycleConflict {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Company:LifecycleConflict";
    'parameters': {
        reason: string;
    };
}

export function isLifecycleConflict(arg: any): arg is ILifecycleConflict {
    return arg && arg.errorName === "Company:LifecycleConflict";
}
