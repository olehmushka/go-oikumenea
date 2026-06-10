"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { getPins, getRecents, type RecentItem } from "@/lib/ontology/recents";
import { TypeBadge } from "./TypeBadge";
import { Mono } from "@/components/ui";

/** Per-browser recents + pins (localStorage). Empty until you open some objects. */
export function RecentsPanel() {
  const [recents, setRecents] = useState<RecentItem[]>([]);
  const [pins, setPins] = useState<RecentItem[]>([]);
  useEffect(() => {
    setRecents(getRecents().slice(0, 8));
    setPins(getPins().slice(0, 8));
  }, []);

  const List = ({ items }: { items: RecentItem[] }) =>
    items.length === 0 ? (
      <p className="text-xs text-slate-400">Nothing yet.</p>
    ) : (
      <ul className="divide-y divide-slate-100">
        {items.map((r) => (
          <li key={r.id} className="flex items-center gap-2 py-1.5 text-sm">
            <TypeBadge type={r.type} />
            <Link href={`/o/${encodeURIComponent(r.id)}`} className="truncate text-indigo-600 hover:underline">
              {r.label || <Mono>{r.id.slice(-8)}</Mono>}
            </Link>
          </li>
        ))}
      </ul>
    );

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <div className="card p-4">
        <h2 className="mb-2 text-sm font-semibold text-slate-900">Pinned</h2>
        <List items={pins} />
      </div>
      <div className="card p-4">
        <h2 className="mb-2 text-sm font-semibold text-slate-900">Recent</h2>
        <List items={recents} />
      </div>
    </div>
  );
}
