import type { FunctionalComponent } from "preact";
import { useState } from "preact/hooks";
import { Lightbulb, Fan, Thermometer, Droplets, User } from "lucide-preact";
import type { RoomState, CameraSnapshotEntity } from "../schema";
import { useLocale } from "../locale";
import { resolveImageUrl } from "../config";

/**
 * Minimal numeric sensor item.
 * Renders nothing if value is missing.
 */
const NumericSensorBarItem: FunctionalComponent<{
  icon: FunctionalComponent<any>;
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
const PeopleCountBarItem: FunctionalComponent<{
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

/**
 * Relay (light / fan) icon.
 * Hidden when state is neither ON nor OFF (unknown).
 */
const RelayBarItem: FunctionalComponent<{
  representation: "light" | "fan";
  on: boolean | null;
  title: string;
}> = ({ representation, on, title }) => {
  if (on === null) return null;
  if (representation === "light") {
    return (
      <Lightbulb
        className={`w-5 h-5 ${on ? "text-yellow-400" : "text-neutral-400"}`}
        title={title}
      />
    );
  }
  if (representation === "fan") {
    return (
      <Fan
        className={`w-5 h-5 ${on ? "text-sky-400 spin-slow" : "text-neutral-400"}`}
        title={title}
      />
    );
  }
  return null;
};

/**
 * Camera snapshot: renders nothing if no images.
 */
const CameraSnapshot: FunctionalComponent<{
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
export const RoomCard: FunctionalComponent<{ room: RoomState }> = ({
  room,
}) => {
  const { getName } = useLocale();
  const [activeCameraIdx, setActiveCameraIdx] = useState(0);

  // Collect camera entities only for tab selection / active camera logic.
  const cameraEntities = room.entities.filter(
    (e) => e.representation === "camera_snapshot",
  ) as CameraSnapshotEntity[];
  const activeCamera = cameraEntities[activeCameraIdx];
  const hasPresence = room.entities.some(
    (e) => e.representation === "presence",
  );

  return (
    <div className="rounded-lg border border-neutral-300 bg-neutral-50 shadow-sm overflow-hidden flex flex-col">
      {/* Header */}
      <div className="border-b border-neutral-200 bg-neutral-100">
        <div className="flex items-center justify-between px-3 py-2">
          <h2 className="font-semibold text-neutral-800">
            {getName(room.localized_name, room.id)}
          </h2>
        </div>
        {cameraEntities.length > 1 && (
          <div className="flex gap-1 px-2 pb-2">
            {cameraEntities.map((cam, idx) => (
              <button
                key={cam.id}
                onClick={() => setActiveCameraIdx(idx)}
                className={`text-xs px-2 py-1 rounded border transition-colors ${
                  idx === activeCameraIdx
                    ? "bg-neutral-800 text-white border-neutral-800"
                    : "bg-white hover:bg-neutral-200 text-neutral-700 border-neutral-300"
                }`}
              >
                {getName(cam.localized_name, cam.id)}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Snapshot area */}
      <div className="relative aspect-video bg-neutral-900 flex items-center justify-center">
        <CameraSnapshot
          camera={activeCamera}
          alt={
            activeCamera
              ? getName(activeCamera.localized_name, activeCamera.id)
              : "Camera snapshot"
          }
        />

        {/* Overlay bar at top */}
        <div className="absolute top-0 left-0 right-0 bg-black/50 backdrop-blur-sm text-white text-xs flex items-center justify-between px-3 py-1">
          {/* Environment metrics: map directly */}
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
              ) : null,
            )}
          </div>

          {/* Relays */}
          <div className="flex items-center gap-3">
            {room.entities.map((e) =>
              e.representation === "light" || e.representation === "fan" ? (
                <RelayBarItem
                  key={e.id}
                  representation={e.representation}
                  on={
                    e.state === "ON" ? true : e.state === "OFF" ? false : null
                  }
                  title={getName(e.localized_name, e.id)}
                />
              ) : null,
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default RoomCard;
