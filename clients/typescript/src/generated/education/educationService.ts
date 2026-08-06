import { IAppointment } from "./appointment";
import { IBuilding } from "./building";
import { IBuildingList } from "./buildingList";
import { ICreateBuildingRequest } from "./createBuildingRequest";
import { ICreateGroupRequest } from "./createGroupRequest";
import { ICreateInstitutionRequest } from "./createInstitutionRequest";
import { ICreatePositionRequest } from "./createPositionRequest";
import { ICreateUnitRequest } from "./createUnitRequest";
import { IDegreeLevelList } from "./degreeLevelList";
import { IDormitoryStay } from "./dormitoryStay";
import { IDormitoryStayList } from "./dormitoryStayList";
import { IEducationPosition } from "./educationPosition";
import { IEducationUnit } from "./educationUnit";
import { IEducationUnitList } from "./educationUnitList";
import { IEndAppointmentRequest } from "./endAppointmentRequest";
import { IEnrollment } from "./enrollment";
import { IEnrollmentList } from "./enrollmentList";
import { IEnrollmentPage } from "./enrollmentPage";
import { IEnrollmentStats } from "./enrollmentStats";
import { IFillPositionRequest } from "./fillPositionRequest";
import { IGroup } from "./group";
import { IGroupList } from "./groupList";
import { IInstitution } from "./institution";
import { IInstitutionKind } from "./institutionKind";
import { IInstitutionKindList } from "./institutionKindList";
import { IInstitutionPage } from "./institutionPage";
import { IInstitutionStats } from "./institutionStats";
import { IPersonAppointmentList } from "./personAppointmentList";
import { IPositionPage } from "./positionPage";
import { IReparentUnitRequest } from "./reparentUnitRequest";
import { IUnitKindList } from "./unitKindList";
import { IUpdateBuildingRequest } from "./updateBuildingRequest";
import { IUpdateGroupRequest } from "./updateGroupRequest";
import { IUpdateInstitutionRequest } from "./updateInstitutionRequest";
import { IUpdatePositionRequest } from "./updatePositionRequest";
import { IUpdateUnitRequest } from "./updateUnitRequest";
import { IUpsertCatalogKindRequest } from "./upsertCatalogKindRequest";
import { IUpsertDormitoryStayRequest } from "./upsertDormitoryStayRequest";
import { IUpsertEnrollmentRequest } from "./upsertEnrollmentRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Institutions, their structure tree (+ closure), buildings, groups, positions/appointments, and the
 * person bindings (enrollments, dorm stays). Reads gate on `education.read`; structure writes on
 * `education.manage`; position/appointment writes on `education.position.manage`; person-binding
 * writes on `education.enrollment.manage`; catalog-kind writes on `education.catalog.manage`. Writes
 * are audited in-process (D-Audit).
 *
 */
