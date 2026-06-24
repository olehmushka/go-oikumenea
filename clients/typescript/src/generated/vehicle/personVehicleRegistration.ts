/** A vehicle registration a person owns, enriched with the vehicle's identity (read-only). */
export interface IPersonVehicleRegistration {
    'id': string;
    'vehicleId': string;
    'vin'?: string | null;
    'typeLabel'?: string | null;
    'brandLabel'?: string | null;
    'modelLabel'?: string | null;
    'registrationNumber': string;
    'countryId': string;
    'subdivisionLabel'?: string | null;
    'status': string;
    'effectiveFrom': string;
    'effectiveTo'?: string | null;
}
