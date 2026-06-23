import { IAddCategoryRequest } from "./addCategoryRequest";
import { IAddRankRequest } from "./addRankRequest";
import { IAddSystemRequest } from "./addSystemRequest";
import { IAddTypeRequest } from "./addTypeRequest";
import { IImportRankSchemeRequest } from "./importRankSchemeRequest";
import { IImportRankSchemeResponse } from "./importRankSchemeResponse";
import { IRank } from "./rank";
import { IRankCategory } from "./rankCategory";
import { IRankGrade } from "./rankGrade";
import { IRankScheme } from "./rankScheme";
import { IRankSystem } from "./rankSystem";
import { IRankType } from "./rankType";
import { IUpdateCategoryRequest } from "./updateCategoryRequest";
import { IUpdateRankRequest } from "./updateRankRequest";
import { IUpdateSystemRequest } from "./updateSystemRequest";
import { IUpdateTypeRequest } from "./updateTypeRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The single system-wide rank scheme (L-OneRankScheme / D-Rank). Reads gate on
 * `rank.scheme.read` (bundled in the unit-reader base role); writes are instance-scope
 * (`rank.scheme.manage`) — both enforced once authorization lands (M7). Writes are audited
 * in-process (D-Audit). Rank is a directory attribute and never an authorization input.
 *
 */
export interface IRankService {
    /** Read the whole scheme (systems -> categories -> types -> ranks) in seniority order. */
    getRankScheme(): Promise<IRankScheme>;
    /** Read the standardized-grade comparator catalog (NATO STANAG 2116), ordered by tier then ordinal. */
    getRankGrades(): Promise<Array<IRankGrade>>;
    /** Add a rank system. Returns Rank:RankCodeConflict if the code is taken among active systems. */
    addSystem(request: IAddSystemRequest): Promise<IRankSystem>;
    /** Edit/reorder a system. `code` is immutable by convention. */
    updateSystem(systemId: string, request: IUpdateSystemRequest): Promise<IRankSystem>;
    /**
     * Import a preset rank-system subtree (system -> categories -> types -> ranks) as a code-keyed
     * idempotent upsert in one transaction (additive; never deletes). Returns a created/updated/
     * skipped summary. Re-importing an unchanged preset reports all-skipped.
     *
     */
    importRankScheme(request: IImportRankSchemeRequest): Promise<IImportRankSchemeResponse>;
    /**
     * Add a category under a system (systemId). Returns Rank:RankSystemNotFound if the system is
     * absent, or Rank:RankCodeConflict if the code is taken among the system's active categories.
     *
     */
    addCategory(request: IAddCategoryRequest): Promise<IRankCategory>;
    /** Edit/reorder a category. `code` is immutable by convention. */
    updateCategory(categoryId: string, request: IUpdateCategoryRequest): Promise<IRankCategory>;
    /**
     * Add a type under a category (categoryId) or nested under a parent type (parentTypeId).
     * Returns Rank:RankCategoryNotFound or Rank:RankTypeNotFound if the named parent is absent,
     * or Rank:RankInvalid if the parent type already holds ranks (leaf types only).
     *
     */
    addType(request: IAddTypeRequest): Promise<IRankType>;
    /** Edit/reorder a type. `code` is immutable. */
    updateType(typeId: string, request: IUpdateTypeRequest): Promise<IRankType>;
    /** Add a rank under a type. Returns Rank:RankTypeNotFound if the parent is absent. */
    addRank(request: IAddRankRequest): Promise<IRank>;
    /** Edit/reorder a rank. `code` is immutable. */
    updateRank(rankId: string, request: IUpdateRankRequest): Promise<IRank>;
    /**
     * Soft-delete a scheme node, blocked if in use (a system with active categories, a category with
     * active types, a type with active ranks or child types, or — post-M5 — a rank held by a person).
     * `level` is one of system | category | type | rank.
     *
     */
    deleteNode(level: string, nodeId: string): Promise<void>;
}

