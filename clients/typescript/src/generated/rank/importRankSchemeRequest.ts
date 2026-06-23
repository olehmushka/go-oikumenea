import { IImportSystem } from "./importSystem";

/**
 * A preset rank-system subtree to import (D-RankSystems): one system with its categories ->
 * types (a tree) -> ranks, each carrying a stable `code`. Applied as a code-keyed idempotent
 * upsert in one transaction (additive; never deletes).
 *
 */
export interface IImportRankSchemeRequest {
    'system': IImportSystem;
}
