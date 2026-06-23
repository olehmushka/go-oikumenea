export interface IRankNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Rank:RankNotFound";
    'parameters': {
        rankId: string;
    };
}

export function isRankNotFound(arg: any): arg is IRankNotFound {
    return arg && arg.errorName === "Rank:RankNotFound";
}
