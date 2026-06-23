export interface IEnrollmentNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Education:EnrollmentNotFound";
    'parameters': {
        enrollmentId: string;
    };
}

export function isEnrollmentNotFound(arg: any): arg is IEnrollmentNotFound {
    return arg && arg.errorName === "Education:EnrollmentNotFound";
}
