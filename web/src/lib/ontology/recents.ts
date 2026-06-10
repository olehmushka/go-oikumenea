// Per-browser recents & pins (localStorage only — no backend, no PII at rest beyond the RID + a
// display label the user just viewed). Surfaced on the Overview page and in the command palette so
// "back to what I was doing" is one keystroke.

export interface RecentItem {
  id: string; // RID
  type: string; // entity_type token
  label: string;
  at: number; // epoch ms
}

const RECENTS_KEY = "oik:recents";
const PINS_KEY = "oik:pins";
const MAX_RECENTS = 30;

function read(key: string): RecentItem[] {
  if (typeof window === "undefined") return [];
  try {
    return JSON.parse(window.localStorage.getItem(key) || "[]") as RecentItem[];
  } catch {
    return [];
  }
}

function write(key: string, items: RecentItem[]) {
  try {
    window.localStorage.setItem(key, JSON.stringify(items));
  } catch {
    /* storage disabled — ignore */
  }
}

/** Record a visited object; most-recent-first, de-duplicated by RID, capped. */
export function pushRecent(item: Omit<RecentItem, "at">) {
  if (typeof window === "undefined" || !item.id) return;
  const next = [
    { ...item, at: Date.now() },
    ...read(RECENTS_KEY).filter((r) => r.id !== item.id),
  ].slice(0, MAX_RECENTS);
  write(RECENTS_KEY, next);
}

export function getRecents(): RecentItem[] {
  return read(RECENTS_KEY);
}

export function getPins(): RecentItem[] {
  return read(PINS_KEY);
}

export function isPinned(id: string): boolean {
  return read(PINS_KEY).some((p) => p.id === id);
}

/** Toggle a pin; returns the new pinned state. */
export function togglePin(item: Omit<RecentItem, "at">): boolean {
  const cur = read(PINS_KEY);
  const exists = cur.some((p) => p.id === item.id);
  const next = exists
    ? cur.filter((p) => p.id !== item.id)
    : [{ ...item, at: Date.now() }, ...cur];
  write(PINS_KEY, next);
  return !exists;
}
