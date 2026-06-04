import { useEffect, useMemo, useRef, useState, type FC } from "react";
import { CalendarClock } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  fetchReservations,
  type ReservationEvent,
} from "../../lib/reservations";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "../ui/dialog";

interface ReservationsPanelProps {
  /** Present for API symmetry; reservations are shown for all rooms, unfiltered. */
  roomId?: string;
}

const HOUR_HEIGHT = 56; // px per hour
const DAY_HEIGHT = HOUR_HEIGHT * 24;
const POLL_INTERVAL_MS = 60_000;

function startOfTodayMs(): number {
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  return d.getTime();
}

/** Minutes from local midnight for an epoch-seconds timestamp, clamped to the day. */
function minutesIntoDay(epochSec: number, dayStartMs: number): number {
  const mins = (epochSec * 1000 - dayStartMs) / 60000;
  return Math.max(0, Math.min(1440, mins));
}

function fmtTime(epochSec: number): string {
  return new Intl.DateTimeFormat("pl-PL", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(epochSec * 1000));
}

function fmtDate(epochSec: number): string {
  return new Intl.DateTimeFormat("pl-PL", {
    weekday: "long",
    day: "2-digit",
    month: "long",
  }).format(new Date(epochSec * 1000));
}

interface LaidOutEvent {
  ev: ReservationEvent;
  col: number;
  cols: number;
}

/**
 * Assign side-by-side columns to overlapping events (interval graph greedy
 * colouring), so simultaneous events render next to each other.
 */
function layoutEvents(events: ReservationEvent[]): LaidOutEvent[] {
  const sorted = [...events].sort((a, b) => a.start - b.start || b.end - a.end);
  const result: LaidOutEvent[] = [];
  let cluster: LaidOutEvent[] = [];
  let clusterEnd = -Infinity;
  let columns: number[] = []; // end time per active column in the current cluster

  const flush = () => {
    for (const item of cluster) item.cols = columns.length || 1;
    cluster = [];
    columns = [];
  };

  for (const ev of sorted) {
    if (cluster.length && ev.start >= clusterEnd) {
      flush();
      clusterEnd = -Infinity;
    }
    let col = columns.findIndex((end) => end <= ev.start);
    if (col === -1) {
      col = columns.length;
      columns.push(ev.end);
    } else {
      columns[col] = ev.end;
    }
    const item: LaidOutEvent = { ev, col, cols: 0 };
    cluster.push(item);
    result.push(item);
    clusterEnd = Math.max(clusterEnd, ev.end);
  }
  flush();
  return result;
}

/**
 * ReservationsPanel — a Google-Calendar-style day view of today's calendar
 * events from Phabricator. Shows all events (no room filtering), lays out
 * overlapping events side-by-side, draws a red "now" line, and opens a detail
 * modal when an event is tapped.
 */
