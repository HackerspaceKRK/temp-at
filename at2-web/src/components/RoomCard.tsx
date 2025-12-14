import { useState, type FC } from "react";
import { Thermometer, Droplets, User, SwitchCamera } from "lucide-react";
import type { RoomState, CameraSnapshotEntity } from "../schema";
import { useLocale } from "../locale";
import { resolveImageUrl } from "../config";
import RelayGroupControl from "./RelayGroupControl";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "./ui/tabs";

import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
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
  title: string;
}> = ({ count, title }) => {
  return (
    <div
      className={`flex items-center gap-1 ${
        count > 0 ? "text-red-400" : "text-neutral-300"
      }`}
      title={title}
    >
      <User className="w-4 h-4" />
      <span>{count}</span>
    </div>
  );
};

/* Removed unused RelayBarItem component (was causing TS warning) */

/**
 * Camera snapshot: renders nothing if no images.
 */
const CameraSnapshot: FC<{
  camera: CameraSnapshotEntity | undefined;
  alt: string;
}> = ({ camera, alt }) => {
  const images = camera?.state?.images;
  if (!images || images.length === 0) return null;
  const sorted = [...images].sort((a, b) => a.width - b.width);
  const srcSet = sorted
    .map((i) => `${resolveImageUrl(i.url)} ${i.width}w`)
    .join(", ");
  const largest = sorted[sorted.length - 1];
  return (
    <img
      src={resolveImageUrl(largest.url)}
      srcSet={srcSet}
      sizes="(max-width: 768px) 100vw, 50vw"
      alt={alt}
      className="object-cover max-h-full w-full rounded-b-md"
      loading="lazy"
    />
  );
};


export const RoomCard: FC<{ room: RoomState }> = ({ room }) => {
  const { getName } = useLocale();

  const cameraEntities = room.entities.filter(
    (e) => e.representation === "camera_snapshot"
  ) as CameraSnapshotEntity[];
  const hasPresence = room.entities.some(
    (e) => e.representation === "presence"
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
                title="People count"
              />
            )}
            {room.entities.map((e) =>
              e.representation === "temperature" &&
              typeof e.state === "number" ? (
                <NumericSensorBarItem
                  key={e.id}
                  icon={Thermometer}
                  value={e.state}
                  unit="°C"
                  precision={1}
                  title={getName(e.localized_name, e.id)}
                />
              ) : e.representation === "humidity" &&
                typeof e.state === "number" ? (
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
          />
        </CardAction>
      </CardHeader>
      <div className="relative">
        <CameraSnapshot
          camera={cameraEntities[currentCameraIndex]}
          alt={`Camera snapshot for room ${getName(room.localized_name, room.id)}`}
        />
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
