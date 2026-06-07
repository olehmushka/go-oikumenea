"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { mutate } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { EntitySelect } from "./EntitySelect";
import { useLocale } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";
import type { Position } from "@/lib/api/types";

/**
 * Add a membership to a unit (person ↔ unit belonging), optionally filling one of the unit's
 * positions — i.e. assigning the person to that billet. Omitting the position is plain belonging.
 * (POST /membership/v1/memberships.)
 */
export function AddMembership({
  unitId,
  positions,
}: {
  unitId: string;
  positions: Position[];
}) {
  const router = useRouter();
  const { locale } = useLocale();
  const [personId, setPersonId] = useState("");
  const [positionId, setPositionId] = useState("");
  const [effectiveFrom, setEffectiveFrom] = useState("");
  const [pickerKey, setPickerKey] = useState(0);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!personId) return;
    setBusy(true);
    setErr(null);
    try {
      await mutate("POST", "/membership/v1/memberships", {
        personId,
        unitId,
        positionId: positionId || undefined,
        effectiveFrom: effectiveFrom ? new Date(effectiveFrom).toISOString() : undefined,
      });
      setPersonId("");
      setPositionId("");
      setEffectiveFrom("");
      setPickerKey((k) => k + 1);
      router.refresh();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="card space-y-3 p-5" onSubmit={submit}>
      <h3 className="text-sm font-semibold text-slate-900">Add membership</h3>
      <p className="text-xs text-slate-500">
        Belong this person to the unit. Pick a position to assign them to that billet, or leave it
        empty for plain belonging.
      </p>
      {err ? <ErrorBox error={err} /> : null}
      <div>
        <label className="label">Person</label>
        <EntitySelect
          key={pickerKey}
          kind="person"
          placeholder="Search a person…"
          onChange={setPersonId}
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="label">Position (optional)</label>
          <select
            className="input"
            value={positionId}
            onChange={(e) => setPositionId(e.target.value)}
          >
            <option value="">— plain belonging —</option>
            {positions
              .filter((p) => p.status !== "abolished")
              .map((p) => (
                <option key={p.id} value={p.id}>
                  {pickLabel(p.title, locale) || p.code} ({p.code})
                </option>
              ))}
          </select>
        </div>
        <div>
          <label className="label">Effective from (optional)</label>
          <input
            type="datetime-local"
            className="input"
            value={effectiveFrom}
            onChange={(e) => setEffectiveFrom(e.target.value)}
          />
        </div>
      </div>
      <button type="submit" className="btn-primary" disabled={busy || !personId}>
        {busy ? "Adding…" : "Add membership"}
      </button>
    </form>
  );
}

/** End an active membership (vacates any filled billet). POST /membership/v1/memberships/{id}/end. */
export function EndMembershipButton({ membershipId }: { membershipId: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  return (
    <span className="inline-flex items-center gap-2">
      <button
        type="button"
        className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
        disabled={busy}
        onClick={async () => {
          if (!window.confirm("End this membership?")) return;
          setBusy(true);
          setErr(null);
          try {
            await mutate("POST", `/membership/v1/memberships/${membershipId}/end`, {});
            router.refresh();
          } catch (e) {
            setErr((e as { errorName?: string })?.errorName ?? "Failed");
            setBusy(false);
          }
        }}
      >
        {busy ? "…" : "End"}
      </button>
      {err && <span className="text-xs text-red-500">{err}</span>}
    </span>
  );
}