export class RankService implements IRankService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Read the whole scheme (systems -> categories -> types -> ranks) in seniority order. */
    public getRankScheme(): Promise<IRankScheme> {
        return this.bridge.call<IRankScheme>(
            "RankService",
            "getRankScheme",
            "GET",
            "/rank/v1/rank-scheme",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Read the standardized-grade comparator catalog (NATO STANAG 2116), ordered by tier then ordinal. */
    public getRankGrades(): Promise<Array<IRankGrade>> {
        return this.bridge.call<Array<IRankGrade>>(
            "RankService",
            "getRankGrades",
            "GET",
            "/rank/v1/rank-grades",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add a rank system. Returns Rank:RankCodeConflict if the code is taken among active systems. */
    public addSystem(request: IAddSystemRequest): Promise<IRankSystem> {
        return this.bridge.call<IRankSystem>(
            "RankService",
            "addSystem",
            "POST",
            "/rank/v1/rank-scheme/systems",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Edit/reorder a system. `code` is immutable by convention. */
    public updateSystem(systemId: string, request: IUpdateSystemRequest): Promise<IRankSystem> {
        return this.bridge.call<IRankSystem>(
            "RankService",
            "updateSystem",
            "PUT",
            "/rank/v1/rank-scheme/systems/{systemId}",
            request,
            __undefined,
            __undefined,
            [
                systemId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Import a preset rank-system subtree (system -> categories -> types -> ranks) as a code-keyed
     * idempotent upsert in one transaction (additive; never deletes). Returns a created/updated/
     * skipped summary. Re-importing an unchanged preset reports all-skipped.
     *
     */
    public importRankScheme(request: IImportRankSchemeRequest): Promise<IImportRankSchemeResponse> {
        return this.bridge.call<IImportRankSchemeResponse>(
            "RankService",
            "importRankScheme",
            "POST",
            "/rank/v1/rank-scheme/import",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Add a category under a system (systemId). Returns Rank:RankSystemNotFound if the system is
     * absent, or Rank:RankCodeConflict if the code is taken among the system's active categories.
     *
     */
    public addCategory(request: IAddCategoryRequest): Promise<IRankCategory> {
        return this.bridge.call<IRankCategory>(
            "RankService",
            "addCategory",
            "POST",
            "/rank/v1/rank-scheme/categories",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Edit/reorder a category. `code` is immutable by convention. */
    public updateCategory(categoryId: string, request: IUpdateCategoryRequest): Promise<IRankCategory> {
        return this.bridge.call<IRankCategory>(
            "RankService",
            "updateCategory",
            "PUT",
            "/rank/v1/rank-scheme/categories/{categoryId}",
            request,
            __undefined,
            __undefined,
            [
                categoryId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Add a type under a category (categoryId) or nested under a parent type (parentTypeId).
     * Returns Rank:RankCategoryNotFound or Rank:RankTypeNotFound if the named parent is absent,
     * or Rank:RankInvalid if the parent type already holds ranks (leaf types only).
     *
     */
    public addType(request: IAddTypeRequest): Promise<IRankType> {
        return this.bridge.call<IRankType>(
            "RankService",
            "addType",
            "POST",
            "/rank/v1/rank-scheme/types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Edit/reorder a type. `code` is immutable. */
    public updateType(typeId: string, request: IUpdateTypeRequest): Promise<IRankType> {
        return this.bridge.call<IRankType>(
            "RankService",
            "updateType",
            "PUT",
            "/rank/v1/rank-scheme/types/{typeId}",
            request,
            __undefined,
            __undefined,
            [
                typeId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a rank under a type. Returns Rank:RankTypeNotFound if the parent is absent. */
    public addRank(request: IAddRankRequest): Promise<IRank> {
        return this.bridge.call<IRank>(
            "RankService",
            "addRank",
            "POST",
            "/rank/v1/rank-scheme/ranks",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Edit/reorder a rank. `code` is immutable. */
    public updateRank(rankId: string, request: IUpdateRankRequest): Promise<IRank> {
        return this.bridge.call<IRank>(
            "RankService",
            "updateRank",
            "PUT",
            "/rank/v1/rank-scheme/ranks/{rankId}",
            request,
            __undefined,
            __undefined,
            [
                rankId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Soft-delete a scheme node, blocked if in use (a system with active categories, a category with
     * active types, a type with active ranks or child types, or — post-M5 — a rank held by a person).
     * `level` is one of system | category | type | rank.
     *
     */
    public deleteNode(level: string, nodeId: string): Promise<void> {
        return this.bridge.call<void>(
            "RankService",
            "deleteNode",
            "DELETE",
            "/rank/v1/rank-scheme/{level}/{nodeId}",
            __undefined,
            __undefined,
            __undefined,
            [
                level,
                nodeId,
            ],
            __undefined,
            __undefined
        );
    }
}
