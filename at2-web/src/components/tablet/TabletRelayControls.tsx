import { type FC } from "react";
import { Fan, Lightbulb } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocale } from "../../locale";
import { apiPath } from "../../config";
import type { RelayEntity } from "../../schema";
import { Switch } from "../ui/switch";

interface TabletRelayControlsProps {
  /** All entities in the room; lights and fans are filtered out internally. */
  entities: RelayEntity[];
}

/**
 * TabletRelayControls renders one full-width row per light/fan relay in a room,
 * suited for touch on a wall tablet. Each row is a card with the device label
 * on the left and a large Switch on the right; tapping anywhere on the row
 * toggles the relay.
 *
 * Authorization is handled at the page level by TabletAuthGate (the tablet
 * holds a long-lived control session), so toggles call the control API
 * directly.
 */
export const TabletRelayControls: FC<TabletRelayControlsProps> = ({
  entities,
}) => {
  const { t } = useTranslation();
  const { getName } = useLocale();

  const relays = entities.filter(
    (e) => e.representation === "light" || e.representation === "fan",
  );

  const executeControl = async (id: string, state: boolean) => {
    try {
      await fetch(apiPath("/api/v1/control-relay"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id, state: state ? "ON" : "OFF" }),
      });
    } catch (err) {
      console.error("Failed to control relay", err);
    }
  };

  const handleTap = (relay: RelayEntity) => {
    if (relay.prohibit_control) return;
    executeControl(relay.id, !(relay.state === "ON"));
  };

  if (relays.length === 0) {
    return (
      <div className="text-sm text-muted-foreground">
        {t("No controllable devices in this room")}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-1.5">
      {relays.map((relay) => {
        const on = relay.state === "ON";
        const isFan = relay.representation === "fan";
        const Icon = isFan ? Fan : Lightbulb;
        const name = getName(relay.localized_name, relay.id);
        return (
          <button
            key={relay.id}
            type="button"
            onClick={() => handleTap(relay)}
            disabled={!!relay.prohibit_control}
            className={[
              "flex w-full items-center justify-between gap-4 rounded-xl border-2 border-border bg-card px-5 py-4 text-left text-foreground transition-colors",
              relay.prohibit_control ? "cursor-not-allowed opacity-50" : "",
            ].join(" ")}
          >
            <div className="flex items-center gap-3">
              <Icon className={`size-7 ${isFan && on ? "spin-slow" : ""}`} />
              <span className="text-lg font-semibold leading-tight">
                {name}
              </span>
            </div>
            <div className="pointer-events-none flex items-center justify-center pr-3 scale-[1.7]">
              <Switch checked={on} disabled={!!relay.prohibit_control} />
            </div>
          </button>
        );
      })}
    </div>
  );
};

export default TabletRelayControls;