export const ReservationsPanel: FC<ReservationsPanelProps> = () => {
  const { t } = useTranslation();
  const [events, setEvents] = useState<ReservationEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [nowMin, setNowMin] = useState(() => {
    const d = new Date();
    return d.getHours() * 60 + d.getMinutes();
  });
  const [selected, setSelected] = useState<ReservationEvent | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const didAutoScroll = useRef(false);

  // Poll the reservations endpoint.
  useEffect(() => {
    let cancelled = false;
    const load = () => {
      fetchReservations()
        .then((data) => {
          if (cancelled) return;
          setEvents(data);
          setError(null);
        })
        .catch((err) => {
          if (cancelled) return;
          setError(String(err?.message ?? err));
        })
        .finally(() => {
          if (!cancelled) setLoaded(true);
        });
    };
    load();
    const id = window.setInterval(load, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  // Tick the "now" line every minute.
  useEffect(() => {
    const id = window.setInterval(() => {
      const d = new Date();
      setNowMin(d.getHours() * 60 + d.getMinutes());
    }, 60_000);
    return () => window.clearInterval(id);
  }, []);

  const dayStartMs = startOfTodayMs();
  const allDay = events.filter((e) => e.is_all_day);
  const timed = useMemo(
    () => layoutEvents(events.filter((e) => !e.is_all_day)),
    [events],
  );

  // Auto-scroll so the current time is visible, once after first load.
  useEffect(() => {
    if (didAutoScroll.current || !loaded || !scrollRef.current) return;
    const el = scrollRef.current;
    const nowTop = (nowMin / 60) * HOUR_HEIGHT;
    el.scrollTop = Math.max(0, nowTop - el.clientHeight / 3);
    didAutoScroll.current = true;
  }, [loaded, nowMin]);

  return (
    <div className="flex h-full flex-col rounded-xl border border-border bg-card">
      <div className="flex items-center gap-2 border-b border-border px-4 py-3">
        <CalendarClock className="size-5 text-muted-foreground" />
        <h2 className="text-base font-semibold">{t("Reservations")}</h2>
        <span className="ml-auto text-sm text-muted-foreground">
          {new Intl.DateTimeFormat("pl-PL", {
            weekday: "long",
            day: "2-digit",
            month: "long",
          }).format(new Date())}
        </span>
      </div>

      {allDay.length > 0 && (
        <div className="flex flex-wrap gap-1.5 border-b border-border px-4 py-2">
          {allDay.map((e) => (
            <button
              key={e.id}
              type="button"
              onClick={() => setSelected(e)}
              className="rounded-md bg-indigo-500/15 px-2 py-1 text-xs font-medium text-indigo-500"
            >
              {e.name}
            </button>
          ))}
        </div>
      )}

      {error ? (
        <div className="flex flex-1 items-center justify-center px-4 text-center text-sm text-muted-foreground">
          {t("Could not load reservations")}
        </div>
      ) : (
        <div ref={scrollRef} className="relative flex-1 overflow-y-auto">
          <div className="relative" style={{ height: DAY_HEIGHT }}>
            {/* Hour grid */}
            {Array.from({ length: 24 }, (_, h) => (
              <div
                key={h}
                className="absolute left-0 right-0 border-t border-border/60"
                style={{ top: h * HOUR_HEIGHT }}
              >
                <span className="absolute -top-2 left-1 text-[10px] tabular-nums text-muted-foreground">
                  {String(h).padStart(2, "0")}:00
                </span>
              </div>
            ))}

            {/* Events */}
            <div className="absolute inset-y-0 left-12 right-2">
              {timed.map(({ ev, col, cols }) => {
                const top = (minutesIntoDay(ev.start, dayStartMs) / 60) * HOUR_HEIGHT;
                const bottom = (minutesIntoDay(ev.end, dayStartMs) / 60) * HOUR_HEIGHT;
                const height = Math.max(18, bottom - top);
                const widthPct = 100 / cols;
                return (
                  <button
                    key={ev.id}
                    type="button"
                    onClick={() => setSelected(ev)}
                    className="absolute overflow-hidden rounded-md border border-indigo-500/40 bg-indigo-500/15 px-2 py-1 text-left transition-colors hover:bg-indigo-500/25"
                    style={{
                      top,
                      height,
                      left: `${col * widthPct}%`,
                      width: `calc(${widthPct}% - 4px)`,
                    }}
                  >
                    <div className="truncate text-xs font-semibold text-indigo-600 dark:text-indigo-300">
                      {ev.name}
                    </div>
                    <div className="truncate text-[10px] text-muted-foreground">
                      {fmtTime(ev.start)}–{fmtTime(ev.end)}
                    </div>
                  </button>
                );
              })}
            </div>

            {/* Now line */}
            <div
              className="pointer-events-none absolute left-10 right-0 z-10 flex items-center"
              style={{ top: (nowMin / 60) * HOUR_HEIGHT }}
            >
              <div className="size-2 rounded-full bg-red-500" />
              <div className="h-px flex-1 bg-red-500" />
            </div>
          </div>
        </div>
      )}

      <Dialog open={!!selected} onOpenChange={(o) => !o && setSelected(null)}>
        <DialogContent>
          {selected && (
            <>
              <DialogHeader>
                <DialogTitle>{selected.name}</DialogTitle>
                <DialogDescription>
                  {selected.is_all_day
                    ? `${fmtDate(selected.start)} · ${t("All day")}`
                    : `${fmtDate(selected.start)} · ${fmtTime(selected.start)}–${fmtTime(selected.end)}`}
                </DialogDescription>
              </DialogHeader>

              {selected.description ? (
                <p className="max-h-60 overflow-y-auto whitespace-pre-wrap text-sm text-foreground">
                  {selected.description}
                </p>
              ) : (
                <p className="text-sm italic text-muted-foreground">
                  {t("No description")}
                </p>
              )}

              <div className="text-sm text-muted-foreground">
                {t("Created by")}:{" "}
                <span className="text-foreground">
                  {selected.created_by || t("Unknown")}
                </span>
              </div>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default ReservationsPanel;
