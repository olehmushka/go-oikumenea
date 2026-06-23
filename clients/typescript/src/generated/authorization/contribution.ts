/** One reason an ALLOW was reached. instanceAdmin true means the instance plane; otherwise the assignment is named. */
export interface IContribution {
    'instanceAdmin': boolean;
    'assignmentId'?: string | null;
    'roleCode'?: string | null;
    'targetUnitId'?: string | null;
    'scope'?: string | null;
    'graphCode'?: string | null;
}
