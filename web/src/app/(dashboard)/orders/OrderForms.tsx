"use client";

import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { api } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { Localized } from "@/components/Localized";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";
import { OrderItemRows, emptyRow, rowsReady, toInputs, typeIndex, type ItemRow } from "./OrderItemRows";
import { isDraft, isIssued } from "./status";
import type { Order, OrderItemInput, OrderType } from "@/lib/api/types";

const ORDER_CATEGORIES = [
  "personnel-list",
  "appointment",
  "leave-travel",
  "discipline-incentive",
  "duty-roster",
];
const ORDER_EFFECTS = ["membership-start", "membership-end", "rank-change", "record-only"];

/**
 * Create a draft order against an issuing unit, with as many items ("points") as the наказ has.
 *
 * Controlled state rather than the FormData pattern used elsewhere in this file: a FormData form
 * cannot express a variable row count, and mixing the two would be worse than either.
 */
export function OrderCreate({
  unitId,
  orderTypes,
}: {
  unitId: string;
  orderTypes: OrderType[];
}) {
  const router = useRouter();
  const tr = useTg();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [number, setNumber] = useState("");
  const [issuedOn, setIssuedOn] = useState("");
  const [rows, setRows] = useState<ItemRow[]>(() => [emptyRow()]);
  const byId = useMemo(() => typeIndex(orderTypes), [orderTypes]);
  const ready = rowsReady(rows, byId);

  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        setBusy(true);
        setErr(null);
        const body: CreateOrderRequestBody = {
          number: number.trim() || undefined,
          issuedOn: issuedOn.trim() || undefined,
          items: toInputs(rows, byId),
        };
        (async () => {
          try {
            const o = await api.order.createOrder(unitId, body as never);
            router.push(`/orders/${o.id}`);
          } catch (e) {
            setErr(e);
            setBusy(false);
          }
        })();
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900"><T>New order (наказ)</T></h3>
      <p className="text-xs text-slate-500">
        <T>Created in DRAFT. Effects apply only when you issue it.</T>
      </p>
      {err ? <ErrorBox error={err} /> : null}
      <div className="grid grid-cols-2 gap-3">
        <input
          className="input"
          placeholder={tr("order number")}
          value={number}
          onChange={(e) => setNumber(e.target.value)}
        />
        <input
          type="date"
          className="input"
          value={issuedOn}
          onChange={(e) => setIssuedOn(e.target.value)}
        />
      </div>

      <OrderItemRows rows={rows} onChange={setRows} orderTypes={orderTypes} disabled={busy} />

      <button type="submit" className="btn-primary" disabled={busy || !ready}>
        {busy ? <T>Creating…</T> : <T>Create order</T>}
      </button>
    </form>
  );
}

/** The create payload — `items` is a list, not a single entry (api/order.conjure.yml CreateOrderRequest). */
type CreateOrderRequestBody = {
  number?: string;
  issuedOn?: string;
  items: OrderItemInput[];
};

/** Edit a DRAFT order's number / issued-on. PUT /order/v1/orders/{id} (rejected once issued). */
export function EditOrder({ order }: { order: Order }) {
  const router = useRouter();
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  if (!isDraft(order.status)) return null;
  if (!open) {
    return (
      <button type="button" className="btn-ghost" onClick={() => setOpen(true)}>
        <T>Edit draft</T>
      </button>
    );
  }
  return (
    <form
      className="mt-2 space-y-2"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        setBusy(true);
        setErr(null);
        (async () => {
          try {
            await api.order.updateOrder(order.id, {
              number: String(f.get("number") || "").trim() || undefined,
              issuedOn: String(f.get("issuedOn") || "").trim() || undefined,
            });
            setOpen(false);
            router.refresh();
          } catch (e) {
            setErr(e);
          } finally {
            setBusy(false);
          }
        })();
      }}
    >
      {err ? <ErrorBox error={err} /> : null}
      <input name="number" className="input" placeholder={tr("number")} defaultValue={order.number ?? ""} />
      <input name="issuedOn" type="date" className="input" defaultValue={order.issuedOn ?? ""} />
      <div className="flex gap-2">
        <button className="btn-primary" disabled={busy}>
          <T>Save</T>
        </button>
        <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
          <T>Cancel</T>
        </button>
      </div>
    </form>
  );
}

