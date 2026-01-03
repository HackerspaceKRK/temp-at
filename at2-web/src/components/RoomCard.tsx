import { useState, useEffect, type FC } from "react";
import { Thermometer, Droplets, User, SwitchCamera, Zap } from "lucide-react";
import type { RoomState, CameraSnapshotEntity } from "../schema";
import { useLocale } from "../locale";
import RelayGroupControl from "./RelayGroupControl";
import CameraSnapshot from "./CameraSnapshot";
import { useAuth } from "../AuthContext";
import { useTranslation } from "react-i18next";
// import { Tabs, TabsList, TabsTrigger, TabsContent } from "./ui/tabs";

import {
  Card,
  CardAction,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "./ui/button";

/**
 * Minimal numeric sensor item.
 * Renders nothing if value is missing.
 */
const NumericSensorBarItem: FC<{
  icon: FC<any>;
  value: number | null;
  unit: string;
  title: string;
  precision?: number;
}> = ({ icon: Icon, value, unit, title, precision = 1 }) => {
  if (value === null || isNaN(value)) return null;
  return (
    <div className="flex items-center gap-1" title={title}>
      <Icon className="w-4 h-4" />
      <span>
        {value.toFixed(precision)}
        {unit}
      </span>
    </div>
  );
};

/**
 * People count item.
 * Always renders (since count is always present); red when count > 0.
 */
const PeopleCountBarItem: FC<{
  count: number;
  lastSeen?: string | null;
  title: string;
}> = ({ count, lastSeen, title }) => {
  const [now, setNow] = useState(new Date());
  const { t } = useTranslation();

  useEffect(() => {
    const interval = setInterval(() => {
      setNow(new Date());
    }, 10000);
    return () => clearInterval(interval);
  }, []);

  if (count === 0 && lastSeen) {
    const date = new Date(lastSeen);
    const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

    let timeString = "";
    if (diffInSeconds < 60) {
      timeString = `${Math.max(0, diffInSeconds)}s`;
    } else if (diffInSeconds < 3600) {
      const mins = Math.floor(diffInSeconds / 60);
      timeString = `${mins}m`;
    } else if (diffInSeconds < 86400) {
      const hours = Math.floor(diffInSeconds / 3600);
      timeString = `${hours}h`;
    } else {
      const days = Math.floor(diffInSeconds / 86400);
      timeString = `${days}d`;
    }

    return (
      <div
        className="flex items-center gap-1 text-neutral-300"
        title={t("Last seen: {{date}}", { date: date.toLocaleString() })}
      >
        <User className="w-4 h-4" />
        <span className="text-xs">{t("{{time}} ago", { time: timeString })}</span>
      </div>
    );
  }

  return (
    <div
      className={`flex items-center gap-1 ${count > 0 ? "text-red-400" : "text-neutral-300"
        }`}
      title={title}
    >
      <User className="w-4 h-4" />
      <span>{count}</span>
    </div>
  );
};

/* Removed unused RelayBarItem component (was causing TS warning) */


export const RoomCard: FC<{ room: RoomState }> = ({ room }) => {
  const { getName } = useLocale();
  const { t } = useTranslation();

  const { user, login } = useAuth();
  const cameraEntities = room.entities.filter(
    (e) => e.type === "camera_snapshot"
  ) as CameraSnapshotEntity[];
  const hasPresence = room.entities.some(
    (e) => e.type === "person"
  );

  const [currentCameraIndex, setCurrentCameraIndex] = useState(0);

  return (
    <Card className="gap-4 pb-0">
      <CardHeader className="items-baseline ">
        <CardTitle className="block  self-center">
          {getName(room.localized_name, room.id)}
        </CardTitle>
        <CardDescription>
          <div className="flex items-center gap-3">
            {hasPresence && (
              <PeopleCountBarItem
                count={room.people_count}
                lastSeen={room.latest_person_detected_at}
                title={t("People count")}
              />
            )}
            {room.entities.map((e) =>
              e.type === "temperature" &&
                typeof e.state === "number" ? (
                <NumericSensorBarItem
                  key={e.id}
                  icon={Thermometer}
                  value={e.state}
                  unit="°C"
                  precision={1}
                  title={getName(e.localized_name, e.id)}
                />
              ) : e.type === "humidity" &&
                typeof e.state === "number" ? (
                <NumericSensorBarItem
                  key={e.id}
                  icon={Droplets}
                  value={e.state}
                  unit="%"
                  precision={0}
                  title={getName(e.localized_name, e.id)}
                />
              ) : e.type === "power_usage" &&
                typeof e.state === "number" ? (
                <NumericSensorBarItem
                  key={e.id}
                  icon={Zap}
                  value={e.state}
                  unit="W"
                  precision={1}
                  title={getName(e.localized_name, e.id)}
                />
              ) : null
            )}
          </div>
        </CardDescription>
        <CardAction className="flex gap-2 self-center">
          <RelayGroupControl
            entities={
              room.entities.filter(
                (e): e is any =>
                  e.representation === "light" || e.representation === "fan"
              ) as any
            }
            kind="light"
            roomId={room.id}
            roomName={getName(room.localized_name, room.id)}
          />
          <RelayGroupControl
            entities={
              room.entities.filter(
                (e): e is any =>
                  e.representation === "light" || e.representation === "fan"
              ) as any
            }
            kind="fan"
            roomId={room.id}
            roomName={getName(room.localized_name, room.id)}
          />
        </CardAction>
      </CardHeader>
      <div className="relative">
        {user ? (
          <CameraSnapshot
            images={cameraEntities[currentCameraIndex]?.state?.images}
            alt={t("Camera snapshot for room {{room}}", { room: getName(room.localized_name, room.id) })}
            className="rounded-b-md"
          />
        ) : (
          <div className="relative overflow-hidden rounded-b-md aspect-video bg-neutral-900 group">
            {cameraEntities[currentCameraIndex]?.state?.low_res_preview && (
              <img
                src={cameraEntities[currentCameraIndex]?.state?.low_res_preview}
                alt="Blurred preview"
                className="absolute inset-0 w-full h-full object-cover blur-xl scale-125"
              />
            )}
            <div className="absolute inset-0 flex flex-col items-center justify-center bg-black/40 backdrop-blur-[2px] transition-all">
              <p className="text-white text-sm mb-3 font-semibold drop-shadow-md">
                {t("Log in to see snapshots")}
              </p>
              <Button
                size="sm"
                variant="outline"
                onClick={login}
                className="shadow-lg hover:scale-105 transition-transform"
              >
                {t("Log In")}
              </Button>
            </div>
          </div>
        )}
        {cameraEntities.length > 1 && (
          <Button
            className="absolute bottom-4 right-4"
            variant={"outline"}
            size={"icon"}
            onClick={() => {
              setCurrentCameraIndex(
                (currentCameraIndex + 1) % cameraEntities.length
              );
            }}
          >
            <SwitchCamera></SwitchCamera>
          </Button>
        )}
      </div>
    </Card>
  );
};

export default RoomCard;
