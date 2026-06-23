export interface IRankTypeNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Rank:RankTypeNotFound";
    'parameters': {
        typeId: string;
    };
}

export function isRankTypeNotFound(arg: any): arg is IRankTypeNotFound {
    return arg && arg.errorName === "Rank:RankTypeNotFound";
}
