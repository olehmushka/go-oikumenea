/** Create a person — no account and no unit required (L-AccountOptional / D-PersonGlobal). */
export interface ICreatePersonRequest {
    'code'?: string | null;
    'displayName': string;
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
    /** ISO/IEC 5218 value; defaults to not_known when omitted. */
    'sex'?: string | null;
    'countryOfBirth'?: string | null;
    'attributes'?: any | null;
}