export interface IEducationService {
    listInstitutionKinds(): Promise<IInstitutionKindList>;
    /** Create or update (by code) an institution-kind catalog entry. */
    upsertInstitutionKind(request: IUpsertCatalogKindRequest): Promise<IInstitutionKind>;
    /** The `university` domain's unit-kind catalog (owned by the tenant service — M41). */
    listUnitKinds(): Promise<IUnitKindList>;
    listDegreeLevels(): Promise<IDegreeLevelList>;
    createInstitution(request: ICreateInstitutionRequest): Promise<IInstitution>;
    /**
     * List institutions, token-paginated, optionally filtered by the facet vocabulary (M58 ticket
     * 5 / D-ObjectFacets). Shadow-gated: an institution IS a `university`-domain tenant
     * organization (M41 / D-UnifiedOrgGraph), so it carries that organization's public/shadow bit
     * and is trimmed by the same rule `listOrganizations` applies. Gated by education.read.
     *
     * Every filter arg here is also an arg of `institutionStats`, and a chart segment's key is a
     * usable value for the arg it came from — that is what makes a dashboard and a list two
     * renderings of one request state.
     *
     */
    listInstitutions(query?: string | null, kindId?: string | null, countryId?: string | null, foundedOnFrom?: string | null, foundedOnTo?: string | null, state?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IInstitutionPage>;
    /**
     * Facet distributions over the institution registry — the dashboard half of the institution
     * facet vocabulary (M58 ticket 5 / D-ObjectFacets). Takes exactly the filter args
     * `listInstitutions` takes, minus paging, so a dashboard and a list are two renderings of one
     * request state.
     *
     * The path is `/stats/institutions` rather than `/institutions/stats` because the server's
     * router rejects a literal path segment that is a sibling of `{institutionId}` — see the
     * route-conflict guard in `internal/platform/transport`.
     *
     */
    institutionStats(facets?: string | null, query?: string | null, kindId?: string | null, countryId?: string | null, foundedOnFrom?: string | null, foundedOnTo?: string | null, state?: string | null): Promise<IInstitutionStats>;
    getInstitution(institutionId: string): Promise<IInstitution>;
    updateInstitution(institutionId: string, request: IUpdateInstitutionRequest): Promise<IInstitution>;
    deleteInstitution(institutionId: string): Promise<void>;
    createUnit(institutionId: string, request: ICreateUnitRequest): Promise<IEducationUnit>;
    /** All active units of an institution with their closure depth from the nearest root. */
    listUnits(institutionId: string): Promise<IEducationUnitList>;
    getUnit(unitId: string): Promise<IEducationUnit>;
    updateUnit(unitId: string, request: IUpdateUnitRequest): Promise<IEducationUnit>;
    /** Move a unit under a new parent, recomputing the closure. Returns Education:UnitCycleDetected on a cycle. */
    reparentUnit(unitId: string, request: IReparentUnitRequest): Promise<IEducationUnit>;
    createBuilding(institutionId: string, request: ICreateBuildingRequest): Promise<IBuilding>;
    listBuildings(institutionId: string): Promise<IBuildingList>;
    getBuilding(buildingId: string): Promise<IBuilding>;
    updateBuilding(buildingId: string, request: IUpdateBuildingRequest): Promise<IBuilding>;
    deleteBuilding(buildingId: string): Promise<void>;
    createGroup(unitId: string, request: ICreateGroupRequest): Promise<IGroup>;
    listGroups(unitId: string): Promise<IGroupList>;
    getGroup(groupId: string): Promise<IGroup>;
    updateGroup(groupId: string, request: IUpdateGroupRequest): Promise<IGroup>;
    deleteGroup(groupId: string): Promise<void>;
    createPosition(institutionId: string, request: ICreatePositionRequest): Promise<IEducationPosition>;
    listPositions(institutionId: string, state?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IPositionPage>;
    getPosition(positionId: string): Promise<IEducationPosition>;
    updatePosition(positionId: string, request: IUpdatePositionRequest): Promise<IEducationPosition>;
    abolishPosition(positionId: string): Promise<IEducationPosition>;
    /** Fill a vacant position. Returns Education:PositionAlreadyFilled if already filled. */
    fillPosition(positionId: string, request: IFillPositionRequest): Promise<IAppointment>;
    endAppointment(appointmentId: string, request: IEndAppointmentRequest): Promise<IAppointment>;
    /** Read-only list of the education positions a person holds, enriched with title + institution. */
    listPersonAppointments(personId: string): Promise<IPersonAppointmentList>;
    /**
     * The enrollments ONE named person holds. Renamed from `listEnrollments` in M58 ticket 7 when
     * the top-level browse below took that name — the HTTP path is unchanged, and the sibling
     * `listPersonAppointments` already used this shape.
     *
     * Holder-scoped (D-PersonReadScope): a caller who may not read this person gets an EMPTY list
     * rather than a 403, because a permission error would confirm the person exists.
     *
     */
    listPersonEnrollments(personId: string): Promise<IEnrollmentList>;
    /**
     * Browse the enrollment register, token-paginated, optionally filtered by the facet vocabulary
     * (M58 ticket 7 / D-ObjectFacets). Gated by education.read.
     *
     * Until M58 ticket 7 an enrollment could be reached only one person at a time, so the
     * population could be interrogated and never described. This endpoint is the browse mode, and
     * it is HOLDER-SCOPED in SQL: an instance admin sees every enrollment, and everyone else sees
     * the enrollments of people they may read (D-PersonReadScope — the holder holds an active
     * membership in a unit of the caller's reach). The scope is part of the query rather than a
     * filter over the page, because trimming a keyset page after it is cut returns a short page
     * with a next-page token still attached (R-06).
     *
     * Every filter arg here is also an arg of `enrollmentStats`, and a chart segment's key is a
     * usable value for the arg it came from — that is what makes a dashboard and a list two
     * renderings of one request state.
     *
     */
    listEnrollments(institutionId?: string | null, programId?: string | null, unitId?: string | null, groupId?: string | null, degreeLevelId?: string | null, status?: string | null, effectiveFromFrom?: string | null, effectiveFromTo?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IEnrollmentPage>;
    /**
     * Facet distributions over the enrollment register — the dashboard half of the enrollment
     * facet vocabulary (M58 ticket 7 / D-ObjectFacets). Takes exactly the filter args
     * `listEnrollments` takes, minus paging, so a dashboard and a list are two renderings of one
     * request state.
     *
     * The path is `/stats/enrollments` rather than `/enrollments/stats` because the server's
     * router rejects a literal path segment that is a sibling of a path parameter — see the
     * route-conflict guard in `internal/platform/transport`.
     *
     */
    enrollmentStats(facets?: string | null, institutionId?: string | null, programId?: string | null, unitId?: string | null, groupId?: string | null, degreeLevelId?: string | null, status?: string | null, effectiveFromFrom?: string | null, effectiveFromTo?: string | null): Promise<IEnrollmentStats>;
    createEnrollment(personId: string, request: IUpsertEnrollmentRequest): Promise<IEnrollment>;
    updateEnrollment(personId: string, enrollmentId: string, request: IUpsertEnrollmentRequest): Promise<IEnrollment>;
    deleteEnrollment(personId: string, enrollmentId: string): Promise<void>;
    listDormitoryStays(personId: string): Promise<IDormitoryStayList>;
    createDormitoryStay(personId: string, request: IUpsertDormitoryStayRequest): Promise<IDormitoryStay>;
    updateDormitoryStay(personId: string, stayId: string, request: IUpsertDormitoryStayRequest): Promise<IDormitoryStay>;
    deleteDormitoryStay(personId: string, stayId: string): Promise<void>;
}

export class EducationService implements IEducationService {
    constructor(private bridge: IHttpApiBridge) {
    }

    public listInstitutionKinds(): Promise<IInstitutionKindList> {
        return this.bridge.call<IInstitutionKindList>(
            "EducationService",
            "listInstitutionKinds",
            "GET",
            "/education/v1/institution-kinds",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Create or update (by code) an institution-kind catalog entry. */
    public upsertInstitutionKind(request: IUpsertCatalogKindRequest): Promise<IInstitutionKind> {
        return this.bridge.call<IInstitutionKind>(
            "EducationService",
            "upsertInstitutionKind",
            "PUT",
            "/education/v1/institution-kinds",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** The `university` domain's unit-kind catalog (owned by the tenant service — M41). */
    public listUnitKinds(): Promise<IUnitKindList> {
        return this.bridge.call<IUnitKindList>(
            "EducationService",
            "listUnitKinds",
            "GET",
            "/education/v1/unit-kinds",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listDegreeLevels(): Promise<IDegreeLevelList> {
        return this.bridge.call<IDegreeLevelList>(
            "EducationService",
            "listDegreeLevels",
            "GET",
            "/education/v1/degree-levels",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public createInstitution(request: ICreateInstitutionRequest): Promise<IInstitution> {
        return this.bridge.call<IInstitution>(
            "EducationService",
            "createInstitution",
            "POST",
            "/education/v1/institutions",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * List institutions, token-paginated, optionally filtered by the facet vocabulary (M58 ticket
     * 5 / D-ObjectFacets). Shadow-gated: an institution IS a `university`-domain tenant
     * organization (M41 / D-UnifiedOrgGraph), so it carries that organization's public/shadow bit
     * and is trimmed by the same rule `listOrganizations` applies. Gated by education.read.
     *
     * Every filter arg here is also an arg of `institutionStats`, and a chart segment's key is a
     * usable value for the arg it came from — that is what makes a dashboard and a list two
     * renderings of one request state.
     *
     */
    public listInstitutions(query?: string | null, kindId?: string | null, countryId?: string | null, foundedOnFrom?: string | null, foundedOnTo?: string | null, state?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IInstitutionPage> {
        return this.bridge.call<IInstitutionPage>(
            "EducationService",
            "listInstitutions",
            "GET",
            "/education/v1/institutions",
            __undefined,
            __undefined,
            {
                "query": query,
                "kindId": kindId,
                "countryId": countryId,
                "foundedOnFrom": foundedOnFrom,
                "foundedOnTo": foundedOnTo,
                "state": state,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the institution registry — the dashboard half of the institution
     * facet vocabulary (M58 ticket 5 / D-ObjectFacets). Takes exactly the filter args
     * `listInstitutions` takes, minus paging, so a dashboard and a list are two renderings of one
     * request state.
     *
     * The path is `/stats/institutions` rather than `/institutions/stats` because the server's
     * router rejects a literal path segment that is a sibling of `{institutionId}` — see the
     * route-conflict guard in `internal/platform/transport`.
     *
     */
    public institutionStats(facets?: string | null, query?: string | null, kindId?: string | null, countryId?: string | null, foundedOnFrom?: string | null, foundedOnTo?: string | null, state?: string | null): Promise<IInstitutionStats> {
        return this.bridge.call<IInstitutionStats>(
            "EducationService",
            "institutionStats",
            "GET",
            "/education/v1/stats/institutions",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "query": query,
                "kindId": kindId,
                "countryId": countryId,
                "foundedOnFrom": foundedOnFrom,
                "foundedOnTo": foundedOnTo,
                "state": state,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public getInstitution(institutionId: string): Promise<IInstitution> {
        return this.bridge.call<IInstitution>(
            "EducationService",
            "getInstitution",
            "GET",
            "/education/v1/institutions/{institutionId}",
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

    public updateInstitution(institutionId: string, request: IUpdateInstitutionRequest): Promise<IInstitution> {
        return this.bridge.call<IInstitution>(
            "EducationService",
            "updateInstitution",
            "PUT",
            "/education/v1/institutions/{institutionId}",
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

    public deleteInstitution(institutionId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationService",
            "deleteInstitution",
            "DELETE",
            "/education/v1/institutions/{institutionId}",
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

    public createUnit(institutionId: string, request: ICreateUnitRequest): Promise<IEducationUnit> {
        return this.bridge.call<IEducationUnit>(
            "EducationService",
            "createUnit",
            "POST",
            "/education/v1/institutions/{institutionId}/units",
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

    /** All active units of an institution with their closure depth from the nearest root. */
    public listUnits(institutionId: string): Promise<IEducationUnitList> {
        return this.bridge.call<IEducationUnitList>(
            "EducationService",
            "listUnits",
            "GET",
            "/education/v1/institutions/{institutionId}/units",
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

    public getUnit(unitId: string): Promise<IEducationUnit> {
        return this.bridge.call<IEducationUnit>(
            "EducationService",
            "getUnit",
            "GET",
            "/education/v1/units/{unitId}",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateUnit(unitId: string, request: IUpdateUnitRequest): Promise<IEducationUnit> {
        return this.bridge.call<IEducationUnit>(
            "EducationService",
            "updateUnit",
            "PUT",
            "/education/v1/units/{unitId}",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Move a unit under a new parent, recomputing the closure. Returns Education:UnitCycleDetected on a cycle. */
    public reparentUnit(unitId: string, request: IReparentUnitRequest): Promise<IEducationUnit> {
        return this.bridge.call<IEducationUnit>(
            "EducationService",
            "reparentUnit",
            "POST",
            "/education/v1/units/{unitId}/reparent",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public createBuilding(institutionId: string, request: ICreateBuildingRequest): Promise<IBuilding> {
        return this.bridge.call<IBuilding>(
            "EducationService",
            "createBuilding",
            "POST",
            "/education/v1/institutions/{institutionId}/buildings",
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

    public listBuildings(institutionId: string): Promise<IBuildingList> {
        return this.bridge.call<IBuildingList>(
            "EducationService",
            "listBuildings",
            "GET",
            "/education/v1/institutions/{institutionId}/buildings",
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

    public getBuilding(buildingId: string): Promise<IBuilding> {
        return this.bridge.call<IBuilding>(
            "EducationService",
            "getBuilding",
            "GET",
            "/education/v1/buildings/{buildingId}",
            __undefined,
            __undefined,
            __undefined,
            [
                buildingId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateBuilding(buildingId: string, request: IUpdateBuildingRequest): Promise<IBuilding> {
        return this.bridge.call<IBuilding>(
            "EducationService",
            "updateBuilding",
            "PUT",
            "/education/v1/buildings/{buildingId}",
            request,
            __undefined,
            __undefined,
            [
                buildingId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteBuilding(buildingId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationService",
            "deleteBuilding",
            "DELETE",
            "/education/v1/buildings/{buildingId}",
            __undefined,
            __undefined,
            __undefined,
            [
                buildingId,
            ],
            __undefined,
            __undefined
        );
    }

    public createGroup(unitId: string, request: ICreateGroupRequest): Promise<IGroup> {
        return this.bridge.call<IGroup>(
            "EducationService",
            "createGroup",
            "POST",
            "/education/v1/units/{unitId}/groups",
            request,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public listGroups(unitId: string): Promise<IGroupList> {
        return this.bridge.call<IGroupList>(
            "EducationService",
            "listGroups",
            "GET",
            "/education/v1/units/{unitId}/groups",
            __undefined,
            __undefined,
            __undefined,
            [
                unitId,
            ],
            __undefined,
            __undefined
        );
    }

    public getGroup(groupId: string): Promise<IGroup> {
        return this.bridge.call<IGroup>(
            "EducationService",
            "getGroup",
            "GET",
            "/education/v1/groups/{groupId}",
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

    public updateGroup(groupId: string, request: IUpdateGroupRequest): Promise<IGroup> {
        return this.bridge.call<IGroup>(
            "EducationService",
            "updateGroup",
            "PUT",
            "/education/v1/groups/{groupId}",
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

    public deleteGroup(groupId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationService",
            "deleteGroup",
            "DELETE",
            "/education/v1/groups/{groupId}",
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

    public createPosition(institutionId: string, request: ICreatePositionRequest): Promise<IEducationPosition> {
        return this.bridge.call<IEducationPosition>(
            "EducationService",
            "createPosition",
            "POST",
            "/education/v1/institutions/{institutionId}/positions",
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

    public listPositions(institutionId: string, state?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IPositionPage> {
        return this.bridge.call<IPositionPage>(
            "EducationService",
            "listPositions",
            "GET",
            "/education/v1/institutions/{institutionId}/positions",
            __undefined,
            __undefined,
            {
                "state": state,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                institutionId,
            ],
            __undefined,
            __undefined
        );
    }

    public getPosition(positionId: string): Promise<IEducationPosition> {
        return this.bridge.call<IEducationPosition>(
            "EducationService",
            "getPosition",
            "GET",
            "/education/v1/positions/{positionId}",
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

    public updatePosition(positionId: string, request: IUpdatePositionRequest): Promise<IEducationPosition> {
        return this.bridge.call<IEducationPosition>(
            "EducationService",
            "updatePosition",
            "PUT",
            "/education/v1/positions/{positionId}",
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

    public abolishPosition(positionId: string): Promise<IEducationPosition> {
        return this.bridge.call<IEducationPosition>(
            "EducationService",
            "abolishPosition",
            "POST",
            "/education/v1/positions/{positionId}/abolish",
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

    /** Fill a vacant position. Returns Education:PositionAlreadyFilled if already filled. */
    public fillPosition(positionId: string, request: IFillPositionRequest): Promise<IAppointment> {
        return this.bridge.call<IAppointment>(
            "EducationService",
            "fillPosition",
            "POST",
            "/education/v1/positions/{positionId}/fill",
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
            "EducationService",
            "endAppointment",
            "POST",
            "/education/v1/appointments/{appointmentId}/end",
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

    /** Read-only list of the education positions a person holds, enriched with title + institution. */
    public listPersonAppointments(personId: string): Promise<IPersonAppointmentList> {
        return this.bridge.call<IPersonAppointmentList>(
            "EducationService",
            "listPersonAppointments",
            "GET",
            "/education/v1/persons/{personId}/appointments",
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

    /**
     * The enrollments ONE named person holds. Renamed from `listEnrollments` in M58 ticket 7 when
     * the top-level browse below took that name — the HTTP path is unchanged, and the sibling
     * `listPersonAppointments` already used this shape.
     *
     * Holder-scoped (D-PersonReadScope): a caller who may not read this person gets an EMPTY list
     * rather than a 403, because a permission error would confirm the person exists.
     *
     */
    public listPersonEnrollments(personId: string): Promise<IEnrollmentList> {
        return this.bridge.call<IEnrollmentList>(
            "EducationService",
            "listPersonEnrollments",
            "GET",
            "/education/v1/persons/{personId}/enrollments",
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

    /**
     * Browse the enrollment register, token-paginated, optionally filtered by the facet vocabulary
     * (M58 ticket 7 / D-ObjectFacets). Gated by education.read.
     *
     * Until M58 ticket 7 an enrollment could be reached only one person at a time, so the
     * population could be interrogated and never described. This endpoint is the browse mode, and
     * it is HOLDER-SCOPED in SQL: an instance admin sees every enrollment, and everyone else sees
     * the enrollments of people they may read (D-PersonReadScope — the holder holds an active
     * membership in a unit of the caller's reach). The scope is part of the query rather than a
     * filter over the page, because trimming a keyset page after it is cut returns a short page
     * with a next-page token still attached (R-06).
     *
     * Every filter arg here is also an arg of `enrollmentStats`, and a chart segment's key is a
     * usable value for the arg it came from — that is what makes a dashboard and a list two
     * renderings of one request state.
     *
     */
    public listEnrollments(institutionId?: string | null, programId?: string | null, unitId?: string | null, groupId?: string | null, degreeLevelId?: string | null, status?: string | null, effectiveFromFrom?: string | null, effectiveFromTo?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IEnrollmentPage> {
        return this.bridge.call<IEnrollmentPage>(
            "EducationService",
            "listEnrollments",
            "GET",
            "/education/v1/enrollments",
            __undefined,
            __undefined,
            {
                "institutionId": institutionId,
                "programId": programId,
                "unitId": unitId,
                "groupId": groupId,
                "degreeLevelId": degreeLevelId,
                "status": status,
                "effectiveFromFrom": effectiveFromFrom,
                "effectiveFromTo": effectiveFromTo,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the enrollment register — the dashboard half of the enrollment
     * facet vocabulary (M58 ticket 7 / D-ObjectFacets). Takes exactly the filter args
     * `listEnrollments` takes, minus paging, so a dashboard and a list are two renderings of one
     * request state.
     *
     * The path is `/stats/enrollments` rather than `/enrollments/stats` because the server's
     * router rejects a literal path segment that is a sibling of a path parameter — see the
     * route-conflict guard in `internal/platform/transport`.
     *
     */
    public enrollmentStats(facets?: string | null, institutionId?: string | null, programId?: string | null, unitId?: string | null, groupId?: string | null, degreeLevelId?: string | null, status?: string | null, effectiveFromFrom?: string | null, effectiveFromTo?: string | null): Promise<IEnrollmentStats> {
        return this.bridge.call<IEnrollmentStats>(
            "EducationService",
            "enrollmentStats",
            "GET",
            "/education/v1/stats/enrollments",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "institutionId": institutionId,
                "programId": programId,
                "unitId": unitId,
                "groupId": groupId,
                "degreeLevelId": degreeLevelId,
                "status": status,
                "effectiveFromFrom": effectiveFromFrom,
                "effectiveFromTo": effectiveFromTo,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public createEnrollment(personId: string, request: IUpsertEnrollmentRequest): Promise<IEnrollment> {
        return this.bridge.call<IEnrollment>(
            "EducationService",
            "createEnrollment",
            "POST",
            "/education/v1/persons/{personId}/enrollments",
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

    public updateEnrollment(personId: string, enrollmentId: string, request: IUpsertEnrollmentRequest): Promise<IEnrollment> {
        return this.bridge.call<IEnrollment>(
            "EducationService",
            "updateEnrollment",
            "PUT",
            "/education/v1/persons/{personId}/enrollments/{enrollmentId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
                enrollmentId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteEnrollment(personId: string, enrollmentId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationService",
            "deleteEnrollment",
            "DELETE",
            "/education/v1/persons/{personId}/enrollments/{enrollmentId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                enrollmentId,
            ],
            __undefined,
            __undefined
        );
    }

    public listDormitoryStays(personId: string): Promise<IDormitoryStayList> {
        return this.bridge.call<IDormitoryStayList>(
            "EducationService",
            "listDormitoryStays",
            "GET",
            "/education/v1/persons/{personId}/dormitory-stays",
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

    public createDormitoryStay(personId: string, request: IUpsertDormitoryStayRequest): Promise<IDormitoryStay> {
        return this.bridge.call<IDormitoryStay>(
            "EducationService",
            "createDormitoryStay",
            "POST",
            "/education/v1/persons/{personId}/dormitory-stays",
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

    public updateDormitoryStay(personId: string, stayId: string, request: IUpsertDormitoryStayRequest): Promise<IDormitoryStay> {
        return this.bridge.call<IDormitoryStay>(
            "EducationService",
            "updateDormitoryStay",
            "PUT",
            "/education/v1/persons/{personId}/dormitory-stays/{stayId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
                stayId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteDormitoryStay(personId: string, stayId: string): Promise<void> {
        return this.bridge.call<void>(
            "EducationService",
            "deleteDormitoryStay",
            "DELETE",
            "/education/v1/persons/{personId}/dormitory-stays/{stayId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                stayId,
            ],
            __undefined,
            __undefined
        );
    }
}
