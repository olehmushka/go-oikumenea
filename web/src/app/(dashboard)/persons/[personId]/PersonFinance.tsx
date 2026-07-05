"use client";

// PersonFinanceManager — a read-only view of the bank accounts a person holds (M44 / D-Finance): each
// held account enriched with the bank (institution) label, currency, account-type label, and the
// person's holding role. The IBAN is never surfaced here (it is decrypted only on the single-account
// read for authorized callers in the Finance workspace). Accounts are created/managed from the Finance
// workspace; here we only surface the holdings on the person object view (mirrors PersonVehicles).

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";

type Held = {
  id: string;
  institutionId: string;
  institutionLabel?: string;
  currency?: string;
  accountTypeLabel?: string;
  role: string;
  status: string;
};

const tail = (id: string) => id.slice(-8);

export function PersonFinanceManager({ personId }: { personId: string }) {
  const [accounts, setAccounts] = useState<Held[] | null>(null);
  const [err, setErr] = useState<unknown>(null);

  useEffect(() => {
    api.finance
      .listPersonAccounts(personId)
      .then((d) => setAccounts(((d as { accounts?: Held[] }).accounts ?? []) as Held[]))
      .catch(setErr);
  }, [personId]);

  if (err) return <ErrorBox error={err} />;

  return (
    <div className="space-y-2 text-sm text-slate-700">
      {accounts && accounts.length === 0 ? <p className="text-slate-400"><T>No accounts.</T></p> : null}
      <ul className="space-y-1">
        {accounts?.map((a) => (
          <li key={a.id}>
            <span className="font-medium">{a.institutionLabel || tail(a.institutionId)}</span>
            {a.accountTypeLabel ? ` · ${a.accountTypeLabel}` : ""}
            {a.currency ? ` · ${a.currency}` : ""} · {a.role} · {a.status}
          </li>
        ))}
      </ul>
      <p className="mt-1 text-xs text-slate-400"><T>Read-only. Accounts are managed from the Finance workspace; the IBAN is never shown here.</T></p>
    </div>
  );
}
