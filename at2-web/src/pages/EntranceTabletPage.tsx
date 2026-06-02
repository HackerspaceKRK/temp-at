import type { FC } from "react";
import { useTranslation } from "react-i18next";
import { RoomStatusTabletTile } from "../components/RoomStatusTabletTile";
import { useLiveRoomStates } from "../useLiveRoomStates";

export const EntranceTabletPage: FC = () => {
  const { t } = useTranslation();
  const rooms = useLiveRoomStates().filter(
    (room) => !room.exclude_from_entrance_tablet,
  );

  return (
    <main className="mx-auto flex h-[calc(100vh-78px)] w-full max-w-[1280px] flex-col px-4 py-4">
      {rooms.length === 0 ? (
        <div className="flex flex-1 items-center justify-center rounded-3xl border border-dashed border-border bg-card text-lg text-muted-foreground">
          {t("Waiting for data...")}
        </div>
      ) : (
        <div className="grid h-full flex-1 auto-rows-fr grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {rooms.map((room) => (
            <RoomStatusTabletTile key={room.id} room={room} />
          ))}
        </div>
      )}
    </main>
  );
};