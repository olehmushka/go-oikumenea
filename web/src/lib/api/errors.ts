// Neutral (non server-only) error helpers usable from both server and client components.
//
// Every backend call goes through the SDK (oikumenea-client), whose typed methods AND generic
// request() throw a ConjureError on a non-2xx response — `.status` + `.body` (the Conjure
// SerializableError envelope). errorInfo() normalizes that (and legacy shapes) into one struct the
// UI can render.

import { isConjureError } from "oikumenea-client";

export interface ErrorInfo {
  status?: number;
  errorName?: string;
  errorCode?: string;
  parameters?: Record<string, unknown>;
  message: string;
}

type Envelope = { errorName?: string; errorCode?: string; parameters?: Record<string, unknown> };

export function errorInfo(error: unknown): ErrorInfo {
  // SDK errors — typed methods and the generic request() both throw ConjureError.
  if (isConjureError(error)) {
    const body = (error.body && typeof error.body === "object" ? error.body : undefined) as
      | Envelope
      | undefined;
    return {
      status: error.status,
      errorName: body?.errorName,
      errorCode: body?.errorCode,
      parameters: body?.parameters,
      message:
        body?.errorName ?? (error.status ? `Request failed (${error.status})` : "Request failed"),
    };
  }
  // A bare SerializableError envelope (defensive — e.g. JSON thrown directly).
  if (error && typeof error === "object" && "errorName" in error) {
    const b = error as Envelope;
    return { errorName: b.errorName, errorCode: b.errorCode, parameters: b.parameters, message: b.errorName ?? "Request failed" };
  }
  if (error instanceof Error) return { message: error.message };
  return { message: "Request failed" };
}

/** Best single-line message for a thrown backend error (forms, toasts). */
export function errorMessage(error: unknown): string {
  return errorInfo(error).message;
}
