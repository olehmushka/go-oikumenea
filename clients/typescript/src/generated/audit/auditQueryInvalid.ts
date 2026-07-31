export interface IAuditQueryInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Audit:AuditQueryInvalid";
    'parameters': {
        reason: string;
    };
}

export function isAuditQueryInvalid(arg: any): arg is IAuditQueryInvalid {
    return arg && arg.errorName === "Audit:AuditQueryInvalid";
}
