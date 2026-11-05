import { WifiOff } from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FC,
  type ReactElement,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import { useAppConfig, type RoomMapEntry } from "../AppConfigContext";
import { Card, CardContent } from "../components/ui/card";
import { useLocale } from "../locale";

// ─── WebSocket state types ───────────────────────────────────────────────────

type ConnectionStatus = "connecting" | "connected" | "forbidden" | "error" | "disabled";

interface WallState {
  wall_number: number;
  state: boolean; // true = closed/up (rooms separated), false = open/down (combined)
  /** canonical wall name as reported by the server */
  canonical: string;
}

interface AmdusiasState {
  /** slug → group number (0 = no group) */
  roomGroups: Record<string, number>;
  /** normalizedWallName → wall info */
  walls: Record<string, WallState>;
  /** slug → channel → level_percent */
  outputLevels: Record<string, Record<number, number>>;
}

// ─── Colour palettes ─────────────────────────────────────────────────────────

const GROUP_STYLES: Record<number, { bg: string; border: string }> = {
  0: { bg: "bg-card", border: "border-border" },
  1: { bg: "bg-blue-500/20", border: "border-blue-500/40" },
  2: { bg: "bg-green-500/20", border: "border-green-500/40" },
  3: { bg: "bg-amber-500/20", border: "border-amber-500/40" },
  4: { bg: "bg-rose-500/20", border: "border-rose-500/40" },
  5: { bg: "bg-purple-500/20", border: "border-purple-500/40" },
  6: { bg: "bg-teal-500/20", border: "border-teal-500/40" },
  7: { bg: "bg-orange-500/20", border: "border-orange-500/40" },
  8: { bg: "bg-pink-500/20", border: "border-pink-500/40" },
};

function groupStyle(group: number): { bg: string; border: string } {
  return GROUP_STYLES[group] ?? GROUP_STYLES[((group - 1) % 8) + 1];
}

// ─── Wall name helpers ────────────────────────────────────────────────────────

function normalizeWall(a: string, b: string): string {
  return [a, b].sort().join("|");
}

function parseWallName(wall: string): [string, string] {
  const parts = wall.split("|");
  return [parts[0] ?? "", parts[1] ?? ""];
}

// ─── Map geometry helpers ─────────────────────────────────────────────────────

interface GridRoom {
  slugName: string;
  name: Record<string, string>;
  squares: [number, number][];
}

interface WallStrip {
  /** normalized name */
  key: string;
  slugA: string;
  slugB: string;
  /** CSS grid column (1-based) */
  gridCol: number;
  /** CSS grid row (1-based) */
  gridRow: number;
  /** "vertical" = between horizontally adjacent rooms, "horizontal" = between vertically adjacent */
  orientation: "vertical" | "horizontal";
}

function buildGridData(rooms: RoomMapEntry[]): {
  gridRooms: GridRoom[];
  walls: WallStrip[];
  minCol: number;
  minRow: number;
  maxCol: number;
  maxRow: number;
} {
  const gridRooms: GridRoom[] = rooms.map((r) => ({
    slugName: r.slug_name,
    name: r.name,
    squares: r.map_squares,
  }));

  // Build lookup: "col,row" → slug
  const squareMap = new Map<string, string>();
  for (const room of gridRooms) {
    for (const [col, row] of room.squares) {
      squareMap.set(`${col},${row}`, room.slugName);
    }
  }

  // Find all adjacent cross-room pairs
  const seenWalls = new Set<string>();
  const walls: WallStrip[] = [];

  for (const room of gridRooms) {
    for (const [col, row] of room.squares) {
      // Right neighbour → vertical wall strip
      const rightSlug = squareMap.get(`${col + 1},${row}`);
      if (rightSlug && rightSlug !== room.slugName) {
        const key = normalizeWall(room.slugName, rightSlug);
        if (!seenWalls.has(key)) {
          seenWalls.add(key);
          walls.push({
            key,
            slugA: room.slugName,
            slugB: rightSlug,
            gridCol: 2 * col,         // interstitial column between col and col+1
            gridRow: 2 * row - 1,
            orientation: "vertical",
          });
        }
      }
      // Bottom neighbour → horizontal wall strip
      const bottomSlug = squareMap.get(`${col},${row + 1}`);
      if (bottomSlug && bottomSlug !== room.slugName) {
        const key = normalizeWall(room.slugName, bottomSlug);
        if (!seenWalls.has(key)) {
          seenWalls.add(key);
          walls.push({
            key,
            slugA: room.slugName,
            slugB: bottomSlug,
            gridCol: 2 * col - 1,
            gridRow: 2 * row,         // interstitial row between row and row+1
            orientation: "horizontal",
          });
        }
      }
    }
  }

  // Bounding box in map-square coordinates
  let minCol = Infinity, minRow = Infinity, maxCol = -Infinity, maxRow = -Infinity;
  for (const room of gridRooms) {
    for (const [col, row] of room.squares) {
      if (col < minCol) minCol = col;
      if (row < minRow) minRow = row;
      if (col > maxCol) maxCol = col;
      if (row > maxRow) maxRow = row;
    }
  }

  return { gridRooms, walls, minCol, minRow, maxCol, maxRow };
}

