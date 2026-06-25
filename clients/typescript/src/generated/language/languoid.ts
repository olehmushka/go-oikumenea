/** A node in the Glottolog forest. `id` is the RID (the reference key); `code` is the stable glottocode; `name` is the locale->text display map. */
export interface ILanguoid {
    /** The languoid's RID (language service); what person/unit/locale links reference. */
    'id': string;
    /** Glottocode (8-char), the stable, locale-agnostic lookup key (e.g. stan1293). */
    'code': string;
    /** family | language | dialect. */
    'level': string;
    /** locale->text display name (all enabled locales; default-locale `name` column + i18n store). */
    'name': { [key: string]: string };
    /** The RID of the immediate parent languoid (absent for a top-level family / isolate). */
    'parentId'?: string | null;
    /** Whether this languoid has non-dialect children (family/language); lets a tree browser show an expand affordance only on expandable nodes. */
    'hasChildren': boolean;
    /** The root-family glottocode (derived via the closure). */
    'familyCode'?: string | null;
    /** ISO 639-3 code, when the languoid has one (families/dialects usually do not). */
    'iso6393'?: string | null;
    /** Glottolog macroarea. */
    'macroarea'?: string | null;
    /** AES endangerment (not_endangered…extinct). */
    'status': string;
}
