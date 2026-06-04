import { useEffect, useMemo, useState, type FC } from "react";
import { ChevronDown, LayoutGrid, Wrench, DoorOpen } from "lucide-react";
import { NavLink, useMatch, useNavigate } from "react-router-dom";
import { useAppConfig } from "../../AppConfigContext";
import { useTheme } from "../../theme";
import { useLocale } from "../../locale";
import { useLiveRoomStates } from "../../useLiveRoomStates";
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

const navItemClass = ({ isActive }: { isActive: boolean }) =>
  [
    "flex items-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
    isActive
      ? "bg-primary text-primary-foreground"
      : "text-foreground hover:bg-muted",
  ].join(" ");

export const TabletNavbar: FC = () => {
  const [now, setNow] = useState(() => new Date());
  const { config } = useAppConfig();
  const { theme } = useTheme();
  const { getName } = useLocale();
  const navigate = useNavigate();
  const rooms = useLiveRoomStates();
  const branding = config?.branding;

  const roomMatch = useMatch("/tablet/room/:id");
  const selectedRoomId = roomMatch?.params.id;
  const selectedRoom = rooms.find((room) => room.id === selectedRoomId);

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
      <div className="flex flex-1 items-center gap-4 px-5">
        {logoUrl ? (
          <img
            src={logoUrl}
            alt={branding?.logo_alt || "Logo"}
            className="max-h-[30px] w-auto"
          />
        ) : null}

        <nav className="flex items-center gap-1">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className={[
                  "flex items-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
                  selectedRoom
                    ? "bg-primary text-primary-foreground"
                    : "text-foreground hover:bg-muted",
                ].join(" ")}
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

          <NavLink to="/tablet/overview" className={navItemClass}>
            <LayoutGrid className="h-4 w-4" />
            Overview
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
