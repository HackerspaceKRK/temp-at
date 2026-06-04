import { useState, type FC } from "react";
import { Fan, Lightbulb } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocale } from "../../locale";
import { useAuth } from "../../AuthContext";
import { apiPath } from "../../config";
import type { RelayEntity } from "../../schema";
import { LoginDialog } from "../LoginDialog";

interface TabletRelayControlsProps {
  /** All entities in the room; lights and fans are filtered out internally. */
  entities: RelayEntity[];
}

/**
 * TabletRelayControls renders one large square button per light/fan relay in a
 * room, suited for touch on a wall tablet. Tapping toggles the relay.
 *
 * Controlling devices requires authentication; while logged out a tap opens the
 * login dialog (proper kiosk auth handling comes later).
 */
export const TabletRelayControls: FC<TabletRelayControlsProps> = ({
  entities,
}) => {
  const { t } = useTranslation();
  const { getName } = useLocale();
  const { user, login } = useAuth();
  const [loginDialogOpen, setLoginDialogOpen] = useState(false);

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
    if (!user) {
      setLoginDialogOpen(true);
      return;
    }
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
    <>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {relays.map((relay) => {
          const on = relay.state === "ON";
          const isFan = relay.representation === "fan";
          const Icon = isFan ? Fan : Lightbulb;
          return (
            <button
              key={relay.id}
              type="button"
              onClick={() => handleTap(relay)}
              disabled={!!relay.prohibit_control}
              className={[
                "flex aspect-square flex-col items-center justify-center gap-2 rounded-2xl border-2 p-3 text-center transition-colors",
                on
                  ? "border-amber-400 bg-amber-400/15 text-amber-500"
                  : "border-border bg-card text-muted-foreground",
                relay.prohibit_control
                  ? "cursor-not-allowed opacity-50"
                  : "active:scale-[0.98]",
              ].join(" ")}
            >
              <Icon
                className={`size-10 ${isFan && on ? "spin-slow" : ""}`}
              />
              <span className="line-clamp-2 text-sm font-semibold leading-tight">
                {getName(relay.localized_name, relay.id)}
              </span>
              <span className="text-xs font-medium uppercase tracking-wide">
                {on ? t("On") : t("Off")}
              </span>
            </button>
          );
        })}
      </div>

      <LoginDialog
        open={loginDialogOpen}
        onOpenChange={setLoginDialogOpen}
        onLogin={login}
      />
    </>
  );
};

export default TabletRelayControls;
