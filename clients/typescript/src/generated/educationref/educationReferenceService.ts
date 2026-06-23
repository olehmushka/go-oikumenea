import { IAccreditationEvent } from "./accreditationEvent";
import { IAccreditationEventList } from "./accreditationEventList";
import { ICourse } from "./course";
import { ICourseList } from "./courseList";
import { ICoursePrerequisite } from "./coursePrerequisite";
import { ICoursePrerequisiteList } from "./coursePrerequisiteList";
import { ICreateCoursePrerequisiteRequest } from "./createCoursePrerequisiteRequest";
import { ICurriculumItem } from "./curriculumItem";
import { ICurriculumItemList } from "./curriculumItemList";
import { ICurriculumVersion } from "./curriculumVersion";
import { ICurriculumVersionList } from "./curriculumVersionList";
import { IGovernanceBody } from "./governanceBody";
import { IGovernanceBodyList } from "./governanceBodyList";
import { IGovernanceMembership } from "./governanceMembership";
import { IGovernanceMembershipList } from "./governanceMembershipList";
import { IGrant } from "./grant";
import { IGrantHolding } from "./grantHolding";
import { IGrantHoldingList } from "./grantHoldingList";
import { IGrantList } from "./grantList";
import { IPolicy } from "./policy";
import { IPolicyList } from "./policyList";
import { IProgram } from "./program";
import { IProgramList } from "./programList";
import { IPublication } from "./publication";
import { IPublicationAuthorship } from "./publicationAuthorship";
import { IPublicationAuthorshipList } from "./publicationAuthorshipList";
import { IPublicationList } from "./publicationList";
import { IQualification } from "./qualification";
import { IQualificationAward } from "./qualificationAward";
import { IQualificationAwardList } from "./qualificationAwardList";
import { IQualificationList } from "./qualificationList";
import { IResearchCentre } from "./researchCentre";
import { IResearchCentreList } from "./researchCentreList";
import { IResearchGroup } from "./researchGroup";
import { IResearchGroupList } from "./researchGroupList";
import { IResearchMembership } from "./researchMembership";
import { IResearchMembershipList } from "./researchMembershipList";
import { IScholarship } from "./scholarship";
import { IScholarshipAward } from "./scholarshipAward";
import { IScholarshipAwardList } from "./scholarshipAwardList";
import { IScholarshipList } from "./scholarshipList";
import { IUpsertAccreditationEventRequest } from "./upsertAccreditationEventRequest";
import { IUpsertCourseRequest } from "./upsertCourseRequest";
import { IUpsertCurriculumItemRequest } from "./upsertCurriculumItemRequest";
import { IUpsertCurriculumVersionRequest } from "./upsertCurriculumVersionRequest";
import { IUpsertGovernanceBodyRequest } from "./upsertGovernanceBodyRequest";
import { IUpsertGovernanceMembershipRequest } from "./upsertGovernanceMembershipRequest";
import { IUpsertGrantHoldingRequest } from "./upsertGrantHoldingRequest";
import { IUpsertGrantRequest } from "./upsertGrantRequest";
import { IUpsertPolicyRequest } from "./upsertPolicyRequest";
import { IUpsertProgramRequest } from "./upsertProgramRequest";
import { IUpsertPublicationAuthorshipRequest } from "./upsertPublicationAuthorshipRequest";
import { IUpsertPublicationRequest } from "./upsertPublicationRequest";
import { IUpsertQualificationAwardRequest } from "./upsertQualificationAwardRequest";
import { IUpsertQualificationRequest } from "./upsertQualificationRequest";
import { IUpsertResearchCentreRequest } from "./upsertResearchCentreRequest";
import { IUpsertResearchGroupRequest } from "./upsertResearchGroupRequest";
import { IUpsertResearchMembershipRequest } from "./upsertResearchMembershipRequest";
import { IUpsertScholarshipAwardRequest } from "./upsertScholarshipAwardRequest";
import { IUpsertScholarshipRequest } from "./upsertScholarshipRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The education reference layer: curriculum/courses, research, governance/policy, credentials,
 * scholarships, and the person↔reference directory links. Reads gate on `education.read`;
 * reference writes on `education.manage`; person-binding writes on `education.enrollment.manage`.
 * Writes are audited in-process (D-Audit).
 *
 */
