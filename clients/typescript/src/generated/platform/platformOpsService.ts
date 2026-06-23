import { IVersionInfo } from "./versionInfo";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/** Unauthenticated operational endpoints owned by the platform module. */
export interface IPlatformOpsService {
    /** Report the binary + applied schema revision. */
    version(): Promise<IVersionInfo>;
    /** Always returns Platform:DemoError (M0 SerializableError smoke test). */
    demoError(): Promise<void>;
}

export class PlatformOpsService implements IPlatformOpsService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Report the binary + applied schema revision. */
    public version(): Promise<IVersionInfo> {
        return this.bridge.call<IVersionInfo>(
            "PlatformOpsService",
            "version",
            "GET",
            "/platform/v1/status/version",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Always returns Platform:DemoError (M0 SerializableError smoke test). */
    public demoError(): Promise<void> {
        return this.bridge.call<void>(
            "PlatformOpsService",
            "demoError",
            "GET",
            "/platform/v1/demo/error",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }
}
