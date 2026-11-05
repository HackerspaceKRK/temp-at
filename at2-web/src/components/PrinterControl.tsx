import { Fragment, type FC } from "react";
import { Printer3d, ArrowRight } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import type { PrinterEntity, PrinterStateValue } from "../schema";
import { useLocale } from "../locale";
import { Button } from "./ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "./ui/popover";
import {
  STATE_STYLES,
  PrinterStateBadge,
  PrinterStatusSummary,
  PrinterNotifyButton,
} from "./printer/PrinterStatusSummary";

// STATE_PRIORITY ranks states for the shared status dot: the most noteworthy
// state of any printer in the room wins.
const STATE_PRIORITY: PrinterStateValue[] = [
  "failed",
  "printing",
  "paused",
  "finished",
  "idle",
  "offline",
];

function aggregateState(entities: PrinterEntity[]): PrinterStateValue {
  const states = entities.map((e) => e.state?.state ?? "offline");
  return STATE_PRIORITY.find((s) => states.includes(s)) ?? "offline";
}

// PrinterSection is the status block of one printer inside the popover.
const PrinterSection: FC<{ entity: PrinterEntity }> = ({ entity }) => {
  const { t } = useTranslation();
  const { getName } = useLocale();
  return (
    <div className="flex flex-col gap-3">
      {/* Header: name + state badge */}
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm font-semibold">
          {getName(entity.localized_name, entity.id)}
        </span>
        <PrinterStateBadge state={entity.state?.state ?? "offline"} />
      </div>

      <PrinterStatusSummary entity={entity} />
      <PrinterNotifyButton entity={entity} />

      <Link
        to={`/printers/${entity.id}`}
        className="flex items-center justify-end gap-1 text-xs text-muted-foreground hover:text-foreground"
      >
        {t("Details")} <ArrowRight className="h-3.5 w-3.5" />
      </Link>
    </div>
  );
};

// PrinterControl is the single printer button of a room card; its popover
// lists every printer in the room.
export const PrinterControl: FC<{
  entities: PrinterEntity[];
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}> = ({ entities, open, onOpenChange }) => {
  const { t } = useTranslation();
  if (entities.length === 0) return null;

  const style = STATE_STYLES[aggregateState(entities)] ?? STATE_STYLES.offline;

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="relative"
          aria-label={t("Printer status")}
        >
          <Printer3d className="w-5 h-5" />
          <span
            className={`absolute -top-1 -right-1 h-2.5 w-2.5 rounded-full ring-2 ring-background ${style.dot} ${
              style.pulse ? "animate-pulse" : ""
            }`}
          />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        collisionPadding={16}
        className="w-[calc(100vw-2rem)] max-w-[32rem] max-h-[var(--radix-popover-content-available-height)] overflow-y-auto"
      >
        <div className="flex flex-col gap-4">
          {entities.map((e, i) => (
            <Fragment key={e.id}>
              {i > 0 && <hr className="border-border" />}
              <PrinterSection entity={e} />
            </Fragment>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
};

export default PrinterControl;
