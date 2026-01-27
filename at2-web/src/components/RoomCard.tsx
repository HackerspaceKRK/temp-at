import { useState, type FC } from "react";
import { Thermometer, Droplets, SwitchCamera, Plug, VideoOff, Bubbles } from "lucide-react";
import type { RoomState, CameraSnapshotEntity, CoEntity, GasEntity, ContactEntity } from "../schema";
import { useLocale } from "../locale";
import RelayGroupControl from "./RelayGroupControl";
import CameraSnapshot from "./CameraSnapshot";
import { useAuth } from "../AuthContext";
import { useTranslation } from "react-i18next";
import { NumericSensorBarItem } from "./NumericSensorBarItem";
import { ContactSensorGroupItem } from "./ContactSensorGroupItem";
import { PeopleCountBarItem } from "./PeopleCountBarItem";

import {
  Card,
  CardAction,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "./ui/button";

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
