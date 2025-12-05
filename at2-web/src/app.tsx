/**
 * Root application component.
 * - Establishes websocket for live room updates
 * - Provides locale context (hardcoded "pl")
 * - Renders responsive grid of RoomCards
 */
import { useEffect, useState } from "react";
import type { FunctionalComponent } from "react";
import "./app.css";
import useWebsocket from "./useWebsocket";
import type { RoomState } from "./schema";
import { LocaleProvider } from "./locale";
import RoomCard from "./components/RoomCard";
import { API_URL } from "./config";
import { Sun, Moon } from "lucide-react";
import { ThemeProvider, useTheme } from "./theme";
import { Button } from "./components/ui/button";

export function App() {
  return (
    <ThemeProvider>
      <LocaleProvider locale="pl">
        <AppContent />
      </LocaleProvider>
    </ThemeProvider>
  );
}

const AppContent: FunctionalComponent = () => {
  const [roomStates, setRoomStates] = useState<{ [key: string]: RoomState }>(
    {}
  );
  const {} = useWebsocket(`${API_URL.replace(/\/$/, "")}/api/v1/live-ws`, {
    binaryType: "arraybuffer",
    onMessage: (msgEvt) => {
      if (msgEvt.data) {
        try {
          const data = JSON.parse(msgEvt.data);
          setRoomStates((prev) => ({ ...prev, [data.id]: data }));
        } catch (err) {
          // eslint-disable-next-line no-console
          console.warn("Bad message payload", err);
        }
      }
    },
  });

 

  const rooms = Object.values(roomStates).sort((a, b) => {
    const snapCountA = a.entities.filter(
      (e) => e.representation === "camera_snapshot"
    ).length;
    const snapCountB = b.entities.filter(
      (e) => e.representation === "camera_snapshot"
    ).length;
    if (snapCountB !== snapCountA) return snapCountB - snapCountA;

    if (b.people_count !== a.people_count) {
      return b.people_count - a.people_count;
    }

    const lightsA = a.entities.filter((e) => e.representation === "light");
    const lightsB = b.entities.filter((e) => e.representation === "light");

    const lightsOnA = lightsA.filter((e) => (e as any).state === "ON").length;
    const lightsOnB = lightsB.filter((e) => (e as any).state === "ON").length;
    if (lightsOnB !== lightsOnA) return lightsOnB - lightsOnA;

    if (lightsB.length !== lightsA.length)
      return lightsB.length - lightsA.length;

    return 0;
  });

  const { theme, toggleTheme } = useTheme();

  return (
    <div className="min-h-screen bg-neutral-200 dark:bg-neutral-900 text-neutral-900 dark:text-neutral-100">
      <div className="max-w-6xl mx-auto">
        <header className="px-4 py-4 flex items-center justify-between">
          <h1 className="text-2xl font-bold text-neutral-900 dark:text-neutral-100">
            Status przestrzeni
          </h1>
          <Button
            variant="outline"
            size="sm"
            onClick={toggleTheme}
            aria-label={
              theme === "dark"
                ? "Przełącz na jasny motyw"
                : "Przełącz na ciemny motyw"
            }
          >
            {theme === "dark" ? "Jasny" : "Ciemny"}
          </Button>
        </header>
        <main className="grid grid-cols-1 md:grid-cols-2 gap-4 px-4 pb-10">
          {rooms.length === 0 && (
            <div className="col-span-full text-center py-10 text-neutral-600">
              Oczekiwanie na dane…
            </div>
          )}
          {rooms.map((room) => (
            <RoomCard key={room.id} room={room} />
          ))}
        </main>
      </div>
    </div>
  );
};
