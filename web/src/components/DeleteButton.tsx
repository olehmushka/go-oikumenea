"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { mutate } from "@/lib/api/client";

/** Generic confirm-then-DELETE button that refreshes the route on success. */
export function DeleteButton({
  path,
  label = "Remove",
  confirm = "Are you sure?",
}: {
  path: string;
  label?: string;
  confirm?: string;
}) {
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
          if (!window.confirm(confirm)) return;
          setBusy(true);
          setErr(null);
          try {
            await mutate("DELETE", path);
            router.refresh();
          } catch (e) {
            const b = e as { errorName?: string };
            setErr(b?.errorName ?? "Failed");
            setBusy(false);
          }
        }}
      >
        {busy ? "…" : label}
      </button>
      {err && <span className="text-xs text-red-500">{err}</span>}
    </span>
  );
}
