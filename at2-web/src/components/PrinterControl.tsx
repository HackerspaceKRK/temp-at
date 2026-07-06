import type { FC } from "react";
import { Box, ArrowRight } from "lucide-react";
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

export const PrinterControl: FC<{
  entity: PrinterEntity;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}> = ({ entity, open, onOpenChange }) => {
  const { t } = useTranslation();
  const { getName } = useLocale();

  const stateValue: PrinterStateValue = entity.state?.state ?? "offline";
  const style = STATE_STYLES[stateValue] ?? STATE_STYLES.offline;

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="relative"
          aria-label={t("Printer status")}
        >
          <Box className="w-5 h-5" />
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
        className="w-[calc(100vw-2rem)] max-w-[32rem]"
      >
        <div className="flex flex-col gap-3">
          {/* Header: name + state badge */}
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-sm font-semibold">
              {getName(entity.localized_name, entity.id)}
            </span>
            <PrinterStateBadge state={stateValue} />
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
      </PopoverContent>
    </Popover>
  );
};

export default PrinterControl;
