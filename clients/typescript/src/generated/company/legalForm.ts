/** A per-country legal form (ТОВ/ПАТ/ФОП, LLC/JSC/GmbH …); countryId null for a generic form. */
export interface ILegalForm {
    'id': string;
    'code': string;
    /** Default-locale fallback + i18n translations. */
    'name': { [key: string]: string };
    'abbreviation'?: string | null;
    'countryId'?: string | null;
    'status': string;
    'sortOrder'?: number | null;
}