/** Create / edit / retire order types (the catalog). */
export function OrderTypeManager({ types }: { types: OrderType[] }) {
  const router = useRouter();
  const tr = useTg();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const run = (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    fn()
      .then(() => {
        after?.();
        router.refresh();
      })
      .catch(setErr)
      .finally(() => setBusy(false));
  };
  return (
    <div className="card space-y-3 p-5">
      <h3 className="text-sm font-semibold text-slate-900"><T>Order types</T></h3>
      {err ? <ErrorBox error={err} /> : null}
      <ul className="space-y-1 text-sm">
        {types.map((t) =>
          editing === t.id ? (
            <li key={t.id}>
              <form
                className="flex items-center gap-2"
                onSubmit={(e) => {
                  e.preventDefault();
                  const f = new FormData(e.currentTarget);
                  run(
                    () =>
                      api.order.updateOrderType(t.id, {
                        name: (String(f.get("name") || "").trim() || undefined) as never,
                        status: (String(f.get("status") || "").trim() || undefined) as never,
                      }),
                    () => setEditing(null),
                  );
                }}
              >
                <input name="name" className="input" defaultValue={t.code} placeholder={tr("name")} />
                <select name="status" className="input" defaultValue={t.status ?? "active"}>
                  <option value="active">{tr("active")}</option>
                  <option value="retired">{tr("retired")}</option>
                </select>
                <button className="btn-primary" disabled={busy}>
                  <T>Save</T>
                </button>
                <button type="button" className="btn-ghost" onClick={() => setEditing(null)}>
                  <T>Cancel</T>
                </button>
              </form>
            </li>
          ) : (
            <li key={t.id} className="flex items-center justify-between gap-2">
              <span>
                <Localized map={t.name} fallback={t.code} />{" "}
                <span className="font-mono text-xs text-slate-400">{t.code}</span>
              </span>
              <span className="flex items-center gap-3">
                <span className="text-xs text-slate-400">{t.status}</span>
                <button
                  type="button"
                  className="text-xs font-medium text-indigo-600 hover:underline"
                  onClick={() => setEditing(t.id)}
                >
                  <T>Edit</T>
                </button>
              </span>
            </li>
          ),
        )}
      </ul>
      <form
        className="grid grid-cols-2 gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          const f = new FormData(e.currentTarget);
          const form = e.currentTarget;
          run(
            () =>
              api.order.createOrderType({
                code: String(f.get("code") || "").trim(),
                name: String(f.get("name") || "").trim() as never,
                category: String(f.get("category") || "") as never,
                effect: String(f.get("effect") || "") as never,
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="code" required className="input" placeholder={tr("code")} />
        <input name="name" required className="input" placeholder={tr("name")} />
        <select name="category" required className="input" defaultValue="">
          <option value="" disabled>
            {tr("category…")}
          </option>
          {ORDER_CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
        <select name="effect" required className="input" defaultValue="">
          <option value="" disabled>
            {tr("effect…")}
          </option>
          {ORDER_EFFECTS.map((x) => (
            <option key={x} value={x}>
              {x}
            </option>
          ))}
        </select>
        <button className="btn-ghost col-span-2" disabled={busy}>
          <T>Add order type</T>
        </button>
      </form>
    </div>
  );
}

/** Issue / revoke actions on an order detail page. */
export function OrderActions({ orderId, status }: { orderId: string; status?: string }) {
  const router = useRouter();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  const act = async (verb: "issue" | "revoke", body?: unknown) => {
    if (verb === "revoke" && !window.confirm("Revoke this order?")) return;
    setBusy(true);
    setErr(null);
    try {
      if (verb === "issue") await api.order.issueOrder(orderId);
      else await api.order.revokeOrder(orderId, (body ?? {}) as never);
      router.refresh();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        <button
          className="btn-primary"
          disabled={busy || !isDraft(status)}
          onClick={() => act("issue")}
        >
          <T>Issue</T>
        </button>
        <button
          className="btn-ghost"
          disabled={busy || !isIssued(status)}
          onClick={() => act("revoke", {})}
        >
          <T>Revoke</T>
        </button>
      </div>
      {err ? <ErrorBox error={err} /> : null}
    </div>
  );
}
