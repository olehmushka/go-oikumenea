"use client";

// The explorer's filter bar (M56 ticket 4, D-ConsoleDashboards). Every control writes the WHOLE
// URLSearchParams back through lib/ontology/filters, so:
//   - the filter set survives a refresh, a shared link and the list↔tree (M57: list↔dashboard) toggle;
//   - changing any filter drops `pageToken` (a keyset cursor minted under other filters is invalid);
//   - a param this type does not declare is preserved in the URL but never sent to the API.
//
// It replaces the two separate <form method="get"> the page used to carry (the org picker and the
// search box), which submitted only their own fields and so silently wiped each other.
//
// It takes the type as a STRING and looks the def up itself — the registry's defs carry functions,
// which cannot cross the server→client prop boundary (DataTable does the same).

import { useRouter, useSearchParams } from "next/navigation";
import { CountrySelect } from "@/components/CountrySelect";
import { EntitySelect } from "@/components/EntitySelect";
import { ScopedSelect } from "@/components/ScopedSelect";
import { SearchSelect } from "@/components/SearchSelect";
import { UnitSelect } from "@/components/UnitSelect";
import { useTg } from "@/lib/locale";
import { holds, type Capabilities, NO_CAPS } from "@/lib/ontology/caps";
import { dayBound, dayInput } from "@/lib/ontology/buckets";
import { exploreHref, filterParams } from "@/lib/ontology/filters";
import { OBJECT_TYPES, type FilterDef } from "@/lib/ontology/registry";
import { ridTail } from "@/lib/ontology/rid";

export function FilterBar({
  type,
  caps = NO_CAPS,
  orgOptions = [],
}: {
  type: string;
  caps?: Capabilities;
  orgOptions?: { id: string; label: string }[];
}) {
  const def = OBJECT_TYPES[type];
  const router = useRouter();
  const sp = useSearchParams();
  const tr = useTg();

  if (!def) return null;

  const params = new URLSearchParams(sp?.toString() ?? "");
  const val = (name: string) => params.get(name) ?? "";

  // replace, not push: filtering is refinement, not navigation — pushing would make Back walk
  // keystroke by keystroke. scroll:false keeps the table where the operator is looking.
  const apply = (patch: Record<string, string | undefined>) => {
    router.replace(exploreHref(type, params, patch), { scroll: false });
  };

  // Cosmetic only: a facet the caller may not read is omitted by the server regardless (D-ObjectFacets
  // rule 2). Every facet today is pii:none/basic with no code, so `holds` returns true.
  const filters = (def.filters ?? []).filter((f) => holds(caps, f.requires));
  const required = filters.filter((f) => f.required);
  const optional = filters.filter((f) => !f.required);
  const searchable = def.list?.searchParam;

  // A required filter (unit.org) gates the listing entirely: until it is set, nothing else is
  // meaningful, so the bar shows only that control.
  const ready = required.every((f) => f.params.every((p) => val(p) !== ""));

  const chips = activeChips(filters, val, tr);
  const hasAnything = chips.length > 0 || val("q") !== "";

  return (
    <div className="mb-4 space-y-2">
      {/* Remount key: UnitSelect / EntitySelect / SearchSelect seed their own state from
          defaultValue, so a chip-clear that only changed the URL would leave a stale picker.
          Re-keying on the whole param string re-seeds every control from the new URL. */}
      <div key={params.toString()} className="flex flex-wrap items-end gap-3">
        {required.map((f) => (
          <Control key={f.key} f={f} val={val} apply={apply} orgOptions={orgOptions} tr={tr} />
        ))}
        {required.length > 0 && optional.length > 0 ? (
          <div className="mx-1 h-8 w-px self-center bg-slate-200" />
        ) : null}
        {ready ? (
          <>
            {searchable ? (
              <label className="flex flex-col gap-1">
                <span className="label mb-0">{tr("Search")}</span>
                <input
                  type="search"
                  className="input max-w-xs"
                  defaultValue={val("q")}
                  placeholder={tr("Search by name or code…")}
                  autoComplete="off"
                  // Free text applies on blur or Enter — it is no longer inside a <form>, so Enter
                  // needs an explicit handler, and blur alone would lose the last keystroke when the
                  // operator clicks a chip directly.
                  onBlur={(e) => {
                    if (e.currentTarget.value.trim() !== val("q")) {
                      apply({ q: e.currentTarget.value.trim() || undefined });
                    }
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      apply({ q: e.currentTarget.value.trim() || undefined });
                    }
                  }}
                />
              </label>
            ) : null}
            {optional.map((f) => (
              <Control key={f.key} f={f} val={val} apply={apply} orgOptions={orgOptions} tr={tr} />
            ))}
          </>
        ) : null}
      </div>

      {hasAnything ? (
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <span className="text-slate-400">{tr("Filters")}:</span>
          {val("q") ? (
            <Chip label={`${tr("Search")}: ${val("q")}`} onClear={() => apply({ q: undefined })} />
          ) : null}
          {chips.map((c) => (
            <Chip
              key={c.key}
              label={c.label}
              onClear={
                c.clearable
                  ? () => apply(Object.fromEntries(c.params.map((p) => [p, undefined])))
                  : undefined
              }
            />
          ))}
          <button
            type="button"
            className="text-slate-500 underline hover:text-slate-700"
            onClick={() =>
              apply({
                // `q` is not a filter param (its wire name is def.list.searchParam), so clear it
                // explicitly rather than letting it survive a "clear all".
                q: undefined,
                ...Object.fromEntries(
                  filterParams(def)
                    .filter((p) => !required.some((r) => r.params.includes(p)))
                    .map((p) => [p, undefined]),
                ),
              })
            }
          >
            {tr("Clear all")}
          </button>
        </div>
      ) : null}
    </div>
  );
}

