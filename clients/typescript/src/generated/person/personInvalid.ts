export interface IPersonInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Person:PersonInvalid";
    'parameters': {
        reason: string;
    };
}

export function isPersonInvalid(arg: any): arg is IPersonInvalid {
    return arg && arg.errorName === "Person:PersonInvalid";
}
