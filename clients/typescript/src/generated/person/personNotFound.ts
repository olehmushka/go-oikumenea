export interface IPersonNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Person:PersonNotFound";
    'parameters': {
        personId: string;
    };
}

export function isPersonNotFound(arg: any): arg is IPersonNotFound {
    return arg && arg.errorName === "Person:PersonNotFound";
}
