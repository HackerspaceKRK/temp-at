import type { FC } from "react";
import { useTranslation } from "react-i18next";
import { Link, Navigate } from "react-router-dom";
import { ArrowRight, Box } from "lucide-react";
import { useLiveRoomStates } from "../useLiveRoomStates";
import { useLocale } from "../locale";
import type { PrinterEntity, RoomState } from "../schema";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import {
  PrinterStateBadge,
  PrinterStatusSummary,
  PrinterNotifyButton,
} from "../components/printer/PrinterStatusSummary";
import { AmsPanel } from "../components/printer/AmsPanel";
import { PrinterStreamView } from "../components/printer/PrinterStreamView";

export interface PrinterWithRoom {
  room: RoomState;
  entity: PrinterEntity;
}

// collectPrinters flattens all printer entities out of the live room states.
export function collectPrinters(rooms: RoomState[]): PrinterWithRoom[] {
  return rooms.flatMap((room) =>
    room.entities
      .filter((e): e is PrinterEntity => e.type === "printer")
      .map((entity) => ({ room, entity })),
  );
}

const PrinterCard: FC<PrinterWithRoom> = ({ room, entity }) => {
  const { getName } = useLocale();

  return (
    <Card className="flex flex-col gap-4">
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle className="min-w-0">
            <Link
              to={`/printers/${entity.id}`}
              className="group flex items-center gap-1.5 hover:underline"
            >
              <span className="truncate">{getName(entity.localized_name, entity.id)}</span>
              <ArrowRight className="h-4 w-4 shrink-0 opacity-0 transition-opacity group-hover:opacity-100" />
            </Link>
          </CardTitle>
          <PrinterStateBadge state={entity.state?.state ?? "offline"} />
        </div>
        <p className="text-xs text-muted-foreground">{getName(room.localized_name, room.id)}</p>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <PrinterStreamView entity={entity} />
        <PrinterStatusSummary entity={entity} />
        {entity.state && <AmsPanel state={entity.state} />}
        <PrinterNotifyButton entity={entity} />
      </CardContent>
    </Card>
  );
};

export const PrintersPage: FC = () => {
  const { t } = useTranslation();
  const liveRooms = useLiveRoomStates();
  const printers = collectPrinters(liveRooms);

  if (printers.length === 1) {
    return <Navigate to={`/printers/${printers[0].entity.id}`} replace />;
  }

  return (
    <main className="grid grid-cols-1 gap-6 px-4 pb-10 md:grid-cols-2 xl:grid-cols-3">
      {printers.length === 0 && (
        <div className="col-span-full flex flex-col items-center py-16 text-muted-foreground">
          <Box className="mb-3 h-10 w-10 opacity-30" />
          <p>{t("No printers found")}</p>
        </div>
      )}
      {printers.map(({ room, entity }) => (
        <PrinterCard key={entity.id} room={room} entity={entity} />
      ))}
    </main>
  );
};