function Chip({ label, onClear }: { label: string; onClear?: () => void }) {
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2 py-0.5 text-slate-700">
      {label}
      {onClear ? (
        <button type="button" onClick={onClear} aria-label="Clear" className="text-slate-400 hover:text-slate-700">
          ✕
        </button>
      ) : null}
    </span>
  );
}

type ChipData = { key: string; label: string; params: string[]; clearable: boolean };

/** One chip per filter that has a value. Ref values render as the RID tail: the picker beside it
 *  already shows the human label, and resolving a name here would cost a round-trip per chip. */
function activeChips(
  filters: FilterDef[],
  val: (name: string) => string,
  tr: (s: string) => string,
): ChipData[] {
  const out: ChipData[] = [];
  for (const f of filters) {
    const values = f.params.map(val);
    if (values.every((v) => v === "")) continue;
    let text: string;
    if (f.kind === "enum") {
      const opt = f.values?.find((o) => o.value === values[0]);
      text = tr(opt?.label ?? values[0]);
    } else if (f.kind === "bool") {
      text = tr(values[0] === "true" ? "Yes" : "No");
    } else if (f.kind === "ref") {
      text = ridTail(values[0]);
    } else if (f.params.length === 2) {
      text = `${values[0] || "…"} – ${values[1] || "…"}`;
    } else {
      text = values[0];
    }
    out.push({
      key: f.key,
      label: `${tr(f.label)}: ${text}`,
      params: f.params,
      // A required filter has no clear: dropping it would 400 the next request.
      clearable: !f.required,
    });
  }
  return out;
}

function Control({
  f,
  val,
  apply,
  orgOptions,
  tr,
}: {
  f: FilterDef;
  val: (name: string) => string;
  apply: (patch: Record<string, string | undefined>) => void;
  orgOptions: { id: string; label: string }[];
  tr: (s: string) => string;
}) {
  const set = (name: string, v: string) => apply({ [name]: v || undefined });

  return (
    <label className="flex flex-col gap-1">
      <span className="label mb-0" title={f.hint ? tr(f.hint) : undefined}>
        {tr(f.label)}
        {f.hint ? <span className="ml-1 cursor-help text-slate-300">ⓘ</span> : null}
      </span>
      <Widget f={f} val={val} set={set} orgOptions={orgOptions} tr={tr} />
    </label>
  );
}

