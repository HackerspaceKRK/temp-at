import { useState, type FC } from "react";
import { Box, Bell, BellRing, Thermometer, Layers, Clock, FileText, Play, CheckCircle2, XCircle } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { PrinterEntity, PrinterStateValue } from "../schema";
import { useLocale } from "../locale";
import { Button } from "./ui/button";
import { Badge } from "./ui/badge";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "./ui/popover";
import { subscribeToPrint, pushSupported } from "../push";
import { formatRemaining, formatTimestamp } from "@/lib/printer";

const STATE_STYLES: Record<
  PrinterStateValue,
  { dot: string; badge: "default" | "secondary" | "destructive" | "outline"; pulse?: boolean }
> = {
  printing: { dot: "bg-green-500", badge: "default", pulse: true },
  paused: { dot: "bg-amber-500", badge: "secondary" },
  finished: { dot: "bg-blue-500", badge: "default" },
  failed: { dot: "bg-red-500", badge: "destructive" },
  idle: { dot: "bg-neutral-400", badge: "outline" },
  offline: { dot: "bg-neutral-600", badge: "outline" },
};

export const PrinterControl: FC<{
  entity: PrinterEntity;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}> = ({ entity, open, onOpenChange }) => {
  const { t } = useTranslation();
  const { getName } = useLocale();
  const [notifyState, setNotifyState] = useState<"idle" | "pending" | "subscribed" | "error">("idle");

  const state = entity.state;
  const stateValue: PrinterStateValue = state?.state ?? "offline";
  const style = STATE_STYLES[stateValue] ?? STATE_STYLES.offline;
  const isActive = stateValue === "printing" || stateValue === "paused";

  const stateLabel: Record<PrinterStateValue, string> = {
    printing: t("Printing"),
    paused: t("Paused"),
    finished: t("Finished"),
    failed: t("Failed"),
    idle: t("Idle"),
    offline: t("Offline"),
  };

  const handleNotify = async () => {
    setNotifyState("pending");
    try {
      await subscribeToPrint(entity.id);
      setNotifyState("subscribed");
    } catch (err) {
      console.error(err);
      setNotifyState("error");
    }
  };

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
      <PopoverContent align="end" className="w-72">
        <div className="flex flex-col gap-3">
          <div className="flex items-center justify-between gap-2">
            <span className="font-semibold text-sm truncate">
              {getName(entity.localized_name, entity.id)}
            </span>
            <Badge variant={style.badge}>{stateLabel[stateValue]}</Badge>
          </div>

          {isActive && (
            <div className="flex flex-col gap-1">
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>{t("Progress")}</span>
                <span className="tabular-nums">{state?.progress ?? 0}%</span>
              </div>
              <div className="h-2 w-full rounded-full bg-muted overflow-hidden">
                <div
                  className="h-full bg-primary transition-all"
                  style={{ width: `${Math.min(100, Math.max(0, state?.progress ?? 0))}%` }}
                />
              </div>
            </div>
          )}

          {state && (
            <div className="flex flex-col gap-1.5 text-sm">
              {state.filename && (
                <div className="flex items-center gap-2 min-w-0">
                  <FileText className="w-4 h-4 shrink-0 text-muted-foreground" />
                  <span className="truncate" title={state.filename}>
                    {state.filename}
                  </span>
                </div>
              )}
              {isActive && (
                <div className="flex items-center gap-2">
                  <Clock className="w-4 h-4 shrink-0 text-muted-foreground" />
                  <span>
                    {t("Remaining")}: {formatRemaining(state.remaining_time)}
                  </span>
                </div>
              )}
              {(state.total_layer_num > 0 || state.layer_num > 0) && (
                <div className="flex items-center gap-2">
                  <Layers className="w-4 h-4 shrink-0 text-muted-foreground" />
                  <span className="tabular-nums">
                    {t("Layer")} {state.layer_num}/{state.total_layer_num}
                  </span>
                </div>
              )}
              <div className="flex items-center gap-2 text-xs text-muted-foreground tabular-nums flex-wrap">
                <Thermometer className="w-4 h-4 shrink-0" />
                <span>
                  {t("Nozzle")} {Math.round(state.nozzle_temp)}/{Math.round(state.nozzle_target)}°C
                </span>
                <span>
                  {t("Bed")} {Math.round(state.bed_temp)}/{Math.round(state.bed_target)}°C
                </span>
                {state.chamber_temp > 0 && (
                  <span>
                    {t("Chamber")} {Math.round(state.chamber_temp)}°C
                  </span>
                )}
              </div>
              {stateValue === "failed" && state.error_code && state.error_code !== "0" && (
                <div className="text-xs text-destructive">
                  {t("Error code")}: {state.error_code}
                </div>
              )}

              {state.started_at > 0 && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Play className="w-3.5 h-3.5 shrink-0" />
                  <span>{t("Started")}: {formatTimestamp(state.started_at)}</span>
                </div>
              )}
              {stateValue === "finished" && state.finished_at > 0 && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <CheckCircle2 className="w-3.5 h-3.5 shrink-0" />
                  <span>{t("Finished")}: {formatTimestamp(state.finished_at)}</span>
                </div>
              )}
              {stateValue === "failed" && state.finished_at > 0 && (
                <div className="flex items-center gap-2 text-xs text-destructive">
                  <XCircle className="w-3.5 h-3.5 shrink-0" />
                  <span>{t("Failed")}: {formatTimestamp(state.finished_at)}</span>
                </div>
              )}
            </div>
          )}

          {!state && (
            <p className="text-sm text-muted-foreground">{t("No data from printer")}</p>
          )}

          {isActive && pushSupported() && (
            <Button
              variant={notifyState === "subscribed" ? "secondary" : "default"}
              size="sm"
              className="w-full"
              disabled={notifyState === "pending" || notifyState === "subscribed"}
              onClick={handleNotify}
            >
              {notifyState === "subscribed" ? (
                <>
                  <BellRing className="w-4 h-4" /> {t("You'll be notified")}
                </>
              ) : (
                <>
                  <Bell className="w-4 h-4" />{" "}
                  {notifyState === "pending"
                    ? t("Enabling…")
                    : t("Notify me about this print")}
                </>
              )}
            </Button>
          )}
          {notifyState === "error" && (
            <p className="text-xs text-destructive">
              {t("Could not enable notifications.")}
            </p>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
};

export default PrinterControl;
