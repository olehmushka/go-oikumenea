"use client";

/** Header affordance that opens the command palette (also bound to ⌘K globally). */
export function SearchTrigger() {
  return (
    <button
      type="button"
      onClick={() => window.dispatchEvent(new Event("oik:open-palette"))}
      className="flex items-center gap-2 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm text-slate-400 transition-colors hover:bg-slate-50"
    >
      <span>Search…</span>
      <kbd className="rounded border border-slate-200 bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-500">
        ⌘K
      </kbd>
    </button>
  );
}
