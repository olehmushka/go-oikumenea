/**
 * A reified person↔religion ordination/standing link (link__clergy_credential). A PUBLIC directory
 * fact, never an authorization input (parallel to D-Rank). Indelible where sacramental — revocation
 * is a status flip (active|suspended|revoked), never a hard delete.
 *
 */
export interface IClergyCredential {
    'id': string;
    'personId': string;
    'clergyGradeId': string;
    /** The grade's stable code (e.g. bishop/imam/rabbi). */
    'gradeCode': string;
    'gradeName': { [key: string]: string };
    /** The organization unit that conferred/recognizes the standing. */
    'orgUnitId': string;
    /** The date the standing was conferred, as a YYYY-MM-DD day string. */
    'grantedOn'?: string | null;
    'conferredByPersonId'?: string | null;
    /** active | suspended | revoked. */
    'status': string;
    'effectiveFrom': string;
    'effectiveTo'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
