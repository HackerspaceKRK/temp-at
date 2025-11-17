import type { FunctionalComponent } from "preact";
import { Lightbulb, Fan } from "lucide-preact";
import { useLocale } from "../locale";
import type { RelayEntity } from "../schema";
import Button from "./ui/Button";
import Dropdown from "./ui/Dropdown";
import Switch from "./ui/Switch";

/**
 * RelayGroupControl
 *
 * A reusable component that renders a single composite control (Button + Dropdown)
 * for a group of relay entities (either lights or fans) in a room.
 *
 * Behavior:
 * - If there are no entities of the requested kind, it renders null.
 * - Shows one button with an icon (light bulb or fan).
 * - If one or more relays of that kind are ON, the icon is colored and a badge shows the count.
 * - Clicking the button opens a dropdown listing each relay with a Switch reflecting its state.
 * - Switches are read-only for now (toggling does nothing).
 *
 * Intended integration:
 * Place inside the RoomCard, once for kind="light" and once for kind="fan".
 */
interface RelayGroupControlProps {
  /**
   * All relay entities for the room; the component will filter by `kind`.
   */
  entities: RelayEntity[];
  /**
   * Relay kind to display ("light" or "fan").
   */
  kind: "light" | "fan";
  /**
   * Optional additional class names for the wrapper.
   */
  className?: string;
  /**
   * Optional room id for aria labelling / debugging.
   */
  roomId?: string;
}

export const RelayGroupControl: FunctionalComponent<RelayGroupControlProps> = ({
  entities,
  kind,
  className,
  roomId,
}) => {
  const { getName } = useLocale();

  // Filter entities by requested kind.
  const filtered = entities.filter((e) => e.representation === kind);

  if (filtered.length === 0) return null;

  const onCount = filtered.reduce(
    (acc, e) => (e.state === "ON" ? acc + 1 : acc),
    0,
  );

  const iconCommon = "w-5 h-5";

  const icon =
    kind === "light" ? (
      <Lightbulb
        className={
          onCount > 0
            ? `${iconCommon} text-yellow-400`
            : `${iconCommon} text-neutral-400 dark:text-neutral-500`
        }
      />
    ) : (
      <Fan
        className={
          onCount > 0
            ? `${iconCommon} text-sky-400 spin-slow`
            : `${iconCommon} text-neutral-400 dark:text-neutral-500`
        }
      />
    );

  const ariaLabel =
    kind === "light" ? "Sterowanie światłami" : "Sterowanie wentylatorami";

  return (
    <div className={className}>
      <Dropdown
        trigger={({ toggle, open, ref }) => (
          <Button
            ref={ref as any}
            variant="neutral"
            size="sm"
            onClick={toggle}
            aria-label={ariaLabel}
            icon={() => icon}
            badgeCount={onCount > 0 ? onCount : undefined}
          >
            {/* Text label could be omitted; keep for clarity */}
            {kind === "light" ? "Światła" : "Wentylatory"}
          </Button>
        )}
        placement="bottom-start"
        portal={false}
        panelClassName="flex flex-col gap-2"
        autoFocus={false}
      >
        <div className="flex flex-col gap-2">
          {filtered.map((relay) => {
            const name = getName(relay.localized_name, relay.id);
            const isOn = relay.state === "ON";
            return (
              <div
                key={relay.id}
                className="flex items-center justify-between gap-3 px-2 py-1 rounded hover:bg-neutral-100 dark:hover:bg-neutral-700 transition-colors"
              >
                <span className="text-xs text-neutral-700 dark:text-neutral-200 truncate">
                  {name}
                </span>
                <Switch
                  size="sm"
                  checked={isOn}
                  onChange={() => {
                    // Read-only for now; no action.
                  }}
                  ariaLabel={name}
                />
              </div>
            );
          })}
          {filtered.length === 0 && (
            <div className="text-xs italic text-neutral-500">
              Brak elementów
            </div>
          )}
        </div>
        <div className="px-2 pt-1 text-[0.6rem] text-neutral-400 dark:text-neutral-500">
          {roomId ? `Room: ${roomId}` : null}
        </div>
      </Dropdown>
    </div>
  );
};

export default RelayGroupControl;
