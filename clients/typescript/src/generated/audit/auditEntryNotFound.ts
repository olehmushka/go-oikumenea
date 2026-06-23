export interface IAuditEntryNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Audit:AuditEntryNotFound";
    'parameters': {
        auditEntryId: string;
    };
}

export function isAuditEntryNotFound(arg: any): arg is IAuditEntryNotFound {
    return arg && arg.errorName === "Audit:AuditEntryNotFound";
}
