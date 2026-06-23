export interface IGroupNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Education:GroupNotFound";
    'parameters': {
        groupId: string;
    };
}

export function isGroupNotFound(arg: any): arg is IGroupNotFound {
    return arg && arg.errorName === "Education:GroupNotFound";
}
