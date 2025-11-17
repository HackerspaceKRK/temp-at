/**
 * Root application component.
 * - Establishes websocket for live room updates
 * - Provides locale context (hardcoded "pl")
 * - Renders responsive grid of RoomCards
 */
import { useEffect, useState } from "preact/hooks";
import "./app.css";
import useWebsocket from "./useWebsocket";
import type { RoomState } from "./schema";
import { LocaleProvider } from "./locale";
import RoomCard from "./components/RoomCard";
import { API_URL } from "./config";

export function App() {
  const { lastMessage } = useWebsocket(
    `${API_URL.replace(/\/$/, "")}/api/v1/live-ws`,
  );

  const [roomStates, setRoomStates] = useState<{ [key: string]: RoomState }>(
    {},
  );

  useEffect(() => {
    if (lastMessage?.data) {
      try {
        const data = JSON.parse(lastMessage.data);
        setRoomStates((prev) => ({ ...prev, [data.id]: data }));
      } catch (err) {
        // eslint-disable-next-line no-console
        console.warn("Bad message payload", err);
      }
    }
  }, [lastMessage]);

  const rooms = Object.values(roomStates).sort((a, b) => {
    // 1. Snapshot count descending (rooms with more camera snapshots first)
    const snapCountA = a.entities.filter(
      (e) => e.representation === "camera_snapshot",
    ).length;
    const snapCountB = b.entities.filter(
      (e) => e.representation === "camera_snapshot",
    ).length;
    if (snapCountB !== snapCountA) {
      return snapCountB - snapCountA;
    }

    // 2. People count descending
    if (b.people_count !== a.people_count) {
      return b.people_count - a.people_count;
    }

    // Collect light entities
    const lightsA = a.entities.filter((e) => e.representation === "light");
    const lightsB = b.entities.filter((e) => e.representation === "light");

    // 3. Number of turned on lights descending
    const lightsOnA = lightsA.filter((e) => (e as any).state === "ON").length;
    const lightsOnB = lightsB.filter((e) => (e as any).state === "ON").length;
    if (lightsOnB !== lightsOnA) {
      return lightsOnB - lightsOnA;
    }

    // 4. Total lights descending
    if (lightsB.length !== lightsA.length) {
      return lightsB.length - lightsA.length;
    }

    return 0;
  });

  return (
    <LocaleProvider locale="pl">
      <div className="min-h-screen bg-neutral-200">
        <div className="max-w-6xl mx-auto">
          <header className="px-4 py-4">
            <h1 className="text-2xl font-bold text-neutral-900">
              Status przestrzeni
            </h1>
          </header>
          <main className="grid grid-cols-1 md:grid-cols-2 gap-4 px-4 pb-10">
            {rooms.length === 0 && (
              <div className="col-span-full text-center text-neutral-600 py-10">
                Oczekiwanie na dane…
              </div>
            )}
            {rooms.map((room) => (
              <RoomCard key={room.id} room={room} />
            ))}
          </main>
        </div>
      </div>
    </LocaleProvider>
  );
}
