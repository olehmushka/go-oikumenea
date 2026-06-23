export interface IAssignmentInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Assignment:AssignmentInvalid";
    'parameters': {
        reason: string;
    };
}

export function isAssignmentInvalid(arg: any): arg is IAssignmentInvalid {
    return arg && arg.errorName === "Assignment:AssignmentInvalid";
}
