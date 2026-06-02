import { useState } from "react";
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

export function useLiveRoomStates() {
  const [roomStates, setRoomStates] = useState<Record<string, RoomState>>({});

  useWebsocket(liveWebsocketUrl(), {
    binaryType: "arraybuffer",
    onMessage: (messageEvent) => {
      if (!messageEvent.data) {
        return;
      }

      try {
        const nextRoom = JSON.parse(messageEvent.data) as RoomState;
        setRoomStates((prev) => ({ ...prev, [nextRoom.id]: nextRoom }));
      } catch (error) {
        console.warn("Bad message payload", error);
      }
    },
  });

  return Object.values(roomStates);
}