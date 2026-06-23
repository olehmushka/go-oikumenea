/**
 * A reified person↔religion lay-affiliation link (link__affiliated_with). GDPR Art. 9 pii:special:
 * the optional free-form `value` belief detail is envelope-encrypted at rest (D-SpecialPII) and
 * returned decrypted to authorized readers (empty once crypto-erased on person purge). The
 * structural anchors (faith/tradition/community/type) are plain references.
 *
 */
export interface IAffiliation {
    'id': string;
    'personId': string;
    'religionId'?: string | null;
    'traditionUnitId'?: string | null;
    'communityUnitId'?: string | null;
    'affiliationTypeId': string;
    'affiliationTypeCode': string;
    'affiliationTypeName': { [key: string]: string };
    /** The decrypted free-form belief detail (pii:special); null when unset or crypto-erased. */
    'value'?: string | null;
    /** active | lapsed | renounced. */
    'status': string;
    'effectiveFrom': string;
    'effectiveTo'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
