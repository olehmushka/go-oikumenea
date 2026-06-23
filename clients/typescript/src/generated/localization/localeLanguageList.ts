import { ILocaleLanguage } from "./localeLanguage";

/** The locale -> canonical-language links (D-Languages, M18). */
export interface ILocaleLanguageList {
    'localeLanguages': Array<ILocaleLanguage>;
}
