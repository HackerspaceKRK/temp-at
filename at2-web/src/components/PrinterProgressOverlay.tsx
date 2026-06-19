import type { FC } from "react";
import { useTranslation } from "react-i18next";
import type { PrinterEntity } from "../schema";
import { formatRemaining } from "@/lib/printer";

/**
 * A slim status bar overlaid on the camera view while a print is running, so
 * the filename / progress / time-left are visible without opening the popover.
 * The bar itself doubles as a progress indicator: the completed portion is bright
 * green and the remaining portion is darkened.
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

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={t("Show printer status")}
      className="relative h-6 w-full overflow-hidden bg-green-950/85 backdrop-blur-sm cursor-pointer text-left hover:brightness-110 transition-[filter]"
    >
      {/* Completed portion (bright green); the rest stays darkened. */}
      <div
        className={`absolute inset-y-0 left-0 ${paused ? "bg-amber-500/70" : "bg-green-500/75"} transition-all`}
        style={{ width: `${progress}%` }}
      />
      <div className="relative flex h-full items-center justify-between gap-2 px-2 text-[11px] font-medium text-white drop-shadow-sm">
        <span className="truncate" title={state.filename}>
          {paused ? `⏸ ${state.filename || t("Printing")}` : state.filename || t("Printing")}
        </span>
        <span className="shrink-0 tabular-nums">
          {progress}% · {formatRemaining(state.remaining_time)}
        </span>
      </div>
    </button>
  );
};

export default PrinterProgressOverlay;
