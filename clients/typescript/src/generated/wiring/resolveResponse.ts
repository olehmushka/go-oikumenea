/**
 * `resolved` maps each FOUND code to its RID; `unresolved` lists the codes with no match, so a
 * connector can act on both without a second call.
 *
 */
export interface IResolveResponse {
    'resolved': { [key: string]: string };
    'unresolved': Array<string>;
}
