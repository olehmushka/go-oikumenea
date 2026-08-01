import { IAddManufacturerRequest } from "./addManufacturerRequest";
import { IBrand } from "./brand";
import { IBrandList } from "./brandList";
import { ICreateVehicleRequest } from "./createVehicleRequest";
import { IManufacturer } from "./manufacturer";
import { IManufacturerList } from "./manufacturerList";
import { IModel } from "./model";
import { IModelList } from "./modelList";
import { IPersonVehicles } from "./personVehicles";
import { IRegisterVehicleRequest } from "./registerVehicleRequest";
import { IRegistration } from "./registration";
import { IRegistrationList } from "./registrationList";
import { IRegistrationNumberType } from "./registrationNumberType";
import { IRegistrationNumberTypeList } from "./registrationNumberTypeList";
import { IUpdateVehicleRequest } from "./updateVehicleRequest";
import { IUpsertBrandRequest } from "./upsertBrandRequest";
import { IUpsertModelRequest } from "./upsertModelRequest";
import { IUpsertNumberTypeRequest } from "./upsertNumberTypeRequest";
import { IUpsertVehicleTypeRequest } from "./upsertVehicleTypeRequest";
import { IVehicle } from "./vehicle";
import { IVehiclePage } from "./vehiclePage";
import { IVehicleStats } from "./vehicleStats";
import { IVehicleType } from "./vehicleType";
import { IVehicleTypeList } from "./vehicleTypeList";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * A vehicle registry: brand/model/type catalogs, vehicles, the brand->manufacturer link, and the
 * ownership+plate registration record. Reads gate on `vehicle.read`; vehicle/registration/
 * manufacturer writes on `vehicle.manage`; catalog-entry writes on `vehicle.catalog.manage`. Writes
 * are audited in-process (D-Audit).
 *
 */
export interface IVehicleService {
    listVehicleTypes(): Promise<IVehicleTypeList>;
    upsertVehicleType(request: IUpsertVehicleTypeRequest): Promise<IVehicleType>;
    listBrands(query?: string | null): Promise<IBrandList>;
    upsertBrand(request: IUpsertBrandRequest): Promise<IBrand>;
    listModels(brandId: string): Promise<IModelList>;
    upsertModel(brandId: string, request: IUpsertModelRequest): Promise<IModel>;
    listRegistrationNumberTypes(): Promise<IRegistrationNumberTypeList>;
    upsertRegistrationNumberType(request: IUpsertNumberTypeRequest): Promise<IRegistrationNumberType>;
    createVehicle(request: ICreateVehicleRequest): Promise<IVehicle>;
    /**
     * List vehicles, token-paginated, narrowed by any combination of the facet filters below
     * (M58 / D-ObjectFacets). Every filter here is also a distribution on `vehicleStats`, so a
     * dashboard and a list are two renderings of one request state. Gated by `vehicle.read`.
     *
     */
    listVehicles(query?: string | null, typeId?: string | null, brandId?: string | null, modelId?: string | null, color?: string | null, status?: string | null, manufactureDateFrom?: string | null, manufactureDateTo?: string | null, registrationCountry?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IVehiclePage>;
    /**
     * Facet distributions over the fleet — the dashboard half of the facet vocabulary (M58 /
     * D-ObjectFacets). Takes exactly the filter args `listVehicles` takes (minus paging) plus an
     * optional `facets` CSV, so a dashboard and a list are two renderings of ONE request state
     * and a chart segment is a link to the same URL with one more filter applied.
     *
     * `totalCount` equals the number of rows exhaustively paging `listVehicles` with these same
     * filters would return. One round-trip serves the whole dashboard.
     *
     * ONE aggregate arm, with no subject and no scoped twin — for the same reason
     * `externalOrgStats` has one, and NOT the audit ledger's reason. `vehicle_vehicles` carries no
     * row-level security, no unit column and no reach predicate: `vehicle.read` held anywhere is
     * the whole visibility decision, so there is nothing for a second arm to narrow.
     *
     * The path is `/stats/vehicles` rather than `/vehicles/stats` because the server's router
     * rejects a literal path segment that is a sibling of `{vehicleId}`.
     *
     */
    vehicleStats(facets?: string | null, query?: string | null, typeId?: string | null, brandId?: string | null, modelId?: string | null, color?: string | null, status?: string | null, manufactureDateFrom?: string | null, manufactureDateTo?: string | null, registrationCountry?: string | null): Promise<IVehicleStats>;
    getVehicle(vehicleId: string): Promise<IVehicle>;
    updateVehicle(vehicleId: string, request: IUpdateVehicleRequest): Promise<IVehicle>;
    deleteVehicle(vehicleId: string): Promise<void>;
    listRegistrations(vehicleId: string): Promise<IRegistrationList>;
    /** Register (or transfer) the vehicle to a new owner; closes any active registration first. */
    registerVehicle(vehicleId: string, request: IRegisterVehicleRequest): Promise<IRegistration>;
    /** End an active registration without a new owner (e.g. deregistration). */
    closeRegistration(registrationId: string): Promise<IRegistration>;
    listManufacturers(brandId: string): Promise<IManufacturerList>;
    addManufacturer(brandId: string, request: IAddManufacturerRequest): Promise<IManufacturer>;
    removeManufacturer(manufacturerId: string): Promise<void>;
    /** Read-only view of the vehicle registrations a person owns (current + historical). */
    listPersonVehicles(personId: string): Promise<IPersonVehicles>;
}

export class VehicleService implements IVehicleService {
    constructor(private bridge: IHttpApiBridge) {
    }