// ─── Map grid component ───────────────────────────────────────────────────────

const CELL_W = 80;
const CELL_H = 72;
const WALL_SIZE = 12;

const RoomMapGrid: FC<{
  rooms: RoomMapEntry[];
  state: AmdusiasState;
  onWallClick: (wallKey: string, slugA: string, slugB: string, currentState: boolean) => void;
}> = ({ rooms, state, onWallClick }) => {
  const { getName } = useLocale();

  const { gridRooms, walls, minCol, minRow, maxCol, maxRow } = useMemo(
    () => buildGridData(rooms),
    [rooms],
  );

  const numCols = maxCol - minCol + 1;
  const numRows = maxRow - minRow + 1;

  const colTemplate = Array.from({ length: 2 * numCols - 1 }, (_, i) =>
    i % 2 === 0 ? `${CELL_W}px` : `${WALL_SIZE}px`,
  ).join(" ");
  const rowTemplate = Array.from({ length: 2 * numRows - 1 }, (_, i) =>
    i % 2 === 0 ? `${CELL_H}px` : `${WALL_SIZE}px`,
  ).join(" ");

  function toCSSCol(col: number) { return 2 * (col - minCol) + 1; }
  function toCSSRow(row: number) { return 2 * (row - minRow) + 1; }
  function wallCSSCol(stripGridCol: number) { return stripGridCol - 2 * (minCol - 1); }
  function wallCSSRow(stripGridRow: number) { return stripGridRow - 2 * (minRow - 1); }

  const occupiedKeys = useMemo(() => {
    const s = new Set<string>();
    for (const r of gridRooms) {
      for (const [col, row] of r.squares) s.add(`${col},${row}`);
    }
    return s;
  }, [gridRooms]);

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: colTemplate,
        gridTemplateRows: rowTemplate,
      }}
    >
      {/* Room cells + same-room connectors */}
      {gridRooms.map((room) => {
        const group = state.roomGroups[room.slugName] ?? 0;
        const { bg, border } = groupStyle(group);
        const squareSet = new Set(room.squares.map(([c, r]) => `${c},${r}`));
        const has = (col: number, row: number, dc: number, dr: number) =>
          squareSet.has(`${col + dc},${row + dr}`);

        // Topmost-leftmost square gets the room name label
        const [labelCol, labelRow] = [...room.squares].sort(
          (a, b) => a[1] - b[1] || a[0] - b[0],
        )[0];

        return [
          // Room squares — remove borders on sides that touch the same room
          ...room.squares.map(([col, row]) => {
            const noRight  = has(col, row,  1,  0);
            const noBottom = has(col, row,  0,  1);
            const noLeft   = has(col, row, -1,  0);
            const noTop    = has(col, row,  0, -1);
            const isLabel  = col === labelCol && row === labelRow;

            return (
              <div
                key={`room-${room.slugName}-${col}-${row}`}
                style={{ gridColumn: toCSSCol(col), gridRow: toCSSRow(row) }}
                className={[
                  "flex items-center justify-center border text-xs font-medium text-center leading-tight px-1",
                  bg,
                  border,
                  noRight  ? "border-r-0" : "",
                  noBottom ? "border-b-0" : "",
                  noLeft   ? "border-l-0" : "",
                  noTop    ? "border-t-0" : "",
                ].filter(Boolean).join(" ")}
              >
                {isLabel ? getName(room.name, room.slugName) : null}
              </div>
            );
          }),

          // Connectors that fill interstitial cells between adjacent same-room squares.
          // Horizontal connectors (wall strip between left and right square) need top+bottom borders.
          // Vertical connectors (wall strip between top and bottom square) need left+right borders.
          ...room.squares.flatMap(([col, row]) => {
            const result: ReactElement[] = [];
            if (has(col, row, 1, 0)) {
              result.push(
                <div
                  key={`conn-r-${room.slugName}-${col}-${row}`}
                  style={{
                    gridColumn: wallCSSCol(2 * col),
                    gridRow: toCSSRow(row),
                  }}
                  className={[bg, "border-t border-b", border].join(" ")}
                />,
              );
            }
            if (has(col, row, 0, 1)) {
              result.push(
                <div
                  key={`conn-b-${room.slugName}-${col}-${row}`}
                  style={{
                    gridColumn: toCSSCol(col),
                    gridRow: wallCSSRow(2 * row),
                  }}
                  className={[bg, "border-l border-r", border].join(" ")}
                />,
              );
            }
            return result;
          }),
        ];
      })}

      {/* Empty cells */}
      {Array.from({ length: numCols }, (_, ci) =>
        Array.from({ length: numRows }, (_, ri) => {
          const col = minCol + ci;
          const row = minRow + ri;
          if (occupiedKeys.has(`${col},${row}`)) return null;
          return (
            <div
              key={`empty-${col}-${row}`}
              style={{ gridColumn: toCSSCol(col), gridRow: toCSSRow(row) }}
            />
          );
        }),
      )}

      {/* Wall strips between different rooms */}
      {walls.map((wall) => {
        const wallInfo = state.walls[wall.key];
        const isClosed = wallInfo === undefined ? true : wallInfo.state;
        const cssCol = wallCSSCol(wall.gridCol);
        const cssRow = wallCSSRow(wall.gridRow);

        const closedCls =
          "bg-slate-500 dark:bg-slate-400 hover:bg-slate-400 dark:hover:bg-slate-300";
        const openCls =
          "bg-slate-200/60 dark:bg-slate-600/40 hover:bg-slate-300/60 dark:hover:bg-slate-500/40 border border-dashed border-slate-300 dark:border-slate-600";

        return (
          <button
            key={wall.key}
            type="button"
            title={isClosed ? "Wall closed — click to open" : "Wall open — click to close"}
            style={{ gridColumn: cssCol, gridRow: cssRow }}
            className={[
              "w-full h-full rounded-sm transition-colors cursor-pointer",
              isClosed ? closedCls : openCls,
            ].join(" ")}
            onClick={() => onWallClick(wall.key, wall.slugA, wall.slugB, isClosed)}
          />
        );
      })}
    </div>
  );
};

