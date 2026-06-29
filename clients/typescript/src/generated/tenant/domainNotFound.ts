export interface IDomainNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Tenant:DomainNotFound";
    'parameters': {
        domainId: string;
    };
}

export function isDomainNotFound(arg: any): arg is IDomainNotFound {
    return arg && arg.errorName === "Tenant:DomainNotFound";
}
