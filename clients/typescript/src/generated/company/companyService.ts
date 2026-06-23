import { IAddCompanyLocationRequest } from "./addCompanyLocationRequest";
import { IAppointment } from "./appointment";
import { IAssignIndustryRequest } from "./assignIndustryRequest";
import { IBeneficiary } from "./beneficiary";
import { IBranch } from "./branch";
import { ICompany } from "./company";
import { ICompanyLocation } from "./companyLocation";
import { ICompanyLocationList } from "./companyLocationList";
import { ICompanyPage } from "./companyPage";
import { ICompanyPosition } from "./companyPosition";
import { ICompanyPositionPage } from "./companyPositionPage";
import { ICreateCompanyRequest } from "./createCompanyRequest";
import { ICreatePositionRequest } from "./createPositionRequest";
import { IEndAppointmentRequest } from "./endAppointmentRequest";
import { IFillPositionRequest } from "./fillPositionRequest";
import { IFounding } from "./founding";
import { IIndustryAssignment } from "./industryAssignment";
import { IIndustryAssignmentList } from "./industryAssignmentList";
import { IIndustryClass } from "./industryClass";
import { IIndustryClassList } from "./industryClassList";
import { ILegalForm } from "./legalForm";
import { ILegalFormList } from "./legalFormList";
import { IOwnershipGraph } from "./ownershipGraph";
import { IPersonCompanyAffiliations } from "./personCompanyAffiliations";
import { IRecordBeneficiaryRequest } from "./recordBeneficiaryRequest";
import { IRecordBranchRequest } from "./recordBranchRequest";
import { IRecordFoundingRequest } from "./recordFoundingRequest";
import { IRecordShareholdingRequest } from "./recordShareholdingRequest";
import { IRecordSuccessionRequest } from "./recordSuccessionRequest";
import { IRegistration } from "./registration";
import { IRegistrationList } from "./registrationList";
import { IRegistrationScheme } from "./registrationScheme";
import { IRegistrationSchemeList } from "./registrationSchemeList";
import { IShareholding } from "./shareholding";
import { ISuccession } from "./succession";
import { IUpdateCompanyRequest } from "./updateCompanyRequest";
import { IUpdatePositionRequest } from "./updatePositionRequest";
import { IUpsertIndustryClassRequest } from "./upsertIndustryClassRequest";
import { IUpsertLegalFormRequest } from "./upsertLegalFormRequest";
import { IUpsertRegistrationRequest } from "./upsertRegistrationRequest";
import { IUpsertSchemeRequest } from "./upsertSchemeRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * A legal-entity registry: companies, registrations, industries, locations, positions/appointments,
 * and the ownership/affiliation graph. Reads gate on `company.read`; company/registration/industry/
 * location/ownership writes on `company.manage`; position/appointment writes on
 * `company.position.manage`; catalog-entry writes on `company.catalog.manage`. Writes are audited
 * in-process (D-Audit).
 *
 */
