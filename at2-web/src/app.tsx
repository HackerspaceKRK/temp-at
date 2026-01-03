/**
 * Root application component.
 * - Establishes websocket for live room updates
 * - Provides locale context (hardcoded "pl")
 * - Renders responsive grid of RoomCards
 */
import { useEffect, useState } from "react";
import type { FC } from "react";
import "./app.css";
import useWebsocket from "./useWebsocket";
import type { RoomState } from "./schema";
import RoomCard from "./components/RoomCard";
import { API_URL } from "./config";
import { AuthProvider, useAuth } from "./AuthContext";
import { useTranslation } from "react-i18next";

import { ThemeProvider } from "./theme";

import { ModeToggle } from "./components/ModeToggle";
import { LanguageToggle } from "./components/LanguageToggle";

export function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <AppContent />
      </AuthProvider>
    </ThemeProvider>
  );
}

const UserControls: FC = () => {
  const { user, login, logout, isLoading } = useAuth();
  const { t } = useTranslation();

  if (isLoading) {
    return <div>...</div>;
  }

  if (user) {
    return (
      <div className="flex items-center gap-4">
        <span className="text-sm">
          {t("Welcome, {{username}}", { username: user.username })}
        </span>
        <button
          onClick={logout}
          className="text-sm underline hover:text-primary"
        >
          {t("Log Out")}
        </button>
      </div>
    );
  }

  return (
    <button onClick={login} className="text-sm font-semibold hover:underline">
      {t("Log In")}
    </button>
  );
};

const AppContent: FC = () => {
  const { t } = useTranslation();
  const [roomStates, setRoomStates] = useState<{ [key: string]: RoomState }>(
    {}
  );
  const [roomScores, setRoomScores] = useState<{ [key: string]: number }>({});
  const { } = useWebsocket(`${API_URL.replace(/\/$/, "")}/api/v1/live-ws`, {
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

  useEffect(() => {
    for (const roomId in roomStates) {
      if (!(roomId in roomScores)) {
        const room = roomStates[roomId];
        let score = 0;
        const snapCount = room.entities.filter(
          (e) => e.type === "camera_snapshot"
        ).length;
        score += snapCount * 20;
        if (snapCount == 0) {
          score -= 1000;
        }

        score += room.people_count * 200;
        const lights = room.entities.filter((e) => e.representation === "light");
        const lightsOn = lights.filter((e) => (e as any).state === "ON").length;
        score += lightsOn * 50;
        score += lights.length * 10;

        setRoomScores((prev) => ({ ...prev, [roomId]: score }));
      }
    }
  }, [roomStates]);


  const rooms = Object.values(roomStates).sort((a, b) => {
    const scoreA = roomScores[a.id] || 0;
    const scoreB = roomScores[b.id] || 0;
    return scoreB - scoreA;
  });

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto">
        <header className="px-4 py-4 flex items-center justify-between">
          <h1 className="text-2xl font-bold">{t("Headquarters")}</h1>
          <div className="flex items-center gap-4">
            <UserControls />
            <LanguageToggle />
            <ModeToggle />
          </div>
        </header>
        <main className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 px-4 pb-10">
          {rooms.length === 0 && (
            <div className="col-span-full text-center py-10 text-neutral-600">
              {t("Waiting for data...")}
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
