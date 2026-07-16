// Advisory client-side handling of regulated/secret action params (D-DataScope; D-ActionInvocation R-33).
// The server's pkg/personalcode validators are AUTHORITATIVE — these mirror them lightly so the operator
// gets immediate feedback and secret fields are masked in the UI. Intentionally small; anything that
// slips through is still rejected server-side. The `sensitivity` tag is single-sourced from the catalog
// (pkg/action.paramSensitivity → ActionParam.sensitivity).

/** Masked (password) input for secret/regulated values; a spectrum is special-category but not secret. */
export function isMasked(sensitivity?: string | null): boolean {
  return sensitivity === "pan" || sensitivity === "iban" || sensitivity === "secret";
}

/** An advisory error message, or null when the value looks valid (or the field isn't sensitive/empty). */
export function validateSensitive(sensitivity: string | null | undefined, raw: string): string | null {
  const v = (raw ?? "").trim();
  if (!sensitivity || v === "") return null;
  switch (sensitivity) {
    case "pan": {
      const digits = v.replace(/\s+/g, "");
      return /^\d{12,19}$/.test(digits) && luhnOk(digits) ? null : "Not a valid card number (Luhn check).";
    }
    case "iban":
      return ibanOk(v) ? null : "Not a valid IBAN (mod-97 check).";
    case "spectrum": {
      const n = Number(v);
      return Number.isFinite(n) && n >= -1 && n <= 1 ? null : "Must be a number in [-1, 1].";
    }
    default:
      return null; // "secret": masked, no format rule
  }
}

function luhnOk(s: string): boolean {
  let sum = 0;
  let alt = false;
  for (let i = s.length - 1; i >= 0; i--) {
    let d = s.charCodeAt(i) - 48;
    if (alt) {
      d *= 2;
      if (d > 9) d -= 9;
    }
    sum += d;
    alt = !alt;
  }
  return sum % 10 === 0;
}

function ibanOk(raw: string): boolean {
  const s = raw.replace(/\s+/g, "").toUpperCase();
  if (!/^[A-Z0-9]{15,34}$/.test(s)) return false;
  const rearranged = s.slice(4) + s.slice(0, 4);
  const numeric = rearranged.replace(/[A-Z]/g, (c) => String(c.charCodeAt(0) - 55));
  let rem = 0;
  for (let i = 0; i < numeric.length; i++) rem = (rem * 10 + (numeric.charCodeAt(i) - 48)) % 97;
  return rem === 1;
}
