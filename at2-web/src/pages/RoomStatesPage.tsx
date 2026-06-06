import { useMemo, type FC } from "react";
import { useTranslation } from "react-i18next";
import { Alerts } from "../components/Alerts";
import RoomCard from "../components/RoomCard";
import { RoomUsageStats } from "../components/RoomUsageStats";
import { scoreRoom, useLiveRoomStates } from "../useLiveRoomStates";

export const RoomStatesPage: FC = () => {
  const { t } = useTranslation();
  const liveRooms = useLiveRoomStates();

  // The room order is determined by a heuristic score, but the score depends on
  // live data (people count, lights, snapshots) that changes constantly. If we
  // re-sorted on every update the cards would jump around. Instead we lock the
  // order to a list of room ids and only recompute it when the *set* of rooms
  // changes (e.g. rooms streaming in on initial load) — not when their states
  // change. The cards themselves still render fresh data every render.
  const orderKey = [...new Set(liveRooms.map((room) => room.id))]
    .sort()
    .join(",");
  const orderedIds = useMemo(
    () =>
      liveRooms
        .slice()
        .sort((left, right) => scoreRoom(right) - scoreRoom(left))
        .map((room) => room.id),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [orderKey],
  );

  const roomsById = new Map(liveRooms.map((room) => [room.id, room]));
  const rooms = orderedIds
    .map((id) => roomsById.get(id))
    .filter((room): room is NonNullable<typeof room> => room !== undefined);

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