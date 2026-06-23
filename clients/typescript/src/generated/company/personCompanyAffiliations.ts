import { IBeneficiary } from "./beneficiary";
import { IFounding } from "./founding";
import { IPersonCompanyAppointment } from "./personCompanyAppointment";
import { IShareholding } from "./shareholding";

/** A person's company links — employment, founding, ownership, and ultimate-beneficiary records. */
export interface IPersonCompanyAffiliations {
    'appointments': Array<IPersonCompanyAppointment>;
    'foundings': Array<IFounding>;
    'shareholdings': Array<IShareholding>;
    'beneficiaryOf': Array<IBeneficiary>;
}
