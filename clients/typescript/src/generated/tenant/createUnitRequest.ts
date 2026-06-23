import { Visibility } from "./visibility";

/** Create a unit. `name` is the default-locale text; other locales are managed via LocalizationService. */
export interface ICreateUnitRequest {
    'code': string;
    'name': string;
    'unitKind'?: string | null;
    'level'?: number | null;
    /** Defaults to PUBLIC. */
    'visibility'?: Visibility | null;
    'metadata'?: any | null;
}
