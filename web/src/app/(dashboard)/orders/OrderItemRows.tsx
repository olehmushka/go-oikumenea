"use client";

// The order's items ("points" of a наказ) as a repeatable row grid — the ONE implementation, shared
// by the create form (OrderForms.OrderCreate) and the draft editor ([orderId]/OrderItemsEditor), so
// the two surfaces cannot drift.
//
// An order carries N items; the console used to hardcode exactly one and expose 4 of the 8 fields,
// which made a rank-change or an appointment order impossible to enter at all (the server requires
// the target its type's effect names).

import { useMemo, useState } from "react";
import { EntitySelect } from "@/components/EntitySelect";
import { ScopedSelect } from "@/components/ScopedSelect";
import { SearchSelect } from "@/components/SearchSelect";
import { useTg } from "@/lib/locale";
import type { OrderItem, OrderItemInput, OrderType } from "@/lib/api/types";

/**
 * One editable item row.
 *
 * Every field is a string, never null/undefined: one uniform "empty means absent" test, and no input
 * ever flips between controlled and uncontrolled. `undefined` reappears only in toInputs, at the
 * serialization boundary.
 */
export type ItemRow = {
  /** client-only React key — never sent; see newUid */
  uid: string;
  typeId: string;
  personId: string;
  unitId: string;
  positionId: string;
  rankId: string;
  effectiveFrom: string;
  effectiveTo: string;
  note: string;
};

// A counter, not the array index and not a RID (new rows have none). The pickers below hold their
// selection in INTERNAL state seeded once from defaultValue, so with index keys, removing row 0 would
// leave row 1's picker instance mounted while the row data shifts up — the form would show the wrong
// person against the wrong type. A stable uid makes removal a real unmount.
let seq = 0;
const newUid = () => `r${++seq}`;

export const emptyRow = (): ItemRow => ({
  uid: newUid(),
  typeId: "",
  personId: "",
  unitId: "",
  positionId: "",
  rankId: "",
  effectiveFrom: "",
  effectiveTo: "",
  note: "",
});

/** Existing items → editable rows (dropping id/orderId/createdAt/updatedAt, which are server-owned). */
export const rowsFromItems = (items: OrderItem[] | undefined): ItemRow[] =>
  (items ?? []).map((it) => ({
    uid: newUid(),
    typeId: it.typeId ?? "",
    personId: it.personId ?? "",
    unitId: it.unitId ?? "",
    positionId: it.positionId ?? "",
    rankId: it.rankId ?? "",
    effectiveFrom: it.effectiveFrom ?? "",
    effectiveTo: it.effectiveTo ?? "",
    note: it.note ?? "",
  }));

/** Copies the previous row's type/targets/dates and clears the person — one type, many people is the
 *  shape of a real наказ, and every fresh UnitSelect otherwise re-asks for the organization. */
export const duplicateRow = (r: ItemRow): ItemRow => ({ ...r, uid: newUid(), personId: "", note: "" });

export const typeIndex = (types: OrderType[]): Map<string, OrderType> =>
  new Map(types.map((t) => [t.id, t]));

/** The effect that drives which targets a row shows and sends; "" when no type is picked yet. */
export const effectOf = (row: ItemRow, byId: Map<string, OrderType>): string =>
  byId.get(row.typeId)?.effect ?? "";

const asOpt = (v: string): string | undefined => (v.trim() === "" ? undefined : v.trim());

/**
 * Rows → wire payload. WHAT ISN'T SHOWN ISN'T SENT: each row is projected through its effect, so a
 * record-only item never ships a rankId the operator typed before switching the type. The values stay
 * in the row (switching the type back restores them — a destructive clear is the bug people actually
 * hit), but they do not reach the server, which only ever checks target PRESENCE, never absence.
 *
 * An UNKNOWN typeId sends everything: failing toward the server's judgment beats silently dropping
 * targets the operator entered.
 */
