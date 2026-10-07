import type { FC } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../AuthContext";
import { useAppConfig } from "../AppConfigContext";
import { useTranslation, Trans } from "react-i18next";
import { Button } from "./ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu";
import { ChevronDown, DoorOpen, LogOut, User as UserIcon } from "lucide-react";
import { format } from "date-fns";
import { cn } from "../lib/utils";

/** Membership (access card) expiration, colored when close to / past expiry. */
const MembershipExpiration: FC<{ timestamp: number }> = ({ timestamp }) => {
  const { t } = useTranslation();
  const expirationDate = new Date(timestamp * 1000);
  const diffDays =
    (expirationDate.getTime() - Date.now()) / (1000 * 60 * 60 * 24);

  let className = "text-xs text-muted-foreground";
  if (diffDays < 3 && diffDays > 0) {
    className = "text-xs text-orange-500";
  } else if (diffDays <= 0) {
    className = "text-xs text-red-500";
  }

  return (
    <span className={className}>
      {t("Access card expiration: {{date}}", {
        date: format(expirationDate, "yyyy-MM-dd HH:mm"),
      })}
    </span>
  );
};

/**
 * Username + membership expiration. The compact variant (mobile top bar) drops
 * the avatar and greeting so it fits next to the menu button and logo.
 */
export const UserSummary: FC<{ compact?: boolean; className?: string }> = ({
  compact,
  className,
}) => {
  const { user } = useAuth();
  if (!user) return null;

  return (
    <div className={cn("flex min-w-0 items-center gap-2", className)}>
      {!compact && <UserIcon className="size-8 shrink-0 p-1 bg-muted rounded-full" />}
      <div
        className={cn(
          "flex min-w-0 flex-col text-sm",
          compact ? "items-end text-right" : "text-left",
        )}
      >
        <span className="max-w-full truncate font-semibold">
          {compact ? (
            user.username
          ) : (
            <Trans
              i18nKey="Welcome, {{username}}"
              values={{ username: user.username }}
              components={{ bold: <span /> }}
            />
          )}
        </span>
        {user.membershipExpirationTimestamp && (
          <MembershipExpiration timestamp={user.membershipExpirationTimestamp} />
        )}
      </div>
    </div>
  );
};

interface UserControlsProps {
  className?: string;
}

const UserControls: FC<UserControlsProps> = ({ className }) => {
  const { user, login, logout, isLoading } = useAuth();
  const { config } = useAppConfig();
  const { t } = useTranslation();

  if (isLoading) {
    return <div>...</div>;
  }

  if (user) {
    return (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            className={cn("h-auto gap-2 py-1.5", className)}
          >
            <UserSummary />
            <ChevronDown className="text-muted-foreground" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-48">
          {config?.has_emergency_entry && (
            <>
              <DropdownMenuItem asChild>
                <Link to="/emergency-entry">
                  <DoorOpen />
                  {t("Emergency entry")}
                </Link>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
            </>
          )}
          <DropdownMenuItem onSelect={logout}>
            <LogOut />
            {t("Log Out")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  }

  return (
    <Button onClick={login} size="sm" className={className}>
      {t("Log In")}
    </Button>
  );
};

export default UserControls;
