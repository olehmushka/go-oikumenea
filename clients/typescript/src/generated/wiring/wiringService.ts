import { ICountryList } from "./countryList";
import { ILanguoidPage } from "./languoidPage";
import { ILegalBasisList } from "./legalBasisList";
import { IResolveRequest } from "./resolveRequest";
import { IResolveResponse } from "./resolveResponse";
import { IWiringSelf } from "./wiringSelf";
import { IWritingSystemList } from "./writingSystemList";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The pull-wiring read API (M53 / D-ConnectorPlane). Machine-facing: every endpoint is gated on a
 * `wiring.*` code held by a service principal, each surface its own code. Reads only.
 *
 */
export interface IWiringService {
    /** Resolve natural keys to RIDs within one catalog (`wiring.resolve`). */
    resolveKeys(request: IResolveRequest): Promise<IResolveResponse>;
    /** The country reference catalog (`wiring.catalog.read`). */
    listCountries(): Promise<ICountryList>;
    /** The ISO-15924 writing-system catalog (`wiring.catalog.read`). */
    listWritingSystems(): Promise<IWritingSystemList>;
    /** The Glottolog languoid catalog, keyset-paginated (`wiring.catalog.read`). */
    listLanguoids(query?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ILanguoidPage>;
    /** The GDPR lawful-basis catalog (`wiring.catalog.read`). */
    listLegalBasisKinds(): Promise<ILegalBasisList>;
    /**
     * The calling connector's own registry row + its sources with cursors (`wiring.cursor.read`).
     * Returns Wiring:ConnectorNotRegistered if the principal has not registered a connector yet.
     *
     */
    readSelf(): Promise<IWiringSelf>;
}

export class WiringService implements IWiringService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Resolve natural keys to RIDs within one catalog (`wiring.resolve`). */
    public resolveKeys(request: IResolveRequest): Promise<IResolveResponse> {
        return this.bridge.call<IResolveResponse>(
            "WiringService",
            "resolveKeys",
            "POST",
            "/wiring/v1/resolve",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** The country reference catalog (`wiring.catalog.read`). */
    public listCountries(): Promise<ICountryList> {
        return this.bridge.call<ICountryList>(
            "WiringService",
            "listCountries",
            "GET",
            "/wiring/v1/catalogs/countries",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** The ISO-15924 writing-system catalog (`wiring.catalog.read`). */
    public listWritingSystems(): Promise<IWritingSystemList> {
        return this.bridge.call<IWritingSystemList>(
            "WiringService",
            "listWritingSystems",
            "GET",
            "/wiring/v1/catalogs/writing-systems",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** The Glottolog languoid catalog, keyset-paginated (`wiring.catalog.read`). */
    public listLanguoids(query?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ILanguoidPage> {
        return this.bridge.call<ILanguoidPage>(
            "WiringService",
            "listLanguoids",
            "GET",
            "/wiring/v1/catalogs/languoids",
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

    /** The GDPR lawful-basis catalog (`wiring.catalog.read`). */
    public listLegalBasisKinds(): Promise<ILegalBasisList> {
        return this.bridge.call<ILegalBasisList>(
            "WiringService",
            "listLegalBasisKinds",
            "GET",
            "/wiring/v1/catalogs/legal-basis",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * The calling connector's own registry row + its sources with cursors (`wiring.cursor.read`).
     * Returns Wiring:ConnectorNotRegistered if the principal has not registered a connector yet.
     *
     */
    public readSelf(): Promise<IWiringSelf> {
        return this.bridge.call<IWiringSelf>(
            "WiringService",
            "readSelf",
            "GET",
            "/wiring/v1/self",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }
}
