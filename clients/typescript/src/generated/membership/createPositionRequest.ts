/** Create a billet in a unit (vacant). The title is the default-locale fallback; additional locales are set via LocalizationService. */
export interface ICreatePositionRequest {
    'code': string;
    /** The default-locale title (the i18n fallback); translatable separately. */
    'title': string;
    'requiredRankId'?: string | null;
    'sortOrder'?: number | null;
}
