export interface IAssignmentConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Assignment:AssignmentConflict";
    'parameters': {
        reason: string;
    };
}

export function isAssignmentConflict(arg: any): arg is IAssignmentConflict {
    return arg && arg.errorName === "Assignment:AssignmentConflict";
}
