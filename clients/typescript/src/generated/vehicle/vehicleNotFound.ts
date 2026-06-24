export interface IVehicleNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Vehicle:VehicleNotFound";
    'parameters': {
        vehicleId: string;
    };
}

export function isVehicleNotFound(arg: any): arg is IVehicleNotFound {
    return arg && arg.errorName === "Vehicle:VehicleNotFound";
}
