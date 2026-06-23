export interface IProfileNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:ProfileNotFound";
    'parameters': {
        unitId: string;
    };
}

export function isProfileNotFound(arg: any): arg is IProfileNotFound {
    return arg && arg.errorName === "Religion:ProfileNotFound";
}
