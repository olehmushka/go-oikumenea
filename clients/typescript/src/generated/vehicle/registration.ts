/**
 * The ownership+plate record (link__registered_to). The owner is a person OR a company
 * (polymorphic). country/subdivision carry the registering country + optional plate region. A
 * re-registration is a new row (the prior closed), so registration IS the ownership history.
 *
 */
export interface IRegistration {
    'id': string;
    'vehicleId': string;
    /** One of person | company. */
    'ownerKind': string;
    'ownerId': string;
    /** Best-effort display label (company legal name for company owners; empty for persons). */
    'ownerLabel'?: string | null;
    'countryId': string;
    /** The plate region — a geo_places RID (placetype=region). */
    'subdivisionId'?: string | null;
    'subdivisionLabel'?: string | null;
    'registrationNumber': string;
    'numberTypeId'?: string | null;
    /** One of active | closed. */
    'status': string;
    'effectiveFrom': string;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
