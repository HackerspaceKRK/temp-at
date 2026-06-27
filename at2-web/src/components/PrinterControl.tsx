import { useState, type FC, type ReactNode } from "react";
import {
  Box,
  Bell,
  BellRing,
  Thermometer,
  Layers,
  Clock,
  FileText,
  Play,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  ImageOff,
  type LucideIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import type { PrinterEntity, PrinterStateValue } from "../schema";
import { useLocale } from "../locale";
import { resolveImageUrl } from "../config";
import { Button } from "./ui/button";
import { Badge } from "./ui/badge";
import { Alert, AlertTitle, AlertDescription } from "./ui/alert";
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

// PrinterThumbnail renders the cached plate preview for the current print, with a
// graceful placeholder while it is unavailable or fails to load.
const PrinterThumbnail: FC<{ entityId: string; cacheBust: number; show: boolean }> = ({
  entityId,
  cacheBust,
  show,
}) => {
  const { t } = useTranslation();
  const [errored, setErrored] = useState(false);
  // entityId can contain slashes (e.g. "bambu/cnc/printer"); the backend route
  // uses a greedy param, so the slashes are passed through as path segments.
  const src = resolveImageUrl(`/api/v1/printer-thumbnail/${entityId}?t=${cacheBust}`);

  if (!show || errored) {
    return (
      <div className="flex aspect-square w-full items-center justify-center rounded-md bg-muted text-muted-foreground">
        <ImageOff className="h-7 w-7" aria-label={t("No thumbnail")} />
      </div>
    );
  }
  return (
    <img
      key={src}
      src={src}
      alt={t("Print thumbnail")}
      loading="lazy"
      decoding="async"
      onError={() => setErrored(true)}
      className="aspect-square w-full rounded-md bg-muted object-cover"
    />
  );
};

// StatRow renders one "label … value" line: the label is muted and the value is
// emphasized and right-aligned, so field names read distinctly from their values.
// The optional icon keeps a consistent left gutter; rows without one stay aligned.
const StatRow: FC<{ icon?: LucideIcon; label: string; children: ReactNode }> = ({
  icon: Icon,
  label,
  children,
}) => (
  <div className="flex items-center gap-2">
    {Icon ? (
      <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
    ) : (
      <span className="h-4 w-4 shrink-0" />
    )}
    <dt className="text-muted-foreground">{label}</dt>
    <dd className="ml-auto font-medium tabular-nums">{children}</dd>
  </div>
);

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
      <PopoverContent align="end" className="w-[32rem]">
        <div className="flex flex-col gap-3">
          {/* Header: name + state badge */}
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-sm font-semibold">
              {getName(entity.localized_name, entity.id)}
            </span>
            <Badge variant={style.badge}>{stateLabel[stateValue]}</Badge>
          </div>

          {/* Progress bar (active prints) */}
          {isActive && (
            <div className="flex flex-col gap-1">
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>{t("Progress")}</span>
                <span className="tabular-nums">{state?.progress ?? 0}%</span>
              </div>
              <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                <div
                  className="h-full bg-primary transition-all"
                  style={{ width: `${Math.min(100, Math.max(0, state?.progress ?? 0))}%` }}
                />
              </div>
            </div>
          )}

          {state && state.print_error && state.print_error !== "00000000" && (
            <Alert variant="destructive">
              <AlertTriangle />
              <AlertTitle>
                {t("Error")} {state.print_error}
              </AlertTitle>
              <AlertDescription>
                {state.print_error_text || t("Unknown error")}
              </AlertDescription>
            </Alert>
          )}

          {state ? (
            <div className="grid grid-cols-[1fr_11rem] gap-4">
              {/* Left column: thumbnail + filename + start time */}
              <div className="flex min-w-0 flex-col gap-2">
                <PrinterThumbnail
                  entityId={entity.id}
                  cacheBust={state.started_at}
                  show={!!state.has_thumbnail}
                />
                {state.filename && (
                  <div className="flex items-start gap-1.5 text-xs">
                    <FileText className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    <span className="break-words font-medium" title={state.filename}>
                      {state.filename}
                    </span>
                  </div>
                )}
                {state.started_at > 0 && (
                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <Play className="h-3.5 w-3.5 shrink-0" />
                    <span className="tabular-nums">{formatTimestamp(state.started_at)}</span>
                  </div>
                )}
              </div>

              {/* Right column: stats — muted label left, emphasized value right */}
              <dl className="flex flex-col gap-2 text-sm">
                {isActive && (
                  <StatRow icon={Clock} label={t("Remaining")}>
                    {formatRemaining(state.remaining_time)}
                  </StatRow>
                )}
                {(state.total_layer_num > 0 || state.layer_num > 0) && (
                  <StatRow icon={Layers} label={t("Layer")}>
                    {state.layer_num}/{state.total_layer_num}
                  </StatRow>
                )}
                <StatRow icon={Thermometer} label={t("Nozzle")}>
                  {Math.round(state.nozzle_temp)}/{Math.round(state.nozzle_target)}°C
                </StatRow>
                <StatRow label={t("Bed")}>
                  {Math.round(state.bed_temp)}/{Math.round(state.bed_target)}°C
                </StatRow>
                {state.chamber_temp > 0 && (
                  <StatRow label={t("Chamber")}>{Math.round(state.chamber_temp)}°C</StatRow>
                )}
                {stateValue === "finished" && state.finished_at > 0 && (
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
                    <span>{t("Finished")}: {formatTimestamp(state.finished_at)}</span>
                  </div>
                )}
                {stateValue === "failed" && state.finished_at > 0 && (
                  <div className="flex items-center gap-2 text-xs text-destructive">
                    <XCircle className="h-3.5 w-3.5 shrink-0" />
                    <span>{t("Failed")}: {formatTimestamp(state.finished_at)}</span>
                  </div>
                )}
              </dl>
            </div>
          ) : (
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
