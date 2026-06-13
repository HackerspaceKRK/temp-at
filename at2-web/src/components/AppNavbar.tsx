import type { FC } from "react";
import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";
import { useAppConfig } from "../AppConfigContext";
import { cn } from "../lib/utils";
import { useTheme } from "../theme";
import { LanguageToggle } from "./LanguageToggle";
import { ModeToggle } from "./ModeToggle";
import UserControls from "./UserControls";

const navItemClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    "rounded-md px-3 py-2 text-sm font-medium transition-colors",
    isActive
      ? "bg-accent text-accent-foreground"
      : "text-muted-foreground hover:text-foreground",
  );

export const AppNavbar: FC = () => {
  const { t } = useTranslation();
  const { theme } = useTheme();
  const { config } = useAppConfig();
  const branding = config?.branding;

  const isDarkMode =
    theme === "dark" ||
    (theme === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  const logoUrl =
    isDarkMode && branding?.logo_dark_url
      ? branding.logo_dark_url
      : branding?.logo_url;

  return (
    <header className="px-4 py-4 flex flex-col sm:flex-row items-center justify-between gap-4">
      <div className="flex flex-col items-center gap-2 sm:flex-row sm:gap-6">
        <a
          href={branding?.logo_link_url || "/"}
          className="flex items-center"
          target={branding?.logo_link_url?.startsWith("http") ? "_blank" : undefined}
          rel={
            branding?.logo_link_url?.startsWith("http")
              ? "noopener noreferrer"
              : undefined
          }
        >
          {logoUrl ? (
            <img
              src={logoUrl}
              alt={branding?.logo_alt || t("Logo")}
              className="max-h-[70px] w-auto transition-transform hover:scale-105"
            />
          ) : (
            <h1 className="text-2xl font-bold">{t("Headquarters")}</h1>
          )}
        </a>
        <nav className="flex items-center gap-1">
          <NavLink to="/" end className={navItemClass}>
            {t("Room Status")}
          </NavLink>
          <NavLink to="/dhcp" className={navItemClass}>
            {t("DHCP")}
          </NavLink>
        </nav>
      </div>
      <div className="flex items-center gap-2 sm:gap-4 flex-wrap justify-center">
        <div className="flex items-center gap-2">
          <LanguageToggle />
          <ModeToggle />
        </div>
        <div className="w-px h-8 bg-border mx-2" />
        <UserControls />
      </div>
    </header>
  );
};