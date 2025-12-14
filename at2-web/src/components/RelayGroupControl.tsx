import type { FC } from "react";
import { Lightbulb, Fan } from "lucide-react";
import { useLocale } from "../locale";
import type { RelayEntity } from "../schema";
import { Button } from "./ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  DropdownMenuItem,
  DropdownMenuLabel,
} from "./ui/dropdown-menu";
import { Switch } from "./ui/switch";

import { Label } from "./ui/label";
import { DropdownMenuSeparator } from "@radix-ui/react-dropdown-menu";
import { Badge } from "./ui/badge";

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

export const RelayGroupControl: FC<RelayGroupControlProps> = ({
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
    0
  );

  const iconCommon = "w-5 h-5";

  const icon =
    kind === "light" ? (
      <Lightbulb className={iconCommon} />
    ) : (
      <Fan className={`${iconCommon} ${onCount > 0 ? "spin-slow" : ""}`} />
    );

  const ariaLabel =
    kind === "light" ? "Sterowanie światłami" : "Sterowanie wentylatorami";

  // Accent color logic: apply explicit hex accent when any relay is ON
  const variant = onCount > 0 ? "outlinePrimary" : "outline";

 

  // NOTE: Badge (orb) color is fixed in Button component; to recolor it we would need to modify Button.tsx.

  return (
    <div className={className}>
      <DropdownMenu modal={true}>
        <DropdownMenuTrigger asChild>
          <Button
            variant={variant}
            size="sm"
            aria-label={ariaLabel}
            aria-haspopup="menu"
            aria-expanded="false"
            aria-controls={`${roomId ?? "room"}-${kind}-relay-menu`}
            className="relative"
          >
            {icon}
         
            {onCount > 0 && (
              <Badge
                variant="default"
                className="absolute -top-2.5 -right-2.5 h-5 min-w-5 px-1 tabular-nums"
              >
                {onCount}
              </Badge>
            )}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          id={`${roomId ?? "room"}-${kind}-relay-menu`}
          role="menu"
        >
          <DropdownMenuLabel className="font-semibold">{ariaLabel}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {filtered.map((entity) => (
            <DropdownMenuItem key={entity.id} role="menuitem">
              <div className="flex items-center space-x-2 justify-between w-full">
                <Label
                  htmlFor={"switch-" + entity.id}
                  className="cursor-pointer font-normal"
                >
                  {getName(entity.localized_name) || entity.id}
                </Label>
                <Switch
                  id={"switch-" + entity.id}
                  checked={entity.state === "ON"}
                />
              </div>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
};

export default RelayGroupControl;
