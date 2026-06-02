import { useEffect, useState, type FC } from "react";
import { useAppConfig } from "../AppConfigContext";
import { useTheme } from "../theme";

function formatTabletDateTime(value: Date): string {
  return new Intl.DateTimeFormat("pl-PL", {
    weekday: "long",
    day: "2-digit",
    month: "long",
    hour: "2-digit",
    minute: "2-digit",
  }).format(value);
}

export const TabletNavbar: FC = () => {
  const [now, setNow] = useState(() => new Date());
  const { config } = useAppConfig();
  const { theme } = useTheme();
  const branding = config?.branding;

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

  return (
    <header className="border-b border-border bg-background px-5 py-4">
      <div className="mx-auto flex w-full max-w-[1280px] items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          {logoUrl ? (
            <img
              src={logoUrl}
              alt={branding?.logo_alt || "Logo"}
              className="max-h-[30px] w-auto"
            />
          ) : null}
          <div className="text-xl font-semibold tracking-tight text-foreground">
            Status pomieszczeń
          </div>
        </div>
        <div className="text-right text-xl font-bold capitalize text-foreground">
          {formatTabletDateTime(now)}
        </div>
      </div>
    </header>
  );
};