import type { FC } from "react";
import { useTranslation } from "react-i18next";
import { FileText, Pause, AlertTriangle } from "lucide-react";
import type { PrinterEntity } from "../schema";
import { formatRemaining } from "@/lib/printer";

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
  const state = entity.state;
  if (!state) return null;

  const progress = Math.min(100, Math.max(0, state.progress ?? 0));
  const paused = state.state === "paused";
  const errored = !!state.print_error && state.print_error !== "00000000";

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={t("Show printer status")}
      className="relative w-full overflow-hidden bg-green-950/85 backdrop-blur-sm cursor-pointer text-left hover:brightness-110 transition-[filter]"
    >
      {/* Completed portion (bright green); the rest stays darkened. */}
      <div
        className={`absolute inset-y-0 left-0 ${paused ? "bg-amber-500/70" : "bg-green-500/75"} transition-all`}
        style={{ width: `${progress}%` }}
      />
      <div className="relative flex items-center gap-2 px-2 py-1.5 text-white drop-shadow-sm">
        {/* Single icon spanning both rows: a warning when errored, otherwise the
            file/pause state of the print. */}
        {errored ? (
          <AlertTriangle className="w-4 h-4 shrink-0 text-amber-200" />
        ) : paused ? (
          <Pause className="w-4 h-4 shrink-0" />
        ) : (
          <FileText className="w-4 h-4 shrink-0" />
        )}
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <div className="flex items-center justify-between gap-2 text-xs font-medium">
            <span className="truncate" title={state.filename}>
              {state.filename || t("Printing")}
            </span>
            <span className="shrink-0 tabular-nums">
              {progress}% · {formatRemaining(state.remaining_time)}
            </span>
          </div>
          {errored && (
            <div
              className="truncate text-[11px] text-amber-200"
              title={state.print_error_text || state.print_error}
            >
              {state.print_error_text || state.print_error}
            </div>
          )}
        </div>
      </div>
    </button>
  );
};

export default PrinterProgressOverlay;
