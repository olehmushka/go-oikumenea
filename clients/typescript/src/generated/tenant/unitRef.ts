/** A lightweight unit reference with its closure depth (ancestor/descendant listings). */
export interface IUnitRef {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    /** Closure distance from the queried unit (shortest path in the DAG). */
    'depth': number;
}
