import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FC,
} from "react";
import {
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  Clock,
  Undo2,
  User,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { QRCodeSVG } from "qrcode.react";
import {
  fetchReservations,
  type ReservationEvent,
} from "../../lib/reservations";
import { ACTIVITY_EVENTS, TABLET_IDLE_MS } from "../../lib/tabletInactivity";
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

const HOUR_HEIGHT = 34; // px per hour
const DAY_HEIGHT = HOUR_HEIGHT * 24;
const POLL_INTERVAL_MS = 60_000;
// Re-centre the "now" line on today's view this often while the kiosk sits idle.
const RESCROLL_INTERVAL_MS = 120_000;

function startOfDay(date: Date): Date {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  return d;
}

function addDays(date: Date, days: number): Date {
  const d = new Date(date);
  d.setDate(d.getDate() + days);
  return d;
}

function isSameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
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
 * ReservationsPanel — a Google-Calendar-style day view of Phabricator calendar
 * events. Shows all events (no room filtering), supports navigating between
 * days, lays out overlapping events side-by-side, draws a red "now" line on
 * today, and opens a detail modal when an event is tapped. On a 24/7 kiosk it
 * returns to today and re-centres the now line after inactivity.
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
  const [day, setDay] = useState<Date>(() => startOfDay(new Date()));
  const scrollRef = useRef<HTMLDivElement | null>(null);

  const isToday = isSameDay(day, new Date());

  // Poll the reservations endpoint for the selected day.
  useEffect(() => {
    let cancelled = false;
    const startSec = Math.floor(day.getTime() / 1000);
    const endSec = Math.floor(addDays(day, 1).getTime() / 1000);
    setLoaded(false);
    const load = () => {
      fetchReservations(startSec, endSec)
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
  }, [day]);

  // Tick the "now" line every minute.
  useEffect(() => {
    const id = window.setInterval(() => {
      const d = new Date();
      setNowMin(d.getHours() * 60 + d.getMinutes());
    }, 60_000);
    return () => window.clearInterval(id);
  }, []);

  // Scroll so the current time sits in the upper third of the viewport.
  const scrollToNow = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const d = new Date();
    const nowTop = ((d.getHours() * 60 + d.getMinutes()) / 60) * HOUR_HEIGHT;
    el.scrollTop = Math.max(0, nowTop - el.clientHeight / 3);
  }, []);

  const dayStartMs = day.getTime();
  const dayStartSec = dayStartMs / 1000;
  const dayEndSec = addDays(day, 1).getTime() / 1000;
  // Only render events that actually overlap the selected day; the API may
  // return neighbours that merely touch the boundary (e.g. an event ending at
  // 00:00), which would otherwise clamp to the grid edge.
  const visibleEvents = useMemo(
    () => events.filter((e) => e.end > dayStartSec && e.start < dayEndSec),
    [events, dayStartSec, dayEndSec],
  );
  const allDay = visibleEvents.filter((e) => e.is_all_day);
  const timed = useMemo(
    () => layoutEvents(visibleEvents.filter((e) => !e.is_all_day)),
    [visibleEvents],
  );

  // Auto-scroll on load / day change: to the "now" line for today, otherwise
  // to a sensible default (early morning). Intentionally not depending on
  // nowMin so we don't fight the user's scrolling every minute.
  useEffect(() => {
    if (!loaded || !scrollRef.current) return;
    if (isToday) scrollToNow();
    else scrollRef.current.scrollTop = 7 * HOUR_HEIGHT;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loaded, day]);

  // Kiosk runs 24/7: after a spell of inactivity, return to today and close any
  // open dialog (sharing InactivityRedirect's idle window), and keep the "now"
  // line re-centred on today's view so it never drifts off-screen.
  const dayRef = useRef(day);
  dayRef.current = day;
  useEffect(() => {
    const lastActivity = { t: Date.now() };
    let lastRescroll = Date.now();
    const onActivity = () => {
      lastActivity.t = Date.now();
    };
    ACTIVITY_EVENTS.forEach((e) =>
      window.addEventListener(e, onActivity, { passive: true }),
    );
    const id = window.setInterval(() => {
      if (Date.now() - lastActivity.t < TABLET_IDLE_MS) return;
      const today = startOfDay(new Date());
      if (!isSameDay(dayRef.current, today)) {
        setDay(today); // refetch + auto-scroll handled by the load effect
      } else if (Date.now() - lastRescroll >= RESCROLL_INTERVAL_MS) {
        scrollToNow();
        lastRescroll = Date.now();
      }
      setSelected((prev) => (prev === null ? prev : null));
    }, 1000);
    return () => {
      ACTIVITY_EVENTS.forEach((e) => window.removeEventListener(e, onActivity));
      window.clearInterval(id);
    };
  }, [scrollToNow]);

  return (
    <div className="flex h-full flex-col rounded-xl border border-border bg-card">
      <div className="flex items-center gap-2 border-b border-border px-4 py-3">
        <CalendarClock className="size-5 text-muted-foreground" />
        <h2 className="text-base font-semibold">{t("Reservations")}</h2>
        <div className="ml-auto flex items-center gap-1">
          {!isToday && (
            <button
              type="button"
              onClick={() => setDay(startOfDay(new Date()))}
              className="flex items-center gap-1.5 rounded-md border border-border bg-muted px-2.5 py-1 text-sm font-medium text-foreground shadow-sm transition-colors hover:bg-accent"
            >
              <Undo2 className="size-4" />
              {t("Today")}
            </button>
          )}
          <span className="px-1 text-sm font-medium tabular-nums text-foreground">
            {new Intl.DateTimeFormat("pl-PL", {
              weekday: "long",
              day: "2-digit",
              month: "long",
            }).format(day)}
          </span>
          <button
            type="button"
            onClick={() => setDay((d) => addDays(d, -1))}
            aria-label={t("Previous day")}
            className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          >
            <ChevronLeft className="size-5" />
          </button>
          <button
            type="button"
            onClick={() => setDay((d) => addDays(d, 1))}
            aria-label={t("Next day")}
            className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          >
            <ChevronRight className="size-5" />
          </button>
        </div>
      </div>

      {allDay.length > 0 && (
        <div className="flex flex-wrap gap-1.5 border-b border-border px-4 py-2">
          {allDay.map((e, i) => (
            <button
              key={e.phid || `${e.start}-${i}`}
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
              {timed.map(({ ev, col, cols }, i) => {
                const top = (minutesIntoDay(ev.start, dayStartMs) / 60) * HOUR_HEIGHT;
                const bottom = (minutesIntoDay(ev.end, dayStartMs) / 60) * HOUR_HEIGHT;
                const height = Math.max(18, bottom - top);
                const widthPct = 100 / cols;
                return (
                  <button
                    key={ev.phid || `${ev.start}-${i}`}
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
                    <div className="flex items-center gap-1 text-[11px] text-muted-foreground">
                      <Clock className="size-3 shrink-0" />
                      <span className="truncate">
                        {fmtTime(ev.start)}–{fmtTime(ev.end)}
                      </span>
                    </div>
                    {ev.created_by && (
                      <div className="flex items-center gap-1 text-[11px] text-muted-foreground/80">
                        <User className="size-3 shrink-0" />
                        <span className="truncate">{ev.created_by}</span>
                      </div>
                    )}
                  </button>
                );
              })}
            </div>

            {/* Now line (only on today's view) */}
            {isToday && (
              <div
                className="pointer-events-none absolute left-10 right-0 z-10 flex items-center"
                style={{ top: (nowMin / 60) * HOUR_HEIGHT }}
              >
                <div className="size-2 rounded-full bg-red-500" />
                <div className="h-px flex-1 bg-red-500" />
              </div>
            )}
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

              {selected.id > 0 && (
                <div className="flex items-center gap-4 rounded-lg border border-border bg-muted/40 p-3">
                  <div className="shrink-0 rounded-md bg-white p-2">
                    <QRCodeSVG value={selected.url} size={96} />
                  </div>
                  <div className="min-w-0 text-sm text-muted-foreground">
                    <div className="font-medium text-foreground">
                      {t("Scan to open in Phabricator")}
                    </div>
                    <div className="mt-1 break-all text-xs">{selected.url}</div>
                  </div>
                </div>
              )}
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default ReservationsPanel;
