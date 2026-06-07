"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { EntitySelect, type EntityKind } from "./EntitySelect";

/**
 * A lookup box that navigates to `<basePath>?<param>=<value>`.
 *
 *  - With `kind`: an entity picker — choose from a searchable dropdown, submit the RID (no typing).
 *  - Without `kind`: a plain text box + "Go" (for free-text filters like an action code).
 */
export function LookupForm({
  basePath,
  param,
  label,
  kind,
  placeholder,
  current,
}: {
  basePath: string;
  param: string;
  label: string;
  kind?: EntityKind;
  placeholder?: string;
  current?: string;
}) {
  const router = useRouter();
  const [value, setValue] = useState(current ?? "");
  const go = (v: string) =>
    router.push(v ? `${basePath}?${param}=${encodeURIComponent(v)}` : basePath);

  if (kind) {
    return (
      <div className="card p-4">
        <label className="label">{label}</label>
        <EntitySelect
          kind={kind}
          defaultValue={current}
          allowEmpty
          placeholder={placeholder ?? "Search…"}
          onChange={go}
        />
      </div>
    );
  }

  return (
    <form
      className="card p-4"
      onSubmit={(e) => {
        e.preventDefault();
        go(value.trim());
      }}
    >
      <label className="label">{label}</label>
      <div className="flex gap-2">
        <input
          className="input"
          placeholder={placeholder}
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
        <button type="submit" className="btn-ghost">
          Go
        </button>
      </div>
    </form>
  );
}