export interface IEducationReferenceService {
    createProgram(institutionId: string, request: IUpsertProgramRequest): Promise<IProgram>;
    listPrograms(institutionId: string): Promise<IProgramList>;
    getProgram(programId: string): Promise<IProgram>;
    updateProgram(programId: string, request: IUpsertProgramRequest): Promise<IProgram>;
    deleteProgram(programId: string): Promise<void>;
    createCourse(institutionId: string, request: IUpsertCourseRequest): Promise<ICourse>;
    listCourses(institutionId: string): Promise<ICourseList>;
    getCourse(courseId: string): Promise<ICourse>;
    updateCourse(courseId: string, request: IUpsertCourseRequest): Promise<ICourse>;
    deleteCourse(courseId: string): Promise<void>;
    createCurriculumVersion(programId: string, request: IUpsertCurriculumVersionRequest): Promise<ICurriculumVersion>;
    listCurriculumVersions(programId: string): Promise<ICurriculumVersionList>;
    getCurriculumVersion(versionId: string): Promise<ICurriculumVersion>;
    updateCurriculumVersion(versionId: string, request: IUpsertCurriculumVersionRequest): Promise<ICurriculumVersion>;
    deleteCurriculumVersion(versionId: string): Promise<void>;
    addCurriculumItem(versionId: string, request: IUpsertCurriculumItemRequest): Promise<ICurriculumItem>;
    listCurriculumItems(versionId: string): Promise<ICurriculumItemList>;
    updateCurriculumItem(itemId: string, request: IUpsertCurriculumItemRequest): Promise<ICurriculumItem>;
    deleteCurriculumItem(itemId: string): Promise<void>;
    addCoursePrerequisite(courseId: string, request: ICreateCoursePrerequisiteRequest): Promise<ICoursePrerequisite>;
    listCoursePrerequisites(courseId: string): Promise<ICoursePrerequisiteList>;
    deleteCoursePrerequisite(prerequisiteId: string): Promise<void>;
    createResearchCentre(institutionId: string, request: IUpsertResearchCentreRequest): Promise<IResearchCentre>;
    listResearchCentres(institutionId: string): Promise<IResearchCentreList>;
    getResearchCentre(centreId: string): Promise<IResearchCentre>;
    updateResearchCentre(centreId: string, request: IUpsertResearchCentreRequest): Promise<IResearchCentre>;
    deleteResearchCentre(centreId: string): Promise<void>;
    createResearchGroup(institutionId: string, request: IUpsertResearchGroupRequest): Promise<IResearchGroup>;
    listResearchGroups(institutionId: string): Promise<IResearchGroupList>;
    getResearchGroup(groupId: string): Promise<IResearchGroup>;
    updateResearchGroup(groupId: string, request: IUpsertResearchGroupRequest): Promise<IResearchGroup>;
    deleteResearchGroup(groupId: string): Promise<void>;
    createGrant(institutionId: string, request: IUpsertGrantRequest): Promise<IGrant>;
    listGrants(institutionId: string): Promise<IGrantList>;
    getGrant(grantId: string): Promise<IGrant>;
    updateGrant(grantId: string, request: IUpsertGrantRequest): Promise<IGrant>;
    deleteGrant(grantId: string): Promise<void>;
    createPublication(request: IUpsertPublicationRequest): Promise<IPublication>;
    listPublications(query?: string | null): Promise<IPublicationList>;
    getPublication(publicationId: string): Promise<IPublication>;
    updatePublication(publicationId: string, request: IUpsertPublicationRequest): Promise<IPublication>;
    deletePublication(publicationId: string): Promise<void>;
    createGovernanceBody(institutionId: string, request: IUpsertGovernanceBodyRequest): Promise<IGovernanceBody>;
    listGovernanceBodies(institutionId: string): Promise<IGovernanceBodyList>;
    getGovernanceBody(bodyId: string): Promise<IGovernanceBody>;
    updateGovernanceBody(bodyId: string, request: IUpsertGovernanceBodyRequest): Promise<IGovernanceBody>;
    deleteGovernanceBody(bodyId: string): Promise<void>;
    createPolicy(institutionId: string, request: IUpsertPolicyRequest): Promise<IPolicy>;
    listPolicies(institutionId: string): Promise<IPolicyList>;
    getPolicy(policyId: string): Promise<IPolicy>;
    updatePolicy(policyId: string, request: IUpsertPolicyRequest): Promise<IPolicy>;
    deletePolicy(policyId: string): Promise<void>;
    createQualification(institutionId: string, request: IUpsertQualificationRequest): Promise<IQualification>;
    listQualifications(institutionId: string): Promise<IQualificationList>;
    getQualification(qualificationId: string): Promise<IQualification>;
    updateQualification(qualificationId: string, request: IUpsertQualificationRequest): Promise<IQualification>;
    deleteQualification(qualificationId: string): Promise<void>;
    createScholarship(request: IUpsertScholarshipRequest): Promise<IScholarship>;
    listScholarships(query?: string | null): Promise<IScholarshipList>;
    getScholarship(scholarshipId: string): Promise<IScholarship>;
    updateScholarship(scholarshipId: string, request: IUpsertScholarshipRequest): Promise<IScholarship>;
    deleteScholarship(scholarshipId: string): Promise<void>;
    createAccreditationEvent(request: IUpsertAccreditationEventRequest): Promise<IAccreditationEvent>;
    listAccreditationEvents(institutionId?: string | null, programId?: string | null): Promise<IAccreditationEventList>;
    getAccreditationEvent(eventId: string): Promise<IAccreditationEvent>;
    updateAccreditationEvent(eventId: string, request: IUpsertAccreditationEventRequest): Promise<IAccreditationEvent>;
    deleteAccreditationEvent(eventId: string): Promise<void>;
    listPublicationAuthorships(personId: string): Promise<IPublicationAuthorshipList>;
    createPublicationAuthorship(personId: string, request: IUpsertPublicationAuthorshipRequest): Promise<IPublicationAuthorship>;
    updatePublicationAuthorship(personId: string, linkId: string, request: IUpsertPublicationAuthorshipRequest): Promise<IPublicationAuthorship>;
    deletePublicationAuthorship(personId: string, linkId: string): Promise<void>;
    listResearchMemberships(personId: string): Promise<IResearchMembershipList>;
    createResearchMembership(personId: string, request: IUpsertResearchMembershipRequest): Promise<IResearchMembership>;
    updateResearchMembership(personId: string, linkId: string, request: IUpsertResearchMembershipRequest): Promise<IResearchMembership>;
    deleteResearchMembership(personId: string, linkId: string): Promise<void>;
    listGrantHoldings(personId: string): Promise<IGrantHoldingList>;
    createGrantHolding(personId: string, request: IUpsertGrantHoldingRequest): Promise<IGrantHolding>;
    updateGrantHolding(personId: string, linkId: string, request: IUpsertGrantHoldingRequest): Promise<IGrantHolding>;
    deleteGrantHolding(personId: string, linkId: string): Promise<void>;
    listGovernanceMemberships(personId: string): Promise<IGovernanceMembershipList>;
    createGovernanceMembership(personId: string, request: IUpsertGovernanceMembershipRequest): Promise<IGovernanceMembership>;
    updateGovernanceMembership(personId: string, linkId: string, request: IUpsertGovernanceMembershipRequest): Promise<IGovernanceMembership>;
    deleteGovernanceMembership(personId: string, linkId: string): Promise<void>;
    listQualificationAwards(personId: string): Promise<IQualificationAwardList>;
    createQualificationAward(personId: string, request: IUpsertQualificationAwardRequest): Promise<IQualificationAward>;
    updateQualificationAward(personId: string, linkId: string, request: IUpsertQualificationAwardRequest): Promise<IQualificationAward>;
    deleteQualificationAward(personId: string, linkId: string): Promise<void>;
    listScholarshipAwards(personId: string): Promise<IScholarshipAwardList>;
    createScholarshipAward(personId: string, request: IUpsertScholarshipAwardRequest): Promise<IScholarshipAward>;
    updateScholarshipAward(personId: string, linkId: string, request: IUpsertScholarshipAwardRequest): Promise<IScholarshipAward>;
    deleteScholarshipAward(personId: string, linkId: string): Promise<void>;
}

