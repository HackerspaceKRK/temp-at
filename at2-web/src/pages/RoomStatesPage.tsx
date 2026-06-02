import type { FC } from "react";
import { useTranslation } from "react-i18next";
import { Alerts } from "../components/Alerts";
import RoomCard from "../components/RoomCard";
import { RoomUsageStats } from "../components/RoomUsageStats";
import { scoreRoom, useLiveRoomStates } from "../useLiveRoomStates";

export const RoomStatesPage: FC = () => {
  const { t } = useTranslation();
  const rooms = useLiveRoomStates()
    .slice()
    .sort((left, right) => scoreRoom(right) - scoreRoom(left));

  return (
    <>
      <Alerts />
      <main className="grid grid-cols-1 gap-6 px-4 pb-10 md:grid-cols-2 lg:grid-cols-3">
        {rooms.length === 0 && (
          <div className="col-span-full py-10 text-center text-neutral-600">
            {t("Waiting for data...")}
          </div>
        )}
        {rooms.map((room) => (
          <RoomCard key={room.id} room={room} />
        ))}
        <RoomUsageStats rooms={rooms} />
      </main>
    </>
  );
};