"use client";

import { errorInfo } from "@/lib/api/errors";

/** Client-side render of a thrown error (SDK ConjureError, or any other). */
export function ErrorBox({ error }: { error: unknown }) {
  const info = errorInfo(error);
  const name = info.errorName ?? (info.status ? `${info.status}` : info.message);
  const detail = info.errorCode ?? (info.errorName ? "" : info.message === name ? "" : info.message);
  const params = info.parameters;
  return (
    <div className="card border-red-200 bg-red-50 p-4">
      <div className="text-sm font-semibold text-red-800">{name}</div>
      {detail && <div className="mt-1 text-sm text-red-700">{detail}</div>}
      {params && Object.keys(params).length > 0 && (
        <pre className="mt-2 overflow-x-auto rounded bg-red-100 p-2 text-xs text-red-900">
          {JSON.stringify(params, null, 2)}
        </pre>
      )}
    </div>
  );
}
