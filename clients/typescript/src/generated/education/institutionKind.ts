/** An instance-admin catalog entry classifying an institution (university/school/…). */
export interface IInstitutionKind {
    'id': string;
    'code': string;
    /** Default-locale fallback + i18n translations. */
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
