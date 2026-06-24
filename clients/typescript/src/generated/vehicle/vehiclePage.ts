import { IVehicle } from "./vehicle";

export interface IVehiclePage {
    'vehicles': Array<IVehicle>;
    'nextPageToken'?: string | null;
}
