import type { FC } from "react";
import { useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Thermometer, Droplets, Plug, Bubbles } from "lucide-react";
import { useLocale } from "../locale";
import { useLiveRoomStates } from "../useLiveRoomStates";
import type {
  CoEntity,
  ContactEntity,
  GasEntity,
  RelayEntity,
} from "../schema";
import { NumericSensorBarItem } from "../components/NumericSensorBarItem";
import { ContactSensorGroupItem } from "../components/ContactSensorGroupItem";
import { PeopleCountBarItem } from "../components/PeopleCountBarItem";
import { TabletRelayControls } from "../components/tablet/TabletRelayControls";
import { ReservationsPanel } from "../components/tablet/ReservationsPanel";

export const TabletRoomPage: FC = () => {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const { getName } = useLocale();
  const rooms = useLiveRoomStates();
  const room = rooms.find((r) => r.id === id);

  if (!room) {
    return (
      <main className="flex h-[calc(100vh-62px)] items-center justify-center text-lg text-muted-foreground">
        {t("Waiting for data...")}
      </main>
    );
  }

  const hasPresence = room.entities.some((e) => e.type === "person");
  const coEntity = room.entities.find((e) => e.type === "co") as
    | CoEntity
    | undefined;
  const gasEntity = room.entities.find((e) => e.type === "gas") as
    | GasEntity
    | undefined;
  const contactSensors = room.entities.filter(
    (e) => e.type === "contact",
  ) as ContactEntity[];
  const relays = room.entities.filter(
    (e): e is RelayEntity =>
      e.representation === "light" || e.representation === "fan",
  );

  return (
    <div className="flex h-[calc(100vh-62px)] flex-col gap-4 p-4 md:flex-row">
      {/* Left: sensors + relay controls */}
      <div className="flex w-full flex-col gap-4 overflow-y-auto md:w-2/5">
        <h1 className="text-2xl font-bold tracking-tight text-foreground">
          {getName(room.localized_name, room.id)}
        </h1>

        <div className="flex flex-wrap items-center gap-x-6 gap-y-3 text-lg">
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
            ) : null,
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
            ) : null,
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
            ) : null,
          )}

          {((coEntity && coEntity.state != null) ||
            (gasEntity && gasEntity.state != null)) && (
            <NumericSensorBarItem
              icon={Bubbles}
              value={coEntity?.state}
              unit="ppm"
              title={[
                coEntity ? getName(coEntity.localized_name, coEntity.id) : null,
                gasEntity
                  ? getName(gasEntity.localized_name, gasEntity.id)
                  : null,
              ]
                .filter(Boolean)
                .join(" / ")}
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

        <div className="mt-2">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
            {t("Controls")}
          </h2>
          <TabletRelayControls entities={relays} />
        </div>
      </div>

      {/* Right: reservations */}
      <div className="w-full md:w-3/5">
        <ReservationsPanel roomId={room.id} />
      </div>
    </div>
  );
};
