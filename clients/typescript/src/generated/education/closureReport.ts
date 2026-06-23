/** The result of a closure verify/rebuild for an institution's unit tree. */
export interface IClosureReport {
    'institutionId': string;
    'missingCount': number;
    'extraCount': number;
    'inDrift': boolean;
}
