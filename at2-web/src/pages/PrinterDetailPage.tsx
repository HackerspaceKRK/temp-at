import type { FC } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { useLiveRoomStates } from "../useLiveRoomStates";
import { useLocale } from "../locale";
import { collectPrinters } from "./PrintersPage";
import {
  PrinterStateBadge,
  PrinterStatusSummary,
  PrinterExtendedStats,
  PrinterHmsList,
  PrinterNotifyButton,
} from "../components/printer/PrinterStatusSummary";
import { AmsPanel } from "../components/printer/AmsPanel";
import { PrinterStreamView } from "../components/printer/PrinterStreamView";

export const PrinterDetailPage: FC = () => {
  const { t } = useTranslation();
  const { getName } = useLocale();
  const liveRooms = useLiveRoomStates();
  const printers = collectPrinters(liveRooms);
  // Splat param: printer ids may contain slashes (e.g. "bambu/x1c").
  const printerId = useParams()["*"] ?? "";

  const printer = printers.find(({ entity }) => entity.id === printerId);

  return (
    <main className="mx-auto flex w-full max-w-3xl flex-col gap-4 px-4 pb-10">
      {printers.length > 1 && (
        <Link
          to="/printers"
          className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" /> {t("All printers")}
        </Link>
      )}

      {!printer ? (
        <p className="py-16 text-center text-muted-foreground">
          {liveRooms.length === 0 ? t("Waiting for data...") : t("Printer not found")}
        </p>
      ) : (
        <>
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              <h1 className="truncate text-xl font-bold">
                {getName(printer.entity.localized_name, printer.entity.id)}
              </h1>
              <p className="text-sm text-muted-foreground">
                {getName(printer.room.localized_name, printer.room.id)}
              </p>
            </div>
            <PrinterStateBadge state={printer.entity.state?.state ?? "offline"} />
          </div>

          <PrinterStreamView entity={printer.entity} />
          <PrinterStatusSummary entity={printer.entity} />
          {printer.entity.state && (
            <>
              <PrinterHmsList state={printer.entity.state} />
              <h2 className="mt-2 text-sm font-semibold text-muted-foreground">
                {t("Details")}
              </h2>
              <PrinterExtendedStats state={printer.entity.state} />
              <h2 className="mt-2 text-sm font-semibold text-muted-foreground">
                {t("Filament")}
              </h2>
              <AmsPanel state={printer.entity.state} />
            </>
          )}
          <PrinterNotifyButton entity={printer.entity} />
        </>
      )}
    </main>
  );
};
