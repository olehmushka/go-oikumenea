export interface IBuildingNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Education:BuildingNotFound";
    'parameters': {
        buildingId: string;
    };
}

export function isBuildingNotFound(arg: any): arg is IBuildingNotFound {
    return arg && arg.errorName === "Education:BuildingNotFound";
}
