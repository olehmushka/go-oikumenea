"use client";

// PersonVehiclesManager — a read-only view of the vehicles a person owns/has owned (M26 / D-Vehicles):
// each registration enriched with the vehicle identity (VIN, type/brand/model labels), plate, country,
// region, status, and the ownership window. Registrations are recorded/transferred from the Vehicles
// workspace; here we only surface them on the person object view (mirrors PersonCompaniesManager).

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";

type Reg = {
  id: string;
  vehicleId: string;
  vin?: string;
  typeLabel?: string;
  brandLabel?: string;
  modelLabel?: string;
  registrationNumber: string;
  subdivisionLabel?: string;
  status: string;
  effectiveFrom: string;
  effectiveTo?: string;
};

const tail = (id: string) => id.slice(-8);

export function PersonVehiclesManager({ personId }: { personId: string }) {
  const tr = useTg();
  const [regs, setRegs] = useState<Reg[] | null>(null);
  const [err, setErr] = useState<unknown>(null);

  useEffect(() => {
    api.vehicle
      .listPersonVehicles(personId)
      .then((d) => setRegs(((d as { registrations?: Reg[] }).registrations ?? []) as Reg[]))
      .catch(setErr);
  }, [personId]);

  if (err) return <ErrorBox error={err} />;

  return (
    <div className="space-y-2 text-sm text-slate-700">
      {regs && regs.length === 0 ? <p className="text-slate-400"><T>No vehicles.</T></p> : null}
      <ul className="space-y-1">
        {regs?.map((r) => {
          const make = [r.brandLabel, r.modelLabel].filter(Boolean).join(" ") || r.typeLabel || tail(r.vehicleId);
          return (
            <li key={r.id}>
              <span className="font-medium">{r.registrationNumber}</span> · {make}
              {r.vin ? ` · ${r.vin}` : ""}
              {r.subdivisionLabel ? ` · ${r.subdivisionLabel}` : ""} · {r.status}
              {` · ${r.effectiveFrom?.slice(0, 10)}→${r.effectiveTo ? r.effectiveTo.slice(0, 10) : tr("now")}`}
            </li>
          );
        })}
      </ul>
      <p className="mt-1 text-xs text-slate-400"><T>Read-only. Vehicle registrations are recorded from the Vehicles workspace.</T></p>
    </div>
  );
}
