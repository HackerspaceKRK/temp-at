import { useState, type FC } from "react";
import { Phone, Delete } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocale } from "../locale";
import { useLiveRoomStates } from "../useLiveRoomStates";
import { kioskMakeCall } from "../lib/kioskApi";

const KEYPAD_KEYS = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "*", "0", "#"];

export const TabletPhonePage: FC = () => {
  const { t } = useTranslation();
  const { getName } = useLocale();
  const rooms = useLiveRoomStates();
  const [number, setNumber] = useState("");

  const phoneRooms = rooms
    .filter((room) => !!room.voip_phone_number)
    .sort((a, b) =>
      getName(a.localized_name, a.id).localeCompare(
        getName(b.localized_name, b.id),
      ),
    );

  const dial = (dest: string) => {
    const target = dest.trim();
    if (!target) return;
    kioskMakeCall(target);
  };

  return (
    <div className="flex h-[calc(100vh-62px)] flex-col gap-4 p-4 md:flex-row">
      {/* Left: rooms with a phone number */}
      <div className="flex w-full flex-col gap-2 overflow-y-auto md:w-1/2">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          {t("Rooms")}
        </h2>
        {phoneRooms.length === 0 ? (
          <div className="text-sm text-muted-foreground">
            {t("No rooms with a phone number")}
          </div>
        ) : (
          phoneRooms.map((room) => (
            <button
              key={room.id}
              type="button"
              onClick={() => dial(room.voip_phone_number!)}
              className="flex items-center justify-between gap-4 rounded-xl border-2 border-border bg-card px-5 py-4 text-left transition-colors active:scale-[0.99] hover:border-emerald-500"
            >
              <span className="text-lg font-semibold">
                {getName(room.localized_name, room.id)}
              </span>
              <span className="flex items-center gap-2 font-mono text-lg text-muted-foreground">
                <Phone className="size-5" />
                {room.voip_phone_number}
              </span>
            </button>
          ))
        )}
      </div>

      {/* Right: manual keypad */}
      <div className="flex w-full flex-col items-center gap-4 md:w-1/2">
        <div className="flex h-16 w-full max-w-sm items-center justify-center rounded-xl border-2 border-border bg-card px-4 text-3xl font-bold tabular-nums tracking-widest">
          {number || <span className="text-muted-foreground">—</span>}
        </div>

        <div className="grid w-full max-w-sm grid-cols-3 gap-3">
          {KEYPAD_KEYS.map((key) => (
            <button
              key={key}
              type="button"
              onClick={() => setNumber((prev) => prev + key)}
              className="flex aspect-square items-center justify-center rounded-xl border-2 border-border bg-card text-3xl font-semibold transition-colors active:scale-[0.97] hover:bg-muted"
            >
              {key}
            </button>
          ))}
        </div>

        <div className="flex w-full max-w-sm items-center gap-3">
          <button
            type="button"
            onClick={() => dial(number)}
            disabled={!number.trim()}
            className="flex h-16 flex-1 items-center justify-center gap-2 rounded-xl bg-emerald-600 text-xl font-bold text-white transition-colors active:scale-[0.99] hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <Phone className="size-6" />
            {t("Call")}
          </button>
          <button
            type="button"
            onClick={() => setNumber((prev) => prev.slice(0, -1))}
            disabled={!number}
            className="flex h-16 w-16 items-center justify-center rounded-xl border-2 border-border bg-card transition-colors active:scale-[0.97] hover:bg-muted disabled:opacity-40"
            aria-label={t("Backspace")}
          >
            <Delete className="size-6" />
          </button>
        </div>
      </div>
    </div>
  );
};
