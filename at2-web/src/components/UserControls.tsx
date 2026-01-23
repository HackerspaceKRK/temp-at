import type { FC } from "react";
import { useAuth } from "../AuthContext";
import { useTranslation, Trans } from "react-i18next";
import { Button } from "./ui/button";
import { User as UserIcon } from "lucide-react";
import { format } from "date-fns";

interface UserControlsProps {
  className?: string;
}

const UserControls: FC<UserControlsProps> = ({ className }) => {
  const { user, login, logout, isLoading } = useAuth();
  const { t } = useTranslation();

  if (isLoading) {
    return <div>...</div>;
  }

  if (user) {
    return (
      <div className={`flex items-center gap-4 ${className}`}>
        <div className="flex items-center gap-2">
          <UserIcon className="size-8 p-1 bg-muted rounded-full" />
          <div className="flex flex-col text-sm">
            <span className="font-semibold">
              <Trans
                i18nKey="Welcome, {{username}}"
                values={{ username: user.username }}
                components={{ bold: <span /> }}
              />
            </span>
            {user.membershipExpirationTimestamp && (
              <span className="text-xs">
                {(() => {
                  const expirationDate = new Date(
                    user.membershipExpirationTimestamp * 1000,
                  );
                  const now = new Date();
                  const diffDays =
                    (expirationDate.getTime() - now.getTime()) /
                    (1000 * 60 * 60 * 24);

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
                })()}
              </span>
            )}
          </div>
        </div>
        <Button onClick={logout} variant="outline" size="sm">
          {t("Log Out")}
        </Button>
      </div>
    );
  }

  return (
    <Button onClick={login} size="sm">
      {t("Log In")}
    </Button>
  );
};

export default UserControls;
