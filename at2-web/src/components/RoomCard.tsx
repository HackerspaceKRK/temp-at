import { useState, useEffect, type FC } from "react";
import { Thermometer, Droplets, User, SwitchCamera, Plug, VideoOff, Grid2X2, Bubbles } from "lucide-react";
import type { RoomState, CameraSnapshotEntity, CoEntity, GasEntity, ContactEntity } from "../schema";
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Button } from "./ui/button";
import { cn } from "@/lib/utils";

/**
 * Minimal numeric sensor item.
 * Renders nothing if value is missing.
 */
/**
 * Numeric sensor item.
 * Supports single or combined values (e.g. CO / Gas).
 * If isAlarm is true, renders with red pulsating style.
 */
const NumericSensorBarItem: FC<{
  icon: FC<any>;
  value?: number | null;
  unit?: string;
  title: string;
  precision?: number;
  // Optional secondary value (e.g. for Gas when combined with CO)
  secondaryValue?: number | null;
  secondaryUnit?: string;
  secondaryPrecision?: number;
  isAlarm?: boolean;
}> = ({
  icon: Icon,
  value,
  unit,
  title,
  precision = 1,
  secondaryValue,
  secondaryUnit,
  secondaryPrecision = 1,
  isAlarm,
}) => {
    const hasValue = value !== null && value !== undefined && !isNaN(value);
    const hasSecondary =
      secondaryValue !== null &&
      secondaryValue !== undefined &&
      !isNaN(secondaryValue);

    if (!hasValue && !hasSecondary) return null;

    return (
      <div
        className={cn(
          "flex items-center gap-1 transition-all",
          isAlarm ? "text-red-500 font-bold animate-alarm" : ""
        )}
        title={title}
      >
        <Icon className="w-4 h-4" />
        <span>
          {hasValue && (
            <>
              {value!.toFixed(precision)}
              <span className="ml-[1px]">{unit}</span>
            </>
          )}
          {hasValue && hasSecondary && <span> / </span>}
          {hasSecondary && (
            <>
              {secondaryValue!.toFixed(secondaryPrecision)}
              <span className="ml-[1px]">{secondaryUnit}</span>
            </>
          )}
        </span>
      </div>
    );
  };

/**
 * Contact sensor group item.
 * Aggregates state of all contact sensors in the room.
 */
/**
 * Contact sensor group item.
 * Aggregates state of all contact sensors in the room.
 */
const ContactSensorGroupItem: FC<{
  sensors: ContactEntity[];
}> = ({ sensors }) => {
  const { getName } = useLocale();
  const { t } = useTranslation();

  if (!sensors || sensors.length === 0) return null;

  const openSensors = sensors.filter((s) => s.state === false);
  const unknownSensors = sensors.filter((s) => s.state === null);
  const allClosed = openSensors.length === 0 && unknownSensors.length === 0;

  // Status text and style
  let statusText = "OK";
  let isDanger = false;

  if (unknownSensors.length > 0) {
    statusText = "?";
    // User requested "?" and red if any is open, but what if only null?
    // "If any one of them has a state of 'null' then display '?'"
    // "If any one of them is open ... also make the item red"
  } else if (!allClosed) {
    statusText = t("{{count}} open", { count: openSensors.length });
    isDanger = true;
  }

  // If any is open, it should be red regardless of unknowns? 
  // Requirement: "If any one of them is open then display ... also make the item red"
  if (openSensors.length > 0) {
    isDanger = true;
  }

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <div
            className={cn(
              "flex items-center gap-1 cursor-pointer",
              isDanger ? "text-red-500 font-bold" : "text-neutral-300"
            )}
          >
            <Grid2X2 className="w-4 h-4" />
            <span>{statusText}</span>
          </div>
        </TooltipTrigger>
        <TooltipContent side="bottom">
          <div className="flex flex-col gap-1">
            <p className="font-semibold text-xs mb-1">{t("Contact sensors")}</p>
            {sensors.map((s) => (
              <div key={s.id} className="flex justify-between gap-4 text-xs">
                <span>{getName(s.localized_name, s.id)}:</span>
                <span
                  className={cn(
                    "font-bold",
                    s.state === false ? "text-red-500" : ""
                  )}
                >
                  {s.state === true
                    ? t("Closed")
                    : s.state === false
                      ? t("Open")
                      : t("Unknown")}
                </span>
              </div>
            ))}
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
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

  // Collect Sensors
  const coEntity = room.entities.find((e) => e.type === "co") as CoEntity | undefined;
  const gasEntity = room.entities.find((e) => e.type === "gas") as GasEntity | undefined;
  const contactSensors = room.entities.filter((e) => e.type === "contact") as ContactEntity[];

  const [currentCameraIndex, setCurrentCameraIndex] = useState(0);

  return (
    <Card className="gap-4 pb-0">
      <CardHeader className="items-center pb-0">
        <CardTitle>
          {getName(room.localized_name, room.id)}
        </CardTitle>
        <CardAction className="row-span-1 flex gap-2">
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
        <CardDescription className="col-span-full">
          <div className="flex items-center gap-x-4 gap-y-2 flex-wrap">
            {hasPresence && (
              <PeopleCountBarItem
                count={room.people_count}
                lastSeen={room.latest_person_detected_at}
                title={t("People count")}
              />
            )}

            {room.entities.map((e) =>
              e.type === "temperature" && typeof e.state === "number" ? (
                <NumericSensorBarItem
                  key={e.id}
                  icon={Thermometer}
                  value={e.state}
                  unit="°C"
                  precision={1}
                  title={getName(e.localized_name, e.id)}
                />
              ) : null
            )}

            {room.entities.map((e) =>
              e.type === "humidity" && typeof e.state === "number" ? (
                <NumericSensorBarItem
                  key={e.id}
                  icon={Droplets}
                  value={e.state}
                  unit="%"
                  precision={0}
                  title={getName(e.localized_name, e.id)}
                />
              ) : null
            )}




            {room.entities.map((e) =>
              e.type === "power_usage" && typeof e.state === "number" ? (
                <NumericSensorBarItem
                  key={e.id}
                  icon={Plug}
                  value={e.state}
                  unit="W"
                  precision={0}
                  title={getName(e.localized_name, e.id)}
                />
              ) : null
            )}


            {((coEntity && coEntity.state !== null && coEntity.state !== undefined) || (gasEntity && gasEntity.state !== null && gasEntity.state !== undefined)) && (
              <NumericSensorBarItem
                icon={Bubbles}
                value={coEntity?.state}
                unit="ppm"
                title={[
                  coEntity ? getName(coEntity.localized_name, coEntity.id) : null,
                  gasEntity ? getName(gasEntity.localized_name, gasEntity.id) : null
                ].filter(Boolean).join(" / ")}
                precision={0}
                secondaryValue={gasEntity?.state}
                secondaryUnit="LEL"
                secondaryPrecision={0}
                isAlarm={(coEntity?.state || 0) > 0 || (gasEntity?.state || 0) > 0}
              />
            )}

            {contactSensors.length > 0 && (
              <ContactSensorGroupItem sensors={contactSensors} />
            )}
          </div>
        </CardDescription>
      </CardHeader>
      <div className="relative">
        {cameraEntities.length > 0 ? (
          user ? (
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
          )
        ) : (
          <div className="relative flex flex-col items-center justify-center aspect-video bg-neutral-900 rounded-b-md text-muted-foreground border-t border-border/50">
            <VideoOff className="w-12 h-12 mb-2 opacity-20" />
            <p className="text-sm font-medium opacity-50">
              {t("No cameras in this room")}
            </p>
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
