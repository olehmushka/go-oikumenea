/**
 * Register the vehicle to a new owner (person or company). Any currently-active registration for
 * the vehicle is closed first (registration is the ownership history), so this also performs a
 * transfer. subdivisionId (the plate region) must be a geo_places region when supplied.
 *
 */
export interface IRegisterVehicleRequest {
    'ownerKind': string;
    'ownerId': string;
    'countryId': string;
    'subdivisionId'?: string | null;
    'registrationNumber': string;
    'numberTypeId'?: string | null;
    'effectiveFrom'?: string | null;
}
