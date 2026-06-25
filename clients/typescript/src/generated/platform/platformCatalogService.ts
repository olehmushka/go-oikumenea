import { ILegalBasisKind } from "./legalBasisKind";
import { ILegalBasisKindList } from "./legalBasisKindList";
import { IUpsertLegalBasisKindRequest } from "./upsertLegalBasisKindRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Cross-cutting platform reference catalogs (D-OverlayFoundation, M29). Today: the GDPR
 * lawful-basis catalog (`platform_legal_basis_kinds`), referenced by every future pii:special
 * overlay store. Reads require `legal-basis.read`; writes the instance-plane `legal-basis.manage`.
 *
 */
export interface IPlatformCatalogService {
    /** List the GDPR lawful-basis catalog (Article 6 bases + Article 9 conditions). */
    listLegalBasisKinds(): Promise<ILegalBasisKindList>;
    /** Add or update a lawful-basis catalog entry (instance-admin; `legal-basis.manage`). */
    upsertLegalBasisKind(code: string, request: IUpsertLegalBasisKindRequest): Promise<ILegalBasisKind>;
}

export class PlatformCatalogService implements IPlatformCatalogService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** List the GDPR lawful-basis catalog (Article 6 bases + Article 9 conditions). */
    public listLegalBasisKinds(): Promise<ILegalBasisKindList> {
        return this.bridge.call<ILegalBasisKindList>(
            "PlatformCatalogService",
            "listLegalBasisKinds",
            "GET",
            "/platform/v1/legal-basis-kinds",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add or update a lawful-basis catalog entry (instance-admin; `legal-basis.manage`). */
    public upsertLegalBasisKind(code: string, request: IUpsertLegalBasisKindRequest): Promise<ILegalBasisKind> {
        return this.bridge.call<ILegalBasisKind>(
            "PlatformCatalogService",
            "upsertLegalBasisKind",
            "PUT",
            "/platform/v1/legal-basis-kinds/{code}",
            request,
            __undefined,
            __undefined,
            [
                code,
            ],
            __undefined,
            __undefined
        );
    }
}
