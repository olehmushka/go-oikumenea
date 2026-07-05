export interface ICardNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Finance:CardNotFound";
    'parameters': {
        cardId: string;
    };
}

export function isCardNotFound(arg: any): arg is ICardNotFound {
    return arg && arg.errorName === "Finance:CardNotFound";
}
