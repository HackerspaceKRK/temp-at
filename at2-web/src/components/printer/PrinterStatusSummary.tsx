import { useState, type FC, type ReactNode } from "react";
import {
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
  Fan,
  Wifi,
  Gauge,
  Lightbulb,
  type LucideIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import type { PrinterEntity, PrinterState, PrinterStateValue } from "../../schema";
import { resolveImageUrl } from "../../config";
import { Button } from "../ui/button";
import { Badge } from "../ui/badge";
import { Alert, AlertTitle, AlertDescription } from "../ui/alert";
import { subscribeToPrint, pushSupported } from "../../push";
import { formatRemaining, formatTimestamp, isPrinterActive } from "@/lib/printer";

export const STATE_STYLES: Record<
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

export function usePrinterStateLabels(): Record<PrinterStateValue, string> {
  const { t } = useTranslation();
  return {
    printing: t("Printing"),
    paused: t("Paused"),
    finished: t("Finished"),
    failed: t("Failed"),
    idle: t("Idle"),
    offline: t("Offline"),
  };
}

export const PrinterStateBadge: FC<{ state: PrinterStateValue }> = ({ state }) => {
  const labels = usePrinterStateLabels();
  const style = STATE_STYLES[state] ?? STATE_STYLES.offline;
  return <Badge variant={style.badge}>{labels[state] ?? state}</Badge>;
};

// PrinterThumbnail renders the cached plate preview for the current print, with a
// graceful placeholder while it is unavailable or fails to load.
export const PrinterThumbnail: FC<{ entityId: string; cacheBust: number; show: boolean }> = ({
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
      <div className="flex aspect-square w-[250px] max-w-full items-center justify-center rounded-md bg-muted text-muted-foreground">
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
      className="aspect-square w-[250px] max-w-full rounded-md bg-muted object-cover"
    />
  );
};

// StatRow renders one "label … value" line: the label is muted and the value is
// emphasized and right-aligned, so field names read distinctly from their values.
// The optional icon keeps a consistent left gutter; rows without one stay aligned.
export const StatRow: FC<{ icon?: LucideIcon; label: string; children: ReactNode }> = ({
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

export const PrinterProgressBar: FC<{ state: PrinterState }> = ({ state }) => {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-1">
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>{t("Progress")}</span>
        <span className="tabular-nums">{state.progress ?? 0}%</span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
        <div
          className="h-full bg-primary transition-all"
          style={{ width: `${Math.min(100, Math.max(0, state.progress ?? 0))}%` }}
        />
      </div>
    </div>
  );
};

export const PrinterErrorAlert: FC<{ state: PrinterState }> = ({ state }) => {
  const { t } = useTranslation();
  if (!state.print_error || state.print_error === "00000000") return null;
  return (
    <Alert variant="destructive">
      <AlertTriangle />
      <AlertTitle>
        {t("Error")} {state.print_error}
      </AlertTitle>
      <AlertDescription>{state.print_error_text || t("Unknown error")}</AlertDescription>
    </Alert>
  );
};

// PrinterNotifyButton subscribes the user to a push notification for the
// currently running print. Renders nothing when inactive or unsupported.
export const PrinterNotifyButton: FC<{ entity: PrinterEntity }> = ({ entity }) => {
  const { t } = useTranslation();
  const [notifyState, setNotifyState] = useState<"idle" | "pending" | "subscribed" | "error">(
    "idle",
  );

  if (!isPrinterActive(entity.state?.state) || !pushSupported()) return null;

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
    <>
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
            {notifyState === "pending" ? t("Enabling…") : t("Notify me about this print")}
          </>
        )}
      </Button>
      {notifyState === "error" && (
        <p className="text-xs text-destructive">{t("Could not enable notifications.")}</p>
      )}
    </>
  );
};

const SPEED_LEVEL_KEYS: Record<number, string> = {
  1: "Silent",
  2: "Standard",
  3: "Sport",
  4: "Ludicrous",
};

// hmsCode renders a raw HMS attr/code pair in Bambu's canonical
// HMS_XXXX_XXXX_XXXX_XXXX format.
export function hmsCode(attr: number, code: number): string {
  const p = (v: number) => (v >>> 0).toString(16).toUpperCase().padStart(8, "0");
  const a = p(attr);
  const c = p(code);
  return `HMS_${a.slice(0, 4)}_${a.slice(4)}_${c.slice(0, 4)}_${c.slice(4)}`;
}

