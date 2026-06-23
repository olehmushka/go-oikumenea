/** An instance-admin catalog entry naming a PAPER kind (passport, driver-license). Stable code + translatable name. */
export interface IDocumentType {
    /** The document type's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic identifier (D-Code); immutable by convention. */
    'code': string;
    /** The translatable label as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /**
     * Optional per-type attribute schema (D-DocumentAttrSchema): when set, a document's
     * `attributes` is validated against it on write. Shape:
     * { "fields": { "<name>": { "type": "string|number|boolean|date", "required": bool, "enum": [...]? } } }.
     *
     */
    'attrSchema'?: any | null;
    /** One of active | retired. */
    'status': string;
    'sortOrder'?: number | null;
    'createdAt': string;
    'updatedAt': string;
}
