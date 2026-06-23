import { IBeneficiary } from "./beneficiary";
import { IBranch } from "./branch";
import { IFounding } from "./founding";
import { IShareholding } from "./shareholding";
import { ISuccession } from "./succession";

/** The ownership/affiliation neighbourhood of one company (one hop in each direction). */
export interface IOwnershipGraph {
    'companyId': string;
    /** Stakes held IN this company. */
    'shareholders': Array<IShareholding>;
    /** Stakes this company holds in OTHER companies (subsidiaries via the ownership DAG). */
    'holdings': Array<IShareholding>;
    'beneficiaries': Array<IBeneficiary>;
    'founders': Array<IFounding>;
    /** Lineage where this company is the predecessor or the successor. */
    'successions': Array<ISuccession>;
    /** Branches of this company. */
    'branches': Array<IBranch>;
}
