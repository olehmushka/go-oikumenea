export interface IDomainCodeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Tenant:DomainCodeConflict";
    'parameters': {
        code: string;
    };
}

export function isDomainCodeConflict(arg: any): arg is IDomainCodeConflict {
    return arg && arg.errorName === "Tenant:DomainCodeConflict";
}
