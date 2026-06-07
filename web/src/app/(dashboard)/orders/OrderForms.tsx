"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { mutate } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { EntitySelect } from "@/components/EntitySelect";
import { Localized } from "@/components/Localized";
import type { Order, OrderType } from "@/lib/api/types";

const ORDER_CATEGORIES = [
  "personnel-list",
  "appointment",
  "leave-travel",
  "discipline-incentive",
  "duty-roster",
];
const ORDER_EFFECTS = ["membership-start", "membership-end", "rank-change", "record-only"];

/** Create an order against a unit with a single initial item (more can be added via the API). */
export function OrderCreate({
  unitId,
  orderTypes,
}: {
  unitId: string;
  orderTypes: OrderType[];
}) {
  const router = useRouter();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        setBusy(true);
        setErr(null);
        const body = {
          number: String(f.get("number") || "").trim() || undefined,
          issuedOn: String(f.get("issuedOn") || "").trim() || undefined,
          items: [
            {
              typeId: String(f.get("typeId") || "").trim(),
              personId: String(f.get("personId") || "").trim(),
              unitId: String(f.get("itemUnitId") || "").trim() || undefined,
              note: String(f.get("note") || "").trim() || undefined,
            },
          ],
        };
        (async () => {
          try {
            const o = await mutate<{ id: string }>(
              "POST",
              `/order/v1/units/${unitId}/orders`,
              body,
            );
            router.push(`/orders/${o.id}`);
          } catch (e) {
            setErr(e);
            setBusy(false);
          }
        })();
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900">New order (наказ)</h3>
      <p className="text-xs text-slate-500">
        Created in DRAFT. Effects apply only when you <em>issue</em> it.
      </p>
      {err ? <ErrorBox error={err} /> : null}
      <div className="grid grid-cols-2 gap-3">
        <input name="number" className="input" placeholder="order number" />
        <input name="issuedOn" type="date" className="input" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <select name="typeId" required className="input" defaultValue="">
          <option value="" disabled>
            item type…
          </option>
          {orderTypes.map((t) => (
            <option key={t.id} value={t.id}>
              {t.code}
            </option>
          ))}
        </select>
        <EntitySelect name="personId" kind="person" required placeholder="subject person…" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <EntitySelect name="itemUnitId" kind="unit" allowEmpty placeholder="item unit (optional)…" />
        <input name="note" className="input" placeholder="note (optional)" />
      </div>
      <button type="submit" className="btn-primary" disabled={busy}>
        {busy ? "Creating…" : "Create order"}
      </button>
    </form>
  );
}

/** Edit a DRAFT order's number / issued-on. PUT /order/v1/orders/{id} (rejected once issued). */
export function EditOrder({ order }: { order: Order }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  if ((order.status ?? "").toUpperCase() !== "DRAFT") return null;
  if (!open) {
    return (
      <button type="button" className="btn-ghost" onClick={() => setOpen(true)}>
        Edit draft
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
            await mutate("PUT", `/order/v1/orders/${order.id}`, {
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
      <input name="number" className="input" placeholder="number" defaultValue={order.number} />
      <input name="issuedOn" type="date" className="input" defaultValue={order.issuedOn} />
      <div className="flex gap-2">
        <button className="btn-primary" disabled={busy}>
          Save
        </button>
        <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
          Cancel
        </button>
      </div>
    </form>
  );
}

/** Create / edit / retire order types (the catalog). */
export function OrderTypeManager({ types }: { types: OrderType[] }) {
  const router = useRouter();
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
      <h3 className="text-sm font-semibold text-slate-900">Order types</h3>
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
                      mutate("PUT", `/order/v1/order-types/${t.id}`, {
                        name: String(f.get("name") || "").trim() || undefined,
                        status: String(f.get("status") || "").trim() || undefined,
                      }),
                    () => setEditing(null),
                  );
                }}
              >
                <input name="name" className="input" defaultValue={t.code} placeholder="name" />
                <select name="status" className="input" defaultValue={t.status ?? "active"}>
                  <option value="active">active</option>
                  <option value="retired">retired</option>
                </select>
                <button className="btn-primary" disabled={busy}>
                  Save
                </button>
                <button type="button" className="btn-ghost" onClick={() => setEditing(null)}>
                  Cancel
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
                  Edit
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
              mutate("POST", "/order/v1/order-types", {
                code: String(f.get("code") || "").trim(),
                name: String(f.get("name") || "").trim(),
                category: String(f.get("category") || ""),
                effect: String(f.get("effect") || ""),
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="code" required className="input" placeholder="code" />
        <input name="name" required className="input" placeholder="name" />
        <select name="category" required className="input" defaultValue="">
          <option value="" disabled>
            category…
          </option>
          {ORDER_CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
        <select name="effect" required className="input" defaultValue="">
          <option value="" disabled>
            effect…
          </option>
          {ORDER_EFFECTS.map((x) => (
            <option key={x} value={x}>
              {x}
            </option>
          ))}
        </select>
        <button className="btn-ghost col-span-2" disabled={busy}>
          Add order type
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
      await mutate("POST", `/order/v1/orders/${orderId}/${verb}`, body);
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
          disabled={busy || status === "ISSUED" || status === "REVOKED"}
          onClick={() => act("issue")}
        >
          Issue
        </button>
        <button
          className="btn-ghost"
          disabled={busy || status === "REVOKED"}
          onClick={() => act("revoke", {})}
        >
          Revoke
        </button>
      </div>
      {err ? <ErrorBox error={err} /> : null}
    </div>
  );
}
