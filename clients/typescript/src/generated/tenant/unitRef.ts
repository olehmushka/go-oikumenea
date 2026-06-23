/** A lightweight unit reference with its closure depth (ancestor/descendant listings). */
export interface IUnitRef {
    'id': string;
    /** Optional human-readable code (absent for a codeless sub-unit; D-UnitCodeLifecycle). */
    'code'?: string | null;
    'name': { [key: string]: string };
    /** Closure distance from the queried unit (shortest path in the DAG). */
    'depth': number;
}
