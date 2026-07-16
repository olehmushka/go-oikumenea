/**
 * One argument of an action type, projected from its Conjure request field (D-ActionTypes, R-29).
 * Descriptive only — used for discoverability (the console action catalog), not write-time
 * validation.
 *
 */
export interface IActionParam {
    /** The request field name, e.g. subjectPersonId, scope. */
    'name': string;
    /** A display type token derived from the Conjure field type, e.g. string, rid, datetime, enum, list<string>. */
    'type': string;
    /** True when the field is non-optional in the request. */
    'required': boolean;
    /** The field's documentation from the contract, when present. */
    'docs'?: string | null;
}