export class EducationReferenceService implements IEducationReferenceService {
    constructor(private bridge: IHttpApiBridge) {
    }

    public createProgram(institutionId: string, request: IUpsertProgramRequest): Promise<IProgram> {
        return this.bridge.call<IProgram>(
            "EducationReferenceService",
            "createProgram",
            "POST",
            "/education/v1/institutions/{institutionId}/programs",
            request,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public listPrograms(institutionId: string): Promise<IProgramList> {
        return this.bridge.call<IProgramList>(
            "EducationReferenceService",
            "listPrograms",
            "GET",
            "/education/v1/institutions/{institutionId}/programs",
            __undefined,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public getProgram(programId: string): Promise<IProgram> {
        return this.bridge.call<IProgram>(
            "EducationReferenceService",
            "getProgram",
            "GET",
            "/education/v1/programs/{programId}",
            __undefined,
            __undefined,
            __undefined,
            [
                programId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateProgram(programId: string, request: IUpsertProgramRequest): Promise<IProgram> {
        return this.bridge.call<IProgram>(
            "EducationReferenceService",
            "updateProgram",
            "PUT",
            "/education/v1/programs/{programId}",
            request,
            __undefined,
            __undefined,
            [
                programId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteProgram(programId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteProgram",
            "DELETE",
            "/education/v1/programs/{programId}",
            __undefined,
            __undefined,
            __undefined,
            [
                programId,
            ],
            __undefined,
            __undefined
        );
    }

    public createCourse(institutionId: string, request: IUpsertCourseRequest): Promise<ICourse> {
        return this.bridge.call<ICourse>(
            "EducationReferenceService",
            "createCourse",
            "POST",
            "/education/v1/institutions/{institutionId}/courses",
            request,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public listCourses(institutionId: string): Promise<ICourseList> {
        return this.bridge.call<ICourseList>(
            "EducationReferenceService",
            "listCourses",
            "GET",
            "/education/v1/institutions/{institutionId}/courses",
            __undefined,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public getCourse(courseId: string): Promise<ICourse> {
        return this.bridge.call<ICourse>(
            "EducationReferenceService",
            "getCourse",
            "GET",
            "/education/v1/courses/{courseId}",
            __undefined,
            __undefined,
            __undefined,
            [
                courseId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateCourse(courseId: string, request: IUpsertCourseRequest): Promise<ICourse> {
        return this.bridge.call<ICourse>(
            "EducationReferenceService",
            "updateCourse",
            "PUT",
            "/education/v1/courses/{courseId}",
            request,
            __undefined,
            __undefined,
            [
                courseId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteCourse(courseId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteCourse",
            "DELETE",
            "/education/v1/courses/{courseId}",
            __undefined,
            __undefined,
            __undefined,
            [
                courseId,
            ],
            __undefined,
            __undefined
        );
    }

    public createCurriculumVersion(programId: string, request: IUpsertCurriculumVersionRequest): Promise<ICurriculumVersion> {
        return this.bridge.call<ICurriculumVersion>(
            "EducationReferenceService",
            "createCurriculumVersion",
            "POST",
            "/education/v1/programs/{programId}/curriculum-versions",
            request,
            __undefined,
            __undefined,
            [
                programId,
            ],
            __undefined,
            __undefined
        );
    }

    public listCurriculumVersions(programId: string): Promise<ICurriculumVersionList> {
        return this.bridge.call<ICurriculumVersionList>(
            "EducationReferenceService",
            "listCurriculumVersions",
            "GET",
            "/education/v1/programs/{programId}/curriculum-versions",
            __undefined,
            __undefined,
            __undefined,
            [
                programId,
            ],
            __undefined,
            __undefined
        );
    }

    public getCurriculumVersion(versionId: string): Promise<ICurriculumVersion> {
        return this.bridge.call<ICurriculumVersion>(
            "EducationReferenceService",
            "getCurriculumVersion",
            "GET",
            "/education/v1/curriculum-versions/{versionId}",
            __undefined,
            __undefined,
            __undefined,
            [
                versionId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateCurriculumVersion(versionId: string, request: IUpsertCurriculumVersionRequest): Promise<ICurriculumVersion> {
        return this.bridge.call<ICurriculumVersion>(
            "EducationReferenceService",
            "updateCurriculumVersion",
            "PUT",
            "/education/v1/curriculum-versions/{versionId}",
            request,
            __undefined,
            __undefined,
            [
                versionId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteCurriculumVersion(versionId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteCurriculumVersion",
            "DELETE",
            "/education/v1/curriculum-versions/{versionId}",
            __undefined,
            __undefined,
            __undefined,
            [
                versionId,
            ],
            __undefined,
            __undefined
        );
    }

    public addCurriculumItem(versionId: string, request: IUpsertCurriculumItemRequest): Promise<ICurriculumItem> {
        return this.bridge.call<ICurriculumItem>(
            "EducationReferenceService",
            "addCurriculumItem",
            "POST",
            "/education/v1/curriculum-versions/{versionId}/items",
            request,
            __undefined,
            __undefined,
            [
                versionId,
            ],
            __undefined,
            __undefined
        );
    }

    public listCurriculumItems(versionId: string): Promise<ICurriculumItemList> {
        return this.bridge.call<ICurriculumItemList>(
            "EducationReferenceService",
            "listCurriculumItems",
            "GET",
            "/education/v1/curriculum-versions/{versionId}/items",
            __undefined,
            __undefined,
            __undefined,
            [
                versionId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateCurriculumItem(itemId: string, request: IUpsertCurriculumItemRequest): Promise<ICurriculumItem> {
        return this.bridge.call<ICurriculumItem>(
            "EducationReferenceService",
            "updateCurriculumItem",
            "PUT",
            "/education/v1/curriculum-items/{itemId}",
            request,
            __undefined,
            __undefined,
            [
                itemId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteCurriculumItem(itemId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteCurriculumItem",
            "DELETE",
            "/education/v1/curriculum-items/{itemId}",
            __undefined,
            __undefined,
            __undefined,
            [
                itemId,
            ],
            __undefined,
            __undefined
        );
    }

    public addCoursePrerequisite(courseId: string, request: ICreateCoursePrerequisiteRequest): Promise<ICoursePrerequisite> {
        return this.bridge.call<ICoursePrerequisite>(
            "EducationReferenceService",
            "addCoursePrerequisite",
            "POST",
            "/education/v1/courses/{courseId}/prerequisites",
            request,
            __undefined,
            __undefined,
            [
                courseId,
            ],
            __undefined,
            __undefined
        );
    }

    public listCoursePrerequisites(courseId: string): Promise<ICoursePrerequisiteList> {
        return this.bridge.call<ICoursePrerequisiteList>(
            "EducationReferenceService",
            "listCoursePrerequisites",
            "GET",
            "/education/v1/courses/{courseId}/prerequisites",
            __undefined,
            __undefined,
            __undefined,
            [
                courseId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteCoursePrerequisite(prerequisiteId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteCoursePrerequisite",
            "DELETE",
            "/education/v1/course-prerequisites/{prerequisiteId}",
            __undefined,
            __undefined,
            __undefined,
            [
                prerequisiteId,
            ],
            __undefined,
            __undefined
        );
    }

    public createResearchCentre(institutionId: string, request: IUpsertResearchCentreRequest): Promise<IResearchCentre> {
        return this.bridge.call<IResearchCentre>(
            "EducationReferenceService",
            "createResearchCentre",
            "POST",
            "/education/v1/institutions/{institutionId}/research-centres",
            request,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public listResearchCentres(institutionId: string): Promise<IResearchCentreList> {
        return this.bridge.call<IResearchCentreList>(
            "EducationReferenceService",
            "listResearchCentres",
            "GET",
            "/education/v1/institutions/{institutionId}/research-centres",
            __undefined,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public getResearchCentre(centreId: string): Promise<IResearchCentre> {
        return this.bridge.call<IResearchCentre>(
            "EducationReferenceService",
            "getResearchCentre",
            "GET",
            "/education/v1/research-centres/{centreId}",
            __undefined,
            __undefined,
            __undefined,
            [
                centreId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateResearchCentre(centreId: string, request: IUpsertResearchCentreRequest): Promise<IResearchCentre> {
        return this.bridge.call<IResearchCentre>(
            "EducationReferenceService",
            "updateResearchCentre",
            "PUT",
            "/education/v1/research-centres/{centreId}",
            request,
            __undefined,
            __undefined,
            [
                centreId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteResearchCentre(centreId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteResearchCentre",
            "DELETE",
            "/education/v1/research-centres/{centreId}",
            __undefined,
            __undefined,
            __undefined,
            [
                centreId,
            ],
            __undefined,
            __undefined
        );
    }

    public createResearchGroup(institutionId: string, request: IUpsertResearchGroupRequest): Promise<IResearchGroup> {
        return this.bridge.call<IResearchGroup>(
            "EducationReferenceService",
            "createResearchGroup",
            "POST",
            "/education/v1/institutions/{institutionId}/research-groups",
            request,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public listResearchGroups(institutionId: string): Promise<IResearchGroupList> {
        return this.bridge.call<IResearchGroupList>(
            "EducationReferenceService",
            "listResearchGroups",
            "GET",
            "/education/v1/institutions/{institutionId}/research-groups",
            __undefined,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public getResearchGroup(groupId: string): Promise<IResearchGroup> {
        return this.bridge.call<IResearchGroup>(
            "EducationReferenceService",
            "getResearchGroup",
            "GET",
            "/education/v1/research-groups/{groupId}",
            __undefined,
            __undefined,
            __undefined,
            [
                groupId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateResearchGroup(groupId: string, request: IUpsertResearchGroupRequest): Promise<IResearchGroup> {
        return this.bridge.call<IResearchGroup>(
            "EducationReferenceService",
            "updateResearchGroup",
            "PUT",
            "/education/v1/research-groups/{groupId}",
            request,
            __undefined,
            __undefined,
            [
                groupId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteResearchGroup(groupId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteResearchGroup",
            "DELETE",
            "/education/v1/research-groups/{groupId}",
            __undefined,
            __undefined,
            __undefined,
            [
                groupId,
            ],
            __undefined,
            __undefined
        );
    }

    public createGrant(institutionId: string, request: IUpsertGrantRequest): Promise<IGrant> {
        return this.bridge.call<IGrant>(
            "EducationReferenceService",
            "createGrant",
            "POST",
            "/education/v1/institutions/{institutionId}/grants",
            request,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public listGrants(institutionId: string): Promise<IGrantList> {
        return this.bridge.call<IGrantList>(
            "EducationReferenceService",
            "listGrants",
            "GET",
            "/education/v1/institutions/{institutionId}/grants",
            __undefined,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public getGrant(grantId: string): Promise<IGrant> {
        return this.bridge.call<IGrant>(
            "EducationReferenceService",
            "getGrant",
            "GET",
            "/education/v1/grants/{grantId}",
            __undefined,
            __undefined,
            __undefined,
            [
                grantId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateGrant(grantId: string, request: IUpsertGrantRequest): Promise<IGrant> {
        return this.bridge.call<IGrant>(
            "EducationReferenceService",
            "updateGrant",
            "PUT",
            "/education/v1/grants/{grantId}",
            request,
            __undefined,
            __undefined,
            [
                grantId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteGrant(grantId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteGrant",
            "DELETE",
            "/education/v1/grants/{grantId}",
            __undefined,
            __undefined,
            __undefined,
            [
                grantId,
            ],
            __undefined,
            __undefined
        );
    }

    public createPublication(request: IUpsertPublicationRequest): Promise<IPublication> {
        return this.bridge.call<IPublication>(
            "EducationReferenceService",
            "createPublication",
            "POST",
            "/education/v1/publications",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listPublications(query?: string | null): Promise<IPublicationList> {
        return this.bridge.call<IPublicationList>(
            "EducationReferenceService",
            "listPublications",
            "GET",
            "/education/v1/publications",
            __undefined,
            __undefined,
            {
                "query": query,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public getPublication(publicationId: string): Promise<IPublication> {
        return this.bridge.call<IPublication>(
            "EducationReferenceService",
            "getPublication",
            "GET",
            "/education/v1/publications/{publicationId}",
            __undefined,
            __undefined,
            __undefined,
            [
                publicationId,
            ],
            __undefined,
            __undefined
        );
    }

    public updatePublication(publicationId: string, request: IUpsertPublicationRequest): Promise<IPublication> {
        return this.bridge.call<IPublication>(
            "EducationReferenceService",
            "updatePublication",
            "PUT",
            "/education/v1/publications/{publicationId}",
            request,
            __undefined,
            __undefined,
            [
                publicationId,
            ],
            __undefined,
            __undefined
        );
    }

    public deletePublication(publicationId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deletePublication",
            "DELETE",
            "/education/v1/publications/{publicationId}",
            __undefined,
            __undefined,
            __undefined,
            [
                publicationId,
            ],
            __undefined,
            __undefined
        );
    }

    public createGovernanceBody(institutionId: string, request: IUpsertGovernanceBodyRequest): Promise<IGovernanceBody> {
        return this.bridge.call<IGovernanceBody>(
            "EducationReferenceService",
            "createGovernanceBody",
            "POST",
            "/education/v1/institutions/{institutionId}/governance-bodies",
            request,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public listGovernanceBodies(institutionId: string): Promise<IGovernanceBodyList> {
        return this.bridge.call<IGovernanceBodyList>(
            "EducationReferenceService",
            "listGovernanceBodies",
            "GET",
            "/education/v1/institutions/{institutionId}/governance-bodies",
            __undefined,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public getGovernanceBody(bodyId: string): Promise<IGovernanceBody> {
        return this.bridge.call<IGovernanceBody>(
            "EducationReferenceService",
            "getGovernanceBody",
            "GET",
            "/education/v1/governance-bodies/{bodyId}",
            __undefined,
            __undefined,
            __undefined,
            [
                bodyId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateGovernanceBody(bodyId: string, request: IUpsertGovernanceBodyRequest): Promise<IGovernanceBody> {
        return this.bridge.call<IGovernanceBody>(
            "EducationReferenceService",
            "updateGovernanceBody",
            "PUT",
            "/education/v1/governance-bodies/{bodyId}",
            request,
            __undefined,
            __undefined,
            [
                bodyId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteGovernanceBody(bodyId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteGovernanceBody",
            "DELETE",
            "/education/v1/governance-bodies/{bodyId}",
            __undefined,
            __undefined,
            __undefined,
            [
                bodyId,
            ],
            __undefined,
            __undefined
        );
    }

    public createPolicy(institutionId: string, request: IUpsertPolicyRequest): Promise<IPolicy> {
        return this.bridge.call<IPolicy>(
            "EducationReferenceService",
            "createPolicy",
            "POST",
            "/education/v1/institutions/{institutionId}/policies",
            request,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public listPolicies(institutionId: string): Promise<IPolicyList> {
        return this.bridge.call<IPolicyList>(
            "EducationReferenceService",
            "listPolicies",
            "GET",
            "/education/v1/institutions/{institutionId}/policies",
            __undefined,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public getPolicy(policyId: string): Promise<IPolicy> {
        return this.bridge.call<IPolicy>(
            "EducationReferenceService",
            "getPolicy",
            "GET",
            "/education/v1/policies/{policyId}",
            __undefined,
            __undefined,
            __undefined,
            [
                policyId,
            ],
            __undefined,
            __undefined
        );
    }

    public updatePolicy(policyId: string, request: IUpsertPolicyRequest): Promise<IPolicy> {
        return this.bridge.call<IPolicy>(
            "EducationReferenceService",
            "updatePolicy",
            "PUT",
            "/education/v1/policies/{policyId}",
            request,
            __undefined,
            __undefined,
            [
                policyId,
            ],
            __undefined,
            __undefined
        );
    }

    public deletePolicy(policyId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deletePolicy",
            "DELETE",
            "/education/v1/policies/{policyId}",
            __undefined,
            __undefined,
            __undefined,
            [
                policyId,
            ],
            __undefined,
            __undefined
        );
    }

    public createQualification(institutionId: string, request: IUpsertQualificationRequest): Promise<IQualification> {
        return this.bridge.call<IQualification>(
            "EducationReferenceService",
            "createQualification",
            "POST",
            "/education/v1/institutions/{institutionId}/qualifications",
            request,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public listQualifications(institutionId: string): Promise<IQualificationList> {
        return this.bridge.call<IQualificationList>(
            "EducationReferenceService",
            "listQualifications",
            "GET",
            "/education/v1/institutions/{institutionId}/qualifications",
            __undefined,
            __undefined,
            __undefined,
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public getQualification(qualificationId: string): Promise<IQualification> {
        return this.bridge.call<IQualification>(
            "EducationReferenceService",
            "getQualification",
            "GET",
            "/education/v1/qualifications/{qualificationId}",
            __undefined,
            __undefined,
            __undefined,
            [
                qualificationId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateQualification(qualificationId: string, request: IUpsertQualificationRequest): Promise<IQualification> {
        return this.bridge.call<IQualification>(
            "EducationReferenceService",
            "updateQualification",
            "PUT",
            "/education/v1/qualifications/{qualificationId}",
            request,
            __undefined,
            __undefined,
            [
                qualificationId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteQualification(qualificationId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteQualification",
            "DELETE",
            "/education/v1/qualifications/{qualificationId}",
            __undefined,
            __undefined,
            __undefined,
            [
                qualificationId,
            ],
            __undefined,
            __undefined
        );
    }

    public createScholarship(request: IUpsertScholarshipRequest): Promise<IScholarship> {
        return this.bridge.call<IScholarship>(
            "EducationReferenceService",
            "createScholarship",
            "POST",
            "/education/v1/scholarships",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listScholarships(query?: string | null): Promise<IScholarshipList> {
        return this.bridge.call<IScholarshipList>(
            "EducationReferenceService",
            "listScholarships",
            "GET",
            "/education/v1/scholarships",
            __undefined,
            __undefined,
            {
                "query": query,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public getScholarship(scholarshipId: string): Promise<IScholarship> {
        return this.bridge.call<IScholarship>(
            "EducationReferenceService",
            "getScholarship",
            "GET",
            "/education/v1/scholarships/{scholarshipId}",
            __undefined,
            __undefined,
            __undefined,
            [
                scholarshipId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateScholarship(scholarshipId: string, request: IUpsertScholarshipRequest): Promise<IScholarship> {
        return this.bridge.call<IScholarship>(
            "EducationReferenceService",
            "updateScholarship",
            "PUT",
            "/education/v1/scholarships/{scholarshipId}",
            request,
            __undefined,
            __undefined,
            [
                scholarshipId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteScholarship(scholarshipId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteScholarship",
            "DELETE",
            "/education/v1/scholarships/{scholarshipId}",
            __undefined,
            __undefined,
            __undefined,
            [
                scholarshipId,
            ],
            __undefined,
            __undefined
        );
    }

    public createAccreditationEvent(request: IUpsertAccreditationEventRequest): Promise<IAccreditationEvent> {
        return this.bridge.call<IAccreditationEvent>(
            "EducationReferenceService",
            "createAccreditationEvent",
            "POST",
            "/education/v1/accreditation-events",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listAccreditationEvents(institutionId?: string | null, programId?: string | null): Promise<IAccreditationEventList> {
        return this.bridge.call<IAccreditationEventList>(
            "EducationReferenceService",
            "listAccreditationEvents",
            "GET",
            "/education/v1/accreditation-events",
            __undefined,
            __undefined,
            {
                "institutionId": institutionId,
                "programId": programId,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public getAccreditationEvent(eventId: string): Promise<IAccreditationEvent> {
        return this.bridge.call<IAccreditationEvent>(
            "EducationReferenceService",
            "getAccreditationEvent",
            "GET",
            "/education/v1/accreditation-events/{eventId}",
            __undefined,
            __undefined,
            __undefined,
            [
                eventId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateAccreditationEvent(eventId: string, request: IUpsertAccreditationEventRequest): Promise<IAccreditationEvent> {
        return this.bridge.call<IAccreditationEvent>(
            "EducationReferenceService",
            "updateAccreditationEvent",
            "PUT",
            "/education/v1/accreditation-events/{eventId}",
            request,
            __undefined,
            __undefined,
            [
                eventId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteAccreditationEvent(eventId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteAccreditationEvent",
            "DELETE",
            "/education/v1/accreditation-events/{eventId}",
            __undefined,
            __undefined,
            __undefined,
            [
                eventId,
            ],
            __undefined,
            __undefined
        );
    }

    public listPublicationAuthorships(personId: string): Promise<IPublicationAuthorshipList> {
        return this.bridge.call<IPublicationAuthorshipList>(
            "EducationReferenceService",
            "listPublicationAuthorships",
            "GET",
            "/education/v1/persons/{personId}/publication-authorships",
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

    public createPublicationAuthorship(personId: string, request: IUpsertPublicationAuthorshipRequest): Promise<IPublicationAuthorship> {
        return this.bridge.call<IPublicationAuthorship>(
            "EducationReferenceService",
            "createPublicationAuthorship",
            "POST",
            "/education/v1/persons/{personId}/publication-authorships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    public updatePublicationAuthorship(personId: string, linkId: string, request: IUpsertPublicationAuthorshipRequest): Promise<IPublicationAuthorship> {
        return this.bridge.call<IPublicationAuthorship>(
            "EducationReferenceService",
            "updatePublicationAuthorship",
            "PUT",
            "/education/v1/persons/{personId}/publication-authorships/{linkId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public deletePublicationAuthorship(personId: string, linkId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deletePublicationAuthorship",
            "DELETE",
            "/education/v1/persons/{personId}/publication-authorships/{linkId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public listResearchMemberships(personId: string): Promise<IResearchMembershipList> {
        return this.bridge.call<IResearchMembershipList>(
            "EducationReferenceService",
            "listResearchMemberships",
            "GET",
            "/education/v1/persons/{personId}/research-memberships",
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

    public createResearchMembership(personId: string, request: IUpsertResearchMembershipRequest): Promise<IResearchMembership> {
        return this.bridge.call<IResearchMembership>(
            "EducationReferenceService",
            "createResearchMembership",
            "POST",
            "/education/v1/persons/{personId}/research-memberships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateResearchMembership(personId: string, linkId: string, request: IUpsertResearchMembershipRequest): Promise<IResearchMembership> {
        return this.bridge.call<IResearchMembership>(
            "EducationReferenceService",
            "updateResearchMembership",
            "PUT",
            "/education/v1/persons/{personId}/research-memberships/{linkId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteResearchMembership(personId: string, linkId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteResearchMembership",
            "DELETE",
            "/education/v1/persons/{personId}/research-memberships/{linkId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public listGrantHoldings(personId: string): Promise<IGrantHoldingList> {
        return this.bridge.call<IGrantHoldingList>(
            "EducationReferenceService",
            "listGrantHoldings",
            "GET",
            "/education/v1/persons/{personId}/grant-holdings",
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

    public createGrantHolding(personId: string, request: IUpsertGrantHoldingRequest): Promise<IGrantHolding> {
        return this.bridge.call<IGrantHolding>(
            "EducationReferenceService",
            "createGrantHolding",
            "POST",
            "/education/v1/persons/{personId}/grant-holdings",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateGrantHolding(personId: string, linkId: string, request: IUpsertGrantHoldingRequest): Promise<IGrantHolding> {
        return this.bridge.call<IGrantHolding>(
            "EducationReferenceService",
            "updateGrantHolding",
            "PUT",
            "/education/v1/persons/{personId}/grant-holdings/{linkId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteGrantHolding(personId: string, linkId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteGrantHolding",
            "DELETE",
            "/education/v1/persons/{personId}/grant-holdings/{linkId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public listGovernanceMemberships(personId: string): Promise<IGovernanceMembershipList> {
        return this.bridge.call<IGovernanceMembershipList>(
            "EducationReferenceService",
            "listGovernanceMemberships",
            "GET",
            "/education/v1/persons/{personId}/governance-memberships",
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

    public createGovernanceMembership(personId: string, request: IUpsertGovernanceMembershipRequest): Promise<IGovernanceMembership> {
        return this.bridge.call<IGovernanceMembership>(
            "EducationReferenceService",
            "createGovernanceMembership",
            "POST",
            "/education/v1/persons/{personId}/governance-memberships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateGovernanceMembership(personId: string, linkId: string, request: IUpsertGovernanceMembershipRequest): Promise<IGovernanceMembership> {
        return this.bridge.call<IGovernanceMembership>(
            "EducationReferenceService",
            "updateGovernanceMembership",
            "PUT",
            "/education/v1/persons/{personId}/governance-memberships/{linkId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteGovernanceMembership(personId: string, linkId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteGovernanceMembership",
            "DELETE",
            "/education/v1/persons/{personId}/governance-memberships/{linkId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public listQualificationAwards(personId: string): Promise<IQualificationAwardList> {
        return this.bridge.call<IQualificationAwardList>(
            "EducationReferenceService",
            "listQualificationAwards",
            "GET",
            "/education/v1/persons/{personId}/qualification-awards",
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

    public createQualificationAward(personId: string, request: IUpsertQualificationAwardRequest): Promise<IQualificationAward> {
        return this.bridge.call<IQualificationAward>(
            "EducationReferenceService",
            "createQualificationAward",
            "POST",
            "/education/v1/persons/{personId}/qualification-awards",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateQualificationAward(personId: string, linkId: string, request: IUpsertQualificationAwardRequest): Promise<IQualificationAward> {
        return this.bridge.call<IQualificationAward>(
            "EducationReferenceService",
            "updateQualificationAward",
            "PUT",
            "/education/v1/persons/{personId}/qualification-awards/{linkId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteQualificationAward(personId: string, linkId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteQualificationAward",
            "DELETE",
            "/education/v1/persons/{personId}/qualification-awards/{linkId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public listScholarshipAwards(personId: string): Promise<IScholarshipAwardList> {
        return this.bridge.call<IScholarshipAwardList>(
            "EducationReferenceService",
            "listScholarshipAwards",
            "GET",
            "/education/v1/persons/{personId}/scholarship-awards",
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

    public createScholarshipAward(personId: string, request: IUpsertScholarshipAwardRequest): Promise<IScholarshipAward> {
        return this.bridge.call<IScholarshipAward>(
            "EducationReferenceService",
            "createScholarshipAward",
            "POST",
            "/education/v1/persons/{personId}/scholarship-awards",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateScholarshipAward(personId: string, linkId: string, request: IUpsertScholarshipAwardRequest): Promise<IScholarshipAward> {
        return this.bridge.call<IScholarshipAward>(
            "EducationReferenceService",
            "updateScholarshipAward",
            "PUT",
            "/education/v1/persons/{personId}/scholarship-awards/{linkId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteScholarshipAward(personId: string, linkId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationReferenceService",
            "deleteScholarshipAward",
            "DELETE",
            "/education/v1/persons/{personId}/scholarship-awards/{linkId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                linkId,
            ],
            __undefined,
            __undefined
        );
    }
}
