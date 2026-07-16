/**
 * The HTTP binding an action invocation targets (D-ActionInvocation, review-2026-09 R-33),
 * derived from the Conjure IR by tools/genactionendpoints — never hand-authored, so it cannot
 * drift from the contract.
 *
 */
export interface IActionEndpoint {
    /** HTTP method, e.g. POST, PUT, DELETE. */
    'method': string;
    /** Conjure path template, e.g. /person/v1/persons/{personId}/emails. */
    'path': string;
    /**
     * Path parameter names in path order. By convention the first is the target object's own
     * RID (the object the action runs on); any remainder are sub-resource ids the caller supplies
     * (e.g. the card id of a finance.card.update).
     *
     */
    'pathParams': Array<string>;
}
