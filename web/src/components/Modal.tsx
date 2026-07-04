"use client";

// Minimal modal primitive. No dialog library is bundled (no radix/shadcn), so this follows the
// codebase's useState-toggle convention and renders through a portal to document.body — that keeps
// the modal's content (which may contain its own <form>) OUT of any surrounding <form>, since nested
// forms are invalid HTML. Closes on backdrop click, Esc, or the ✕ button.

import { useEffect } from "react";
import { createPortal } from "react-dom";

export function Modal({
  open,
  title,
  onClose,
  children,
}: {
  open: boolean;
  title?: React.ReactNode;
  onClose: () => void;
  children: React.ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open || typeof document === "undefined") return null;

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-slate-900/40 p-4 sm:items-center"
      onMouseDown={onClose}
    >
      <div
        className="card my-8 w-full max-w-lg p-5"
        onMouseDown={(e) => e.stopPropagation()}
        // React synthetic events bubble through the portal along the React tree, so a <form> inside the
        // modal would otherwise submit an ancestor form that's a React parent. Contain the submit here.
        onSubmit={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-start justify-between gap-4">
          {title ? <h2 className="text-sm font-semibold text-slate-900">{title}</h2> : <span />}
          <button
            type="button"
            className="text-slate-400 hover:text-slate-700"
            aria-label="Close"
            onClick={onClose}
          >
            ✕
          </button>
        </div>
        {children}
      </div>
    </div>,
    document.body,
  );
}