export const toInputs = (rows: ItemRow[], byId: Map<string, OrderType>): OrderItemInput[] =>
  rows.map((r) => {
    const effect = byId.has(r.typeId) ? effectOf(r, byId) : "*";
    const wantsUnit = effect === "*" || effect === "membership-start" || effect === "membership-end";
    const wantsRank = effect === "*" || effect === "rank-change";
    return {
      typeId: r.typeId.trim(),
      personId: r.personId.trim(),
      unitId: wantsUnit ? asOpt(r.unitId) : undefined,
      positionId: wantsUnit ? asOpt(r.positionId) : undefined,
      rankId: wantsRank ? asOpt(r.rankId) : undefined,
      effectiveFrom: asOpt(r.effectiveFrom),
      effectiveTo: asOpt(r.effectiveTo),
      note: asOpt(r.note),
    };
  });

export type ProblemKey =
  | "Pick an item type."
  | "Pick a subject person."
  | "This item's type needs a unit or a position."
  | "This item's type needs a rank.";

/**
 * MIRROR of internal/order/domain/order.go:188 RequiredTargetsPresent, which the application enforces
 * on BOTH create and update (internal/order/application/service.go:350). This is a UX affordance —
 * the server still decides, and its Order:OrderInvalid surfaces in the ErrorBox above the form.
 */
export function rowProblem(row: ItemRow, effect: string): ProblemKey | null {
  if (row.typeId.trim() === "") return "Pick an item type.";
  if (row.personId.trim() === "") return "Pick a subject person.";
  switch (effect) {
    case "membership-start":
    case "membership-end":
      return row.unitId.trim() === "" && row.positionId.trim() === ""
        ? "This item's type needs a unit or a position."
        : null;
    case "rank-change":
      return row.rankId.trim() === "" ? "This item's type needs a rank." : null;
    default:
      return null;
  }
}

/** True when every row is complete — the submit gate for both consumers. */
export const rowsReady = (rows: ItemRow[], byId: Map<string, OrderType>): boolean =>
  rows.length > 0 && rows.every((r) => rowProblem(r, effectOf(r, byId)) === null);

