import type { FC } from "react";
import { Thermometer, Droplets, User } from "lucide-react";
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
} from "@/components/ui/card"

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
      className="object-cover w-full h-full"
      loading="lazy"
    />
  );
};

/**
 * Room card.
 * Directly maps entities to bar items; no intermediate arrays.
 * No placeholders are shown for missing data.
 */
export const RoomCard: FC<{ room: RoomState }> = ({ room }) => {
  const { getName } = useLocale();

  const cameraEntities = room.entities.filter(
    (e) => e.representation === "camera_snapshot"
  ) as CameraSnapshotEntity[];
  const hasPresence = room.entities.some(
    (e) => e.representation === "presence"
  );

  return (
    <Card className="gap-4 py-4">
      <CardHeader>
        <CardTitle>{getName(room.localized_name, room.id)}</CardTitle>
      </CardHeader>
      {/* Metrics bar above snapshot */}
      <div className="flex items-center justify-between px-3  text-xs">
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
        <div className="flex items-center gap-3">
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
        </div>
      </div>
      <Tabs
        defaultValue={cameraEntities[0]?.id}
      >
        {/* Snapshot area */}
        <div className="aspect-video bg-neutral-900 dark:bg-neutral-900 flex items-center justify-center">
          {cameraEntities.map((cam) => (
            <TabsContent
              key={cam.id}
              value={cam.id}
              className="w-full h-full p-0 m-0"
            >
              <CameraSnapshot
                camera={cam}
                alt={getName(cam.localized_name, cam.id)}
              />
            </TabsContent>
          ))}
        </div>
        {/* Camera tabs inside card */}
        <div className="flex px-3">
          {cameraEntities.length > 1 && (
            <TabsList>
              {cameraEntities.map((cam) => (
                <TabsTrigger key={cam.id} value={cam.id}>
                  {getName(cam.localized_name, cam.id)}
                </TabsTrigger>
              ))}
            </TabsList>
          )}
        </div>
      </Tabs>
    </Card>
  );
};

export default RoomCard;
