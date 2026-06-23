import { IAddLocaleRequest } from "./addLocaleRequest";
import { ILocale } from "./locale";
import { ILocaleLanguageList } from "./localeLanguageList";
import { ILocaleList } from "./localeList";
import { IUpdateLocaleRequest } from "./updateLocaleRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The supported-locale registry and the polymorphic translation store (D-i18n). Reads are gated
 * by `locale.read`/`translation.read`; writes are instance-admin (`locale.manage`/
 * `translation.manage`) — enforced once authorization lands (M7).
 *
 */
export interface ILocalizationService {
    /** List the supported locales in display order. */
    listLocales(): Promise<ILocaleList>;
    /**
     * List each supported locale's canonical Glottolog language (D-Languages, M18). Read-only — the
     * links are reconciled by the language-scheme import, not edited here.
     *
     */
    listLocaleLanguages(): Promise<ILocaleLanguageList>;
    /** Add a supported locale. Returns Localization:LocaleCodeConflict if the code exists. */
    addLocale(request: IAddLocaleRequest): Promise<ILocale>;
    /** Enable/disable, rename, set default, or reorder a locale. */
    updateLocale(localeCode: string, request: IUpdateLocaleRequest): Promise<ILocale>;
    /** All translations of one entity, as field -> (locale -> text), for editing. */
    getTranslations(entityType: string, entityId: string): Promise<{ [key: string]: { [key: string]: string } }>;
    /** Upsert translations for one entity (one or many fields/locales). Returns the stored set. */
    putTranslations(entityType: string, entityId: string, translations: { [key: string]: { [key: string]: string } }): Promise<{ [key: string]: { [key: string]: string } }>;
}

export class LocalizationService implements ILocalizationService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** List the supported locales in display order. */
    public listLocales(): Promise<ILocaleList> {
        return this.bridge.call<ILocaleList>(
            "LocalizationService",
            "listLocales",
            "GET",
            "/localization/v1/locales",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * List each supported locale's canonical Glottolog language (D-Languages, M18). Read-only — the
     * links are reconciled by the language-scheme import, not edited here.
     *
     */
    public listLocaleLanguages(): Promise<ILocaleLanguageList> {
        return this.bridge.call<ILocaleLanguageList>(
            "LocalizationService",
            "listLocaleLanguages",
            "GET",
            "/localization/v1/locale-languages",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add a supported locale. Returns Localization:LocaleCodeConflict if the code exists. */
    public addLocale(request: IAddLocaleRequest): Promise<ILocale> {
        return this.bridge.call<ILocale>(
            "LocalizationService",
            "addLocale",
            "POST",
            "/localization/v1/locales",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Enable/disable, rename, set default, or reorder a locale. */
    public updateLocale(localeCode: string, request: IUpdateLocaleRequest): Promise<ILocale> {
        return this.bridge.call<ILocale>(
            "LocalizationService",
            "updateLocale",
            "PUT",
            "/localization/v1/locales/{localeCode}",
            request,
            __undefined,
            __undefined,
            [
                localeCode,
            ],
            __undefined,
            __undefined
        );
    }

    /** All translations of one entity, as field -> (locale -> text), for editing. */
    public getTranslations(entityType: string, entityId: string): Promise<{ [key: string]: { [key: string]: string } }> {
        return this.bridge.call<{ [key: string]: { [key: string]: string } }>(
            "LocalizationService",
            "getTranslations",
            "GET",
            "/localization/v1/translations/{entityType}/{entityId}",
            __undefined,
            __undefined,
            __undefined,
            [
                entityType,
                entityId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Upsert translations for one entity (one or many fields/locales). Returns the stored set. */
    public putTranslations(entityType: string, entityId: string, translations: { [key: string]: { [key: string]: string } }): Promise<{ [key: string]: { [key: string]: string } }> {
        return this.bridge.call<{ [key: string]: { [key: string]: string } }>(
            "LocalizationService",
            "putTranslations",
            "PUT",
            "/localization/v1/translations/{entityType}/{entityId}",
            translations,
            __undefined,
            __undefined,
            [
                entityType,
                entityId,
            ],
            __undefined,
            __undefined
        );
    }
}
