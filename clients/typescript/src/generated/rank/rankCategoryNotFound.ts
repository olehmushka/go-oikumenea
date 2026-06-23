export interface IRankCategoryNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Rank:RankCategoryNotFound";
    'parameters': {
        categoryId: string;
    };
}

export function isRankCategoryNotFound(arg: any): arg is IRankCategoryNotFound {
    return arg && arg.errorName === "Rank:RankCategoryNotFound";
}