// PrinterExtendedStats shows the detail-page-only stats: fans, WiFi, speed
// level, nozzle and lights.
export const PrinterExtendedStats: FC<{ state: PrinterState }> = ({ state }) => {
  const { t } = useTranslation();
  const lightsOn = (state.lights ?? []).filter((l) => l.mode !== "off");
  return (
    <dl className="grid grid-cols-1 gap-x-8 gap-y-2 text-sm sm:grid-cols-2">
      <StatRow icon={Fan} label={t("Part cooling fan")}>
        {state.fan_cooling}%
      </StatRow>
      <StatRow icon={Fan} label={t("Aux fan")}>
        {state.fan_aux}%
      </StatRow>
      <StatRow icon={Fan} label={t("Chamber fan")}>
        {state.fan_chamber}%
      </StatRow>
      {state.wifi_signal && (
        <StatRow icon={Wifi} label={t("WiFi signal")}>
          {state.wifi_signal}
        </StatRow>
      )}
      {state.speed_level > 0 && (
        <StatRow icon={Gauge} label={t("Speed")}>
          {t(SPEED_LEVEL_KEYS[state.speed_level] ?? String(state.speed_level))}
        </StatRow>
      )}
      {state.nozzle_diameter && (
        <StatRow label={t("Nozzle type")}>
          {state.nozzle_diameter}mm{state.nozzle_type ? ` · ${state.nozzle_type.replaceAll("_", " ")}` : ""}
        </StatRow>
      )}
      {lightsOn.length > 0 && (
        <StatRow icon={Lightbulb} label={t("Lights")}>
          {lightsOn.map((l) => l.node.replaceAll("_", " ")).join(", ")}
        </StatRow>
      )}
    </dl>
  );
};

// PrinterHmsList renders active HMS (health management system) error codes.
export const PrinterHmsList: FC<{ state: PrinterState }> = ({ state }) => {
  const { t } = useTranslation();
  if (!state.hms || state.hms.length === 0) return null;
  return (
    <Alert>
      <AlertTriangle />
      <AlertTitle>{t("Printer reports issues (HMS)")}</AlertTitle>
      <AlertDescription>
        <ul className="list-inside list-disc">
          {state.hms.map((h, i) => (
            <li key={i} className="font-mono text-xs">
              {hmsCode(h.attr, h.code)}
            </li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  );
};

// PrinterStatusSummary is the shared status block used by the room popover,
// the /printers list and the printer detail page: progress bar, error alert,
// plate thumbnail + filename on the left and the core stats on the right.
export const PrinterStatusSummary: FC<{
  entity: PrinterEntity;
  /** Hide the plate-preview column (e.g. when a camera view is shown above). */
  showThumbnail?: boolean;
}> = ({ entity, showThumbnail = true }) => {
  const { t } = useTranslation();
  const state = entity.state;
  const stateValue: PrinterStateValue = state?.state ?? "offline";
  const isActive = isPrinterActive(stateValue);

  if (!state) {
    return <p className="text-sm text-muted-foreground">{t("No data from printer")}</p>;
  }

  return (
    <div className="flex flex-col gap-3">
      {isActive && <PrinterProgressBar state={state} />}
      <PrinterErrorAlert state={state} />
      <div
        className={
          showThumbnail
            ? "grid grid-cols-1 gap-4 min-[400px]:grid-cols-[1fr_11rem]"
            : "grid grid-cols-1 gap-4"
        }
      >
        {showThumbnail && (
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
        )}

        <dl className="flex flex-col gap-2 text-sm">
          {!showThumbnail && state.filename && (
            <StatRow icon={FileText} label={t("File")}>
              <span className="break-all font-medium" title={state.filename}>
                {state.filename}
              </span>
            </StatRow>
          )}
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
              <span>
                {t("Finished")}: {formatTimestamp(state.finished_at)}
              </span>
            </div>
          )}
          {stateValue === "failed" && state.finished_at > 0 && (
            <div className="flex items-center gap-2 text-xs text-destructive">
              <XCircle className="h-3.5 w-3.5 shrink-0" />
              <span>
                {t("Failed")}: {formatTimestamp(state.finished_at)}
              </span>
            </div>
          )}
        </dl>
      </div>
    </div>
  );
};
