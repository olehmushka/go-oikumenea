import { IDomain } from "./domain";

/** The domain (org-kind) catalog, in display order. */
export interface IDomainList {
    'domains': Array<IDomain>;
}
