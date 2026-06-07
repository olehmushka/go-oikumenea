"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { mutate } from "@/lib/api/client";

/**
 * Generic confirm-then-call button for lifecycle / delete actions (POST/PUT/DELETE + optional body).
 * Refreshes the route on success and shows the API's errorName inline on failure. Use for
 * deactivate / reactivate / purge / abolish / revoke / retire / transition / delete.
 */
export function ActionButton({
  method,
  path,
  body,
  label,
  confirm,
  tone = "ghost",
  disabled = false,
}: {
  method: "POST" | "PUT" | "DELETE";
  path: string;
  body?: unknown;
  label: string;
  confirm?: string;
  tone?: "ghost" | "danger" | "primary";
  disabled?: boolean;
}) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const cls =
    tone === "danger"
      ? "text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
      : tone === "primary"
        ? "btn-primary"
        : "btn-ghost";
  return (
    <span className="inline-flex items-center gap-2">
      <button
        type="button"
        className={cls}
        disabled={busy || disabled}
        onClick={async () => {
          if (confirm && !window.confirm(confirm)) return;
          setBusy(true);
          setErr(null);
          try {
            await mutate(method, path, body);
            router.refresh();
          } catch (e) {
            setErr((e as { errorName?: string })?.errorName ?? "Failed");
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