export function OrderItemRows({
  rows,
  onChange,
  orderTypes,
  minRows = 1,
  disabled = false,
  labels,
}: {
  rows: ItemRow[];
  onChange: (rows: ItemRow[]) => void;
  orderTypes: OrderType[];
  /** the server's floor: an order needs at least one item (application/service.go:170) */
  minRows?: number;
  disabled?: boolean;
  /** rid → human label, for rows that arrive preselected (the draft editor's existing items). The
   *  pickers can only show a RID for a value they did not resolve themselves. */
  labels?: Record<string, string>;
}) {
  const tr = useTg();
  const byId = useMemo(() => typeIndex(orderTypes), [orderTypes]);
  // Grows as the operator picks: a DUPLICATED row carries its source's unit, and its fresh picker
  // instance would otherwise render that unit as a raw RID.
  const [seen, setSeen] = useState<Record<string, string>>({});
  const labelOf = (id: string) => (id ? (seen[id] ?? labels?.[id]) : undefined);
  const remember = (id: string, label?: string) => {
    if (id && label && label !== id) setSeen((m) => (m[id] === label ? m : { ...m, [id]: label }));
  };

  const patch = (uid: string, part: Partial<ItemRow>) =>
    onChange(rows.map((r) => (r.uid === uid ? { ...r, ...part } : r)));
  const remove = (uid: string) => onChange(rows.filter((r) => r.uid !== uid));

  return (
    <div className="space-y-3">
      {rows.map((row, i) => {
        const effect = effectOf(row, byId);
        const problem = rowProblem(row, effect);
        const wantsUnit = effect === "membership-start" || effect === "membership-end";
        const wantsRank = effect === "rank-change";
        return (
          <div key={row.uid} className="rounded-md border border-slate-200 p-3">
            <div className="mb-2 flex items-center justify-between">
              <span className="text-xs font-medium text-slate-500">
                {tr("item")} {i + 1}
                {effect ? <span className="ml-2 font-normal text-slate-400">{effect}</span> : null}
              </span>
              <button
                type="button"
                className="text-xs text-red-600 hover:underline disabled:text-slate-300 disabled:no-underline"
                disabled={disabled || rows.length <= minRows}
                title={rows.length <= minRows ? tr("An order needs at least one item.") : undefined}
                onClick={() => remove(row.uid)}
              >
                {tr("Remove")}
              </button>
            </div>

            <div className="grid gap-2 sm:grid-cols-2">
              <select
                className="input"
                value={row.typeId}
                disabled={disabled}
                onChange={(e) => patch(row.uid, { typeId: e.target.value })}
              >
                <option value="">{tr("item type…")}</option>
                {orderTypes
                  // Retired types stay selectable only where a draft already references one, so an
                  // existing item keeps working while no NEW row can pick it. UI policy; the server
                  // does not enforce it.
                  .filter((t) => t.status !== "retired" || t.id === row.typeId)
                  .map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.code}
                    </option>
                  ))}
              </select>

              {/* SearchSelect, not EntitySelect: the latter fetches one page of ≤200 persons and
                  filters client-side, which at registry scale shows a subset as if it were the whole
                  directory — the operator types a real name, sees nothing, concludes it is absent. */}
              <SearchSelect
                kind="person"
                defaultValue={row.personId}
                defaultLabel={labelOf(row.personId)}
                placeholder={tr("subject person…")}
                onChange={(id, label) => {
                  remember(id, label);
                  patch(row.uid, { personId: id });
                }}
              />

              {wantsUnit ? (
                <>
                  <EntitySelect
                    kind="unit"
                    defaultValue={row.unitId}
                    defaultLabel={labelOf(row.unitId)}
                    allowEmpty
                    placeholder={tr("item unit (optional)…")}
                    onChange={(id, label) => {
                      remember(id, label);
                      // a position belongs to one unit — changing the unit invalidates it
                      patch(row.uid, { unitId: id, positionId: "" });
                    }}
                  />
                  {/* Positions have no global list, so the billet picker is scoped to THIS row's
                      unit and stays disabled until one is chosen. */}
                  <ScopedSelect
                    path={
                      row.unitId
                        ? `/membership/v1/units/${encodeURIComponent(row.unitId)}/positions`
                        : null
                    }
                    pick={(d) => (d as { positions?: unknown[] })?.positions ?? []}
                    value={row.positionId}
                    onChange={(id) => patch(row.uid, { positionId: id })}
                    emptyLabel="item position (optional)…"
                    unscopedLabel="pick an item unit first"
                    className="input"
                  />
                </>
              ) : null}

              {wantsRank ? (
                <EntitySelect
                  kind="rank"
                  defaultValue={row.rankId}
                  allowEmpty
                  placeholder={tr("item rank…")}
                  onChange={(id) => patch(row.uid, { rankId: id })}
                />
              ) : null}

              <label className="flex flex-col gap-1">
                <span className="label mb-0">{tr("Effective from")}</span>
                <input
                  type="date"
                  className="input"
                  value={row.effectiveFrom}
                  disabled={disabled}
                  onChange={(e) => patch(row.uid, { effectiveFrom: e.target.value })}
                />
              </label>
              <label className="flex flex-col gap-1">
                <span className="label mb-0">{tr("Effective to")}</span>
                <input
                  type="date"
                  className="input"
                  value={row.effectiveTo}
                  disabled={disabled}
                  onChange={(e) => patch(row.uid, { effectiveTo: e.target.value })}
                />
              </label>

              <input
                className="input sm:col-span-2"
                placeholder={tr("note (optional)")}
                value={row.note}
                disabled={disabled}
                onChange={(e) => patch(row.uid, { note: e.target.value })}
              />
            </div>

            {wantsUnit ? (
              <p className="mt-2 text-xs text-slate-400">
                {tr("A position, when set, overrides the unit.")}
              </p>
            ) : null}
            {/* Always visible, not submit-triggered, so a disabled submit is never unexplained. */}
            {problem ? <p className="mt-2 text-xs text-amber-700">{tr(problem)}</p> : null}
          </div>
        );
      })}

      <div className="flex gap-2">
        <button
          type="button"
          className="btn-ghost"
          disabled={disabled}
          onClick={() => onChange([...rows, emptyRow()])}
        >
          {tr("Add item")}
        </button>
        <button
          type="button"
          className="btn-ghost"
          disabled={disabled || rows.length === 0}
          onClick={() => onChange([...rows, duplicateRow(rows[rows.length - 1])])}
        >
          {tr("Duplicate last row")}
        </button>
      </div>
    </div>
  );
}
