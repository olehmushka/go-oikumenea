export interface IRankSystemNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Rank:RankSystemNotFound";
    'parameters': {
        systemId: string;
    };
}

export function isRankSystemNotFound(arg: any): arg is IRankSystemNotFound {
    return arg && arg.errorName === "Rank:RankSystemNotFound";
}
