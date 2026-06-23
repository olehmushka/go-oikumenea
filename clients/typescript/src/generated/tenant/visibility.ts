/** A unit's read-time visibility. SHADOW units are filtered by the shadow gate (M7). */
export namespace Visibility {
    export type PUBLIC = "PUBLIC";
    export type SHADOW = "SHADOW";

    export const PUBLIC = "PUBLIC" as "PUBLIC";
    export const SHADOW = "SHADOW" as "SHADOW";
}

export type Visibility = keyof typeof Visibility;
