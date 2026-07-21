/** A GDPR lawful-basis kind. Code-keyed — no RID (it is a resolve non-target). */
export interface ILegalBasisEntry {
    'code': string;
    'name': string;
    /** art6 | art9. */
    'article': string;
    'status': string;
}
