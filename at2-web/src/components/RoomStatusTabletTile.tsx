import { useEffect, useState, type FC, type ReactNode } from "react";
import { Fan, Grid2X2, Lightbulb, User } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocale } from "../locale";
import type { ContactEntity, RelayEntity, RoomState } from "../schema";
import { Card } from "./ui/card";

function formatElapsedTime(timestamp: string | null, now: Date): string | null {
  if (!timestamp) {
    return null;
  }

  const parsed = new Date(timestamp);
  const diffInSeconds = Math.max(
    0,
    Math.floor((now.getTime() - parsed.getTime()) / 1000),
  );

  if (diffInSeconds < 60) {
    return `${diffInSeconds}s`;
  }
  if (diffInSeconds < 3600) {
    return `${Math.floor(diffInSeconds / 60)}m`;
  }
  if (diffInSeconds < 86400) {
    return `${Math.floor(diffInSeconds / 3600)}h`;
  }

  return `${Math.floor(diffInSeconds / 86400)}d`;
}

type PieceProps = {
  positionClassName: string;
  stateClassName: string;
  icon: ReactNode;
  subtitle: string;
};

const Piece: FC<PieceProps> = ({
  positionClassName,
  stateClassName,
  icon,
  subtitle,
}) => (
  <div
    className={`${positionClassName} ${stateClassName} flex h-full min-h-0 flex-col items-center justify-center gap-1 overflow-hidden rounded-xl border p-2 sm:gap-2 sm:p-3`}
  >
    <div>{icon}</div>
    <div className="break-words px-1 text-center text-[11px] font-semibold leading-tight sm:text-xs">
      {subtitle}
    </div>
  </div>
);

export const RoomStatusTabletTile: FC<{ room: RoomState }> = ({ room }) => {
  const { t } = useTranslation();
  const { getName } = useLocale();
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      setNow(new Date());
    }, 30000);

    return () => window.clearInterval(intervalId);
  }, []);

  const relays = room.entities.filter(
    (entity): entity is RelayEntity =>
      entity.type === "relay" ||
      entity.representation === "light" ||
      entity.representation === "fan",
  );
  const contactSensors = room.entities.filter(
    (entity): entity is ContactEntity => entity.representation === "contact",
  );

  const hasPresence = room.entities.some(
    (entity) => entity.representation === "presence" || entity.type === "person",
  );

  const totalLights = relays.filter(
    (entity) => entity.representation === "light",
  ).length;
  const totalFans = relays.filter(
    (entity) => entity.representation === "fan",
  ).length;

  const lightsOn = relays.filter(
    (entity) => entity.representation === "light" && entity.state === "ON",
  ).length;
  const fansOn = relays.filter(
    (entity) => entity.representation === "fan" && entity.state === "ON",
  ).length;
  const windowsOpen = contactSensors.filter((entity) => entity.state === false).length;

  const occupied = room.people_count > 0;
  const elapsed = formatElapsedTime(room.latest_person_detected_at, now);

  const mutedProblemClass = occupied
    ? "border-4 border-rose-300 bg-transparent text-rose-300"
    : "border-rose-600 bg-rose-600 text-white";

  const peopleSubtitle = occupied
    ? t("People inside: {{count}}", { count: room.people_count })
    : elapsed
      ? t("Last seen {{time}} ago", { time: elapsed })
      : t("No recent presence");

  return (
    <Card className="h-full min-h-0 gap-2 border-border bg-card p-3 shadow-sm sm:gap-3 sm:p-4">
      <div className="shrink-0 text-sm font-semibold tracking-tight text-foreground sm:text-base">
        {getName(room.localized_name, room.id)}
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-2 grid-rows-2 gap-2">
        {hasPresence && (
          <Piece
            positionClassName="col-start-1 row-start-1"
            stateClassName={
              occupied
                ? "border-4 border-emerald-500 bg-transparent text-emerald-400"
                : "border-border bg-muted text-muted-foreground"
            }
            icon={<User className="size-8 sm:size-10" />}
            subtitle={peopleSubtitle}
          />
        )}

        {contactSensors.length > 0 && (
          <Piece
            positionClassName="col-start-2 row-start-1"
            stateClassName={
              windowsOpen > 0
                ? mutedProblemClass
                : "border-4 border-emerald-500 bg-transparent text-emerald-400"
            }
            icon={<Grid2X2 className="size-8 sm:size-10" />}
            subtitle={windowsOpen > 0 ? t("Open") : t("Closed")}
          />
        )}

        {totalLights > 0 && (
          <Piece
            positionClassName="col-start-1 row-start-2"
            stateClassName={
              lightsOn > 0
                ? mutedProblemClass
                : "border-4 border-emerald-500 bg-transparent text-emerald-400"
            }
            icon={<Lightbulb className="size-8 sm:size-10" />}
            subtitle={
              lightsOn > 0
                ? t("Lights on: {{count}}", { count: lightsOn })
                : t("Disabled")
            }
          />
        )}

        {totalFans > 0 && (
          <Piece
            positionClassName="col-start-2 row-start-2"
            stateClassName={
              fansOn > 0
                ? mutedProblemClass
                : "border-4 border-emerald-500 bg-transparent text-emerald-400"
            }
            icon={<Fan className="size-8 sm:size-10" />}
            subtitle={
              fansOn > 0
                ? t("Fans on: {{count}}", { count: fansOn })
                : t("Disabled")
            }
          />
        )}
      </div>
    </Card>
  );
};
