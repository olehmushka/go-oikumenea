import { ICanonicalEnvelope } from "./canonicalEnvelope";
import { IImportResult } from "./importResult";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The generic reference-data import endpoint (instance-scope, import.manage). Called by the
 * hermenea companion's loader; idempotent and non-destructive.
 *
 */
export interface IImportService {
    /** Idempotently upsert a canonical envelope into the {objectType} catalog. */
    importObjects(objectType: string, envelope: ICanonicalEnvelope): Promise<IImportResult>;
}

export class ImportService implements IImportService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Idempotently upsert a canonical envelope into the {objectType} catalog. */
    public importObjects(objectType: string, envelope: ICanonicalEnvelope): Promise<IImportResult> {
        return this.bridge.call<IImportResult>(
            "ImportService",
            "importObjects",
            "POST",
            "/import/v1/import/{objectType}",
            envelope,
            __undefined,
            __undefined,
            [
                objectType,
            ],
            __undefined,
            __undefined
        );
    }
}
