// Order status, read the way the wire actually carries it.
//
// The contract emits LOWERCASE (`draft` | `issued` | `revoked` — internal/order/domain/order.go:80,
// transport/service.go:268). Several console sites compared against "ISSUED"/"REVOKED", which never
// matched: the Issue button stayed enabled on an already-issued order, and every status pill rendered
// slate. One helper, used everywhere, so the comparison cannot be re-typed wrongly.
//
// Pure module (no "use client"): imported by both the server pages and the client forms.

export type OrderStatus = "draft" | "issued" | "revoked";

/** Lowercase/trim the wire value; "" when absent or unrecognized. */
export function normStatus(s?: string | null): OrderStatus | "" {
  const v = String(s ?? "").trim().toLowerCase();
  return v === "draft" || v === "issued" || v === "revoked" ? v : "";
}

/** A draft is the only mutable state: header edits and item edits are rejected once issued. */
export function isDraft(s?: string | null): boolean {
  return normStatus(s) === "draft";
}

export function isIssued(s?: string | null): boolean {
  return normStatus(s) === "issued";
}

export function isRevoked(s?: string | null): boolean {
  return normStatus(s) === "revoked";
}

/** Pill tone for a status, shared by the list, the detail page and the drawer. */
export function statusTone(s?: string | null): "slate" | "green" | "red" {
  const v = normStatus(s);
  if (v === "issued") return "green";
  if (v === "revoked") return "red";
  return "slate";
}
