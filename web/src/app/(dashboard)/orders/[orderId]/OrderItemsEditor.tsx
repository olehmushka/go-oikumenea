"use client";

// Add / remove / edit the items of a DRAFT order.
//
// FULL REPLACEMENT, no optimistic concurrency: `PUT /order/v1/orders/{id}` with `items` present
// DELETES the draft's items and re-inserts the list it is given (internal/order/application/
// service.go:186-197), and the endpoint carries no If-Match. So a stale editor — someone else edited
// the same draft in another tab — silently overwrites. The key={order.updatedAt} mount from the page
// narrows that window on every successful save; nothing closes it. Acceptable for a single-operator
// admin console editing drafts, but do not read optimistic concurrency into this.
//
// It sends ONLY `items`. The header (number / issuedOn) belongs to EditOrder, and keeping the two
// patches disjoint is what stops an unrelated "fix the order number" save from rewriting the item
// list out of whatever happened to be in the DOM.

import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { api } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { useTg } from "@/lib/locale";
import {
  OrderItemRows,
  rowsFromItems,
  rowsReady,
  toInputs,
  typeIndex,
  type ItemRow,
} from "../OrderItemRows";
import { isDraft } from "../status";
import type { Order, OrderType } from "@/lib/api/types";

export function OrderItemsEditor({
  order,
  orderTypes,
  labels,
}: {
  order: Order;
  orderTypes: OrderType[];
  /** rid → human label, already resolved server-side for the item list above, so the preselected
   *  pickers show names instead of RIDs without a second round-trip. */
  labels?: Record<string, string>;
}) {
  const router = useRouter();
  const tr = useTg();
  const byId = useMemo(() => typeIndex(orderTypes), [orderTypes]);
  const initial = useMemo(() => rowsFromItems(order.items), [order.items]);
  const [rows, setRows] = useState<ItemRow[]>(initial);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);

  if (!isDraft(order.status)) return null;

  // uid is client-only, so compare the serialized payloads — what would actually be sent.
  const dirty = JSON.stringify(toInputs(rows, byId)) !== JSON.stringify(toInputs(initial, byId));
  const ready = rowsReady(rows, byId);

  const save = () => {
    setBusy(true);
    setErr(null);
    api.order
      .updateOrder(order.id, { items: toInputs(rows, byId) } as never)
      .then(() => router.refresh())
      .catch(setErr)
      .finally(() => setBusy(false));
  };

  return (
    <div className="mt-4 space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-slate-900">{tr("Draft items")}</h3>
        <span className="text-xs text-slate-400">{tr("Items lock when the order is issued.")}</span>
      </div>
      {err ? <ErrorBox error={err} /> : null}
      <OrderItemRows rows={rows} onChange={setRows} orderTypes={orderTypes} disabled={busy} labels={labels} />
      <div className="flex gap-2">
        <button type="button" className="btn-primary" disabled={busy || !dirty || !ready} onClick={save}>
          {busy ? tr("Saving…") : tr("Save items")}
        </button>
        <button
          type="button"
          className="btn-ghost"
          disabled={busy || !dirty}
          onClick={() => {
            setRows(initial);
            setErr(null);
          }}
        >
          {tr("Reset")}
        </button>
      </div>
    </div>
  );
}
