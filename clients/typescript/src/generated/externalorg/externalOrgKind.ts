/** An external-org kind (party/government_body/military/ngo/registrant/other). */
export interface IExternalOrgKind {
    'id': string;
    'code': string;
    /** Default-locale fallback + i18n translations. */
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
