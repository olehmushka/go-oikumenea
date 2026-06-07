"use client";

/** Client-side render of a thrown error or Conjure SerializableError body. */
export function ErrorBox({ error }: { error: unknown }) {
  let name = "Request failed";
  let detail = "";
  let params: Record<string, unknown> | undefined;
  const b = error as
    | { errorName?: string; errorCode?: string; parameters?: Record<string, unknown> }
    | undefined;
  if (b && typeof b === "object" && (b.errorName || b.errorCode)) {
    name = b.errorName ?? "Error";
    detail = b.errorCode ?? "";
    params = b.parameters;
  } else if (error instanceof Error) {
    detail = error.message;
  }
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
