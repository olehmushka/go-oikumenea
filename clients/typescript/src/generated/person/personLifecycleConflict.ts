export interface IPersonLifecycleConflict {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Person:PersonLifecycleConflict";
    'parameters': {
        reason: string;
    };
}

export function isPersonLifecycleConflict(arg: any): arg is IPersonLifecycleConflict {
    return arg && arg.errorName === "Person:PersonLifecycleConflict";
}
