import type { PrinterEntity, PrinterStateValue } from "../schema";

// formatRemaining renders a minutes value as "Xh Ym" / "Ym" (or "—" when unknown).
export function formatRemaining(minutes: number): string {
  if (!minutes || minutes <= 0) return "—";
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

// formatTimestamp renders a unix-millis timestamp as a localized date+time,
// or "" when unknown.
export function formatTimestamp(ms: number): string {
  if (!ms || ms <= 0) return "";
  return new Date(ms).toLocaleString();
}

// isPrinterActive returns true while a print is running or paused.
export function isPrinterActive(state: PrinterStateValue | undefined): boolean {
  return state === "printing" || state === "paused";
}

// activePrinters returns the printer entities in a room that are currently
// printing or paused.
export function activePrinters(entities: PrinterEntity[]): PrinterEntity[] {
  return entities.filter((e) => isPrinterActive(e.state?.state));
}