    public listVehicleTypes(): Promise<IVehicleTypeList> {
        return this.bridge.call<IVehicleTypeList>(
            "VehicleService",
            "listVehicleTypes",
            "GET",
            "/vehicle/v1/vehicle-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertVehicleType(request: IUpsertVehicleTypeRequest): Promise<IVehicleType> {
        return this.bridge.call<IVehicleType>(
            "VehicleService",
            "upsertVehicleType",
            "PUT",
            "/vehicle/v1/vehicle-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listBrands(query?: string | null): Promise<IBrandList> {
        return this.bridge.call<IBrandList>(
            "VehicleService",
            "listBrands",
            "GET",
            "/vehicle/v1/brands",
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

    public upsertBrand(request: IUpsertBrandRequest): Promise<IBrand> {
        return this.bridge.call<IBrand>(
            "VehicleService",
            "upsertBrand",
            "PUT",
            "/vehicle/v1/brands",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listModels(brandId: string): Promise<IModelList> {
        return this.bridge.call<IModelList>(
            "VehicleService",
            "listModels",
            "GET",
            "/vehicle/v1/brands/{brandId}/models",
            __undefined,
            __undefined,
            __undefined,
            [
                brandId,
            ],
            __undefined,
            __undefined
        );
    }

    public upsertModel(brandId: string, request: IUpsertModelRequest): Promise<IModel> {
        return this.bridge.call<IModel>(
            "VehicleService",
            "upsertModel",
            "PUT",
            "/vehicle/v1/brands/{brandId}/models",
            request,
            __undefined,
            __undefined,
            [
                brandId,
            ],
            __undefined,
            __undefined
        );
    }

    public listRegistrationNumberTypes(): Promise<IRegistrationNumberTypeList> {
        return this.bridge.call<IRegistrationNumberTypeList>(
            "VehicleService",
            "listRegistrationNumberTypes",
            "GET",
            "/vehicle/v1/registration-number-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertRegistrationNumberType(request: IUpsertNumberTypeRequest): Promise<IRegistrationNumberType> {
        return this.bridge.call<IRegistrationNumberType>(
            "VehicleService",
            "upsertRegistrationNumberType",
            "PUT",
            "/vehicle/v1/registration-number-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public createVehicle(request: ICreateVehicleRequest): Promise<IVehicle> {
        return this.bridge.call<IVehicle>(
            "VehicleService",
            "createVehicle",
            "POST",
            "/vehicle/v1/vehicles",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * List vehicles, token-paginated, narrowed by any combination of the facet filters below
     * (M58 / D-ObjectFacets). Every filter here is also a distribution on `vehicleStats`, so a
     * dashboard and a list are two renderings of one request state. Gated by `vehicle.read`.
     *
     */
    public listVehicles(query?: string | null, typeId?: string | null, brandId?: string | null, modelId?: string | null, color?: string | null, status?: string | null, manufactureDateFrom?: string | null, manufactureDateTo?: string | null, registrationCountry?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IVehiclePage> {
        return this.bridge.call<IVehiclePage>(
            "VehicleService",
            "listVehicles",
            "GET",
            "/vehicle/v1/vehicles",
            __undefined,
            __undefined,
            {
                "query": query,
                "typeId": typeId,
                "brandId": brandId,
                "modelId": modelId,
                "color": color,
                "status": status,
                "manufactureDateFrom": manufactureDateFrom,
                "manufactureDateTo": manufactureDateTo,
                "registrationCountry": registrationCountry,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the fleet — the dashboard half of the facet vocabulary (M58 /
     * D-ObjectFacets). Takes exactly the filter args `listVehicles` takes (minus paging) plus an
     * optional `facets` CSV, so a dashboard and a list are two renderings of ONE request state
     * and a chart segment is a link to the same URL with one more filter applied.
     *
     * `totalCount` equals the number of rows exhaustively paging `listVehicles` with these same
     * filters would return. One round-trip serves the whole dashboard.
     *
     * ONE aggregate arm, with no subject and no scoped twin — for the same reason
     * `externalOrgStats` has one, and NOT the audit ledger's reason. `vehicle_vehicles` carries no
     * row-level security, no unit column and no reach predicate: `vehicle.read` held anywhere is
     * the whole visibility decision, so there is nothing for a second arm to narrow.
     *
     * The path is `/stats/vehicles` rather than `/vehicles/stats` because the server's router
     * rejects a literal path segment that is a sibling of `{vehicleId}`.
     *
     */
    public vehicleStats(facets?: string | null, query?: string | null, typeId?: string | null, brandId?: string | null, modelId?: string | null, color?: string | null, status?: string | null, manufactureDateFrom?: string | null, manufactureDateTo?: string | null, registrationCountry?: string | null): Promise<IVehicleStats> {
        return this.bridge.call<IVehicleStats>(
            "VehicleService",
            "vehicleStats",
            "GET",
            "/vehicle/v1/stats/vehicles",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "query": query,
                "typeId": typeId,
                "brandId": brandId,
                "modelId": modelId,
                "color": color,
                "status": status,
                "manufactureDateFrom": manufactureDateFrom,
                "manufactureDateTo": manufactureDateTo,
                "registrationCountry": registrationCountry,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public getVehicle(vehicleId: string): Promise<IVehicle> {
        return this.bridge.call<IVehicle>(
            "VehicleService",
            "getVehicle",
            "GET",
            "/vehicle/v1/vehicles/{vehicleId}",
            __undefined,
            __undefined,
            __undefined,
            [
                vehicleId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateVehicle(vehicleId: string, request: IUpdateVehicleRequest): Promise<IVehicle> {
        return this.bridge.call<IVehicle>(
            "VehicleService",
            "updateVehicle",
            "PUT",
            "/vehicle/v1/vehicles/{vehicleId}",
            request,
            __undefined,
            __undefined,
            [
                vehicleId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteVehicle(vehicleId: string): Promise<void> {
        return this.bridge.call<void>(
            "VehicleService",
            "deleteVehicle",
            "DELETE",
            "/vehicle/v1/vehicles/{vehicleId}",
            __undefined,
            __undefined,
            __undefined,
            [
                vehicleId,
            ],
            __undefined,
            __undefined
        );
    }

    public listRegistrations(vehicleId: string): Promise<IRegistrationList> {
        return this.bridge.call<IRegistrationList>(
            "VehicleService",
            "listRegistrations",
            "GET",
            "/vehicle/v1/vehicles/{vehicleId}/registrations",
            __undefined,
            __undefined,
            __undefined,
            [
                vehicleId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Register (or transfer) the vehicle to a new owner; closes any active registration first. */
    public registerVehicle(vehicleId: string, request: IRegisterVehicleRequest): Promise<IRegistration> {
        return this.bridge.call<IRegistration>(
            "VehicleService",
            "registerVehicle",
            "POST",
            "/vehicle/v1/vehicles/{vehicleId}/registrations",
            request,
            __undefined,
            __undefined,
            [
                vehicleId,
            ],
            __undefined,
            __undefined
        );
    }

    /** End an active registration without a new owner (e.g. deregistration). */
    public closeRegistration(registrationId: string): Promise<IRegistration> {
        return this.bridge.call<IRegistration>(
            "VehicleService",
            "closeRegistration",
            "POST",
            "/vehicle/v1/registrations/{registrationId}/close",
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

    public listManufacturers(brandId: string): Promise<IManufacturerList> {
        return this.bridge.call<IManufacturerList>(
            "VehicleService",
            "listManufacturers",
            "GET",
            "/vehicle/v1/brands/{brandId}/manufacturers",
            __undefined,
            __undefined,
            __undefined,
            [
                brandId,
            ],
            __undefined,
            __undefined
        );
    }

    public addManufacturer(brandId: string, request: IAddManufacturerRequest): Promise<IManufacturer> {
        return this.bridge.call<IManufacturer>(
            "VehicleService",
            "addManufacturer",
            "POST",
            "/vehicle/v1/brands/{brandId}/manufacturers",
            request,
            __undefined,
            __undefined,
            [
                brandId,
            ],
            __undefined,
            __undefined
        );
    }

    public removeManufacturer(manufacturerId: string): Promise<void> {
        return this.bridge.call<void>(
            "VehicleService",
            "removeManufacturer",
            "DELETE",
            "/vehicle/v1/manufacturers/{manufacturerId}",
            __undefined,
            __undefined,
            __undefined,
            [
                manufacturerId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Read-only view of the vehicle registrations a person owns (current + historical). */
    public listPersonVehicles(personId: string): Promise<IPersonVehicles> {
        return this.bridge.call<IPersonVehicles>(
            "VehicleService",
            "listPersonVehicles",
            "GET",
            "/vehicle/v1/persons/{personId}/vehicles",
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