// ─── Volume slider component ──────────────────────────────────────────────────

const VolumeSlider: FC<{
  label: string;
  levelPercent: number | undefined;
  onCommit: (level: number) => void;
}> = ({ label, levelPercent, onCommit }) => {
  const value = levelPercent ?? 0;
  const [localValue, setLocalValue] = useState<number>(value);
  const isDragging = useRef(false);

  // Sync from server when not dragging
  useEffect(() => {
    if (!isDragging.current) {
      setLocalValue(value);
    }
  }, [value]);

  return (
    <div className="flex flex-col items-center gap-2" style={{ width: 76 }}>
      <span className="text-xs tabular-nums text-muted-foreground w-10 text-center">
        {Math.round(localValue)}%
      </span>
      {/* Vertical slider via CSS transform */}
      <div
        style={{ height: 140, width: 76, display: "flex", alignItems: "center", justifyContent: "center" }}
      >
        <input
          type="range"
          min={0}
          max={100}
          step={1}
          value={localValue}
          style={{
            width: 120,
            transform: "rotate(-90deg)",
            cursor: "pointer",
            accentColor: "var(--primary)",
          }}
          onMouseDown={() => {
            isDragging.current = true;
          }}
          onChange={(e) => {
            setLocalValue(parseFloat(e.target.value));
          }}
          onMouseUp={() => {
            isDragging.current = false;
            onCommit(localValue);
          }}
          onTouchEnd={() => {
            isDragging.current = false;
            onCommit(localValue);
          }}
        />
      </div>
      {/* Fixed-height label so all sliders stay vertically aligned */}
      <span
        className="text-xs font-medium text-center leading-tight flex items-center justify-center"
        style={{ width: 76, height: "2.5rem" }}
      >
        {label}
      </span>
    </div>
  );
};

