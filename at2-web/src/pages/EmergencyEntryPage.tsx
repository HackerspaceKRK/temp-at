import { Check, DoorOpen, Loader2, Lock, WifiOff, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState, type FC, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useAuth } from "../AuthContext";
import { Alert, AlertDescription } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { apiPath } from "../config";
import { useLocale } from "../locale";
import { cn } from "../lib/utils";

type EmergencyEntryStatus =
  | "disabled"
  | "outside_network"
  | "not_logged_in"
  | "forbidden"
  | "membership_expired"
  | "ok";

interface EmergencyEntryDoor {
  id: string;
  name: string;
  localized_name?: Record<string, string> | null;
}

interface EmergencyEntryStatusResponse {
  status: EmergencyEntryStatus;
  doors: EmergencyEntryDoor[];
}

type DoorState =
  | { state: "idle" }
  | { state: "pending" }
  | { state: "success" }
  | { state: "error"; message: string };

const SUCCESS_DISPLAY_MS = 4000;

export const EmergencyEntryPage: FC = () => {
  const { t } = useTranslation();
  const { user, isLoading: authLoading, login } = useAuth();
  const [data, setData] = useState<EmergencyEntryStatusResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch(apiPath("api/v1/emergency-entry"));
      if (!res.ok) throw new Error(await res.text());
      setData(await res.json());
      setError(null);
    } catch {
      setError(t("Error loading data"));
    }
  }, [t]);

  // Re-check whenever the login state changes.
  useEffect(() => {
    if (authLoading) return;
    fetchStatus();
  }, [authLoading, user, fetchStatus]);

  let content: ReactNode;
  if (error) {
    content = (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  } else if (!data || authLoading) {
    content = (
      <div className="py-10 text-center text-muted-foreground">{t("Loading...")}</div>
    );
  } else {
    switch (data.status) {
      case "disabled":
        content = <Notice>{t("Emergency entry is not configured.")}</Notice>;
        break;
      case "outside_network":
        content = (
          <Notice icon={<WifiOff className="size-10 text-orange-500" />}>
            <p className="font-medium text-foreground">
              {t("Please connect to the Hackerspace Wi-Fi network to use this feature.")}
            </p>
            <p>{t("If you are using a VPN, disable it temporarily.")}</p>
          </Notice>
        );
        break;
      case "not_logged_in":
        content = (
          <Notice icon={<Lock className="size-10 text-primary" />}>
            <p>{t("You must be logged in to use this feature.")}</p>
            <Button onClick={login} className="mt-2">
              {t("Log In")}
            </Button>
          </Notice>
        );
        break;
      case "forbidden":
        content = (
          <Notice icon={<X className="size-10 text-destructive" />}>
            {t("You do not have permission to use emergency entry.")}
          </Notice>
        );
        break;
      case "membership_expired":
        content = (
          <Notice icon={<X className="size-10 text-destructive" />}>
            {t("Your access card has expired.")}
          </Notice>
        );
        break;
      case "ok":
        content =
          data.doors.length === 0 ? (
            <Notice>{t("No doors are configured.")}</Notice>
          ) : (
            <div className="flex flex-col gap-4">
              {data.doors.map((door) => (
                <DoorButton key={door.id} door={door} onDenied={fetchStatus} />
              ))}
            </div>
          );
        break;
    }
  }

  return (
    <main className="flex justify-center px-4 pb-10">
      <Card className="w-full max-w-2xl">
        <CardHeader>
          <CardTitle>{t("Emergency entry")}</CardTitle>
        </CardHeader>
        <CardContent>{content}</CardContent>
      </Card>
    </main>
  );
};

const Notice: FC<{ icon?: ReactNode; children: ReactNode }> = ({ icon, children }) => (
  <div className="flex flex-col items-center gap-3 py-8 text-center text-muted-foreground">
    {icon}
    {children}
  </div>
);

const DoorButton: FC<{ door: EmergencyEntryDoor; onDenied: () => void }> = ({
  door,
  onDenied,
}) => {
  const { t } = useTranslation();
  const { getName } = useLocale();
  const [doorState, setDoorState] = useState<DoorState>({ state: "idle" });
  const resetTimer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(resetTimer.current), []);

  const open = async () => {
    window.clearTimeout(resetTimer.current);
    setDoorState({ state: "pending" });
    try {
      const res = await fetch(apiPath("api/v1/emergency-entry/open"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: door.id }),
      });
      if (res.status === 401 || res.status === 403) {
        // Access changed since the page loaded; re-check to show why.
        setDoorState({ state: "idle" });
        onDenied();
        return;
      }
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(body?.error || `HTTP ${res.status}`);
      }
      setDoorState({ state: "success" });
      resetTimer.current = window.setTimeout(
        () => setDoorState({ state: "idle" }),
        SUCCESS_DISPLAY_MS,
      );
    } catch (err) {
      setDoorState({ state: "error", message: (err as Error).message });
    }
  };

  const name = getName(door.localized_name, door.name) || door.name || door.id;

  return (
    <div className="flex flex-col gap-2">
      <Button
        variant="outline"
        onClick={open}
        disabled={doorState.state === "pending"}
        className={cn(
          "h-28 flex-col gap-2 text-lg whitespace-normal [&_svg:not([class*='size-'])]:size-8",
          doorState.state === "success" && "border-green-600 bg-green-600 text-white hover:bg-green-600/90 hover:text-white dark:bg-green-600 dark:hover:bg-green-600/90",
        )}
      >
        {doorState.state === "pending" ? (
          <Loader2 className="animate-spin" />
        ) : doorState.state === "success" ? (
          <Check />
        ) : (
          <DoorOpen />
        )}
        <span>{doorState.state === "success" ? t("Door opened") : name}</span>
      </Button>
      {doorState.state === "error" && (
        <p className="text-center text-sm text-destructive">
          {t("Failed to open the door: {{error}}", { error: doorState.message })}
        </p>
      )}
    </div>
  );
};
