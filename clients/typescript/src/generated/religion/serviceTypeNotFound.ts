export interface IServiceTypeNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:ServiceTypeNotFound";
    'parameters': {
        serviceTypeId: string;
    };
}

export function isServiceTypeNotFound(arg: any): arg is IServiceTypeNotFound {
    return arg && arg.errorName === "Religion:ServiceTypeNotFound";
}