// ─── Notice helper (matches EmergencyEntryPage pattern) ──────────────────────

const Notice: FC<{ icon?: ReactNode; children: ReactNode }> = ({ icon, children }) => (
  <div className="flex flex-col items-center gap-3 py-8 text-center text-muted-foreground">
    {icon}
    {children}
  </div>
);

// ─── Main page ────────────────────────────────────────────────────────────────

export const SoundSystemPage: FC = () => {
  const { t } = useTranslation();
  const { config, isLoading: configLoading } = useAppConfig();
  const { getName } = useLocale();

  const wsUrl = config?.audio_system_ws_url ?? "";
  const rooms = config?.rooms_map ?? [];

  const [status, setStatus] = useState<ConnectionStatus>(wsUrl ? "connecting" : "disabled");
  const [errorMessage, setErrorMessage] = useState<string>("");
  const [amdusiasState, setAmdusiasState] = useState<AmdusiasState>({
    roomGroups: {},
    walls: {},
    outputLevels: {},
  });

  const wsRef = useRef<WebSocket | null>(null);
  const msgIdRef = useRef(0);
  const openedRef = useRef(false);

  // Process a single event from the server
  const applyEvent = useCallback((event: Record<string, unknown>) => {
    const action = event.action as string | undefined;

    if (action === "change_room_group") {
      const room = event.room as string;
      const group = event.group as number;
      setAmdusiasState((prev) => ({
        ...prev,
        roomGroups: { ...prev.roomGroups, [room]: group },
      }));
    } else if (action === "change_room_wall") {
      const wall = event.wall as string;
      const wallNumber = event.wall_number as number;
      const wallState = event.state as boolean;
      const [a, b] = parseWallName(wall);
      const key = normalizeWall(a, b);
      setAmdusiasState((prev) => ({
        ...prev,
        walls: {
          ...prev.walls,
          [key]: { wall_number: wallNumber, state: wallState, canonical: wall },
        },
      }));
    } else if (action === "set_output_level") {
      const room = event.room as string;
      const channel = event.channel as number;
      const levelPercent = event.level_percent as number | undefined;
      if (levelPercent !== undefined) {
        setAmdusiasState((prev) => ({
          ...prev,
          outputLevels: {
            ...prev.outputLevels,
            [room]: {
              ...(prev.outputLevels[room] ?? {}),
              [channel]: levelPercent,
            },
          },
        }));
      }
    }
  }, []);

  // Connect (or reconnect) WebSocket
  const connect = useCallback(() => {
    if (!wsUrl) return;
    // Detach and close any previous socket before opening a new one
    if (wsRef.current) {
      wsRef.current.onopen = null;
      wsRef.current.onmessage = null;
      wsRef.current.onerror = null;
      wsRef.current.onclose = null;
      wsRef.current.close();
      wsRef.current = null;
    }
    openedRef.current = false;
    setStatus("connecting");

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      openedRef.current = true;
      setStatus("connected");
    };

    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data as string) as Record<string, unknown>;
        if (msg.type === "snapshot") {
          // Bulk-initialise state
          setAmdusiasState({ roomGroups: {}, walls: {}, outputLevels: {} });
          const events = (msg.state as Record<string, unknown>[]) ?? [];
          for (const e of events) applyEvent(e);
        } else if (msg.type !== "ack") {
          applyEvent(msg);
        }
      } catch {
        // ignore parse errors
      }
    };

    ws.onerror = () => {
      // handled in onclose
    };

    ws.onclose = () => {
      if (openedRef.current) {
        // Was connected, lost connection
        setStatus("error");
        setErrorMessage(t("Connection to the sound system was lost."));
        return;
      }
      // Never opened — check for 403
      const httpUrl = wsUrl.replace(/^wss?:\/\//, "https://");
      fetch(httpUrl, { signal: AbortSignal.timeout(5000) })
        .then(async (res) => {
          if (res.status === 403) {
            try {
              const body = (await res.json()) as { detail?: string };
              if (typeof body.detail === "string" && body.detail.endsWith("IP not allowed")) {
                setStatus("forbidden");
                return;
              }
            } catch {
              // ignore JSON parse error
            }
          }
          setStatus("error");
          setErrorMessage(t("Could not connect to the sound system."));
        })
        .catch(() => {
          // CORS or network error — can't tell the difference from 403; assume network issue
          setStatus("forbidden");
        });
    };

    return () => {
      ws.close();
    };
  }, [wsUrl, applyEvent, t]);

  useEffect(() => {
    if (!wsUrl) {
      setStatus("disabled");
      return;
    }
    const cleanup = connect();
    return cleanup;
  }, [wsUrl, connect]);

  // Send a message to the server
  const send = useCallback((payload: Record<string, unknown>) => {
    const ws = wsRef.current;
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(payload));
    }
  }, []);

  const nextId = useCallback(() => {
    msgIdRef.current += 1;
    return msgIdRef.current;
  }, []);

  const handleWallClick = useCallback(
    (wallKey: string, slugA: string, slugB: string, currentClosed: boolean) => {
      const wallInfo = amdusiasState.walls[wallKey];
      // Use canonical form if known, else use slugA|slugB
      const wallName = wallInfo?.canonical ?? `${slugA}|${slugB}`;
      send({
        action: "change_room_wall",
        id: nextId(),
        wall: wallName,
        state: !currentClosed,
      });
    },
    [amdusiasState.walls, send, nextId],
  );

  const handleSetAllWalls = useCallback(
    (closed: boolean) => {
      const { walls } = buildGridData(rooms);
      for (const wall of walls) {
        const wallInfo = amdusiasState.walls[wall.key];
        const wallName = wallInfo?.canonical ?? wall.key;
        send({ action: "change_room_wall", id: nextId(), wall: wallName, state: closed });
      }
    },
    [rooms, amdusiasState.walls, send, nextId],
  );

  const handleVolumeChange = useCallback(
    (slug: string, level: number) => {
      send({
        action: "set_output_level",
        id: nextId(),
        room: slug,
        level_percent: level,
      });
    },
    [send, nextId],
  );

  // ── Rooms ordered for volume sliders (left-right, top-bottom) ──────────────
  const orderedRooms = useMemo(() => {
    return [...rooms].sort((a, b) => {
      const aMinRow = Math.min(...a.map_squares.map(([, r]) => r));
      const bMinRow = Math.min(...b.map_squares.map(([, r]) => r));
      if (aMinRow !== bMinRow) return aMinRow - bMinRow;
      const aMinCol = Math.min(...a.map_squares.map(([c]) => c));
      const bMinCol = Math.min(...b.map_squares.map(([c]) => c));
      return aMinCol - bMinCol;
    });
  }, [rooms]);

  // ── Per-room average output level (across channels) ────────────────────────
  function roomOutputLevel(slug: string): number | undefined {
    const channels = amdusiasState.outputLevels[slug];
    if (!channels) return undefined;
    const vals = Object.values(channels);
    if (vals.length === 0) return undefined;
    return vals.reduce((a, b) => a + b, 0) / vals.length;
  }

  // ── Status-based content ───────────────────────────────────────────────────
  let pageContent: ReactNode;

  if (configLoading) {
    pageContent = (
      <div className="flex items-center justify-center py-24">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-muted-foreground border-t-transparent" />
      </div>
    );
  } else if (status === "disabled") {
    pageContent = (
      <Card className="w-full max-w-2xl mx-auto">
        <CardContent>
          <Notice>{t("Sound system is not configured.")}</Notice>
        </CardContent>
      </Card>
    );
  } else if (status === "forbidden") {
    pageContent = (
      <Card className="w-full max-w-2xl mx-auto">
        <CardContent>
          <Notice icon={<WifiOff className="size-10 text-orange-500" />}>
            <p className="font-medium text-foreground">
              {t("Please connect to the Hackerspace Wi-Fi network to use this feature.")}
            </p>
            <p>{t("If you are using a VPN, disable it temporarily.")}</p>
          </Notice>
        </CardContent>
      </Card>
    );
  } else if (status === "connecting") {
    pageContent = (
      <div className="flex items-center justify-center py-24">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-muted-foreground border-t-transparent" />
      </div>
    );
  } else if (status === "error") {
    pageContent = (
      <Card className="w-full max-w-2xl mx-auto">
        <CardContent>
          <Notice>
            <p className="font-medium text-foreground">{errorMessage || t("Connection error.")}</p>
            <button
              type="button"
              className="mt-2 text-sm text-primary underline underline-offset-2 cursor-pointer"
              onClick={connect}
            >
              {t("Retry")}
            </button>
          </Notice>
        </CardContent>
      </Card>
    );
  } else {
    // connected
    const helpUrl = config?.audio_system_help_url
      ? getName(config.audio_system_help_url, "")
      : undefined;

    pageContent = (
      <div className="flex justify-center">
        {/*
         * CSS Grid: map (col 1) and sliders (col 2) are each an auto-sized cell.
         * Help box spans both columns, so its edges are guaranteed to align with
         * the outer edges of the two panels below it.
         * On small screens a single auto column stacks everything vertically.
         */}
        <div className="grid gap-6 grid-cols-[auto] lg:grid-cols-[auto_auto]">
          {/* Help box — spans both columns on large screens */}
          <div
            className="col-span-1 lg:col-span-2 rounded-xl border bg-muted/30 p-4 text-sm text-muted-foreground"
            style={{ minWidth: 0 }}
          >
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-1">
              {t("Instructions")}
            </p>
            <p>
              {t(
                "Rooms can be grouped by opening the walls between them. Click a wall strip on the map to toggle it open or closed. Rooms in the same group share a colour.",
              )}
              {helpUrl && (
                <>
                  {" "}
                  <a
                    href={helpUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary underline underline-offset-2 hover:text-primary/80"
                  >
                    {t("More information")} ↗
                  </a>
                </>
              )}
            </p>
          </div>

          {/* Map — column 1 */}
          <div className="flex flex-col gap-3 rounded-xl border bg-muted/30 p-4">
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-base font-semibold text-muted-foreground uppercase tracking-wide">
                {t("Rooms Map")}
              </h2>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => handleSetAllWalls(false)}
                  className="rounded-md border border-border bg-background px-2 py-1 text-xs font-medium text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
                >
                  {t("Open all")}
                </button>
                <button
                  type="button"
                  onClick={() => handleSetAllWalls(true)}
                  className="rounded-md border border-border bg-background px-2 py-1 text-xs font-medium text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
                >
                  {t("Close all")}
                </button>
              </div>
            </div>
            {rooms.length > 0 ? (
              <div className="overflow-auto">
                <RoomMapGrid
                  rooms={rooms}
                  state={amdusiasState}
                  onWallClick={handleWallClick}
                />
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">{t("No rooms configured.")}</p>
            )}
          </div>

          {/* Volume sliders — column 2 */}
          <div className="flex flex-col gap-3 rounded-xl border bg-muted/30 p-4">
            <h2 className="text-base font-semibold text-muted-foreground uppercase tracking-wide">
              {t("Volume")}
            </h2>
            <div className="flex flex-row flex-wrap gap-3 items-end">
              {orderedRooms.map((room) => (
                <VolumeSlider
                  key={room.slug_name}
                  label={getName(room.name, room.slug_name)}
                  levelPercent={roomOutputLevel(room.slug_name)}
                  onCommit={(level) => handleVolumeChange(room.slug_name, level)}
                />
              ))}
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <main className="px-4 py-6">
      {pageContent}
    </main>
  );
};
