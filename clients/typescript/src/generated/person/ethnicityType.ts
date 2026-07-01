/**
 * A declared-ethnicity vocabulary entry in an instance-admin-managed, HIERARCHICAL taxonomy
 * (D-PhysicalIdentity amendment, M43). Stable code + translatable name + optional parent (tree
 * structure). `languages`/`countries` are GROUP-level ethnolinguistic + homeland associations
 * (reference metadata about the group; NEVER a person's datum — a person's ethnicity and
 * languages stay independent). This catalog is plaintext; a person's SELECTION is encrypted.
 *
 */
export interface IEthnicityType {
    'id': string;
    /** Stable, locale-agnostic identifier (D-Code). */
    'code': string;
    /** The translatable label as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /** The RID of the immediate parent group (absent for forest roots). */
    'parentId'?: string | null;
    /** Whether this group has children (lets a tree browser show the expand affordance). */
    'hasChildren': boolean;
    /** Wikidata Q-id anchor (external linkage; from the opt-in import). */
    'wikidataId'?: string | null;
    /** One of active | retired. */
    'status': string;
    'sortOrder'?: number | null;
    /** Associated-language RIDs (Glottolog languoids) — group-level, populated on getEthnicityType. */
    'languages': Array<string>;
    /** Homeland-country RIDs (geo_countries) — group-level, populated on getEthnicityType. */
    'countries': Array<string>;
}
