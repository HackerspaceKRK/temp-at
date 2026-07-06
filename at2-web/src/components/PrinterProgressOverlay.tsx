import type { FC } from "react";
import { useTranslation } from "react-i18next";
import { FileText, Pause, AlertTriangle } from "lucide-react";
import { Link } from "react-router-dom";
import type { PrinterEntity } from "../schema";
import { formatRemaining, printerPreviewUrl } from "@/lib/printer";
import { useAuth } from "../AuthContext";

/**
 * A status bar overlaid on the camera view while a print is running, so the
 * filename / progress / time-left (and any active error) are visible without
 * opening the popover. The bar itself doubles as a progress indicator: the
 * completed portion is bright green and the remaining portion is darkened.
 */
export const PrinterProgressOverlay: FC<{
  entity: PrinterEntity;
  onClick?: () => void;
}> = ({ entity, onClick }) => {
  const { t } = useTranslation();
  const { user } = useAuth();
  const state = entity.state;
  if (!state) return null;

  const progress = Math.min(100, Math.max(0, state.progress ?? 0));
  const paused = state.state === "paused";
  const errored = !!state.print_error && state.print_error !== "00000000";
  const previewUrl = user ? printerPreviewUrl(entity) : state.low_res_preview;

  return (
    <div className="relative w-full overflow-hidden bg-green-950/85 backdrop-blur-sm">
      {/* Completed portion (bright green); the rest stays darkened. */}
      <div
        className={`absolute inset-y-0 left-0 ${paused ? "bg-amber-500/70" : "bg-green-500/75"} transition-all`}
        style={{ width: `${progress}%` }}
      />
      <div className="relative flex min-h-16 items-stretch gap-2 p-2 text-white drop-shadow-sm">
        {previewUrl && (
          <Link
            to="/printers"
            className="relative shrink-0 overflow-hidden rounded border border-white/30 bg-black/30"
            title={t("Open printers")}
            aria-label={t("Open printers")}
            onClick={(e) => e.stopPropagation()}
          >
            <img
              src={previewUrl}
              alt={t("Printer camera preview")}
              className={`h-full w-20 object-cover ${user ? "" : "scale-125 blur-xl"}`}
              loading="lazy"
            />
          </Link>
        )}
        <button
          type="button"
          onClick={onClick}
          aria-label={t("Show printer status")}
          className="flex min-w-0 flex-1 items-center gap-2 text-left hover:brightness-110 transition-[filter]"
        >
        {/* Single icon spanning both rows: a warning when errored, otherwise the
            file/pause state of the print. */}
        {errored ? (
          <AlertTriangle className="w-4 h-4 shrink-0 text-amber-200" />
        ) : paused ? (
          <Pause className="w-4 h-4 shrink-0" />
        ) : (
          <FileText className="w-4 h-4 shrink-0" />
        )}
        <div className="flex min-w-0 flex-1 flex-col justify-center gap-1">
          <div className="flex items-center justify-between gap-2 text-xs font-medium">
            <span className="truncate" title={state.filename}>
              {state.filename || t("Printing")}
            </span>
          </div>
          <div
            className={`truncate text-[11px] ${errored ? "text-amber-200" : "text-white/90"}`}
            title={errored ? state.print_error_text || state.print_error : undefined}
          >
            {errored
              ? state.print_error_text || state.print_error
              : `${progress}% · ${formatRemaining(state.remaining_time)}`}
          </div>
        </div>
        </button>
      </div>
    </div>
  );
};

export default PrinterProgressOverlay;
