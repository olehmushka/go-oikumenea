import { IPersonVehicleRegistration } from "./personVehicleRegistration";

/** A person's vehicle registrations (current + historical, newest first). */
export interface IPersonVehicles {
    'registrations': Array<IPersonVehicleRegistration>;
}
