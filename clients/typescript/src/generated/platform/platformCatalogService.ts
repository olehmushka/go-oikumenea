import { IColor } from "./color";
import { IColorList } from "./colorList";
import { ILegalBasisKind } from "./legalBasisKind";
import { ILegalBasisKindList } from "./legalBasisKindList";
import { IUpsertColorRequest } from "./upsertColorRequest";
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
    /** List the color catalog (D-Color), optionally filtered to one domain (eye | hair | vehicle). */
    listColors(locales: ReadonlyArray<string>, domain?: string | null): Promise<IColorList>;
    /** Add or update a color (instance-admin; `color.manage`). Upserts on (domain, code). */
    upsertColor(request: IUpsertColorRequest): Promise<IColor>;
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

    /** List the color catalog (D-Color), optionally filtered to one domain (eye | hair | vehicle). */
    public listColors(locales: ReadonlyArray<string>, domain?: string | null): Promise<IColorList> {
        return this.bridge.call<IColorList>(
            "PlatformCatalogService",
            "listColors",
            "GET",
            "/platform/v1/colors",
            __undefined,
            __undefined,
            {
                "locales": locales,
                "domain": domain,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add or update a color (instance-admin; `color.manage`). Upserts on (domain, code). */
    public upsertColor(request: IUpsertColorRequest): Promise<IColor> {
        return this.bridge.call<IColor>(
            "PlatformCatalogService",
            "upsertColor",
            "PUT",
            "/platform/v1/colors",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }
}
