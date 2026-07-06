import type { FC } from "react";
import { Droplets, Thermometer } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { PrinterState, PrinterTray } from "../../schema";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "../ui/tooltip";

// trayCssColor converts Bambu's RRGGBBAA hex into a CSS color ("" when unknown).
function trayCssColor(color: string): string {
  if (!color || color.length < 6) return "";
  return `#${color.slice(0, 6)}`;
}

// trayTextClass picks readable text on top of the filament color.
function trayTextClass(color: string): string {
  if (!color || color.length < 6) return "";
  const r = parseInt(color.slice(0, 2), 16);
  const g = parseInt(color.slice(2, 4), 16);
  const b = parseInt(color.slice(4, 6), 16);
  const luma = 0.299 * r + 0.587 * g + 0.114 * b;
  return luma > 140 ? "text-neutral-900" : "text-white";
}

// FilamentSlot renders one AMS tray as an OrcaSlicer-style card: filament color
// as background, material type on top, remaining-filament bar at the bottom.
const FilamentSlot: FC<{ tray: PrinterTray; label: string }> = ({ tray, label }) => {
  const { t } = useTranslation();
  const cssColor = trayCssColor(tray.color);

  const slot = (
    <div className="flex min-w-0 flex-col items-center gap-1">
      <span className="text-[10px] font-medium text-muted-foreground">{label}</span>
      <div
        className={`flex h-16 w-full flex-col items-center justify-center overflow-hidden rounded-md border ${
          tray.empty ? "border-dashed bg-muted/40" : "border-border"
        } ${!tray.empty && cssColor ? trayTextClass(tray.color) : ""}`}
        style={!tray.empty && cssColor ? { backgroundColor: cssColor } : undefined}
      >
        {tray.empty ? (
          <span className="text-xs text-muted-foreground">{t("Empty")}</span>
        ) : (
          <>
            <span className="max-w-full truncate px-1 text-xs font-semibold drop-shadow-sm">
              {tray.type}
            </span>
            {/* {tray.remain >= 0 && (
              <span className="text-[10px] opacity-80 drop-shadow-sm">{tray.remain}%</span>
            )} */}
          </>
        )}
      </div>
      {/* Remaining-filament bar */}
      <div className="h-1 w-full overflow-hidden rounded-full bg-muted">
        {!tray.empty && tray.remain >= 0 && (
          <div
            className="h-full rounded-full bg-primary/70"
            style={{ width: `${Math.min(100, Math.max(0, tray.remain))}%` }}
          />
        )}
      </div>
    </div>
  );

  if (tray.empty && !tray.sub_brand) return slot;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{slot}</TooltipTrigger>
      <TooltipContent>
        {tray.empty
          ? t("Empty")
          : [tray.sub_brand || tray.type, tray.remain >= 0 ? `${tray.remain}%` : null]
              .filter(Boolean)
              .join(" · ")}
      </TooltipContent>
    </Tooltip>
  );
};

// AmsPanel shows every AMS unit (humidity, temperature, four filament slots)
// plus the external spool holder when present.
export const AmsPanel: FC<{ state: PrinterState }> = ({ state }) => {
  const { t } = useTranslation();
  const units = state.ams ?? [];
  const vt = state.vt_tray;
  if (units.length === 0 && !vt) return null;

  return (
    <div className="flex flex-col gap-3">
      {units.map((unit) => (
        <div key={unit.id} className="rounded-lg border p-3">
          <div className="mb-2 flex items-center justify-between text-xs text-muted-foreground">
            <span className="font-semibold text-foreground">
              {t("AMS")} {units.length > 1 ? unit.id + 1 : ""}
            </span>
            <span className="flex items-center gap-3">
              {unit.humidity > 0 && (
                <span className="flex items-center gap-1" title={t("Humidity level (1 = driest)")}>
                  <Droplets className="h-3.5 w-3.5" />
                  {unit.humidity}/5
                </span>
              )}
              {unit.temp > 0 && (
                <span className="flex items-center gap-1">
                  <Thermometer className="h-3.5 w-3.5" />
                  {Math.round(unit.temp)}°C
                </span>
              )}
            </span>
          </div>
          <div className="grid grid-cols-4 gap-2">
            {unit.trays.map((tray, i) => (
              <FilamentSlot
                key={tray.id}
                tray={tray}
                label={`${String.fromCharCode(65 + unit.id)}${i + 1}`}
              />
            ))}
          </div>
        </div>
      ))}
      {vt && !vt.empty && (
        <div className="rounded-lg border p-3">
          <div className="mb-2 text-xs font-semibold">{t("External spool")}</div>
          <div className="grid grid-cols-4 gap-2">
            <FilamentSlot tray={vt} label={t("Ext")} />
          </div>
        </div>
      )}
    </div>
  );
};
