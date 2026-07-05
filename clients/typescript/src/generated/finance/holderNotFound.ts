export interface IHolderNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Finance:HolderNotFound";
    'parameters': {
        holderId: string;
    };
}

export function isHolderNotFound(arg: any): arg is IHolderNotFound {
    return arg && arg.errorName === "Finance:HolderNotFound";
}