export interface ICompanyService {
    listLegalForms(): Promise<ILegalFormList>;
    upsertLegalForm(request: IUpsertLegalFormRequest): Promise<ILegalForm>;
    listRegistrationSchemes(): Promise<IRegistrationSchemeList>;
    upsertRegistrationScheme(request: IUpsertSchemeRequest): Promise<IRegistrationScheme>;
    listIndustryClasses(): Promise<IIndustryClassList>;
    upsertIndustryClass(request: IUpsertIndustryClassRequest): Promise<IIndustryClass>;
    createCompany(request: ICreateCompanyRequest): Promise<ICompany>;
    listCompanies(query?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ICompanyPage>;
    getCompany(companyId: string): Promise<ICompany>;
    updateCompany(companyId: string, request: IUpdateCompanyRequest): Promise<ICompany>;
    deleteCompany(companyId: string): Promise<void>;
    listRegistrations(companyId: string): Promise<IRegistrationList>;
    addRegistration(companyId: string, request: IUpsertRegistrationRequest): Promise<IRegistration>;
    updateRegistration(registrationId: string, request: IUpsertRegistrationRequest): Promise<IRegistration>;
    deleteRegistration(registrationId: string): Promise<void>;
    listIndustries(companyId: string): Promise<IIndustryAssignmentList>;
    assignIndustry(companyId: string, request: IAssignIndustryRequest): Promise<IIndustryAssignment>;
    removeIndustry(assignmentId: string): Promise<void>;
    listCompanyLocations(companyId: string): Promise<ICompanyLocationList>;
    addCompanyLocation(companyId: string, request: IAddCompanyLocationRequest): Promise<ICompanyLocation>;
    removeCompanyLocation(companyLocationId: string): Promise<void>;
    createPosition(companyId: string, request: ICreatePositionRequest): Promise<ICompanyPosition>;
    listPositions(companyId: string, state?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ICompanyPositionPage>;
    getPosition(positionId: string): Promise<ICompanyPosition>;
    updatePosition(positionId: string, request: IUpdatePositionRequest): Promise<ICompanyPosition>;
    abolishPosition(positionId: string): Promise<ICompanyPosition>;
    /** Fill a vacant position. Returns Company:PositionAlreadyFilled if already filled. */
    fillPosition(positionId: string, request: IFillPositionRequest): Promise<IAppointment>;
    endAppointment(appointmentId: string, request: IEndAppointmentRequest): Promise<IAppointment>;
    getOwnershipGraph(companyId: string): Promise<IOwnershipGraph>;
    recordFounding(companyId: string, request: IRecordFoundingRequest): Promise<IFounding>;
    removeFounding(foundingId: string): Promise<void>;
    recordShareholding(companyId: string, request: IRecordShareholdingRequest): Promise<IShareholding>;
    removeShareholding(shareholdingId: string): Promise<void>;
    recordBeneficiary(companyId: string, request: IRecordBeneficiaryRequest): Promise<IBeneficiary>;
    removeBeneficiary(beneficiaryId: string): Promise<void>;
    recordSuccession(companyId: string, request: IRecordSuccessionRequest): Promise<ISuccession>;
    removeSuccession(successionId: string): Promise<void>;
    recordBranch(companyId: string, request: IRecordBranchRequest): Promise<IBranch>;
    removeBranch(branchId: string): Promise<void>;
    /** Read-only view of a person's company links (employment, founding, ownership, beneficiary-of). */
    listPersonCompanyAffiliations(personId: string): Promise<IPersonCompanyAffiliations>;
}

export class CompanyService implements ICompanyService {
    constructor(private bridge: IHttpApiBridge) {
    }

    public listLegalForms(): Promise<ILegalFormList> {
        return this.bridge.call<ILegalFormList>(
            "CompanyService",
            "listLegalForms",
            "GET",
            "/company/v1/legal-forms",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertLegalForm(request: IUpsertLegalFormRequest): Promise<ILegalForm> {
        return this.bridge.call<ILegalForm>(
            "CompanyService",
            "upsertLegalForm",
            "PUT",
            "/company/v1/legal-forms",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listRegistrationSchemes(): Promise<IRegistrationSchemeList> {
        return this.bridge.call<IRegistrationSchemeList>(
            "CompanyService",
            "listRegistrationSchemes",
            "GET",
            "/company/v1/registration-schemes",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertRegistrationScheme(request: IUpsertSchemeRequest): Promise<IRegistrationScheme> {
        return this.bridge.call<IRegistrationScheme>(
            "CompanyService",
            "upsertRegistrationScheme",
            "PUT",
            "/company/v1/registration-schemes",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listIndustryClasses(): Promise<IIndustryClassList> {
        return this.bridge.call<IIndustryClassList>(
            "CompanyService",
            "listIndustryClasses",
            "GET",
            "/company/v1/industry-classes",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertIndustryClass(request: IUpsertIndustryClassRequest): Promise<IIndustryClass> {
        return this.bridge.call<IIndustryClass>(
            "CompanyService",
            "upsertIndustryClass",
            "PUT",
            "/company/v1/industry-classes",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public createCompany(request: ICreateCompanyRequest): Promise<ICompany> {
        return this.bridge.call<ICompany>(
            "CompanyService",
            "createCompany",
            "POST",
            "/company/v1/companies",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listCompanies(query?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ICompanyPage> {
        return this.bridge.call<ICompanyPage>(
            "CompanyService",
            "listCompanies",
            "GET",
            "/company/v1/companies",
            __undefined,
            __undefined,
            {
                "query": query,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public getCompany(companyId: string): Promise<ICompany> {
        return this.bridge.call<ICompany>(
            "CompanyService",
            "getCompany",
            "GET",
            "/company/v1/companies/{companyId}",
            __undefined,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateCompany(companyId: string, request: IUpdateCompanyRequest): Promise<ICompany> {
        return this.bridge.call<ICompany>(
            "CompanyService",
            "updateCompany",
            "PUT",
            "/company/v1/companies/{companyId}",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteCompany(companyId: string): Promise<void> {
        return this.bridge.call<void>(
            "CompanyService",
            "deleteCompany",
            "DELETE",
            "/company/v1/companies/{companyId}",
            __undefined,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public listRegistrations(companyId: string): Promise<IRegistrationList> {
        return this.bridge.call<IRegistrationList>(
            "CompanyService",
            "listRegistrations",
            "GET",
            "/company/v1/companies/{companyId}/registrations",
            __undefined,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public addRegistration(companyId: string, request: IUpsertRegistrationRequest): Promise<IRegistration> {
        return this.bridge.call<IRegistration>(
            "CompanyService",
            "addRegistration",
            "POST",
            "/company/v1/companies/{companyId}/registrations",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateRegistration(registrationId: string, request: IUpsertRegistrationRequest): Promise<IRegistration> {
        return this.bridge.call<IRegistration>(
            "CompanyService",
            "updateRegistration",
            "PUT",
            "/company/v1/registrations/{registrationId}",
            request,
            __undefined,
            __undefined,
            [
                registrationId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteRegistration(registrationId: string): Promise<void> {
        return this.bridge.call<void>(
            "CompanyService",
            "deleteRegistration",
            "DELETE",
            "/company/v1/registrations/{registrationId}",
            __undefined,
            __undefined,
            __undefined,
            [
                registrationId,
            ],
            __undefined,
            __undefined
        );
    }

    public listIndustries(companyId: string): Promise<IIndustryAssignmentList> {
        return this.bridge.call<IIndustryAssignmentList>(
            "CompanyService",
            "listIndustries",
            "GET",
            "/company/v1/companies/{companyId}/industries",
            __undefined,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public assignIndustry(companyId: string, request: IAssignIndustryRequest): Promise<IIndustryAssignment> {
        return this.bridge.call<IIndustryAssignment>(
            "CompanyService",
            "assignIndustry",
            "POST",
            "/company/v1/companies/{companyId}/industries",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeIndustry(assignmentId: string): Promise<void> {
        return this.bridge.call<void>(
            "CompanyService",
            "removeIndustry",
            "DELETE",
            "/company/v1/industries/{assignmentId}",
            __undefined,
            __undefined,
            __undefined,
            [
                assignmentId,
            ],
            __undefined,
            __undefined
        );
    }

    public listCompanyLocations(companyId: string): Promise<ICompanyLocationList> {
        return this.bridge.call<ICompanyLocationList>(
            "CompanyService",
            "listCompanyLocations",
            "GET",
            "/company/v1/companies/{companyId}/locations",
            __undefined,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public addCompanyLocation(companyId: string, request: IAddCompanyLocationRequest): Promise<ICompanyLocation> {
        return this.bridge.call<ICompanyLocation>(
            "CompanyService",
            "addCompanyLocation",
            "POST",
            "/company/v1/companies/{companyId}/locations",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeCompanyLocation(companyLocationId: string): Promise<void> {
        return this.bridge.call<void>(
            "CompanyService",
            "removeCompanyLocation",
            "DELETE",
            "/company/v1/company-locations/{companyLocationId}",
            __undefined,
            __undefined,
            __undefined,
            [
                companyLocationId,
            ],
            __undefined,
            __undefined
        );
    }

    public createPosition(companyId: string, request: ICreatePositionRequest): Promise<ICompanyPosition> {
        return this.bridge.call<ICompanyPosition>(
            "CompanyService",
            "createPosition",
            "POST",
            "/company/v1/companies/{companyId}/positions",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public listPositions(companyId: string, state?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ICompanyPositionPage> {
        return this.bridge.call<ICompanyPositionPage>(
            "CompanyService",
            "listPositions",
            "GET",
            "/company/v1/companies/{companyId}/positions",
            __undefined,
            __undefined,
            {
                "state": state,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public getPosition(positionId: string): Promise<ICompanyPosition> {
        return this.bridge.call<ICompanyPosition>(
            "CompanyService",
            "getPosition",
            "GET",
            "/company/v1/positions/{positionId}",
            __undefined,
            __undefined,
            __undefined,
            [
                positionId,
            ],
            __undefined,
            __undefined
        );
    }

    public updatePosition(positionId: string, request: IUpdatePositionRequest): Promise<ICompanyPosition> {
        return this.bridge.call<ICompanyPosition>(
            "CompanyService",
            "updatePosition",
            "PUT",
            "/company/v1/positions/{positionId}",
            request,
            __undefined,
            __undefined,
            [
                positionId,
            ],
            __undefined,
            __undefined
        );
    }

    public abolishPosition(positionId: string): Promise<ICompanyPosition> {
        return this.bridge.call<ICompanyPosition>(
            "CompanyService",
            "abolishPosition",
            "POST",
            "/company/v1/positions/{positionId}/abolish",
            __undefined,
            __undefined,
            __undefined,
            [
                positionId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Fill a vacant position. Returns Company:PositionAlreadyFilled if already filled. */
    public fillPosition(positionId: string, request: IFillPositionRequest): Promise<IAppointment> {
        return this.bridge.call<IAppointment>(
            "CompanyService",
            "fillPosition",
            "POST",
            "/company/v1/positions/{positionId}/fill",
            request,
            __undefined,
            __undefined,
            [
                positionId,
            ],
            __undefined,
            __undefined
        );
    }

    public endAppointment(appointmentId: string, request: IEndAppointmentRequest): Promise<IAppointment> {
        return this.bridge.call<IAppointment>(
            "CompanyService",
            "endAppointment",
            "POST",
            "/company/v1/appointments/{appointmentId}/end",
            request,
            __undefined,
            __undefined,
            [
                appointmentId,
            ],
            __undefined,
            __undefined
        );
    }

    public getOwnershipGraph(companyId: string): Promise<IOwnershipGraph> {
        return this.bridge.call<IOwnershipGraph>(
            "CompanyService",
            "getOwnershipGraph",
            "GET",
            "/company/v1/companies/{companyId}/ownership-graph",
            __undefined,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public recordFounding(companyId: string, request: IRecordFoundingRequest): Promise<IFounding> {
        return this.bridge.call<IFounding>(
            "CompanyService",
            "recordFounding",
            "POST",
            "/company/v1/companies/{companyId}/foundings",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeFounding(foundingId: string): Promise<void> {
        return this.bridge.call<void>(
            "CompanyService",
            "removeFounding",
            "DELETE",
            "/company/v1/foundings/{foundingId}",
            __undefined,
            __undefined,
            __undefined,
            [
                foundingId,
            ],
            __undefined,
            __undefined
        );
    }

    public recordShareholding(companyId: string, request: IRecordShareholdingRequest): Promise<IShareholding> {
        return this.bridge.call<IShareholding>(
            "CompanyService",
            "recordShareholding",
            "POST",
            "/company/v1/companies/{companyId}/shareholdings",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeShareholding(shareholdingId: string): Promise<void> {
        return this.bridge.call<void>(
            "CompanyService",
            "removeShareholding",
            "DELETE",
            "/company/v1/shareholdings/{shareholdingId}",
            __undefined,
            __undefined,
            __undefined,
            [
                shareholdingId,
            ],
            __undefined,
            __undefined
        );
    }

    public recordBeneficiary(companyId: string, request: IRecordBeneficiaryRequest): Promise<IBeneficiary> {
        return this.bridge.call<IBeneficiary>(
            "CompanyService",
            "recordBeneficiary",
            "POST",
            "/company/v1/companies/{companyId}/beneficiaries",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeBeneficiary(beneficiaryId: string): Promise<void> {
        return this.bridge.call<void>(
            "CompanyService",
            "removeBeneficiary",
            "DELETE",
            "/company/v1/beneficiaries/{beneficiaryId}",
            __undefined,
            __undefined,
            __undefined,
            [
                beneficiaryId,
            ],
            __undefined,
            __undefined
        );
    }

    public recordSuccession(companyId: string, request: IRecordSuccessionRequest): Promise<ISuccession> {
        return this.bridge.call<ISuccession>(
            "CompanyService",
            "recordSuccession",
            "POST",
            "/company/v1/companies/{companyId}/successions",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeSuccession(successionId: string): Promise<void> {
        return this.bridge.call<void>(
            "CompanyService",
            "removeSuccession",
            "DELETE",
            "/company/v1/successions/{successionId}",
            __undefined,
            __undefined,
            __undefined,
            [
                successionId,
            ],
            __undefined,
            __undefined
        );
    }

    public recordBranch(companyId: string, request: IRecordBranchRequest): Promise<IBranch> {
        return this.bridge.call<IBranch>(
            "CompanyService",
            "recordBranch",
            "POST",
            "/company/v1/companies/{companyId}/branches",
            request,
            __undefined,
            __undefined,
            [
                companyId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeBranch(branchId: string): Promise<void> {
        return this.bridge.call<void>(
            "CompanyService",
            "removeBranch",
            "DELETE",
            "/company/v1/branches/{branchId}",
            __undefined,
            __undefined,
            __undefined,
            [
                branchId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Read-only view of a person's company links (employment, founding, ownership, beneficiary-of). */
    public listPersonCompanyAffiliations(personId: string): Promise<IPersonCompanyAffiliations> {
        return this.bridge.call<IPersonCompanyAffiliations>(
            "CompanyService",
            "listPersonCompanyAffiliations",
            "GET",
            "/company/v1/persons/{personId}/company-affiliations",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }
}
