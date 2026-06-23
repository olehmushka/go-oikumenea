/** The result of a taxonomy closure verify/rebuild. */
export interface IClosureReport {
    'missingCount': number;
    'extraCount': number;
    'inDrift': boolean;
}
