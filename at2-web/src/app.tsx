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
import Footer from "./components/Footer";
import { API_URL } from "./config";
import { AuthProvider } from "./AuthContext";
import { AppConfigProvider, useAppConfig } from "./AppConfigContext";
import { useTranslation } from "react-i18next";
import { RoomUsageStats } from "./components/RoomUsageStats";

import { ThemeProvider, useTheme } from "./theme";

import { ModeToggle } from "./components/ModeToggle";
import { LanguageToggle } from "./components/LanguageToggle";
import { Alerts } from "./components/Alerts";
import UserControls from "./components/UserControls";

export function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <AppConfigProvider>
          <AppContent />
        </AppConfigProvider>
      </AuthProvider>
    </ThemeProvider>
  );
}

const AppContent: FC = () => {
  const { t } = useTranslation();
  const { theme } = useTheme();
  const { config } = useAppConfig();
  const branding = config?.branding;
  const [roomStates, setRoomStates] = useState<{ [key: string]: RoomState }>(
    {},
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
          (e) => e.type === "camera_snapshot",
        ).length;
        score += snapCount * 20;
        if (snapCount == 0) {
          score -= 1000;
        }

        score += room.people_count * 200;
        const lights = room.entities.filter(
          (e) => e.representation === "light",
        );
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

  const isDarkMode =
    theme === "dark" ||
    (theme === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  const logoUrl =
    isDarkMode && branding?.logo_dark_url
      ? branding.logo_dark_url
      : branding?.logo_url;

  return (
    <div className="min-h-screen flex flex-col bg-background text-foreground">
      <div className="w-full flex-grow">
        <header className="px-4 py-4 flex flex-col sm:flex-row items-center justify-between gap-4">
          <a
            href={branding?.logo_link_url || "/"}
            className="flex items-center"
            target={
              branding?.logo_link_url?.startsWith("http") ? "_blank" : undefined
            }
            rel={
              branding?.logo_link_url?.startsWith("http")
                ? "noopener noreferrer"
                : undefined
            }
          >
            {logoUrl ? (
              <img
                src={logoUrl}
                alt={branding?.logo_alt || t("Logo")}
                className="max-h-[70px] w-auto transition-transform hover:scale-105"
              />
            ) : (
              <h1 className="text-2xl font-bold">{t("Headquarters")}</h1>
            )}
          </a>
          <div className="flex items-center gap-2 sm:gap-4 flex-wrap justify-center">
            <div className="flex items-center gap-2">
              <LanguageToggle />
              <ModeToggle />
            </div>
            <div className="w-px h-8 bg-border mx-2" />
            <UserControls />
          </div>
        </header>
        <Alerts />
        <main className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 px-4 pb-10">
          {rooms.length === 0 && (
            <div className="col-span-full text-center py-10 text-neutral-600">
              {t("Waiting for data...")}
            </div>
          )}
          {rooms.map((room) => (
            <RoomCard key={room.id} room={room} />
          ))}
          <RoomUsageStats rooms={rooms} />
        </main>
      </div>
      <Footer />
    </div>
  );
};