function Widget({
  f,
  val,
  set,
  orgOptions,
  tr,
}: {
  f: FilterDef;
  val: (name: string) => string;
  set: (name: string, v: string) => void;
  orgOptions: { id: string; label: string }[];
  tr: (s: string) => string;
}) {
  const p0 = f.params[0];

  if (f.kind === "enum") {
    return (
      <select className="input max-w-xs" value={val(p0)} onChange={(e) => set(p0, e.target.value)}>
        <option value="">{tr("Any")}</option>
        {(f.values ?? []).map((o) => (
          <option key={o.value} value={o.value}>
            {tr(o.label)}
          </option>
        ))}
      </select>
    );
  }

  if (f.kind === "bool") {
    return (
      <select className="input max-w-xs" value={val(p0)} onChange={(e) => set(p0, e.target.value)}>
        <option value="">{tr("Any")}</option>
        <option value="true">{tr("Yes")}</option>
        <option value="false">{tr("No")}</option>
      </select>
    );
  }

  // A `code` facet is an OPEN text dimension — no CHECK set to enumerate and no RID to pick. Where a
  // catalog exists behind it (audit.action → the R-29 action types) it gets a select over that
  // catalog; otherwise a free-text box, because the honest control for an open set is typing.
  if (f.kind === "code") {
    if (f.catalog === "actionType") {
      return (
        <ScopedSelect
          path="/audit/v1/action-types"
          // The catalog is keyed by `code`, not by a RID — map it onto the {id, code} shape the
          // control reads, so the value it selects IS the code the filter arg takes.
          pick={(d) =>
            (Array.isArray(d) ? d : []).map((a) => {
              const code = (a as { code?: string }).code ?? "";
              return { id: code, code };
            })
          }
          value={val(p0)}
          onChange={(id) => set(p0, id)}
          unscopedLabel=""
        />
      );
    }
    return (
      <input
        type="text"
        className="input w-44"
        defaultValue={val(p0)}
        autoComplete="off"
        onBlur={(e) => {
          if (e.currentTarget.value.trim() !== val(p0)) set(p0, e.currentTarget.value.trim());
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            set(p0, e.currentTarget.value.trim());
          }
        }}
      />
    );
  }

  // ARITY COMES FROM params, NOT kind: unit.level is a numeric-range with ONE param (the contract's
  // pre-existing scalar arg). Rendering a min/max pair off the kind would send args that do not exist.
  if (f.kind === "date-range" || f.kind === "numeric-range") {
    const inputType = f.kind === "date-range" ? "date" : "number";
    // A datetime-typed range (audit's since/until) still gets a DATE picker — an operator filters by
    // day, not by second — but the value SENT has to be that day's RFC-3339 endpoints, because the
    // contract's arg is a timestamp and a bare YYYY-MM-DD is a 400. Lower bound → start of day, upper
    // bound → end of day, so picking the same date twice selects exactly that day. dayInput() renders
    // a stored timestamp back as a date, so the control round-trips.
    const wire = (raw: string, isUpper: boolean) =>
      f.argType === "datetime" ? dayBound(raw, isUpper) : raw;
    return (
      <span className="flex items-center gap-1">
        {f.params.map((p, i) => (
          <input
            key={p}
            type={inputType}
            className="input w-36"
            defaultValue={f.argType === "datetime" ? dayInput(val(p)) : val(p)}
            onBlur={(e) => {
              const next = wire(e.currentTarget.value, i === f.params.length - 1);
              if (next !== val(p)) set(p, next);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                set(p, wire(e.currentTarget.value, i === f.params.length - 1));
              }
            }}
          />
        ))}
      </span>
    );
  }

  // ref
  switch (f.control) {
    case "org":
      return (
        <select className="input max-w-xs" value={val(p0)} onChange={(e) => set(p0, e.target.value)}>
          <option value="">{tr("Select an organization…")}</option>
          {orgOptions.map((o) => (
            <option key={o.id} value={o.id}>
              {o.label}
            </option>
          ))}
        </select>
      );
    case "country":
      return (
        <CountrySelect name="" value={val(p0)} onChange={(id) => set(p0, id)} />
      );
    case "unit":
      return (
        <UnitSelect defaultValue={val(p0)} allowEmpty onChange={(id) => set(p0, id)} />
      );
    case "person":
      // SearchSelect, not EntitySelect: the latter fetches one page of ≤200 and filters client-side,
      // which silently lies about the directory at registry scale. This hits the server `query` arg.
      return (
        <SearchSelect kind="person" defaultValue={val(p0)} onChange={(id) => set(p0, id)} />
      );
    case "unitKind":
      return (
        <ScopedSelect
          path={val(f.dependsOn ?? "") ? `/tenant/v1/unit-kinds?domain=${encodeURIComponent(val(f.dependsOn!))}` : null}
          pick={(d) => (d as { unitKinds?: unknown[] })?.unitKinds ?? []}
          value={val(p0)}
          onChange={(id) => set(p0, id)}
          unscopedLabel="pick a domain first"
        />
      );
    case "position":
      return (
        <ScopedSelect
          path={val(f.dependsOn ?? "") ? `/membership/v1/units/${encodeURIComponent(val(f.dependsOn!))}/positions` : null}
          pick={(d) => (d as { positions?: unknown[] })?.positions ?? []}
          value={val(p0)}
          onChange={(id) => set(p0, id)}
          unscopedLabel="pick a unit first"
        />
      );
    // Models are listable only per brand (`GET /brands/{brandId}/models`), so this is scoped rather
    // than a flat EntitySelect — the unitKind→domain shape, one catalog level down.
    case "model":
      return (
        <ScopedSelect
          path={val(f.dependsOn ?? "") ? `/vehicle/v1/brands/${encodeURIComponent(val(f.dependsOn!))}/models` : null}
          pick={(d) => (d as { models?: unknown[] })?.models ?? []}
          value={val(p0)}
          onChange={(id) => set(p0, id)}
          unscopedLabel="pick a brand first"
        />
      );
    default:
      return (
        <EntitySelect
          kind={f.control ?? "role"}
          defaultValue={val(p0)}
          allowEmpty
          onChange={(id) => set(p0, id)}
        />
      );
  }
}
