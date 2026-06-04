import type { FC } from "react";
import { CalendarClock } from "lucide-react";

interface ReservationsPanelProps {
  roomId: string;
}

/**
 * ReservationsPanel — shows upcoming reservations for a room on the tablet
 * room page.
 *
 * TODO: wire up to the reservations backend. For now this is a placeholder so
 * the room page layout is complete.
 */
export const ReservationsPanel: FC<ReservationsPanelProps> = ({ roomId }) => {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border bg-card/50 p-6 text-center text-muted-foreground">
      <CalendarClock className="h-10 w-10 opacity-40" />
      <div className="text-lg font-semibold">Reservations</div>
      <div className="text-sm">
        TODO: reservations for room <span className="font-mono">{roomId}</span>{" "}
        will be shown here.
      </div>
    </div>
  );
};

export default ReservationsPanel;
