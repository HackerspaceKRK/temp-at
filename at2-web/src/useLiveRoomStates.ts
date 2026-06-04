import {
  createContext,
  createElement,
  useCallback,
  useContext,
  useRef,
  useState,
  type ReactNode,
} from "react";
import useWebsocket from "./useWebsocket";
import { liveWebsocketUrl } from "./config";
import type { RoomState } from "./schema";

export function scoreRoom(room: RoomState): number {
  let score = 0;
  const snapshotCount = room.entities.filter(
    (entity) => entity.type === "camera_snapshot",
  ).length;
  score += snapshotCount * 20;
  if (snapshotCount === 0) {
    score -= 1000;
  }

  score += room.people_count * 200;

  const lights = room.entities.filter(
    (entity) => entity.representation === "light",
  );
  const lightsOn = lights.filter((entity) => entity.state === "ON").length;
  score += lightsOn * 50;
  score += lights.length * 10;

  return score;
}

interface LiveStateContextValue {
  rooms: RoomState[];
}

const LiveStateContext = createContext<LiveStateContextValue | null>(null);

/**
 * LiveStateProvider owns the single application-wide WebSocket connection to
 * `/api/v1/live-ws`. It is mounted once near the root so both the normal web UI
 * and the tablet views share a single socket.
 *
 * It also handles automatic reloading: the backend sends a `server_info`
 * message (with its version) as the first frame on every connection. The first
 * version we see becomes the baseline; if a later (post-reconnect) frame reports
 * a different version, the backend was redeployed and we reload to pick up the
 * new frontend build.
 */
export function LiveStateProvider({ children }: { children: ReactNode }) {
  const [roomStates, setRoomStates] = useState<Record<string, RoomState>>({});
  const baselineVersionRef = useRef<string | null>(null);

  const onMessage = useCallback((messageEvent: MessageEvent) => {
    if (!messageEvent.data) {
      return;
    }

    let parsed: unknown;
    try {
      parsed = JSON.parse(messageEvent.data);
    } catch (error) {
      console.warn("Bad message payload", error);
      return;
    }

    // Control messages carry a top-level `type`; room states do not.
    if (
      parsed &&
      typeof parsed === "object" &&
      (parsed as { type?: string }).type === "server_info"
    ) {
      const version = (parsed as { version?: string }).version ?? "";
      if (baselineVersionRef.current === null) {
        baselineVersionRef.current = version;
      } else if (baselineVersionRef.current !== version) {
        console.log(
          `Server version changed (${baselineVersionRef.current} -> ${version}), reloading`,
        );
        window.location.reload();
      }
      return;
    }

    const nextRoom = parsed as RoomState;
    setRoomStates((prev) => ({ ...prev, [nextRoom.id]: nextRoom }));
  }, []);

  useWebsocket(liveWebsocketUrl(), {
    binaryType: "arraybuffer",
    onMessage,
  });

  const rooms = Object.values(roomStates);

  return createElement(
    LiveStateContext.Provider,
    { value: { rooms } },
    children,
  );
}

export function useLiveRoomStates(): RoomState[] {
  const context = useContext(LiveStateContext);
  if (!context) {
    throw new Error("useLiveRoomStates must be used within a LiveStateProvider");
  }
  return context.rooms;
}
