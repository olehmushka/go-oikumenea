export interface ILinkNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Company:LinkNotFound";
    'parameters': {
        linkId: string;
    };
}

export function isLinkNotFound(arg: any): arg is ILinkNotFound {
    return arg && arg.errorName === "Company:LinkNotFound";
}
