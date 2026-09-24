import { useSyncExternalStore } from "react";

let cursor: number | null = null;
const listeners = new Set<() => void>();

export function setChartCursor(ts: number | null) {
  if (ts === cursor) return;
  cursor = ts;
  listeners.forEach((l) => l());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useChartCursor() {
  return useSyncExternalStore(subscribe, () => cursor);
}
