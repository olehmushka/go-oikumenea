/**
 * Update a person's names (canonical + CLDR parts), birthdate, date_of_death, sex,
 * country_of_birth, and attributes. Omitted fields are unchanged; an empty string clears an
 * optional name part. `code` is immutable by convention and ranks are set via setRank (per system).
 *
 */
export interface IUpdatePersonRequest {
    'displayName'?: string | null;
    'title'?: string | null;
    'given'?: string | null;
    'given2'?: string | null;
    'surname'?: string | null;
    'surnamePrefix'?: string | null;
    'surname2'?: string | null;
    'generation'?: string | null;
    'credentials'?: string | null;
    'preferred'?: string | null;
    'birthdate'?: string | null;
    'dateOfDeath'?: string | null;
    'sex'?: string | null;
    'countryOfBirth'?: string | null;
    'attributes'?: any | null;
}
