import { useEffect, useMemo, useState, type FC } from "react";
import { ChevronDown, LayoutGrid, Wrench, DoorOpen, Phone } from "lucide-react";
import { NavLink, useMatch, useNavigate } from "react-router-dom";
import { useAppConfig } from "../../AppConfigContext";
import { useTheme } from "../../theme";
import { useLocale } from "../../locale";
import { useLiveRoomStates } from "../../useLiveRoomStates";
import { useTabletSession } from "./TabletSessionContext";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";

function formatHour(value: Date): string {
  return new Intl.DateTimeFormat("pl-PL", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(value);
}

const NAV_ITEM_BASE =
  "flex h-full items-center gap-2 px-4 text-sm font-medium transition-colors";
const navItemInactive = "text-foreground hover:bg-muted";
const navItemActive = "bg-primary text-primary-foreground";

const navItemClass = ({ isActive }: { isActive: boolean }) =>
  `${NAV_ITEM_BASE} ${isActive ? navItemActive : navItemInactive}`;

export const TabletNavbar: FC = () => {
  const [now, setNow] = useState(() => new Date());
  const { config } = useAppConfig();
  const { theme } = useTheme();
  const { getName } = useLocale();
  const navigate = useNavigate();
  const rooms = useLiveRoomStates();
  const { initialRoomId } = useTabletSession();
  const branding = config?.branding;

  const roomMatch = useMatch("/tablet/room/:id");
  const selectedRoomId = roomMatch?.params.id;
  const selectedRoom = rooms.find((room) => room.id === selectedRoomId);
  const initialRoom = rooms.find((room) => room.id === initialRoomId);

  // When the kiosk's initial page was a room and we've navigated away from any
  // room page, the Room control collapses into a quick link back to that room.
  const showRoomLink = !!initialRoomId && !roomMatch;

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      setNow(new Date());
    }, 30000);

    return () => window.clearInterval(intervalId);
  }, []);

  const isDarkMode =
    theme === "dark" ||
    (theme === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  const logoUrl =
    isDarkMode && branding?.logo_dark_url
      ? branding.logo_dark_url
      : branding?.logo_url;

  const homePath = initialRoomId
    ? `/tablet/room/${initialRoomId}`
    : "/tablet/overview";

  const sortedRooms = useMemo(
    () =>
      [...rooms].sort((a, b) =>
        getName(a.localized_name, a.id).localeCompare(
          getName(b.localized_name, b.id),
        ),
      ),
    [rooms, getName],
  );

  return (
    <header className="flex h-[62px] items-stretch border-b border-border bg-background">
      <div className="flex flex-1 items-stretch">
        {logoUrl ? (
          <button
            type="button"
            onClick={() => navigate(homePath)}
            className="flex items-center pl-5 pr-2"
            aria-label="Home"
          >
            <img
              src={logoUrl}
              alt={branding?.logo_alt || "Logo"}
              className="h-full w-auto py-2"
            />
          </button>
        ) : null}

        <nav className="flex items-stretch">
          {showRoomLink ? (
            <button
              type="button"
              onClick={() => navigate(homePath)}
              className={`${NAV_ITEM_BASE} ${navItemInactive}`}
            >
              <DoorOpen className="h-4 w-4" />
              {getName(initialRoom?.localized_name, initialRoomId ?? undefined) ||
                "Room"}
            </button>
          ) : (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  className={`${NAV_ITEM_BASE} ${selectedRoom ? navItemActive : navItemInactive}`}
                >
                  <DoorOpen className="h-4 w-4" />
                  {selectedRoom
                    ? getName(selectedRoom.localized_name, selectedRoom.id)
                    : "Room"}
                  <ChevronDown className="h-4 w-4" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                align="start"
                className="max-h-[70vh] overflow-y-auto"
              >
                <DropdownMenuLabel>Rooms</DropdownMenuLabel>
                <DropdownMenuSeparator />
                {sortedRooms.length === 0 ? (
                  <DropdownMenuItem disabled>No rooms</DropdownMenuItem>
                ) : (
                  sortedRooms.map((room) => (
                    <DropdownMenuItem
                      key={room.id}
                      onSelect={() => navigate(`/tablet/room/${room.id}`)}
                      className="cursor-pointer"
                    >
                      {getName(room.localized_name, room.id)}
                    </DropdownMenuItem>
                  ))
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          )}

          <NavLink to="/tablet/overview" className={navItemClass}>
            <LayoutGrid className="h-4 w-4" />
            Overview
          </NavLink>
          <NavLink to="/tablet/phone" className={navItemClass}>
            <Phone className="h-4 w-4" />
            Phone
          </NavLink>
          <NavLink to="/tablet/debug" className={navItemClass}>
            <Wrench className="h-4 w-4" />
            Debug
          </NavLink>
        </nav>
      </div>

      <div className="flex items-center border-l border-border px-6 text-3xl font-bold tabular-nums text-foreground">
        {formatHour(now)}
      </div>
    </header>
  );
};
